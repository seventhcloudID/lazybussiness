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
	var igErrMsg string
	var tiktokScheduleID string
	var tiktokErrMsg string

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
			imageURLs, err = d.renderAndURLs(job, mem.Brand, parts)
			if err != nil {
				renderErr := "render: " + err.Error()
				igErrMsg = renderErr
				tiktokErrMsg = renderErr
				imageURLs = nil
			}
			if len(imageURLs) > 0 && coverURL != "" {
				imageURLs = append([]string{coverURL}, imageURLs...)
			}
		}

		igMediaID, igErrMsg = d.publishReplizCarouselID(cfg, "instagram", instagramID, imageURLs, caption, igErrMsg)
		if igMediaID != "" {
			igContainer = igMediaID
		}
		tiktokScheduleID, tiktokErrMsg = d.publishReplizCarouselID(cfg, "tiktok", tiktokID, imageURLs, caption, tiktokErrMsg)

		carouselFailed := (cfg.HasChannel("instagram") && igErrMsg != "") || (cfg.HasChannel("tiktok") && tiktokErrMsg != "")

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
			j.IGError = igErrMsg
			j.TikTokScheduleID = tiktokScheduleID
			j.TikTokError = tiktokErrMsg
			j.BufferPostID = tiktokScheduleID
			j.BufferError = tiktokErrMsg
			j.FinishedAt = time.Now().UTC()
			if carouselFailed {
				j.Status = StatusSkippedIG
				errs := make([]string, 0, 2)
				if strings.TrimSpace(igErrMsg) != "" {
					errs = append(errs, "ig: "+igErrMsg)
				}
				if strings.TrimSpace(tiktokErrMsg) != "" {
					errs = append(errs, "tiktok: "+tiktokErrMsg)
				}
				j.Error = strings.Join(errs, "; ")
			} else {
				j.Status = StatusDone
				j.Error = ""
			}
		})
		log.Printf("lazy job %s done threads=%d thread_cover=%v carousel_cover=%v ig=%v tiktok=%v",
			job.ID, len(threadIDs), thumbURL != "", coverURL != "", igMediaID != "", tiktokScheduleID != "")
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
	name, err := ai.SaveThumbnailJPEG(dir, result.PNG)
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

// ensureJobCarouselURLs memakai image_urls tersimpan atau render ulang dari parts + cover.
func (d *Deps) ensureJobCarouselURLs(job Job) ([]string, error) {
	if len(job.ImageURLs) >= 2 {
		return d.ensureJPEGCarouselURLs(job.ImageURLs)
	}
	parts := append([]string(nil), job.Parts...)
	if len(parts) < 2 {
		parts = append([]string(nil), job.PrefilledParts...)
	}
	if len(parts) < 2 {
		return nil, fmt.Errorf("carousel belum siap — butuh minimal 2 bagian teks")
	}
	brand := ""
	if d.Memory != nil {
		brand = strings.TrimSpace(d.Memory.Get().Brand)
	}
	urls, err := d.renderAndURLs(job, brand, parts)
	if err != nil {
		return nil, err
	}
	cover := strings.TrimSpace(job.CoverURL)
	if cover == "" {
		cover = strings.TrimSpace(job.ThumbURL)
	}
	if cover == "" {
		cover = strings.TrimSpace(job.PrefilledThumbURL)
	}
	if cover != "" {
		urls = append([]string{cover}, urls...)
	}
	if len(urls) < 2 {
		return nil, fmt.Errorf("carousel belum siap — butuh minimal 2 gambar (cover + slide)")
	}
	return d.ensureJPEGCarouselURLs(urls)
}

func (d *Deps) publishReplizCarouselID(cfg Config, platform, accountID string, imageURLs []string, caption, priorErr string) (scheduleID, errMsg string) {
	if priorErr != "" {
		return "", priorErr
	}
	if !cfg.HasChannel(platform) {
		return "", ""
	}
	label := platform
	if platform == "instagram" {
		label = "Instagram"
	} else if platform == "tiktok" {
		label = "TikTok"
	}
	if d.Publisher == nil {
		return "", fmt.Sprintf("akun Repliz %s belum dipilih — atur di /app/akun", label)
	}
	ok := false
	switch platform {
	case "instagram":
		ok = d.Publisher.InstagramOK(accountID)
	case "tiktok":
		ok = d.Publisher.TikTokOK(accountID)
	}
	if !ok {
		return "", fmt.Sprintf("akun Repliz %s belum dipilih — atur di /app/akun", label)
	}
	if !d.publicOK() {
		return "", "PUBLIC_BASE_URL belum valid — carousel butuh URL gambar publik HTTPS"
	}
	if len(imageURLs) < 2 {
		return "", "carousel membutuhkan minimal 2 gambar (cover + slide)"
	}
	if strings.EqualFold(platform, "tiktok") {
		var convErr error
		imageURLs, convErr = d.prepareTikTokCarouselURLs(imageURLs)
		if convErr != nil {
			return "", convErr.Error()
		}
	}
	var id string
	var err error
	switch platform {
	case "instagram":
		id, err = d.Publisher.PublishIGCarousel(accountID, imageURLs, caption)
	case "tiktok":
		id, err = d.Publisher.PublishTikTokCarousel(accountID, imageURLs, caption)
	default:
		return "", fmt.Sprintf("kanal %s tidak dikenal", platform)
	}
	if err != nil {
		log.Printf("lazy repliz %s: %v", platform, err)
		return "", err.Error()
	}
	log.Printf("lazy repliz %s schedule: %s", platform, id)
	return id, ""
}

// RenderPartsPublic menulis JPEG per slide dan mengembalikan URL publik.
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
		name := fmt.Sprintf("%02d.jpg", i+1)
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
