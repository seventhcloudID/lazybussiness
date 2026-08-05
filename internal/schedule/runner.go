package schedule

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"threads-dashboard/internal/threads"
)

// ProcessWith is used by the Lazy ticker (manual queue, independent of Lazy ON/OFF).
func (s *Store) ProcessWith(th *threads.Client) {
	ProcessDue(s, th)
}

// ProcessDue claims and publishes at most one due post.
func ProcessDue(store *Store, th *threads.Client) {
	if store == nil || th == nil || !th.Connected() {
		return
	}
	store.Prune(30)
	post, ok := store.ClaimDue(time.Now())
	if !ok {
		return
	}
	log.Printf("schedule: publish %s run_at=%s", post.ID, post.RunAt.Format(time.RFC3339))
	ids, err := publish(th, post)
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

func publish(th *threads.Client, post Post) ([]string, error) {
	parts := post.Parts
	if len(parts) == 0 && strings.TrimSpace(post.Text) != "" {
		parts = []string{post.Text}
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("konten kosong")
	}

	var ids []string
	prev := strings.TrimSpace(post.ReplyToID)
	for i, text := range parts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		form := url.Values{"text": {text}}
		mt := "TEXT"
		if i == 0 {
			switch strings.ToUpper(post.MediaType) {
			case "IMAGE":
				if post.ImageURL != "" {
					mt = "IMAGE"
					form.Set("image_url", post.ImageURL)
				}
			case "VIDEO":
				if post.VideoURL != "" {
					mt = "VIDEO"
					form.Set("video_url", post.VideoURL)
				}
			}
			if post.ReplyControl != "" {
				form.Set("reply_control", post.ReplyControl)
			}
		}
		form.Set("media_type", mt)
		if prev != "" {
			form.Set("reply_to_id", prev)
		}

		var created struct {
			ID string `json:"id"`
		}
		var lastErr error
		for attempt := 1; attempt <= 3; attempt++ {
			container, err := th.CreateContainer(form)
			if err != nil {
				lastErr = fmt.Errorf("bagian %d container: %w", i+1, err)
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
			_ = json.Unmarshal(container, &created)
			if created.ID == "" {
				lastErr = fmt.Errorf("bagian %d: container id kosong", i+1)
				break
			}
			pub, err := th.PublishContainer(created.ID)
			if err != nil {
				lastErr = fmt.Errorf("bagian %d publish: %w", i+1, err)
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
			var published struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(pub, &published)
			id := published.ID
			if id == "" {
				id = created.ID
			}
			ids = append(ids, id)
			prev = id
			lastErr = nil
			break
		}
		if lastErr != nil {
			return ids, lastErr
		}
		if i < len(parts)-1 {
			time.Sleep(2 * time.Second)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("tidak ada bagian yang ter-publish")
	}
	return ids, nil
}
