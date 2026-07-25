package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// YouTubeSource is a trending video picked for youtube_to_utas generation.
type YouTubeSource struct {
	VideoID  string `json:"video_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Channel  string `json:"channel,omitempty"`
	WhyHot   string `json:"why_hot,omitempty"`
	Window   string `json:"window,omitempty"` // today | week
	ThumbURL string `json:"thumb_url,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

var (
	reYTWatch  = regexp.MustCompile(`(?i)(?:youtube\.com/watch\?[^#]*v=|youtube\.com/live/|youtube\.com/shorts/|youtu\.be/)([A-Za-z0-9_-]{6,})`)
	reYTEmbed  = regexp.MustCompile(`(?i)youtube\.com/embed/([A-Za-z0-9_-]{6,})`)
	reYTBareID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
)

// ParseYouTubeVideoID extracts an 11-char (typical) video id from URL or bare id.
func ParseYouTubeVideoID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if reYTBareID.MatchString(raw) {
		return raw
	}
	if m := reYTWatch.FindStringSubmatch(raw); len(m) > 1 {
		return m[1]
	}
	if m := reYTEmbed.FindStringSubmatch(raw); len(m) > 1 {
		return m[1]
	}
	if u, err := url.Parse(raw); err == nil {
		if v := u.Query().Get("v"); reYTBareID.MatchString(v) {
			return v
		}
	}
	return ""
}

func YouTubeWatchURL(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "https://www.youtube.com/watch?v=" + id
}

func YouTubeThumbCandidates(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	base := "https://i.ytimg.com/vi/" + id + "/"
	return []string{
		base + "maxresdefault.jpg",
		base + "sddefault.jpg",
		base + "hqdefault.jpg",
		base + "mqdefault.jpg",
	}
}

// FindTrendingYouTube uses Gemini + Google Search to pick one hot video for the account niches.
func (c *Client) FindTrendingYouTube(mem Memory) (*YouTubeSource, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("AI belum dikonfigurasi")
	}
	niches := NicheList(mem)
	if len(niches) == 0 {
		return nil, fmt.Errorf("isi niche dulu sebelum kategori YouTube → utas")
	}
	today := time.Now().Format("2006-01-02")
	payload, _ := json.MarshalIndent(map[string]any{
		"niches": niches,
		"today":  today,
	}, "", "  ")

	system := `Kamu riset konten YouTube yang SEDANG RAME (hari ini atau minggu ini) untuk niche yang diberikan.
Pakai Google Search. Jangan mengarang video_id atau URL.

Tugas: pilih TEPAT SATU video YouTube publik yang relevan dengan niche, lagi dibahas / trending baru-baru ini.
Prioritas: minggu ini; kalau ada yang lagi naik hari ini, lebih baik.

Jawab HANYA satu objek JSON (tanpa markdown):
{
  "video_id": "11-char youtube id",
  "url": "https://www.youtube.com/watch?v=...",
  "title": "judul video",
  "channel": "nama channel",
  "why_hot": "1-2 kalimat kenapa ini rame / relevan sekarang",
  "window": "today" atau "week"
}

Aturan:
- url HARUS youtube.com atau youtu.be yang valid.
- video_id HARUS cocok dengan url.
- Jangan pilih video berumur bertahun-tahun kecuali lagi viral ulang minggu ini (sebutkan di why_hot).
- Hindari konten NSFW / clickbait kosong tanpa substansi untuk niche.`

	user := "Cari 1 video YouTube yang lagi rame untuk niche ini:\n" + string(payload)

	var content string
	var usage *TokenUsage
	var err error
	switch c.provider {
	case "gemini", "google":
		content, usage, err = c.chatGeminiSearch(system, user)
	default:
		return nil, fmt.Errorf("kategori YouTube → utas butuh AI_PROVIDER=gemini (Google Search grounding)")
	}
	if err != nil {
		return nil, err
	}
	if c.quota != nil {
		c.quota.record(usage)
	}

	raw := extractJSON(content)
	var src YouTubeSource
	if err := json.Unmarshal([]byte(raw), &src); err != nil {
		return nil, fmt.Errorf("gagal parse hasil cari YouTube: %w", err)
	}
	id := ParseYouTubeVideoID(src.VideoID)
	if id == "" {
		id = ParseYouTubeVideoID(src.URL)
	}
	if id == "" {
		return nil, fmt.Errorf("Gemini tidak mengembalikan video YouTube yang valid — coba lagi")
	}
	src.VideoID = id
	src.URL = YouTubeWatchURL(id)
	src.Title = strings.TrimSpace(src.Title)
	src.Channel = strings.TrimSpace(src.Channel)
	src.WhyHot = strings.TrimSpace(src.WhyHot)
	if src.Window != "today" && src.Window != "week" {
		src.Window = "week"
	}
	return &src, nil
}

// SummarizeYouTubeVideo asks Gemini to digest a public YouTube URL (video understanding).
func (c *Client) SummarizeYouTubeVideo(watchURL string) (string, error) {
	watchURL = strings.TrimSpace(watchURL)
	if watchURL == "" {
		return "", fmt.Errorf("url youtube kosong")
	}
	if c.provider != "gemini" && c.provider != "google" {
		return "", nil
	}
	system := `Ringkas video YouTube untuk bahan utas Threads.
Fokus: klaim utama, kontradiksi/insight, angka/fakta spesifik, angle yang bisa viral.
Jawab teks biasa 5-10 kalimat (bukan JSON). Jangan transcript penuh.`
	user := "Analisis video ini sebagai bahan konten niche (bukan review spoiler panjang)."
	content, usage, err := c.chatGeminiYouTube(system, user, watchURL)
	if err != nil {
		return "", err
	}
	if c.quota != nil {
		c.quota.record(usage)
	}
	return strings.TrimSpace(content), nil
}

// MirrorYouTubeThumbnail downloads YT thumb, saves under media/thumbs, returns public or relative URL.
func MirrorYouTubeThumbnail(videoID, publicBase string) (string, error) {
	id := ParseYouTubeVideoID(videoID)
	if id == "" {
		return "", fmt.Errorf("video_id tidak valid")
	}
	hc := &http.Client{Timeout: 45 * time.Second}
	var body []byte
	var lastErr error
	for _, src := range YouTubeThumbCandidates(id) {
		req, err := http.NewRequest(http.MethodGet, src, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "lazybussiness-yt-thumb/1.0")
		res, err := hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		res.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if res.StatusCode >= 400 || len(raw) < 2000 {
			lastErr = fmt.Errorf("HTTP %d dari %s", res.StatusCode, src)
			continue
		}
		// maxresdefault kadang 120x90 placeholder abu-abu — skip sangat kecil
		if len(raw) < 8000 && strings.Contains(src, "maxresdefault") {
			lastErr = fmt.Errorf("placeholder maxres")
			continue
		}
		body = raw
		break
	}
	if len(body) == 0 {
		if lastErr != nil {
			return "", fmt.Errorf("gagal unduh thumb YouTube: %w", lastErr)
		}
		return "", fmt.Errorf("gagal unduh thumb YouTube")
	}

	dir := DefaultThumbMediaDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("yt-%s-%d.jpg", id, time.Now().UnixNano()%100000)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	rel := "/media/thumbs/" + name
	publicBase = strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if publicBase != "" {
		return publicBase + rel, nil
	}
	return rel, nil
}
