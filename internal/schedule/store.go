package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusDone      = "done"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Post is a manually scheduled Threads post (single or thread parts).
type Post struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	RunAt        time.Time `json:"run_at"`
	MediaType    string    `json:"media_type,omitempty"`
	Text         string    `json:"text,omitempty"`
	Parts        []string  `json:"parts,omitempty"`
	ImageURL     string    `json:"image_url,omitempty"`
	VideoURL     string    `json:"video_url,omitempty"`
	ReplyControl string    `json:"reply_control,omitempty"`
	ReplyToID    string    `json:"reply_to_id,omitempty"`
	ThreadsIDs   []string  `json:"threads_ids,omitempty"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
}

type fileShape struct {
	Posts []Post `json:"posts"`
}

type Store struct {
	mu   sync.Mutex
	path string
	list []Post
}

func NewStoreAt(path string) *Store {
	s := &Store{path: path, list: []Post{}}
	s.load()
	return s
}

func (s *Store) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var f fileShape
	if json.Unmarshal(b, &f) == nil && f.Posts != nil {
		s.list = f.Posts
	}
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(fileShape{Posts: s.list}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

type CreateInput struct {
	RunAt        time.Time
	MediaType    string
	Text         string
	Parts        []string
	ImageURL     string
	VideoURL     string
	ReplyControl string
	ReplyToID    string
}

func (s *Store) Create(in CreateInput) (Post, error) {
	parts := normalizeParts(in.Parts, in.Text)
	if len(parts) == 0 {
		return Post{}, fmt.Errorf("teks / parts wajib diisi")
	}
	if in.RunAt.IsZero() {
		return Post{}, fmt.Errorf("run_at wajib (RFC3339)")
	}
	// Minimal 1 menit ke depan — hindari "langsung post" karena detik datetime-local.
	if !in.RunAt.After(time.Now().Add(55 * time.Second)) {
		return Post{}, fmt.Errorf("run_at harus minimal ~1 menit ke depan")
	}
	mt := strings.ToUpper(strings.TrimSpace(in.MediaType))
	if mt == "" {
		mt = "TEXT"
	}
	p := Post{
		ID:           newID(),
		Status:       StatusPending,
		RunAt:        in.RunAt.UTC(),
		MediaType:    mt,
		Text:         parts[0],
		Parts:        parts,
		ImageURL:     strings.TrimSpace(in.ImageURL),
		VideoURL:     strings.TrimSpace(in.VideoURL),
		ReplyControl: strings.TrimSpace(in.ReplyControl),
		ReplyToID:    strings.TrimSpace(in.ReplyToID),
		CreatedAt:    time.Now().UTC(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, p)
	if err := s.saveLocked(); err != nil {
		s.list = s.list[:len(s.list)-1]
		return Post{}, err
	}
	return p, nil
}

func normalizeParts(parts []string, text string) []string {
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		if t := strings.TrimSpace(text); t != "" {
			out = []string{t}
		}
	}
	return out
}

func (s *Store) List(includeDone bool) []Post {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Post, 0, len(s.list))
	for _, p := range s.list {
		if !includeDone && (p.Status == StatusDone || p.Status == StatusCancelled) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RunAt.Before(out[j].RunAt)
	})
	return out
}

func (s *Store) Get(id string) (Post, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.list {
		if p.ID == id {
			return p, true
		}
	}
	return Post{}, false
}

func (s *Store) Cancel(id string) (Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID != id {
			continue
		}
		if s.list[i].Status != StatusPending {
			return Post{}, fmt.Errorf("hanya pending yang bisa dibatalkan (status=%s)", s.list[i].Status)
		}
		s.list[i].Status = StatusCancelled
		s.list[i].FinishedAt = time.Now().UTC()
		if err := s.saveLocked(); err != nil {
			return Post{}, err
		}
		return s.list[i], nil
	}
	return Post{}, fmt.Errorf("jadwal tidak ditemukan")
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID != id {
			continue
		}
		s.list = append(s.list[:i], s.list[i+1:]...)
		return s.saveLocked()
	}
	return fmt.Errorf("jadwal tidak ditemukan")
}

// ClaimDue marks the next due pending post as running.
func (s *Store) ClaimDue(now time.Time) (Post, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now = now.UTC()
	idx := -1
	dirty := false
	for i := range s.list {
		p := s.list[i]
		if p.Status != StatusPending {
			continue
		}
		if p.RunAt.IsZero() {
			// Data korup / parse gagal — jangan auto-publish.
			continue
		}
		if len(p.ThreadsIDs) > 0 {
			// Sudah pernah terpublish — anggap selesai, jangan claim lagi.
			s.list[i].Status = StatusDone
			if s.list[i].FinishedAt.IsZero() {
				s.list[i].FinishedAt = time.Now().UTC()
			}
			dirty = true
			continue
		}
		if p.RunAt.After(now) {
			continue
		}
		if idx < 0 || p.RunAt.Before(s.list[idx].RunAt) {
			idx = i
		}
	}
	if idx < 0 {
		if dirty {
			_ = s.saveLocked()
		}
		return Post{}, false
	}
	s.list[idx].Status = StatusRunning
	s.list[idx].StartedAt = time.Now().UTC()
	_ = s.saveLocked()
	return s.list[idx], true
}

func (s *Store) Update(id string, fn func(*Post)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID != id {
			continue
		}
		fn(&s.list[i])
		return s.saveLocked()
	}
	return fmt.Errorf("jadwal tidak ditemukan")
}

// RecoverStuck turns abandoned "running" posts back to a safe state after restart.
// Posts that already have ThreadsIDs are marked done (idempotent — no re-publish).
func (s *Store) RecoverStuck() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for i := range s.list {
		if s.list[i].Status != StatusRunning {
			continue
		}
		n++
		if len(s.list[i].ThreadsIDs) > 0 {
			s.list[i].Status = StatusDone
			s.list[i].Error = "recovered after restart (sudah terpublish)"
			if s.list[i].FinishedAt.IsZero() {
				s.list[i].FinishedAt = time.Now().UTC()
			}
			continue
		}
		// Belum ada bukti publish — kembalikan ke antrian, tapi jangan
		// geser run_at. ClaimDue tetap hormati jadwal.
		s.list[i].Status = StatusPending
		s.list[i].StartedAt = time.Time{}
		s.list[i].Error = "recovered after restart"
	}
	if n > 0 {
		_ = s.saveLocked()
	}
	return n
}

// Prune removes old done/failed/cancelled older than keepDays.
func (s *Store) Prune(keepDays int) {
	if keepDays <= 0 {
		keepDays = 30
	}
	cut := time.Now().UTC().AddDate(0, 0, -keepDays)
	s.mu.Lock()
	defer s.mu.Unlock()
	dst := s.list[:0]
	for _, p := range s.list {
		if (p.Status == StatusDone || p.Status == StatusFailed || p.Status == StatusCancelled) &&
			!p.FinishedAt.IsZero() && p.FinishedAt.Before(cut) {
			continue
		}
		dst = append(dst, p)
	}
	s.list = dst
	_ = s.saveLocked()
}
