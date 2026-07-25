package lazy

import (
	"fmt"
	"log"
	"sync"
	"time"

	"threads-dashboard/internal/threads"
)

type Scheduler struct {
	deps   *Deps
	stopCh chan struct{}
	once   sync.Once
	mu     sync.Mutex
	busy   bool
}

func NewScheduler(deps *Deps) *Scheduler {
	return &Scheduler{
		deps:   deps,
		stopCh: make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	// Job yang putus tengah jalan (restart server) → kembalikan ke pending
	n := s.deps.Store.RecoverStuckJobs()
	if n > 0 {
		log.Printf("lazy: recover %d job stuck running → pending", n)
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

	cfg := s.deps.Store.GetConfig()
	loc := s.deps.Store.Location()
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	keepFrom := now.AddDate(0, 0, -14).Format("2006-01-02")
	_ = s.deps.Store.PruneOldJobs(keepFrom)

	if cfg.Enabled {
		if err := s.EnsureDayPlan(now, cfg.PostsPerDay, s.deps.Threads); err != nil {
			log.Printf("lazy plan: %v", err)
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

// EnsureDayPlan creates today's pending jobs if missing (idempotent).
func (s *Scheduler) EnsureDayPlan(now time.Time, postsPerDay int, client *threads.Client) error {
	loc := s.deps.Store.Location()
	now = now.In(loc)
	date := now.Format("2006-01-02")
	if s.deps.Store.HasPlanForDate(date) {
		return nil
	}
	if postsPerDay < 5 {
		postsPerDay = 5
	}
	slots := BestSlotTimes(client, loc, now, postsPerDay)
	slots = adjustPastSlots(slots, now, 90*time.Minute)

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
	for _, j := range s.deps.Store.ListJobs() {
		if j.ID == id {
			return j, true
		}
	}
	return Job{}, false
}

type Status struct {
	Config          Config         `json:"config"`
	Enabled         bool           `json:"enabled"`
	Timezone        string         `json:"timezone"`
	Today           string         `json:"today"`
	Counts          map[string]int `json:"counts"`
	JobsToday       []Job          `json:"jobs_today"`
	NextPending     *Job           `json:"next_pending,omitempty"`
	PublicBaseURL   string         `json:"public_base_url"`
	PublicOK        bool           `json:"public_ok"`
	ThreadsOK       bool           `json:"threads_ok"`
	InstagramOK     bool           `json:"instagram_ok"`
	AIOK            bool           `json:"ai_ok"`
	ThumbOK         bool           `json:"thumb_ok"`
	Warnings        []string       `json:"warnings"`
}

func (s *Scheduler) Status() Status {
	cfg := s.deps.Store.GetConfig()
	loc := s.deps.Store.Location()
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	jobs := s.deps.Store.JobsForDate(today)
	counts := s.deps.Store.CountTodayByStatus(today)

	st := Status{
		Config:        cfg,
		Enabled:       cfg.Enabled,
		Timezone:      cfg.Timezone,
		Today:         today,
		Counts:        counts,
		JobsToday:     jobs,
		PublicBaseURL: s.deps.Public,
		PublicOK:      s.deps.publicOK(),
		ThreadsOK:     s.deps.Threads != nil && s.deps.Threads.Connected(),
		InstagramOK:   s.deps.IG != nil && s.deps.IG.Connected(),
		AIOK:          s.deps.AI != nil && s.deps.AI.Enabled(),
		ThumbOK:       s.deps.Thumb != nil && s.deps.Thumb.Enabled(),
	}
	var warns []string
	if !st.AIOK {
		warns = append(warns, "AI_API_KEY belum di-set")
	}
	if !st.ThreadsOK {
		warns = append(warns, "Token Threads belum terhubung")
	}
	if !st.InstagramOK {
		warns = append(warns, "Token Instagram belum — IG akan di-skip")
	}
	if !st.ThumbOK {
		warns = append(warns, "OPENAI_API_KEY belum — thumbnail utas Threads di-skip")
	}
	if !st.PublicOK {
		warns = append(warns, "PUBLIC_BASE_URL belum valid — IG + attach thumbnail Threads butuh URL publik HTTPS")
	}
	if cfg.Enabled && !st.ThreadsOK {
		warns = append(warns, "Otomasi ON tapi Threads offline")
	}
	st.Warnings = warns

	for i := range jobs {
		if jobs[i].Status == StatusPending {
			j := jobs[i]
			st.NextPending = &j
			break
		}
	}
	return st
}
