package lazy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"threads-dashboard/internal/ai"
	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/threads"
)

type Deps struct {
	Store   *Store
	Threads *threads.Client
	IG      *instagram.Client
	AI      *ai.Client
	Memory  *ai.MemoryStore
	Public  string // PUBLIC_BASE_URL, no trailing slash
}

func (d *Deps) publicOK() bool {
	u := strings.TrimSpace(d.Public)
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}

func (d *Deps) RunJob(job Job) {
	_ = d.Store.UpdateJob(job.ID, func(j *Job) {
		j.Status = StatusRunning
		j.StartedAt = time.Now().UTC()
		j.Error = ""
	})

	err := d.runOnce(job)
	if err != nil {
		log.Printf("lazy job %s failed: %v", job.ID, err)
		_ = d.Store.UpdateJob(job.ID, func(j *Job) {
			j.Status = StatusFailed
			j.Error = err.Error()
			j.FinishedAt = time.Now().UTC()
		})
		return
	}
}

func (d *Deps) runOnce(job Job) error {
	if d.AI == nil || !d.AI.Enabled() {
		return fmt.Errorf("AI belum dikonfigurasi")
	}
	if d.Threads == nil || !d.Threads.Connected() {
		return fmt.Errorf("token Threads belum terhubung")
	}

	cfg := d.Store.GetConfig()
	mem := d.Memory.Get()

	var lastErr error
	var parts []string
	var caption, title string
	var threadIDs []string
	var imageURLs []string
	var igContainer string
	igSkipped := false
	igErr := ""

	for attempt := 1; attempt <= 2; attempt++ {
		lastErr = nil
		gen, err := d.AI.GenerateContent(nil, mem, ai.GenerateRequest{
			Topic: cfg.TopicHint,
			Count: 1,
		})
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}
		if len(gen.Drafts) == 0 || len(gen.Drafts[0].Parts) < 2 {
			lastErr = fmt.Errorf("generate menghasilkan kurang dari 2 bagian")
			continue
		}
		draft := gen.Drafts[0]
		parts = draft.Parts
		title = draft.Title
		if gen.DailyFocus != nil {
			_ = d.Memory.SetDaily(*gen.DailyFocus)
		}
		_ = d.Memory.RecordGeneration(ai.GenHistory{
			Topic:         cfg.TopicHint,
			Instructions:  mem.Instructions,
			Drafts:        gen.Drafts,
			Consideration: gen.Consideration,
		})

		car, err := d.AI.GenerateCarousel(mem, ai.CarouselRequest{
			Parts: parts,
			Brand: mem.Brand,
			Topic: cfg.TopicHint,
		})
		if err == nil && car != nil {
			caption = car.Caption
			if car.Title != "" {
				title = car.Title
			}
		}

		threadIDs, err = d.publishThreads(parts)
		if err != nil {
			lastErr = fmt.Errorf("threads: %w", err)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		_ = d.Store.UpdateJob(job.ID, func(j *Job) {
			j.Parts = parts
			j.Title = title
			j.Caption = caption
			j.ThreadsIDs = threadIDs
		})

		// IG path
		if d.IG == nil || !d.IG.Connected() {
			igSkipped = true
			igErr = "token Instagram belum terhubung"
		} else if !d.publicOK() {
			igSkipped = true
			igErr = "PUBLIC_BASE_URL belum di-set (butuh URL publik HTTPS)"
		} else {
			imageURLs, err = d.renderAndURLs(job, mem.Brand, parts)
			if err != nil {
				igSkipped = true
				igErr = "render: " + err.Error()
			} else {
				out, err := d.IG.PublishCarousel(imageURLs, caption)
				if err != nil {
					igSkipped = true
					igErr = "ig: " + err.Error()
				} else if out != nil {
					if c, ok := out["container"].(string); ok {
						igContainer = c
					}
				}
			}
		}

		_ = d.Store.UpdateJob(job.ID, func(j *Job) {
			j.Parts = parts
			j.Title = title
			j.Caption = caption
			j.ThreadsIDs = threadIDs
			j.ImageURLs = imageURLs
			j.IGContainer = igContainer
			j.FinishedAt = time.Now().UTC()
			if igSkipped {
				j.Status = StatusSkippedIG
				j.Error = igErr
			} else {
				j.Status = StatusDone
				j.Error = ""
			}
		})
		log.Printf("lazy job %s done threads=%d ig_skip=%v", job.ID, len(threadIDs), igSkipped)
		return nil
	}
	return lastErr
}

