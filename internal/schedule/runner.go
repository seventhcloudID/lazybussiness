package schedule

import (
	"fmt"
	"log"
	"strings"
	"time"
)

type PublishFunc func(post Post) ([]string, error)

// ProcessWith is used by the Lazy ticker (manual queue, independent of Lazy ON/OFF).
func (s *Store) ProcessWith(pub PublishFunc) {
	ProcessDue(s, pub)
}

// ProcessDue claims and publishes at most one due post.
func ProcessDue(store *Store, pub PublishFunc) {
	if store == nil || pub == nil {
		return
	}
	store.Prune(30)
	now := time.Now()
	post, ok := store.ClaimDue(now)
	if !ok {
		return
	}
	if post.RunAt.IsZero() || post.RunAt.After(now.UTC().Add(30*time.Second)) {
		log.Printf("schedule: skip %s run_at=%s (belum due)", post.ID, post.RunAt.Format(time.RFC3339))
		_ = store.Update(post.ID, func(p *Post) {
			p.Status = StatusPending
			p.StartedAt = time.Time{}
			p.Error = "claim dibatalkan: belum due"
		})
		return
	}
	if len(post.ThreadsIDs) > 0 {
		log.Printf("schedule: skip %s sudah punya threads_ids", post.ID)
		_ = store.Update(post.ID, func(p *Post) {
			p.Status = StatusDone
			if p.FinishedAt.IsZero() {
				p.FinishedAt = time.Now().UTC()
			}
		})
		return
	}
	log.Printf("schedule: publish %s run_at=%s now=%s", post.ID, post.RunAt.Format(time.RFC3339), now.UTC().Format(time.RFC3339))
	ids, err := pub(post)
	if err != nil {
		log.Printf("schedule: gagal %s: %v", post.ID, err)
		_ = store.Update(post.ID, func(p *Post) {
			p.Status = StatusFailed
			p.Error = err.Error()
			p.ThreadsIDs = ids
			p.FinishedAt = time.Now().UTC()
		})
		return
	}
	_ = store.Update(post.ID, func(p *Post) {
		p.Status = StatusDone
		p.Error = ""
		p.ThreadsIDs = ids
		p.FinishedAt = time.Now().UTC()
	})
	log.Printf("schedule: ok %s threads=%v", post.ID, ids)
}

func PartsOf(post Post) []string {
	parts := post.Parts
	if len(parts) == 0 && strings.TrimSpace(post.Text) != "" {
		parts = []string{post.Text}
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ValidateParts(post Post) error {
	if len(PartsOf(post)) == 0 {
		return fmt.Errorf("konten kosong")
	}
	return nil
}
