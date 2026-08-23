package lazy

import (
	"fmt"
	"log"
	"sync"
	"time"

	"threads-dashboard/internal/threads"
)

type Scheduler struct {
	deps    *Deps
	stopCh  chan struct{}
	once    sync.Once
	mu      sync.Mutex
	busy    bool
	started bool
}

func NewScheduler(deps *Deps) *Scheduler {
	return &Scheduler{
		deps:   deps,
		stopCh: make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	// Job yang putus tengah jalan (restart server) → kembalikan ke pending
	n := s.deps.Store.RecoverStuckJobs()
	if n > 0 {
		log.Printf("lazy: recover %d job stuck running → pending", n)
	}
	if s.deps.Schedule != nil {
		if sn := s.deps.Schedule.RecoverStuck(); sn > 0 {
			log.Printf("schedule: recover %d post stuck running → pending", sn)
		}
	}
	go s.loop()
	log.Println("lazy scheduler aktif (tick 1m)")
}

func (s *Scheduler) Stop() {
	s.once.Do(func() { close(s.stopCh) })
}

func (s *Scheduler) loop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	s.tick()
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return
	}
	s.busy = true
	s.mu.Unlock()

	// Manual schedule queue — jalan meski Lazy OFF.
	if s.deps.Schedule != nil && s.deps.Publisher != nil {
		s.deps.Schedule.ProcessWith(s.deps.Publisher.PublishScheduled)
	}

	cfg := s.deps.Store.GetConfig()
	loc := s.deps.Store.Location()
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	keepFrom := now.AddDate(0, 0, -30).Format("2006-01-02")
	_ = s.deps.Store.PruneOldJobs(keepFrom)

	if cfg.Enabled {
		if err := s.EnsureDayPlan(now, cfg.PostsPerDay, s.deps.Threads); err != nil {
			log.Printf("lazy plan: %v", err)
		}
		// Siapkan juga jadwal besok supaya UI bisa tampilkan jam post berikutnya.
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 12, 0, 0, 0, loc)
		if err := s.EnsureDayPlan(tomorrow, cfg.PostsPerDay, s.deps.Threads); err != nil {
			log.Printf("lazy plan tomorrow: %v", err)
		}
	}

	if !cfg.Enabled {
		// jarang log biar tidak spam
		if now.Minute()%15 == 0 {
			log.Printf("lazy tick: otomasi OFF")
		}
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
		return
	}

	pending := 0
	var nextAt time.Time
	for _, j := range s.deps.Store.JobsForDate(today) {
		if j.Status == StatusPending {
			pending++
			if nextAt.IsZero() || j.ScheduledAt.Before(nextAt) {
				nextAt = j.ScheduledAt
			}
		}
	}

	due := s.deps.Store.DueJobs(now)
	if len(due) == 0 {
		if now.Minute()%5 == 0 {
			nextStr := "-"
			if !nextAt.IsZero() {
				nextStr = nextAt.In(loc).Format("15:04")
			}
			log.Printf("lazy tick: ON pending=%d due=0 next=%s", pending, nextStr)
		}
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
		return
	}

	job := due[0]
	log.Printf("lazy menjalankan job %s scheduled=%s (pending=%d due=%d)",
		job.ID, job.ScheduledAt.In(loc).Format(time.RFC3339), pending, len(due))

	go func(j Job) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("lazy panic job %s: %v", j.ID, rec)
				_ = s.deps.Store.UpdateJob(j.ID, func(job *Job) {
					job.Status = StatusFailed
					job.Error = fmt.Sprintf("panic: %v", rec)
					job.FinishedAt = time.Now().UTC()
				})
			}
			s.mu.Lock()
			s.busy = false
			s.mu.Unlock()
		}()
		s.deps.RunJob(j)
	}(job)
}

