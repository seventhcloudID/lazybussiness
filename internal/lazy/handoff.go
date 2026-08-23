package lazy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ContentHandoff is editorial content + cover from /app/generate for the next Lazy job.
type ContentHandoff struct {
	Parts      []string  `json:"parts"`
	Title      string    `json:"title,omitempty"`
	CoverTitle string    `json:"cover_title,omitempty"`
	ThumbURL   string    `json:"thumb_url,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

func normalizeCoverTemplate(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	switch id {
	case "edge-clean", "split-roomy", "inset-editorial", "left-cut", "right-cut", "low-editorial":
	 return id
	default:
		return "edge-clean"
	}
}

func lazyCoverTemplate(cfg Config) string {
	if t := strings.TrimSpace(cfg.CoverTemplate); t != "" {
		return normalizeCoverTemplate(t)
	}
	if t := strings.TrimSpace(os.Getenv("LAZY_COVER_TEMPLATE")); t != "" {
		return normalizeCoverTemplate(t)
	}
	return "edge-clean"
}

func (s *Store) loadHandoff() {
	if s.handoffPath == "" {
		return
	}
	b, err := os.ReadFile(s.handoffPath)
	if err != nil {
		return
	}
	var h ContentHandoff
	if json.Unmarshal(b, &h) == nil && len(h.Parts) >= 2 {
		s.pendingHandoff = &h
	}
}

func (s *Store) saveHandoffLocked() error {
	if s.handoffPath == "" {
		return nil
	}
	if s.pendingHandoff == nil {
		_ = os.Remove(s.handoffPath)
		return nil
	}
	b, err := json.MarshalIndent(s.pendingHandoff, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.handoffPath, b, 0o644)
}

func (s *Store) applyHandoffToJob(j *Job, h *ContentHandoff) {
	j.PrefilledParts = append([]string(nil), h.Parts...)
	j.PrefilledTitle = strings.TrimSpace(h.Title)
	j.PrefilledCoverTitle = strings.TrimSpace(h.CoverTitle)
	j.PrefilledThumbURL = strings.TrimSpace(h.ThumbURL)
}

func (s *Store) attachHandoffToEarliestPendingLocked(h *ContentHandoff) (jobID string, ok bool) {
	if h == nil || len(h.Parts) < 2 {
		return "", false
	}
	best := -1
	for i := range s.jobs {
		j := &s.jobs[i]
		if j.Status != StatusPending {
			continue
		}
		if len(j.PrefilledParts) >= 2 {
			continue
		}
		if best < 0 || j.ScheduledAt.Before(s.jobs[best].ScheduledAt) {
			best = i
		}
	}
	if best < 0 {
		return "", false
	}
	s.applyHandoffToJob(&s.jobs[best], h)
	return s.jobs[best].ID, true
}

func (s *Store) tryApplyPendingHandoffLocked() {
	if s.pendingHandoff == nil {
		return
	}
	if _, ok := s.attachHandoffToEarliestPendingLocked(s.pendingHandoff); ok {
		s.pendingHandoff = nil
		_ = s.saveHandoffLocked()
	}
}

// EnqueueHandoff queues Generate output for the next pending Lazy job.
func (s *Store) EnqueueHandoff(h ContentHandoff) (attachedJobID string, pending bool, err error) {
	h.Parts = cleanHandoffParts(h.Parts)
	if len(h.Parts) < 2 {
		return "", false, fmt.Errorf("utas minimal 2 bagian")
	}
	h.Title = strings.TrimSpace(h.Title)
	h.CoverTitle = strings.TrimSpace(h.CoverTitle)
	h.ThumbURL = strings.TrimSpace(h.ThumbURL)
	if h.ThumbURL == "" {
		return "", false, fmt.Errorf("cover Edge Clean wajib — generate thumbnail di /app/generate dulu")
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.attachHandoffToEarliestPendingLocked(&h); ok {
		if err := s.saveJobsLocked(); err != nil {
			return "", false, err
		}
		return id, false, nil
	}
	s.pendingHandoff = &h
	if err := s.saveHandoffLocked(); err != nil {
		return "", false, err
	}
	return "", true, nil
}

// PendingHandoff returns a copy of the queued handoff, if any.
func (s *Store) PendingHandoff() *ContentHandoff {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingHandoff == nil {
		return nil
	}
	out := *s.pendingHandoff
	out.Parts = append([]string(nil), out.Parts...)
	return &out
}

func cleanHandoffParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (d *Deps) resolveThumbPublicURL(relOrAbs string) (string, error) {
	u := strings.TrimSpace(relOrAbs)
	if u == "" {
		return "", fmt.Errorf("URL cover kosong")
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u, nil
	}
	if !d.publicOK() {
		return "", fmt.Errorf("PUBLIC_BASE_URL belum di-set — cover Generate butuh URL publik untuk publish")
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return strings.TrimRight(d.Public, "/") + u, nil
}

func handoffPathFor(jobsPath string) string {
	dir := filepath.Dir(jobsPath)
	return filepath.Join(dir, "lazy_handoff.json")
}