func (d *Deps) publishThreads(parts []string) ([]string, error) {
	var ids []string
	var prevID string
	for i, text := range parts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		form := url.Values{
			"media_type": {"TEXT"},
			"text":       {text},
		}
		if prevID != "" {
			form.Set("reply_to_id", prevID)
		}

		var created struct {
			ID string `json:"id"`
		}
		var pub json.RawMessage
		var lastErr error
		for attempt := 1; attempt <= 3; attempt++ {
			container, err := d.Threads.CreateContainer(form)
			if err != nil {
				lastErr = fmt.Errorf("bagian %d container: %w", i+1, err)
				if isMediaNotFound(err) && prevID != "" && attempt < 3 {
					time.Sleep(time.Duration(attempt*3) * time.Second)
					continue
				}
				return ids, lastErr
			}
			_ = json.Unmarshal(container, &created)
			if created.ID == "" {
				return ids, fmt.Errorf("bagian %d: container id kosong", i+1)
			}
			pub, err = d.Threads.PublishContainer(created.ID)
			if err != nil {
				lastErr = fmt.Errorf("bagian %d publish: %w", i+1, err)
				if isMediaNotFound(err) && attempt < 3 {
					time.Sleep(time.Duration(attempt*3) * time.Second)
					continue
				}
				return ids, lastErr
			}
			lastErr = nil
			break
		}
		if lastErr != nil {
			return ids, lastErr
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
		prevID = id
		// Parent harus sempat propagate sebelum reply berikutnya (hindari 4279009 di bagian lanjut).
		if i < len(parts)-1 {
			time.Sleep(2500 * time.Millisecond)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("tidak ada bagian yang terpublish")
	}
	return ids, nil
}

func isMediaNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "4279009") ||
		strings.Contains(s, "Media Tidak Ditemukan") ||
		strings.Contains(s, "Media Not Found") ||
		strings.Contains(s, "does not exist")
}

func (d *Deps) renderAndURLs(job Job, brand string, parts []string) ([]string, error) {
	return RenderPartsPublic(d.Store.MediaDir(), d.Public, brand, job.Date+"/"+job.ID, parts)
}

// RenderPartsPublic menulis PNG per slide dan mengembalikan URL publik.
func RenderPartsPublic(mediaDir, publicBase, brand, subdir string, parts []string) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if base == "" || !(strings.HasPrefix(base, "https://") || strings.HasPrefix(base, "http://")) {
		return nil, fmt.Errorf("PUBLIC_BASE_URL belum di-set (butuh URL publik HTTPS)")
	}
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) < 2 {
		return nil, fmt.Errorf("butuh minimal 2 slide teks")
	}
	if len(cleaned) > 10 {
		cleaned = cleaned[:10]
	}
	subdir = strings.Trim(strings.ReplaceAll(subdir, "..", ""), "/\\")
	if subdir == "" {
		subdir = time.Now().UTC().Format("20060102-150405")
	}
	dir := filepath.Join(mediaDir, filepath.FromSlash(subdir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var urls []string
	total := len(cleaned)
	for i, p := range cleaned {
		name := fmt.Sprintf("%02d.png", i+1)
		path := filepath.Join(dir, name)
		if err := RenderSlidePNG(path, brand, p, i+1, total); err != nil {
			return nil, err
		}
		urls = append(urls, fmt.Sprintf("%s/media/lazy/%s/%s", base, strings.ReplaceAll(subdir, "\\", "/"), name))
	}
	return urls, nil
}