// EnsureDayPlan creates pending jobs for the calendar day of `day` if missing (idempotent).
func (s *Scheduler) EnsureDayPlan(day time.Time, postsPerDay int, client *threads.Client) error {
	loc := s.deps.Store.Location()
	day = day.In(loc)
	date := day.Format("2006-01-02")
	if s.deps.Store.HasPlanForDate(date) {
		return nil
	}
	if postsPerDay < 5 {
		postsPerDay = 5
	}
	slots := BestSlotTimes(client, loc, day, postsPerDay)
	today := time.Now().In(loc).Format("2006-01-02")
	if date == today {
		slots = adjustPastSlots(slots, time.Now().In(loc), 90*time.Minute)
	}

	jobs := make([]Job, 0, len(slots))
	for i, t := range slots {
		jobs = append(jobs, Job{
			ID:          fmt.Sprintf("%s-%02d", date, i+1),
			Date:        date,
			ScheduledAt: t.UTC(),
			Status:      StatusPending,
		})
	}
	if err := s.deps.Store.AddJobs(jobs); err != nil {
		return err
	}
	log.Printf("lazy rencana %s: %d slot", date, len(jobs))
	return nil
}

func adjustPastSlots(slots []time.Time, now time.Time, gap time.Duration) []time.Time {
	if len(slots) == 0 {
		return slots
	}
	out := make([]time.Time, len(slots))
	cursor := now.Add(2 * time.Minute).Truncate(time.Minute)
	for i, t := range slots {
		if t.Before(now) {
			out[i] = cursor
			cursor = cursor.Add(gap)
		} else {
			out[i] = t
			if t.After(cursor) {
				cursor = t.Add(gap)
			} else {
				cursor = cursor.Add(gap)
			}
		}
	}
	return ensureMinGap(out, gap)
}

// RunNow enqueues one job and runs it in the background (avoids nginx 504).
func (s *Scheduler) RunNow() (Job, error) {
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return Job{}, fmt.Errorf("scheduler sedang menjalankan job lain — tunggu selesai")
	}
	s.busy = true
	s.mu.Unlock()

	loc := s.deps.Store.Location()
	now := time.Now().In(loc)
	date := now.Format("2006-01-02")
	id := fmt.Sprintf("%s-now-%d", date, now.Unix()%100000)
	job := Job{
		ID:          id,
		Date:        date,
		ScheduledAt: now.UTC(),
		Status:      StatusPending,
	}
	if err := s.deps.Store.AddJobs([]Job{job}); err != nil {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
		return Job{}, err
	}

	go func(j Job) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("lazy panic run-now %s: %v", j.ID, rec)
				_ = s.deps.Store.UpdateJob(j.ID, func(job *Job) {
					job.Status = StatusFailed
					job.Error = fmt.Sprintf("panic: %v", rec)
					job.FinishedAt = time.Now().UTC()
				})
			}
			s.mu.Lock()
			s.busy = false
			s.mu.Unlock()
		}()
		log.Printf("lazy run-now mulai %s", j.ID)
		s.deps.RunJob(j)
	}(job)

	return job, nil
}

// ReplanToday clears today's queue and builds a fresh schedule (pending slots from now).
func (s *Scheduler) ReplanToday() error {
	cfg := s.deps.Store.GetConfig()
	loc := s.deps.Store.Location()
	now := time.Now().In(loc)
	date := now.Format("2006-01-02")
	if err := s.deps.Store.ClearDatePlan(date); err != nil {
		return err
	}
	return s.EnsureDayPlan(now, cfg.PostsPerDay, s.deps.Threads)
}

func (s *Scheduler) GetJob(id string) (Job, bool) {
	return s.deps.Store.GetJob(id)
}

