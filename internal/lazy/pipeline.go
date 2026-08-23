package lazy

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"threads-dashboard/internal/ai"
	buff "threads-dashboard/internal/buffer"
	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/schedule"
	"threads-dashboard/internal/threads"
)

type Deps struct {
	Store            *Store
	Threads          *threads.Client
	IG               *instagram.Client
	AI               *ai.Client
	Thumb            *ai.ThumbnailClient
	Buffer           *buff.Client
	Memory           *ai.MemoryStore
	Public           string // PUBLIC_BASE_URL, no trailing slash
	ThumbDir         string // optional per-account thumbs dir
	Schedule         *schedule.Store
	Publisher        Publisher
	ResolveAccountID func(platform string) string
}

func (d *Deps) accountID(platform string) string {
	if d.ResolveAccountID == nil {
		return ""
	}
	return strings.TrimSpace(d.ResolveAccountID(platform))
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
	cfg := d.Store.GetConfig()
	if len(cfg.Channels) == 0 {
		return fmt.Errorf("pilih minimal satu kanal di Lazy Business")
	}
	threadsID := d.accountID("threads")
	instagramID := d.accountID("instagram")
	tiktokID := d.accountID("tiktok")
	if cfg.HasChannel("threads") && (d.Publisher == nil || !d.Publisher.ThreadsOK(threadsID)) {
		return fmt.Errorf("akun Threads workspace belum dipilih — atur di halaman Akun")
	}
	mem := d.Memory.Get()

	var lastErr error
	var parts []string
	var caption, title string
	var threadIDs []string
	var thumbURL string
	var coverURL string
	var imageURLs []string
	var igContainer, igMediaID string
	var bufferPostID string
	bufferErr := ""
	igSkipped := false
	igErr := ""

	useHandoff := len(job.PrefilledParts) >= 2
	prefilledThumb := strings.TrimSpace(job.PrefilledThumbURL)
	coverTitle := strings.TrimSpace(job.PrefilledCoverTitle)

	for attempt := 1; attempt <= 2; attempt++ {
		lastErr = nil
		thumbURL = ""
		coverURL = ""
		var genCoverTitle string

		if useHandoff {
			parts = append([]string(nil), job.PrefilledParts...)
			title = strings.TrimSpace(job.PrefilledTitle)
			if coverTitle == "" {
				coverTitle = parts[0]
			}
			log.Printf("lazy job %s pakai handoff Generate (%d bagian, cover=%v)", job.ID, len(parts), prefilledThumb != "")
		} else {
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
			genCoverTitle = gen.CoverTitle
			if gen.DailyFocus != nil {
				_ = d.Memory.SetDaily(*gen.DailyFocus)
			}
			_ = d.Memory.RecordGeneration(ai.GenHistory{
				Topic:         cfg.TopicHint,
				Instructions:  mem.Instructions,
				Drafts:        gen.Drafts,
				Consideration: gen.Consideration,
			})
		}

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

		if coverTitle == "" {
			coverTitle = strings.TrimSpace(genCoverTitle)
		}
		if coverTitle == "" {
			coverTitle = parts[0]
		}

		coverTpl := lazyCoverTemplate(cfg)

		needVisualCover := cfg.HasChannel("threads") || cfg.HasChannel("instagram") || cfg.HasChannel("tiktok")
		var sharedCoverURL string
		if needVisualCover {
			if prefilledThumb != "" {
				sharedCoverURL, err = d.resolveThumbPublicURL(prefilledThumb)
				if err != nil {
					lastErr = fmt.Errorf("cover (reuse Generate): %w", err)
					log.Printf("lazy job %s cover reuse: %v", job.ID, err)
					time.Sleep(time.Duration(attempt) * 2 * time.Second)
					continue
				}
				log.Printf("lazy job %s reuse cover Generate: %s", job.ID, sharedCoverURL)
			} else {
				sharedCoverURL, err = d.generateEdgeCleanCover(job, parts[0], coverTitle, mem.Brand, coverTpl)
				if err != nil {
					lastErr = fmt.Errorf("cover visual: %w", err)
					log.Printf("lazy job %s cover visual: %v", job.ID, err)
					time.Sleep(time.Duration(attempt) * 2 * time.Second)
					continue
				}
				log.Printf("lazy job %s satu cover Edge Clean untuk semua kanal: %s", job.ID, sharedCoverURL)
			}
			if cfg.HasChannel("threads") {
				thumbURL = sharedCoverURL
			}
			if cfg.HasChannel("instagram") || cfg.HasChannel("tiktok") {
				coverURL = sharedCoverURL
			}
		}

		if cfg.HasChannel("threads") {
			threadIDs, err = d.publishThreads(threadsID, parts, thumbURL)
			if err != nil {
				lastErr = fmt.Errorf("threads: %w", err)
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
		}

		_ = d.Store.UpdateJob(job.ID, func(j *Job) {
			j.Parts = parts
			j.Title = title
			j.Caption = caption
			j.ThreadsIDs = threadIDs
			j.ThumbURL = thumbURL
			j.CoverURL = coverURL
		})

		needSlides := d.publicOK() && (cfg.HasChannel("instagram") || cfg.HasChannel("tiktok"))

		if needSlides {
			// Instagram dan TikTok selalu menerima paket yang sama:
			// slide 1 = cover/hook, slide 2+ = isi carousel.
			imageURLs, err = d.renderAndURLs(job, mem.Brand, parts)
			if err != nil {
				igSkipped = true
				igErr = "render: " + err.Error()
				imageURLs = nil
			}
			if len(imageURLs) > 0 && coverURL != "" {
				// Urutan publish carousel: cover visual dahulu, baru isi.
				imageURLs = append([]string{coverURL}, imageURLs...)
			}
		}

		// IG path
		if !cfg.HasChannel("instagram") {
			// Kanal ini tidak dipilih untuk workspace.
		} else if d.Publisher == nil || !d.Publisher.InstagramOK(instagramID) {
			if !igSkipped {
				igSkipped = true
				igErr = "akun Repliz Instagram belum terhubung"
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
			id, err := d.Publisher.PublishIGCarousel(instagramID, imageURLs, caption)
			if err != nil {
				igSkipped = true
				igErr = "ig: " + err.Error()
			} else {
				igMediaID = id
				igContainer = id
			}
		}

		if cfg.HasChannel("tiktok") {
			if d.Publisher == nil || !d.Publisher.TikTokOK(tiktokID) {
				bufferErr = "akun TikTok Repliz belum dipilih — atur di /app/akun"
			} else if !d.publicOK() {
				bufferErr = "PUBLIC_BASE_URL belum valid — TikTok butuh URL gambar publik HTTPS"
			} else if len(imageURLs) < 2 {
				bufferErr = "TikTok membutuhkan minimal 2 gambar (cover + slide carousel)"
			} else {
				id, pubErr := d.Publisher.PublishTikTokCarousel(tiktokID, imageURLs, caption)
				if pubErr != nil {
					bufferErr = pubErr.Error()
					log.Printf("lazy job %s repliz tiktok: %v", job.ID, pubErr)
				} else {
					bufferPostID = id
					log.Printf("lazy job %s repliz tiktok schedule: %s", job.ID, id)
				}
			}
		}

		_ = d.Store.UpdateJob(job.ID, func(j *Job) {
			j.Parts = parts
			j.Title = title
			j.Caption = caption
			j.ThreadsIDs = threadIDs
			j.ThumbURL = thumbURL
			j.CoverURL = coverURL
			j.ImageURLs = imageURLs
			j.IGContainer = igContainer
			j.IGMediaID = igMediaID
			j.BufferPostID = bufferPostID
			j.BufferError = bufferErr
			j.FinishedAt = time.Now().UTC()
			if igSkipped || bufferErr != "" {
				j.Status = StatusSkippedIG
				errs := make([]string, 0, 2)
				if strings.TrimSpace(igErr) != "" {
					errs = append(errs, igErr)
				}
				if strings.TrimSpace(bufferErr) != "" {
					errs = append(errs, "tiktok: "+bufferErr)
				}
				j.Error = strings.Join(errs, "; ")
			} else {
				j.Status = StatusDone
				j.Error = ""
			}
		})
		log.Printf("lazy job %s done threads=%d thread_cover=%v carousel_cover=%v ig_skip=%v tiktok=%v",
			job.ID, len(threadIDs), thumbURL != "", coverURL != "", igSkipped, bufferPostID != "")
		return nil
	}
	return lastErr
}

// generateEdgeCleanCover membuat cover 4:5 + panel putih seperti auto thumbnail
// di /app/generate. Aplikasi merender teks/handle sendiri agar tidak gibberish.
func (d *Deps) generateEdgeCleanCover(job Job, hook, coverTitle, brand, template string) (string, error) {
	if d.Thumb == nil || !d.Thumb.Enabled() {
		return "", fmt.Errorf("AI_API_KEY belum — cover carousel tidak bisa dibuat")
	}
	if !d.publicOK() {
		return "", fmt.Errorf("PUBLIC_BASE_URL belum di-set — cover carousel tidak bisa dipublish")
	}
	title := strings.TrimSpace(coverTitle)
	if title == "" {
		title = strings.TrimSpace(hook)
	}
	if title == "" {
		return "", fmt.Errorf("judul cover kosong")
	}
	hook = strings.TrimSpace(hook)
	if hook == "" {
		hook = title
	}
	crop := true
	result, err := d.Thumb.GenerateRequest(ai.ThumbnailRequest{
		Hook:            hook,
		Model:           lazyThumbModel(),
		Size:            "1080x1350",
		Quality:         lazyThumbQuality(),
		AspectRatio:     "4:5",
		Crop43:          &crop,
		CustomOnly:      true,
		Extra:           ai.BuildCoverBackgroundPrompt(hook, ""),
		OverlayPanel:    true,
		OverlayTitle:    title,
		OverlayHandle:   strings.TrimPrefix(strings.TrimSpace(brand), "@"),
		OverlayTemplate: template,
	})
	if err != nil {
		return "", err
	}
	baseThumb := strings.TrimSpace(d.ThumbDir)
	if baseThumb == "" {
		baseThumb = ai.DefaultThumbMediaDir()
	}
	dir := filepath.Join(baseThumb, job.Date)
	name, err := ai.SaveThumbnailPNG(dir, result.PNG)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(d.Public, "/") + "/media/thumbs/" + job.Date + "/" + name, nil
}

func lazyThumbModel() string {
	if v := strings.TrimSpace(os.Getenv("LAZY_THUMB_MODEL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("OPENAI_IMAGE_MODEL")); v != "" {
		return v
	}
	return "cx/gpt-5.5-image"
}
func lazyThumbSize() string {
	// Semua kanal memakai cover portrait 4:5 agar satu creative direction,
	// kualitas, dan framing-nya konsisten dengan /app/generate.
	return "1080x1350"
}
func lazyThumbQuality() string {
	return envOr("LAZY_THUMB_QUALITY", "high")
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func (d *Deps) publishThreads(accountID string, parts []string, thumbURL string) ([]string, error) {
	if d.Publisher != nil {
		return d.Publisher.PublishThreads(accountID, parts, strings.TrimSpace(thumbURL), "")
	}
	return nil, fmt.Errorf("publisher Repliz belum siap")
}

func (d *Deps) renderAndURLs(job Job, brand string, parts []string) ([]string, error) {
	tpl := d.Store.GetConfig().CarouselTemplate
	return RenderPartsPublic(d.Store.MediaDir(), d.Public, brand, job.Date+"/"+job.ID, parts, tpl)
}

// RenderPartsPublic menulis PNG per slide dan mengembalikan URL publik.
func RenderPartsPublic(mediaDir, publicBase, brand, subdir string, parts []string, template string) ([]string, error) {
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
		if err := RenderSlidePNG(path, brand, p, i+1, total, template); err != nil {
			return nil, err
		}
		urls = append(urls, fmt.Sprintf("%s/media/lazy/%s/%s", base, strings.ReplaceAll(subdir, "\\", "/"), name))
	}
	return urls, nil
}

func extractPublishedID(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.RawMessage:
		var p struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(t, &p) == nil {
			return strings.TrimSpace(p.ID)
		}
	case map[string]any:
		if id, ok := t["id"].(string); ok {
			return strings.TrimSpace(id)
		}
	case []byte:
		var p struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(t, &p) == nil {
			return strings.TrimSpace(p.ID)
		}
	}
	return ""
}
