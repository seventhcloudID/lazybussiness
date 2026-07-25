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
	buff "threads-dashboard/internal/buffer"
	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/threads"
)

type Deps struct {
	Store   *Store
	Threads *threads.Client
	IG      *instagram.Client
	AI      *ai.Client
	Thumb   *ai.ThumbnailClient
	Buffer  *buff.Client
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
	var thumbURL string
	var imageURLs []string
	var igContainer string
	var bufferPostID string
	bufferErr := ""
	igSkipped := false
	igErr := ""

	for attempt := 1; attempt <= 2; attempt++ {
		lastErr = nil
		thumbURL = ""
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

		// Thumbnail ChatGPT dari hook (bagian 1) — khusus utas Threads, bukan slide IG.
		thumbURL, err = d.generateThreadsThumb(job, parts[0])
		if err != nil {
			log.Printf("lazy job %s thumb: %v (lanjut publish TEXT)", job.ID, err)
			thumbURL = ""
		}

		threadIDs, err = d.publishThreads(parts, thumbURL)
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
			j.ThumbURL = thumbURL
		})

		needSlides := d.publicOK() && (
			(d.IG != nil && d.IG.Connected()) ||
				(d.Buffer != nil && d.Buffer.Enabled()))

		if needSlides {
			imageURLs, err = d.renderAndURLs(job, mem.Brand, parts)
			if err != nil {
				igSkipped = true
				igErr = "render: " + err.Error()
				imageURLs = nil
			}
		}

		// IG path
		if d.IG == nil || !d.IG.Connected() {
			if !igSkipped {
				igSkipped = true
				igErr = "token Instagram belum terhubung"
			}
		} else if !d.publicOK() {
			igSkipped = true
			igErr = "PUBLIC_BASE_URL belum di-set (butuh URL publik HTTPS)"
		} else if len(imageURLs) < 2 {
			igSkipped = true
			if igErr == "" {
				igErr = "slide carousel belum siap"
			}
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

		// Buffer TikTok — Notify Me (antrian; user post manual di HP)
		if d.Buffer != nil && d.Buffer.Enabled() {
			if !d.publicOK() {
				bufferErr = "PUBLIC_BASE_URL belum valid"
			} else if len(imageURLs) < 1 {
				bufferErr = "slide carousel belum siap"
			} else {
				cap := caption
				if strings.TrimSpace(cap) == "" && len(parts) > 0 {
					cap = parts[0]
				}
				res, err := d.Buffer.QueueTikTokPhotos(cap, title, imageURLs)
				if err != nil {
					bufferErr = err.Error()
					log.Printf("lazy job %s buffer: %v", job.ID, err)
				} else if res != nil {
					bufferPostID = res.PostID
					log.Printf("lazy job %s buffer tiktok notify-me id=%s", job.ID, bufferPostID)
				}
			}
		}

		_ = d.Store.UpdateJob(job.ID, func(j *Job) {
			j.Parts = parts
			j.Title = title
			j.Caption = caption
			j.ThreadsIDs = threadIDs
			j.ThumbURL = thumbURL
			j.ImageURLs = imageURLs
			j.IGContainer = igContainer
			j.BufferPostID = bufferPostID
			j.BufferError = bufferErr
			j.FinishedAt = time.Now().UTC()
			if igSkipped {
				j.Status = StatusSkippedIG
				j.Error = igErr
			} else {
				j.Status = StatusDone
				j.Error = ""
			}
		})
		log.Printf("lazy job %s done threads=%d thumb=%v ig_skip=%v buffer=%v", job.ID, len(threadIDs), thumbURL != "", igSkipped, bufferPostID != "")
		return nil
	}
	return lastErr
}

func lazyThumbModel() string {
	return envOr("LAZY_THUMB_MODEL", "gpt-image-2")
}
func lazyThumbSize() string {
	return envOr("LAZY_THUMB_SIZE", "1024x768")
}
func lazyThumbQuality() string {
	// user: auto/high — default high; boleh override ke auto
	return envOr("LAZY_THUMB_QUALITY", "high")
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// generateThreadsThumb membuat thumbnail 4:3 dari hook utas via ChatGPT Image.
func (d *Deps) generateThreadsThumb(job Job, hook string) (string, error) {
	if d.Thumb == nil || !d.Thumb.Enabled() {
		return "", fmt.Errorf("OPENAI_API_KEY belum di-set")
	}
	hook = strings.TrimSpace(hook)
	if hook == "" {
		return "", fmt.Errorf("hook kosong")
	}
	crop := true
	result, err := d.Thumb.GenerateRequest(ai.ThumbnailRequest{
		Hook:    hook,
		Model:   lazyThumbModel(),
		Size:    lazyThumbSize(),
		Quality: lazyThumbQuality(),
		Crop43:  &crop,
	})
	if err != nil {
		return "", err
	}
	// Simpan di folder job biar rapi; tetap di-serve via /media/thumbs atau copy path.
	dir := filepath.Join(ai.DefaultThumbMediaDir(), job.Date)
	name, err := ai.SaveThumbnailPNG(dir, result.PNG)
	if err != nil {
		return "", err
	}
	rel := "/media/thumbs/" + job.Date + "/" + name
	if !d.publicOK() {
		return "", fmt.Errorf("PUBLIC_BASE_URL belum di-set — thumb tersimpan di %s tapi tidak bisa di-attach ke Threads", rel)
	}
	return strings.TrimRight(d.Public, "/") + rel, nil
}

func (d *Deps) publishThreads(parts []string, thumbURL string) ([]string, error) {
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
		if i == 0 && strings.TrimSpace(thumbURL) != "" {
			form.Set("media_type", "IMAGE")
			form.Set("image_url", strings.TrimSpace(thumbURL))
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