type Status struct {
	Config        Config         `json:"config"`
	Enabled       bool           `json:"enabled"`
	Timezone      string         `json:"timezone"`
	Today         string         `json:"today"`
	Tomorrow      string         `json:"tomorrow"`
	Counts        map[string]int `json:"counts"`
	JobsToday     []Job          `json:"jobs_today"`
	JobsTomorrow  []Job          `json:"jobs_tomorrow"`
	NextPending   *Job           `json:"next_pending,omitempty"`
	NextTomorrow  *Job           `json:"next_tomorrow,omitempty"`
	PublicBaseURL string         `json:"public_base_url"`
	PublicOK      bool           `json:"public_ok"`
	ThreadsOK     bool           `json:"threads_ok"`
	InstagramOK   bool           `json:"instagram_ok"`
	TikTokOK      bool           `json:"tiktok_ok"`
	AIOK          bool           `json:"ai_ok"`
	ThumbOK       bool           `json:"thumb_ok"`
	BufferOK      bool           `json:"buffer_ok"`
	Warnings      []string       `json:"warnings"`
}

func (s *Scheduler) Status() Status {
	cfg := s.deps.Store.GetConfig()
	loc := s.deps.Store.Location()
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	jobs := s.deps.Store.JobsForDate(today)
	jobsTom := s.deps.Store.JobsForDate(tomorrow)
	counts := s.deps.Store.CountTodayByStatus(today)

	thumbProviderOK := s.deps.Thumb != nil && s.deps.Thumb.Enabled()
	bufferOK := s.deps.Buffer != nil && s.deps.Buffer.Enabled()
	tiktokBufferOK := false
	if bufferOK {
		st := s.deps.Buffer.Status()
		tiktokBufferOK = st["tiktok_ok"] == true
	}
	threadsID := s.deps.accountID("threads")
	instagramID := s.deps.accountID("instagram")
	tiktokID := s.deps.accountID("tiktok")
	st := Status{
		Config:        cfg,
		Enabled:       cfg.Enabled,
		Timezone:      cfg.Timezone,
		Today:         today,
		Tomorrow:      tomorrow,
		Counts:        counts,
		JobsToday:     jobs,
		JobsTomorrow:  jobsTom,
		PublicBaseURL: s.deps.Public,
		PublicOK:      s.deps.publicOK(),
		ThreadsOK:     !cfg.HasChannel("threads") || (s.deps.Publisher != nil && s.deps.Publisher.ThreadsOK(threadsID)),
		InstagramOK:   !cfg.HasChannel("instagram") || (s.deps.Publisher != nil && s.deps.Publisher.InstagramOK(instagramID)),
		TikTokOK:      !cfg.HasChannel("tiktok") || tiktokBufferOK || (s.deps.Publisher != nil && s.deps.Publisher.TikTokOK(tiktokID)),
		AIOK:          s.deps.AI != nil && s.deps.AI.Enabled(),
		ThumbOK:       !cfg.HasChannel("threads") || thumbProviderOK,
		BufferOK:      bufferOK,
	}
	var warns []string
	if !st.AIOK {
		warns = append(warns, "AI_API_KEY belum di-set")
	}
	if cfg.HasChannel("threads") && !st.ThreadsOK {
		warns = append(warns, "Akun Threads belum dipilih untuk workspace ini")
	}
	if cfg.HasChannel("instagram") && !st.InstagramOK {
		warns = append(warns, "Akun Instagram belum dipilih untuk workspace ini")
	}
	if cfg.HasChannel("tiktok") && !st.TikTokOK {
		warns = append(warns, "Buffer TikTok belum siap — isi Buffer key + hubungkan channel TikTok di Akun")
	}
	if cfg.HasChannel("threads") && !thumbProviderOK {
		warns = append(warns, "Model gambar belum siap — Threads membutuhkan cover + utas")
	}
	if !st.PublicOK {
		warns = append(warns, "PUBLIC_BASE_URL belum valid — Instagram dan TikTok butuh URL publik HTTPS")
	}
	if cfg.Enabled && !st.ThreadsOK {
		warns = append(warns, "Otomasi ON tapi Repliz Threads belum terhubung")
	}
	st.Warnings = warns

	for i := range jobs {
		if jobs[i].Status == StatusPending {
			j := jobs[i]
			st.NextPending = &j
			break
		}
	}
	for i := range jobsTom {
		if jobsTom[i].Status == StatusPending {
			j := jobsTom[i]
			st.NextTomorrow = &j
			break
		}
	}
	return st
}
