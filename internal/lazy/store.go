package lazy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusDone      = "done"
	StatusFailed    = "failed"
	StatusSkippedIG = "skipped_ig" // Threads ok, IG skipped/failed after Threads
)

type Config struct {
	Enabled            bool   `json:"enabled"`
	PostsPerDay        int    `json:"posts_per_day"`
	Timezone           string `json:"timezone"`
	TopicHint          string `json:"topic_hint,omitempty"`
	ThumbnailEnabled   bool   `json:"thumbnail_enabled"` // thumbnail utas Threads (OpenAI)
}

type Job struct {
	ID          string    `json:"id"`
	Date        string    `json:"date"` // YYYY-MM-DD in local TZ
	ScheduledAt time.Time `json:"scheduled_at"`
	Status      string    `json:"status"`
	Title       string    `json:"title,omitempty"`
	Parts       []string  `json:"parts,omitempty"`
	Caption     string    `json:"caption,omitempty"`
	ThreadsIDs  []string  `json:"threads_ids,omitempty"`
	ThumbURL    string    `json:"thumb_url,omitempty"` // Threads utas thumbnail (ChatGPT)
	ImageURLs   []string  `json:"image_urls,omitempty"`
	IGContainer string    `json:"ig_container,omitempty"`
	BufferPostID  string   `json:"buffer_post_id,omitempty"`   // TikTok
	BufferError   string   `json:"buffer_error,omitempty"`    // TikTok
	BufferXPostID string   `json:"buffer_x_post_id,omitempty"` // X/Twitter thread
	BufferXError  string   `json:"buffer_x_error,omitempty"`
	Error         string   `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	FinishedAt  time.Time `json:"finished_at,omitempty"`
}

type jobFile struct {
	Jobs []Job `json:"jobs"`
}

type Store struct {
	mu         sync.Mutex
	configPath string
	jobsPath   string
	mediaDir   string
	cfg        Config
	jobs       []Job
}

func NewStore() *Store {
	return NewStoreAt(
		filepath.Join(".data", "lazy_config.json"),
		filepath.Join(".data", "lazy_jobs.json"),
		filepath.Join(".data", "lazy-media"),
	)
}

func NewStoreAt(configPath, jobsPath, mediaDir string) *Store {
	s := &Store{
		configPath: configPath,
		jobsPath:   jobsPath,
		mediaDir:   mediaDir,
		cfg: Config{
			Enabled:          false,
			PostsPerDay:      5,
			Timezone:         "Asia/Jakarta",
			ThumbnailEnabled: true,
		},
	}
	s.load()
	return s
}

func (s *Store) MediaDir() string { return s.mediaDir }

func (s *Store) load() {
	if b, err := os.ReadFile(s.configPath); err == nil {
		// thumbnail_enabled: default true kalau field belum ada di file lama
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(b, &raw)
		var c Config
		if json.Unmarshal(b, &c) == nil {
			if c.PostsPerDay < 5 {
				c.PostsPerDay = 5
			}
			if c.Timezone == "" {
				c.Timezone = "Asia/Jakarta"
			}
			if _, ok := raw["thumbnail_enabled"]; !ok {
				c.ThumbnailEnabled = true
			}
			s.cfg = c
		}
	}
	if b, err := os.ReadFile(s.jobsPath); err == nil {
		var f jobFile
		if json.Unmarshal(b, &f) == nil {
			s.jobs = f.Jobs
		}
	}
	if s.jobs == nil {
		s.jobs = []Job{}
	}
	_ = os.MkdirAll(s.mediaDir, 0o755)
}

func (s *Store) saveConfigLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, b, 0o600)
}

func (s *Store) saveJobsLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.jobsPath), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(jobFile{Jobs: s.jobs}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.jobsPath, b, 0o600)
}

func (s *Store) GetConfig() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *Store) SetConfig(c Config) (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.PostsPerDay < 5 {
		c.PostsPerDay = 5
	}
	if c.PostsPerDay > 12 {
		c.PostsPerDay = 12
	}
	if c.Timezone == "" {
		c.Timezone = "Asia/Jakarta"
	}
	s.cfg = c
	return s.cfg, s.saveConfigLocked()
}

func (s *Store) Location() *time.Location {
	cfg := s.GetConfig()
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc, _ = time.LoadLocation("Asia/Jakarta")
	}
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return loc
}

func (s *Store) ListJobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Job, len(s.jobs))
	copy(out, s.jobs)
	return out
}

func (s *Store) JobsForDate(date string) []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Job
	for _, j := range s.jobs {
		if j.Date == date {
			out = append(out, j)
		}
	}
	return out
}

func (s *Store) HasPlanForDate(date string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.Date == date {
			return true
		}
	}
	return false
}

func (s *Store) AddJobs(jobs []Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, jobs...)
	return s.saveJobsLocked()
}

func (s *Store) UpdateJob(id string, mut func(*Job)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.jobs {
		if s.jobs[i].ID == id {
			mut(&s.jobs[i])
			return s.saveJobsLocked()
		}
	}
	return os.ErrNotExist
}

func (s *Store) DueJobs(now time.Time) []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Job
	for _, j := range s.jobs {
		if j.Status != StatusPending {
			continue
		}
		if !j.ScheduledAt.After(now) {
			out = append(out, j)
		}
	}
	// earliest first
	for i := 0; i < len(out); i++ {
		for k := i + 1; k < len(out); k++ {
			if out[k].ScheduledAt.Before(out[i].ScheduledAt) {
				out[i], out[k] = out[k], out[i]
			}
		}
	}
	return out
}

// RecoverStuckJobs turns abandoned "running" jobs back to pending after restart.
func (s *Store) RecoverStuckJobs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for i := range s.jobs {
		if s.jobs[i].Status == StatusRunning {
			s.jobs[i].Status = StatusPending
			s.jobs[i].Error = "recovered after restart"
			n++
		}
	}
	if n > 0 {
		_ = s.saveJobsLocked()
	}
	return n
}

// ClearDatePlan removes today's jobs so a fresh plan can be generated.
func (s *Store) ClearDatePlan(date string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var kept []Job
	for _, j := range s.jobs {
		if j.Date != date {
			kept = append(kept, j)
		}
	}
	s.jobs = kept
	return s.saveJobsLocked()
}

func (s *Store) CountTodayByStatus(date string) map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := map[string]int{}
	for _, j := range s.jobs {
		if j.Date != date {
			continue
		}
		m[j.Status]++
		m["total"]++
	}
	return m
}

// PruneOldJobs keeps recent history compact (last 14 days).
func (s *Store) PruneOldJobs(keepFromDate string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var kept []Job
	for _, j := range s.jobs {
		if j.Date >= keepFromDate {
			kept = append(kept, j)
		}
	}
	s.jobs = kept
	return s.saveJobsLocked()
}
