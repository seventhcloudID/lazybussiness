package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	coverfonts "threads-dashboard/internal/lazy/fonts"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Output thumbnail 4:3 yang cukup untuk Threads — tidak perlu resolusi raksasa.
const (
	thumbOutW = 1024
	thumbOutH = 768 // 4:3 Threads
	igOutW    = 1080
	igOutH    = 1350 // 4:5 Instagram feed
)

// ThumbnailClient calls OpenAI-compatible Images API for Threads utas thumbnails.
// Combo chat (GPTPlus) di /images/generations di-route ke Codex dan gagal —
// sama seperti laciguru: map ke model gambar ChatGPT di gateway.
const (
	defaultImageModel  = "cx/gpt-5.5-image"
	fallbackImageModel = "gemini/gemini-2.5-flash-image"
)

type ThumbnailClient struct {
	apiKeys []string
	baseURL string
	model   string
	size    string
	quality string
	http    *http.Client
	keyMu   sync.Mutex
	keyIdx  int
}

func NewThumbnailFromEnv() *ThumbnailClient {
	keys := collectThumbnailKeys()
	base := strings.TrimRight(env("OPENAI_BASE_URL", env("AI_BASE_URL", "https://api.openai.com")), "/")
	raw := strings.TrimSpace(env("OPENAI_IMAGE_MODEL", env("LAZY_THUMB_MODEL", env("AI_MODEL", ""))))
	return &ThumbnailClient{
		apiKeys: keys,
		baseURL: base,
		model:   resolveImageModel(raw),
		size:    env("OPENAI_IMAGE_SIZE", "1024x768"),
		quality: env("OPENAI_IMAGE_QUALITY", "high"),
		http:    &http.Client{Timeout: 180 * time.Second},
	}
}

// resolveImageModel: GPTPlus/tiga-awan = chat combo → model gambar gateway.
func resolveImageModel(requested string) string {
	m := strings.TrimSpace(requested)
	if m == "" || isChatComboModel(m) {
		return defaultImageModel
	}
	return m
}

func isChatComboModel(m string) bool {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "gptplus", "tiga-awan":
		return true
	default:
		return false
	}
}

func collectThumbnailKeys() []string {
	// Satu gateway: hanya AI_API_KEY. Tidak pakai key OpenAI resmi / Gemini UI.
	return collectAPIKeysFromEnv()
}

func collectOpenAIKeys() []string {
	return collectThumbnailKeys()
}

func (c *ThumbnailClient) reloadKeys() {
	if c == nil {
		return
	}
	c.keyMu.Lock()
	c.apiKeys = collectOpenAIKeys()
	c.keyIdx = 0
	c.keyMu.Unlock()
}

// ApplyStoredOpenAIKeys menyimpan key dari UI lalu reload.
func (c *ThumbnailClient) ApplyStoredOpenAIKeys(keys []string) error {
	if c == nil {
		return fmt.Errorf("thumbnail client nil")
	}
	if err := saveStoredOpenAIKeys(keys); err != nil {
		return err
	}
	c.reloadKeys()
	return nil
}

// ClearStoredOpenAIKeys menghapus key UI; key .env tetap dipakai.
func (c *ThumbnailClient) ClearStoredOpenAIKeys() error {
	if c == nil {
		return fmt.Errorf("thumbnail client nil")
	}
	if err := clearStoredOpenAIKeys(); err != nil {
		return err
	}
	c.reloadKeys()
	return nil
}

func (c *ThumbnailClient) KeysStatus() map[string]any {
	if c == nil {
		return map[string]any{"enabled": false}
	}
	c.keyMu.Lock()
	n := len(c.apiKeys)
	masked := make([]string, 0, n)
	for _, k := range c.apiKeys {
		masked = append(masked, maskKey(k))
	}
	c.keyMu.Unlock()
	stored := loadStoredOpenAIKeys()
	envN := len(collectAPIKeysFromEnv())
	return map[string]any{
		"enabled":     n > 0,
		"provider":    "openai",
		"model":       c.Model(),
		"total":       n,
		"store_count": len(stored),
		"env_count":   envN,
		"key_hint":    firstKeyHint(c),
		"masked":      masked,
		"note":        "Thumbnail memakai gateway AI yang sama (AI_BASE_URL + AI_API_KEY).",
	}
}

func firstKeyHint(c *ThumbnailClient) string {
	if c == nil {
		return ""
	}
	c.keyMu.Lock()
	defer c.keyMu.Unlock()
	if len(c.apiKeys) == 0 {
		return ""
	}
	return maskKey(c.apiKeys[0])
}

func (c *ThumbnailClient) Enabled() bool {
	return c != nil && len(c.apiKeys) > 0 && strings.TrimSpace(c.model) != ""
}

func (c *ThumbnailClient) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

func (c *ThumbnailClient) Size() string {
	if c == nil {
		return ""
	}
	return c.size
}

func (c *ThumbnailClient) Quality() string {
	if c == nil {
		return ""
	}
	return c.quality
}

func (c *ThumbnailClient) Defaults() map[string]any {
	lazyModel := resolveImageModel(env("LAZY_THUMB_MODEL", env("OPENAI_IMAGE_MODEL", env("AI_MODEL", ""))))
	lazySize := "1024x768"
	lazyQuality := env("LAZY_THUMB_QUALITY", "high")
	return map[string]any{
		"enabled":  c.Enabled(),
		"provider": "openai",
		"model":    c.Model(),
		"size":     c.Size(),
		"quality":  c.Quality(),
		"preset": map[string]any{
			"model":        lazyModel,
			"size":         lazySize,
			"quality":      lazyQuality,
			"aspect_ratio": "4:3",
			"crop_4_3":     true, // normalisasi output ke 1024×768 (letterbox jika perlu)
		},
		"models": []string{
			defaultImageModel,
			fallbackImageModel,
		},
		"sizes": []string{
			"1024x768",
			"1536x1024",
			"1024x1024",
		},
		"qualities":   []string{"low", "medium", "high", "auto"},
		"output_size": fmt.Sprintf("%dx%d", thumbOutW, thumbOutH),
		"formats": []map[string]any{
			{"id": "threads", "label": "Threads 4:3", "aspect_ratio": "4:3", "output_size": fmt.Sprintf("%dx%d", thumbOutW, thumbOutH)},
			{"id": "instagram", "label": "Instagram 4:5", "aspect_ratio": "4:5", "output_size": fmt.Sprintf("%dx%d", igOutW, igOutH)},
		},
		"prompt_template":  ThumbPromptTemplate,
		"prompt_header":    ThumbPromptHeader,
		"prompt_rules":     ThumbPromptRules,
		"prompt_rules_ref": ThumbPromptRulesRef,
	}
}

// ThumbnailRequest allows per-call overrides (lab / A-B testing).
type ThumbnailRequest struct {
	Hook            string `json:"hook"`
	Text            string `json:"text"`
	Model           string `json:"model"`
	Size            string `json:"size"`
	Quality         string `json:"quality"`
	Extra           string `json:"extra"`           // catatan tambahan ke prompt
	Crop43          *bool  `json:"crop_4_3"`        // default true — normalisasi rasio output
	AspectRatio     string `json:"aspect_ratio"`    // "4:3" (Threads) | "4:5" (Instagram)
	CustomOnly      bool   `json:"custom_only"`     // pakai Extra sebagai prompt penuh (ignore template)
	ReferenceImage  string `json:"reference_image"` // data URL — match gaya & layout referensi
	Freeform        bool   `json:"freeform"`        // chat/DALL·E — jangan pakai aturan thumbnail Threads
	OverlayPanel    bool   `json:"overlay_white_panel"`
	OverlayTitle    string `json:"overlay_title,omitempty"`
	OverlayHandle   string `json:"overlay_handle,omitempty"`
	OverlayCTA      string `json:"overlay_cta,omitempty"`
	OverlayTemplate string `json:"overlay_template,omitempty"`
}

const thumbnailDeviceGeometryGuard = `DEVICE GEOMETRY — MANDATORY:
- Do not add a phone or laptop unless it is essential to the scene. Use at most ONE primary electronic device: one phone OR one laptop.
- A screen is a single flat surface physically enclosed inside the device bezel/frame. It must share the exact perspective, orientation, and occlusion of the opaque device body.
- Never create a detached, floating, duplicated, transparent, or see-through screen; never place a screen panel behind a phone; never let a display pass through the body; never create a broken bezel, warped hinge, disconnected keyboard, or extra display plate.
- Keep the screen dark, blank, or softly reflective with no readable UI, text, dashboard, spreadsheet, chat, cards, icons, or logos.
- Hands, device, table, hinge, keyboard, and screen must overlap in physically correct depth order. If coherent device geometry is uncertain, omit the device and use another relevant physical prop.`

func appendThumbnailDeviceGeometryGuard(prompt string) string {
	if strings.Contains(prompt, "DEVICE GEOMETRY — MANDATORY") {
		return prompt
	}
	return strings.TrimSpace(prompt) + "\n\n" + thumbnailDeviceGeometryGuard
}

const coverBackgroundOverlayGuard = `OUTPUT RULE — OVERRIDES ALL OTHER LAYOUT DIRECTION:
The app renders a white panel and title AFTER generation. Output PHOTO BACKGROUND ONLY.
Do NOT draw any text, letters, numbers, logos, headlines, yellow keywords, panels, cards, borders, shapes, shadows, or graphic overlays on the image.`

func appendCoverBackgroundOverlayGuard(prompt string) string {
	low := strings.ToLower(prompt)
	if strings.Contains(low, "photo background only") || strings.Contains(prompt, "FOTO LATAR") {
		return prompt
	}
	if strings.Contains(prompt, coverBackgroundOverlayGuard) {
		return prompt
	}
	return strings.TrimSpace(prompt) + "\n\n" + coverBackgroundOverlayGuard
}

// BuildCoverBackgroundPrompt asks the image model for a photo-only 4:5 background.
// Text and the Edge Clean panel are rendered in code via OverlayPanel.
func BuildCoverBackgroundPrompt(hook, coverBrief string) string {
	hook = strings.TrimSpace(hook)
	if hook == "" {
		hook = "(hook kosong)"
	}
	if len(hook) > 2000 {
		hook = hook[:2000] + "…"
	}
	brief := strings.TrimSpace(coverBrief)
	if brief == "" {
		brief = "(tentukan adegan editorial aktif dari hook; jangan generik)"
	}
	return fmt.Sprintf(`Buat FOTO LATAR editorial portrait 4:5, 1080×1350 untuk cover konten. Aplikasi menambahkan panel putih dan teks SETELAH gambar dibuat — model HANYA menghasilkan foto tanpa tulisan.

Hook/topik: %s
Ide adegan: %s

WAJIB:
- Satu foto editorial realistis dengan momen aktif yang relevan hook.
- Focal point di area atas/tengah; area bawah ~45%% bebas wajah, tangan, dan objek penting (akan ditutup panel).
- Bright editorial daylight, exposure terang, white balance netral, warna natural hidup.

LARANGAN KERAS:
- Teks, huruf, angka, logo, watermark, headline, kata kuning, panel, kartu, border, shape, UI, kolase, atau overlay grafis.
- Orang pasif menatap layar tanpa emosi/aksi kuat.
- Layar perangkat berisi UI/teks; layar melayang, duplikat, atau transparan.
- Sepia, vintage, murky, underexposed, faded, atau vignette gelap.

%s`, hook, brief, coverBackgroundOverlayGuard)
}

func (c *ThumbnailClient) currentKey() string {
	c.keyMu.Lock()
	defer c.keyMu.Unlock()
	if len(c.apiKeys) == 0 {
		return ""
	}
	if c.keyIdx < 0 || c.keyIdx >= len(c.apiKeys) {
		c.keyIdx = 0
	}
	return c.apiKeys[c.keyIdx]
}

func (c *ThumbnailClient) rotateKey() (string, bool) {
	c.keyMu.Lock()
	defer c.keyMu.Unlock()
	if len(c.apiKeys) <= 1 {
		return "", false
	}
	c.keyIdx = (c.keyIdx + 1) % len(c.apiKeys)
	return c.apiKeys[c.keyIdx], true
}

// Header tetap; {{hook}} diganti konteks bagian 1.
const ThumbPromptHeader = `buatkan thumbnail Threads landscape 4:3 (lebar > tinggi), kanvas penuh tanpa letterbox.
konteks:
{{hook}}

`

const ThumbPromptHeaderIG = `buatkan gambar feed Instagram portrait 4:5 (tinggi > lebar, mirip 1080x1350), isi penuh frame.
konteks:
{{hook}}

`

// Aturan desain — bagian yang boleh diedit di lab.
const ThumbPromptRules = `desain poster rapi: hierarki jelas, spacing sadar, visual-first.
warna & pencahayaan wajib: bright editorial daylight, exposure terang dan bersih, white balance netral, warna natural hidup, skin tone sehat, putih tetap putih. Oranye/kuning hanya aksen brand/teks, bukan color cast seluruh gambar. Detail shadow tetap terlihat dan kontras tegas tanpa black crush.
safe box wajib: semua judul dan CTA harus utuh; jangan tempatkan teks di 18% area terbawah dan jangan ada huruf melewati batas kanvas.
teks: 1 klaim dari hook. Tumpukan judul cover di optical center (~30% dari atas), BUKAN nempel langit atas.
skala wajib: baris setup sedang + 1 KATA subjek kuning 2.5–3× lebih besar + 1 baris janji kecil (bukan judul ketiga).
margin aman ~8% semua sisi; judul jangan nutup kepala/objek utama; shadow tipis bukan halo tebal.
jangan menjiplak frasa contoh — turunkan judul dari hook ini.
1 subjek visual yang membuktikan klaim yang sama.
dont:
- jangan sepia, vintage/retro, brown atau olive color grading, murky, gloomy, underexposed, faded, washed-out, desaturated, atau vignette gelap
- jangan 3 baris sama besar / kata kuning berupa frasa sambung
- jangan judul samar yang tidak menyebut topik
- jangan menjiplak contoh frasa dari prompt
- jangan menaruh judul mepet ke tepi atas
- jangan CTA mepet pojok atau beda baseline
- jangan UI kecil di tepi, ikon app, kartu "Sponsored", watermark, teks mikro tak terbaca
- jangan tangan/jari/wajah cacat, blob, artefak, teks gibberish
- jangan menulis ulang hook/narasi atau banyak label/bullet
- jangan potong elemen penting di pinggir`

// Aturan saat ada gambar referensi — gaya mirip, konten/pose ikut hook.
const ThumbPromptRulesRef = `pakai gambar referensi sebagai template gaya: salin palet warna, gaya ilustrasi/ikon, tipografi, spacing, latar, dan tingkat flat/detail.
komposisi boleh mirip kerangka umum, TAPI sesuaikan subjek, pose, aksi, ekspresi, dan metafora visual agar relevan dengan hook (jangan copy pose/objek referensi kalau tidak cocok konteks).
teks di gambar: satu judul yang menyebut TOPIK hook (bukan puisi/teka-teki); gaya huruf mengikuti referensi, tajam & terbaca.
jangan menjiplak frasa contoh dari additional direction — turunkan judul dari hook ini.
penting: safe area kuat (~10–12% dari tepi, terutama atas); komposisi bersih 1 fokus utama.
dont:
- jangan judul samar yang tidak menyebut topik
- jangan menjiplak contoh frasa dari prompt
- jangan menaruh judul mepet ke tepi atas
- jangan UI kecil di tepi, ikon app, kartu iklan mikro, teks gibberish
- jangan tangan/jari/wajah cacat atau artefak blur aneh
- jangan 100% menjiplak adegan/pose/objek dari referensi
- jangan ganti mood/palet/style tipografi secara drastis
- jangan menulis ulang hook/narasi sebagai paragraf`

// Template penuh = header + rules.
const ThumbPromptTemplate = ThumbPromptHeader + ThumbPromptRules

// BuildThumbnailPrompt — brief tetap dari user, tanpa tambahan aturan.
// Konteks = hook bagian 1 utas.
func BuildThumbnailPrompt(hook string) string {
	return BuildThumbnailPromptAspect(hook, "4:3")
}

func BuildThumbnailPromptAspect(hook, aspect string) string {
	hook = strings.TrimSpace(hook)
	if hook == "" {
		hook = "(hook kosong)"
	}
	if len(hook) > 2000 {
		hook = hook[:2000] + "…"
	}
	header := ThumbPromptHeader
	if normalizeAspectRatio(aspect) == "4:5" {
		header = ThumbPromptHeaderIG
	}
	return strings.ReplaceAll(header+ThumbPromptRules, "{{hook}}", hook)
}

// BuildThumbnailPromptWithRules sama BuildThumbnailPrompt tapi rules custom.
func BuildThumbnailPromptWithRules(hook, rules string) string {
	hook = strings.TrimSpace(hook)
	if hook == "" {
		hook = "(hook kosong)"
	}
	if len(hook) > 2000 {
		hook = hook[:2000] + "…"
	}
	rules = strings.TrimSpace(rules)
	if rules == "" {
		rules = ThumbPromptRules
	}
	return strings.ReplaceAll(ThumbPromptHeader, "{{hook}}", hook) + rules
}

type ThumbnailResult struct {
	PNG      []byte
	Prompt   string
	Model    string
	Size     string
	Width    int
	Height   int
	Provider string
}

func (c *ThumbnailClient) GenerateFreeform(prompt, referenceImage string) (*ThumbnailResult, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt gambar kosong")
	}
	return c.GenerateRequest(ThumbnailRequest{
		Extra:          prompt,
		CustomOnly:     true,
		Freeform:       true,
		Size:           "1024x1024",
		Quality:        "medium",
		ReferenceImage: referenceImage,
	})
}

func (c *ThumbnailClient) GenerateRequest(req ThumbnailRequest) (*ThumbnailResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("thumbnail belum dikonfigurasi — set AI_API_KEY")
	}
	hook := strings.TrimSpace(req.Hook)
	if hook == "" {
		hook = strings.TrimSpace(req.Text)
	}
	if hook == "" && !req.CustomOnly {
		return nil, fmt.Errorf("hook utas (bagian 1) wajib diisi")
	}

	model := resolveImageModel(strings.TrimSpace(req.Model))
	if model == "" {
		model = c.model
	}
	if model == "" {
		model = defaultImageModel
	}
	size := normalizeImageSize(strings.TrimSpace(req.Size))
	if size == "" {
		if req.Freeform {
			size = "1024x1024"
		} else {
			size = normalizeImageSize(c.size)
		}
	}
	quality := strings.TrimSpace(req.Quality)
	if quality == "" {
		quality = c.quality
	}

	var prompt string
	extra := strings.TrimSpace(req.Extra)
	if req.CustomOnly {
		prompt = extra
		if prompt == "" {
			return nil, fmt.Errorf("prompt custom kosong")
		}
	} else {
		prompt = BuildThumbnailPromptAspect(hook, req.AspectRatio)
		if extra != "" {
			prompt += "\n\nAdditional direction from editor:\n" + extra
		}
	}
	if !req.Freeform {
		prompt = appendThumbnailDeviceGeometryGuard(prompt)
	}
	if req.OverlayPanel && !req.Freeform {
		prompt = appendCoverBackgroundOverlayGuard(prompt)
	}

	refURL, err := normalizeReferenceImage(req.ReferenceImage)
	if err != nil {
		return nil, err
	}
	aspect := normalizeAspectRatio(req.AspectRatio)
	low := strings.ToLower(prompt)
	if !req.Freeform && !req.CustomOnly {
		if refURL != "" && !strings.Contains(low, "template gaya") && !strings.Contains(low, "sesuaikan subjek") {
			prompt += "\n\nGambar referensi = template GAYA saja (warna, tipografi, style visual). Sesuaikan pose/aksi/subjek/metafora dengan hook — jangan copy 100% adegan referensi."
			low = strings.ToLower(prompt)
		}
		if aspect == "4:5" && !strings.Contains(low, "4:5") && !strings.Contains(low, "portrait") {
			prompt += "\n\nFormat wajib: portrait 4:5 (tinggi lebih besar dari lebar), isi penuh frame."
		}
		if !strings.Contains(low, "3–6") && !strings.Contains(low, "3-6") && !strings.Contains(low, "maksimal 3") {
			prompt += "\n\nTeks di gambar: satu judul yang menyebut TOPIK hook (bukan puisi/teka-teki). Jangan tulis ulang narasi. Jangan menjiplak contoh frasa."
			low = strings.ToLower(prompt)
		}
		if !strings.Contains(low, "10–12%") && !strings.Contains(low, "10-12%") && !strings.Contains(low, "mepet") {
			prompt += "\n\nJaga safe area: judul/teks jangan mepet tepi atas — sisakan padding ~10–12% dari atas & tepi."
			low = strings.ToLower(prompt)
		}
		if !strings.Contains(low, "cacat") && !strings.Contains(low, "gibberish") && !strings.Contains(low, "teks mikro") {
			prompt += "\n\nKualitas: komposisi bersih (1 fokus), tanpa UI kecil di tepi, tanpa teks mikro/gibberish, tanpa tangan/jari/wajah cacat."
		}
	}

	var images []string
	if refURL != "" {
		images = []string{refURL}
	}
	rawPNG, err := c.generatePNG(prompt, model, size, quality, images, aspect)
	if err != nil {
		return nil, err
	}

	crop := true
	if req.Freeform {
		crop = false
	}
	if req.Crop43 != nil {
		crop = *req.Crop43
	}

	outPNG, w, h, err := normalizeThumbnailCanvas(rawPNG, aspect, crop)
	if err != nil {
		return nil, err
	}
	if req.OverlayPanel {
		rawTitle := strings.TrimSpace(req.OverlayTitle)
		if rawTitle == "" {
			rawTitle = hook
		}
		// The generated cover follows the current brief. Account-specific series
		// text may still be written explicitly in the brief, but is never injected.
		title := coverHeadlineForHandle(compactCoverHeadline(rawTitle), req.OverlayHandle)
		outPNG, err = renderCodedCover(outPNG, title, req.OverlayHandle, req.OverlayCTA, req.OverlayTemplate)
		if err != nil {
			return nil, fmt.Errorf("render white panel: %w", err)
		}
	}

	return &ThumbnailResult{
		PNG:      outPNG,
		Prompt:   prompt,
		Model:    model,
		Size:     fmt.Sprintf("%dx%d", w, h),
		Width:    w,
		Height:   h,
		Provider: "openai",
	}, nil
}

const bimoseptCoverSeries = "1 hari 1 tips cuan dari internet."

var bimoseptSeriesPrefixRE = regexp.MustCompile(`(?i)^\s*(?:serial\s+)?1\s+hari\s+1\s+tips\s+(?:cari\s+)?cuan\s+dari\s+internet\s*[.!?:-]*\s*`)

func coverHeadlineForHandle(title, handle string) string {
	_ = handle
	return strings.TrimSpace(title)
}

func renderWhiteTextPanel(rawPNG []byte, title, handle, cta string) ([]byte, error) {
	return renderCodedCover(rawPNG, title, handle, cta, "inset-editorial")
}

type codedCoverTemplate struct {
	ID                 string
	PanelX, PanelY     float64
	PanelRight, PanelB float64
	InsetX, TopPad     float64
	TitleScale         float64
}

const coverEmphasisScale = 1.0
const coverSeriesScale = 1.22

var coverTextColor = color.RGBA{R: 15, G: 39, B: 71, A: 255}
var coverSeriesColor = color.RGBA{R: 37, G: 88, B: 218, A: 255}
var coverDividerColor = color.RGBA{R: 188, G: 201, B: 218, A: 255}

func codedCoverTemplateFor(id string) codedCoverTemplate {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "edge-clean":
		return codedCoverTemplate{ID: "edge-clean", PanelX: 0, PanelY: 0.585, PanelRight: 1, PanelB: 1, InsetX: 0.07, TopPad: 0.038, TitleScale: 0.079}
	case "split-roomy":
		return codedCoverTemplate{ID: "split-roomy", PanelX: 0, PanelY: 0.54, PanelRight: 1, PanelB: 1, InsetX: 0.075, TopPad: 0.036, TitleScale: 0.077}
	case "left-cut":
		return codedCoverTemplate{ID: "left-cut", PanelX: 0, PanelY: 0.545, PanelRight: 0.84, PanelB: 0.955, InsetX: 0.06, TopPad: 0.038, TitleScale: 0.074}
	case "right-cut":
		return codedCoverTemplate{ID: "right-cut", PanelX: 0.16, PanelY: 0.545, PanelRight: 1, PanelB: 0.955, InsetX: 0.06, TopPad: 0.038, TitleScale: 0.074}
	case "low-editorial":
		return codedCoverTemplate{ID: "low-editorial", PanelX: 0.045, PanelY: 0.61, PanelRight: 0.955, PanelB: 0.975, InsetX: 0.055, TopPad: 0.036, TitleScale: 0.071}
	default:
		return codedCoverTemplate{ID: "inset-editorial", PanelX: 0.055, PanelY: 0.565, PanelRight: 0.945, PanelB: 0.948, InsetX: 0.055, TopPad: 0.038, TitleScale: 0.076}
	}
}

func renderCodedCover(rawPNG []byte, title, handle, cta, templateID string) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(rawPNG))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 320 || h < 320 {
		return nil, fmt.Errorf("canvas terlalu kecil: %dx%d", w, h)
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)

	tpl := codedCoverTemplateFor(templateID)
	panelYRatio := tpl.PanelY
	if wordCount := len(strings.Fields(title)); wordCount > 0 && wordCount <= 7 {
		panelYRatio += 0.045
	}
	panelX := int(float64(w) * tpl.PanelX)
	panelY := int(float64(h) * panelYRatio)
	panelR := int(float64(w) * tpl.PanelRight)
	panelB := int(float64(h) * tpl.PanelB)
	panel := image.Rect(panelX, panelY, panelR, panelB)
	draw.Draw(out, panel, image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}), image.Point{}, draw.Src)

	regularFont, err := opentype.Parse(coverfonts.Regular())
	if err != nil {
		return nil, err
	}
	boldFont, err := opentype.Parse(coverfonts.SemiBold())
	if err != nil {
		return nil, err
	}
	makeFace := func(ft *opentype.Font, size float64) (font.Face, error) {
		return opentype.NewFace(ft, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	}

	inset := int(float64(w) * tpl.InsetX)
	x := panelX + inset
	maxTextW := panel.Dx() - 2*inset
	top := panelY + int(float64(h)*tpl.TopPad)
	bottomPad := int(float64(h) * 0.022)
	black := image.NewUniform(coverTextColor)
	emphasisColor := image.NewUniform(coverTextColor)
	seriesColor := image.NewUniform(coverSeriesColor)

	handle = strings.TrimSpace(strings.TrimPrefix(handle, "@"))
	if handle != "" {
		face, faceErr := makeFace(regularFont, float64(w)*0.026)
		if faceErr != nil {
			return nil, faceErr
		}
		d := font.Drawer{Dst: out, Src: black, Face: face, Dot: fixed.P(x, top+face.Metrics().Ascent.Ceil())}
		d.DrawString(handle)
		dividerY := top + face.Metrics().Height.Ceil() + int(float64(h)*0.012)
		dividerH := max(2, int(float64(h)*0.0015))
		draw.Draw(out, image.Rect(x, dividerY, x+maxTextW, dividerY+dividerH), image.NewUniform(coverDividerColor), image.Point{}, draw.Src)
		top = dividerY + dividerH + int(float64(h)*0.021)
		_ = face.Close()
	}

	cta = strings.ReplaceAll(strings.TrimSpace(cta), "→", "->")
	ctaHeight := 0
	var ctaFace font.Face
	if cta != "" {
		ctaFace, err = makeFace(regularFont, float64(w)*0.023)
		if err != nil {
			return nil, err
		}
		ctaHeight = ctaFace.Metrics().Height.Ceil() + int(float64(h)*0.038)
		defer ctaFace.Close()
	}

	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	if title == "" {
		title = "Untitled"
	}
	availableH := panelB - bottomPad - ctaHeight - top
	var titleRegularFace font.Face
	var titleBoldFace font.Face
	var titleSeriesFace font.Face
	var lines []coverTextLine
	titleWords := splitCoverEmphasis(title)
	titleScale := tpl.TitleScale
	if titleScale <= 0 {
		titleScale = 0.07
	}
	minTitleSize := float64(w) * 0.034
	for size := float64(w) * titleScale; size >= minTitleSize; size -= 2 {
		regularFace, faceErr := makeFace(boldFont, size)
		if faceErr != nil {
			return nil, faceErr
		}
		boldFace, faceErr := makeFace(boldFont, size*coverEmphasisScale)
		if faceErr != nil {
			_ = regularFace.Close()
			return nil, faceErr
		}
		seriesFace, faceErr := makeFace(boldFont, size*coverSeriesScale)
		if faceErr != nil {
			_ = regularFace.Close()
			_ = boldFace.Close()
			return nil, faceErr
		}
		candidate := wrapCoverText(regularFace, boldFace, seriesFace, titleWords, maxTextW)
		if coverLinesHeight(regularFace, boldFace, seriesFace, candidate) <= availableH {
			titleRegularFace, titleBoldFace, titleSeriesFace, lines = regularFace, boldFace, seriesFace, candidate
			break
		}
		_ = regularFace.Close()
		_ = boldFace.Close()
		_ = seriesFace.Close()
	}
	if titleRegularFace == nil {
		titleRegularFace, err = makeFace(boldFont, minTitleSize)
		if err != nil {
			return nil, err
		}
		titleBoldFace, err = makeFace(boldFont, minTitleSize*coverEmphasisScale)
		if err != nil {
			_ = titleRegularFace.Close()
			return nil, err
		}
		titleSeriesFace, err = makeFace(boldFont, minTitleSize*coverSeriesScale)
		if err != nil {
			_ = titleRegularFace.Close()
			_ = titleBoldFace.Close()
			return nil, err
		}
		lines = wrapCoverText(titleRegularFace, titleBoldFace, titleSeriesFace, titleWords, maxTextW)
	}
	defer titleRegularFace.Close()
	defer titleBoldFace.Close()
	defer titleSeriesFace.Close()
	yTop := top
	emphasisStroke := max(1, w/1080)
	for _, line := range lines {
		y := yTop + coverLineAscent(titleRegularFace, titleBoldFace, titleSeriesFace, line)
		cursorX := x
		for i, word := range line {
			face := titleRegularFace
			ink := black
			if word.Bold {
				face = titleBoldFace
				ink = emphasisColor
			}
			if word.Accent {
				face = titleSeriesFace
				ink = seriesColor
			}
			if i > 0 {
				cursorX += font.MeasureString(titleRegularFace, " ").Ceil()
			}
			stroke := 0
			if word.Bold {
				stroke = emphasisStroke
			}
			displayText := coverDisplayWord(word)
			for offset := 0; offset <= stroke; offset++ {
				d := font.Drawer{Dst: out, Src: ink, Face: face, Dot: fixed.P(cursorX+offset, y)}
				d.DrawString(displayText)
			}
			cursorX += font.MeasureString(face, displayText).Ceil()
		}
		yTop += coverLineHeight(titleRegularFace, titleBoldFace, titleSeriesFace, line)
	}

	if ctaFace != nil {
		baseline := panelB - bottomPad - ctaFace.Metrics().Descent.Ceil()
		d := font.Drawer{Dst: out, Src: black, Face: ctaFace, Dot: fixed.P(x, baseline)}
		d.DrawString(cta)
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, out); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

type coverTextWord struct {
	Text       string
	Bold       bool
	Accent     bool
	BreakAfter bool
}

type coverTextLine []coverTextWord

func splitCoverEmphasis(text string) []coverTextWord {
	plain := strings.Fields(text)
	if len(plain) == 0 {
		return nil
	}
	boldFrom := len(plain)
	for i := len(plain) - 2; i >= 0; i-- {
		if strings.ContainsAny(plain[i][len(plain[i])-1:], ".!?:") {
			boldFrom = i + 1
			break
		}
	}
	if boldFrom == len(plain)-1 && len(plain) >= 3 {
		boldFrom--
	}
	if boldFrom == len(plain) {
		switch {
		case len(plain) >= 6:
			boldFrom = len(plain) - (len(plain)+2)/3
		case len(plain) >= 3:
			boldFrom = len(plain) - 2
		default:
			boldFrom = 0
		}
	}
	seriesEnd := -1
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), strings.ToLower(bimoseptCoverSeries)) {
		for i, word := range plain {
			if strings.ContainsAny(word[len(word)-1:], ".!?:") {
				seriesEnd = i
				break
			}
		}
	}
	words := make([]coverTextWord, len(plain))
	for i, word := range plain {
		clean := strings.Trim(word, `.,!?;:"'()[]`)
		isKeyword := len([]rune(clean)) >= 3 && strings.ToUpper(clean) == clean && strings.ToLower(clean) != clean
		inSeries := seriesEnd >= 0 && i <= seriesEnd
		words[i] = coverTextWord{Text: word, Bold: inSeries || i >= boldFrom || isKeyword, Accent: inSeries, BreakAfter: i == seriesEnd}
	}
	return words
}

func coverWordFace(regularFace, boldFace, seriesFace font.Face, word coverTextWord) font.Face {
	if word.Accent && seriesFace != nil {
		return seriesFace
	}
	if word.Bold {
		return boldFace
	}
	return regularFace
}

func coverTextWidth(regularFace, boldFace, seriesFace font.Face, words coverTextLine) int {
	width := 0
	for i, word := range words {
		if i > 0 {
			width += font.MeasureString(regularFace, " ").Ceil()
		}
		face := coverWordFace(regularFace, boldFace, seriesFace, word)
		width += font.MeasureString(face, coverDisplayWord(word)).Ceil()
	}
	return width
}

func coverLineAscent(regularFace, boldFace, seriesFace font.Face, line coverTextLine) int {
	ascent := 0
	for _, word := range line {
		ascent = max(ascent, coverWordFace(regularFace, boldFace, seriesFace, word).Metrics().Ascent.Ceil())
	}
	return ascent
}

func coverLineHeight(regularFace, boldFace, seriesFace font.Face, line coverTextLine) int {
	height := 0
	for _, word := range line {
		height = max(height, coverWordFace(regularFace, boldFace, seriesFace, word).Metrics().Height.Ceil())
	}
	return int(float64(height) * 1.08)
}

func coverLinesHeight(regularFace, boldFace, seriesFace font.Face, lines []coverTextLine) int {
	height := 0
	for _, line := range lines {
		height += coverLineHeight(regularFace, boldFace, seriesFace, line)
	}
	return height
}

func wrapCoverText(regularFace, boldFace, seriesFace font.Face, words []coverTextWord, maxWidth int) []coverTextLine {
	if len(words) == 0 {
		return nil
	}
	lines := make([]coverTextLine, 0, 4)
	line := coverTextLine{words[0]}
	for _, word := range words[1:] {
		if line[len(line)-1].BreakAfter {
			lines = append(lines, line)
			line = coverTextLine{word}
			continue
		}
		candidate := append(append(coverTextLine{}, line...), word)
		if coverTextWidth(regularFace, boldFace, seriesFace, candidate) <= maxWidth {
			line = candidate
			continue
		}
		lines = append(lines, line)
		line = coverTextLine{word}
	}
	return append(lines, line)
}

func coverDisplayWord(word coverTextWord) string {
	return word.Text
}

func fitCoverLineEllipsis(regularFace, boldFace, seriesFace font.Face, line coverTextLine, maxWidth int) coverTextLine {
	trimmed := append(coverTextLine{}, line...)
	for len(trimmed) > 0 {
		candidate := append(coverTextLine{}, trimmed...)
		candidate[len(candidate)-1].Text += "…"
		if coverTextWidth(regularFace, boldFace, seriesFace, candidate) <= maxWidth {
			return candidate
		}
		trimmed = trimmed[:len(trimmed)-1]
	}
	return coverTextLine{{Text: "…", Bold: true}}
}

func normalizeThumbnailCanvas(rawPNG []byte, aspect string, crop bool) ([]byte, int, int, error) {
	outW, outH := outputSizeForAspect(aspect)
	img, _, err := image.Decode(bytes.NewReader(rawPNG))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if !crop {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, 0, 0, err
		}
		return buf.Bytes(), w, h, nil
	}

	// Image gateways may ignore 1080x1350 and return 1024x1536 (2:3). A
	// cover-crop then removes the bottom of a poster, exactly where generated
	// titles and CTAs tend to live. Preserve the complete portrait artwork and
	// letterbox only the narrow side gutters when the upstream ratio differs.
	if normalizeAspectRatio(aspect) == "4:5" && !sameAspectRatio(w, h, outW, outH) {
		return fitCanvas(rawPNG, outW, outH)
	}
	return coverCanvas(rawPNG, outW, outH)
}

func sameAspectRatio(w, h, targetW, targetH int) bool {
	if w <= 0 || h <= 0 || targetW <= 0 || targetH <= 0 {
		return false
	}
	left := w * targetH
	right := h * targetW
	diff := left - right
	if diff < 0 {
		diff = -diff
	}
	maxValue := left
	if right > maxValue {
		maxValue = right
	}
	return diff*100 <= maxValue // within 1%
}

func (c *ThumbnailClient) generatePNG(prompt, model, size, quality string, images []string, aspect string) ([]byte, error) {
	models := []string{resolveImageModel(model)}
	if models[0] != fallbackImageModel {
		models = append(models, fallbackImageModel)
	}
	attempts := [][]string{images}
	if len(images) == 1 {
		attempts = append(attempts, nil)
	}
	var lastErr error
	for _, m := range models {
		for _, imgs := range attempts {
			png, err := c.generatePNGWithKeys(prompt, m, size, quality, imgs, aspect)
			if err == nil {
				return png, nil
			}
			lastErr = err
			if isWrongImageModelErr(err) {
				break
			}
		}
		if lastErr != nil && !isWrongImageModelErr(lastErr) {
			return nil, lastErr
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("gagal generate thumbnail")
	}
	return nil, lastErr
}

func (c *ThumbnailClient) generatePNGWithKeys(prompt, model, size, quality string, images []string, aspect string) ([]byte, error) {
	var lastErr error
	tried := map[string]bool{}
	for attempt := 0; attempt < len(c.apiKeys); attempt++ {
		key := c.currentKey()
		if key == "" || tried[key] {
			if next, ok := c.rotateKey(); ok {
				key = next
			} else {
				break
			}
		}
		tried[key] = true
		png, err := c.generatePNGOnce(key, prompt, model, size, quality, images, aspect)
		if err == nil {
			return png, nil
		}
		lastErr = err
		if isWrongImageModelErr(err) {
			return nil, err
		}
		if isRetryableImageErr(err) {
			if _, ok := c.rotateKey(); ok {
				continue
			}
		}
		break
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("gagal generate thumbnail")
	}
	return nil, lastErr
}

func isRetryableImageErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "rate") ||
		strings.Contains(s, "429") ||
		strings.Contains(s, "quota") ||
		strings.Contains(s, "insufficient_quota") ||
		strings.Contains(s, "overloaded")
}

func isWrongImageModelErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "codex") ||
		strings.Contains(s, "gpt-5.6-sol") ||
		strings.Contains(s, "not supported") ||
		strings.Contains(s, "does not support") ||
		strings.Contains(s, "model_not_found") ||
		strings.Contains(s, "model not found") ||
		strings.Contains(s, "imageoutput") ||
		strings.Contains(s, "bukan image model")
}

func imagesGenerationsURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/images/generations"
	}
	return base + "/v1/images/generations"
}

func normalizeAspectRatio(ar string) string {
	switch strings.TrimSpace(ar) {
	case "4:5", "4/5", "ig", "instagram":
		return "4:5"
	default:
		return "4:3"
	}
}

func outputSizeForAspect(ar string) (int, int) {
	if normalizeAspectRatio(ar) == "4:5" {
		return igOutW, igOutH
	}
	return thumbOutW, thumbOutH
}

// Gateway ChatGPT image sering mengabaikan size (tetap kotak).
// aspect_ratio yang menentukan landscape/portrait; size hanya hint.
func normalizeImageSize(size string) string {
	size = strings.TrimSpace(size)
	switch size {
	case "1024x1024", "1024x768", "1536x1024", "1024x1536", "1080x1350":
		return size
	default:
		return "1024x1024"
	}
}

// normalizeReferenceImage menerima data URL atau raw base64 → data:image/...;base64,...
func normalizeReferenceImage(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > 6<<20 { // ~6MB string
		return "", fmt.Errorf("gambar referensi terlalu besar (maks ~4MB)")
	}
	if strings.HasPrefix(raw, "data:image/") {
		if !strings.Contains(raw, ";base64,") {
			return "", fmt.Errorf("gambar referensi harus data URL base64")
		}
		return raw, nil
	}
	// raw base64
	return "data:image/png;base64," + raw, nil
}

func (c *ThumbnailClient) generatePNGOnce(apiKey, prompt, model, size, quality string, images []string, aspect string) ([]byte, error) {
	aspect = normalizeAspectRatio(aspect)
	if aspect == "4:5" {
		if size == "" || size == "1024x768" || size == "1536x1024" || size == "1024x1536" {
			size = "1080x1350"
		}
	} else if size == "" || size == "1024x768" {
		size = "1536x1024"
	}
	body := map[string]any{
		"model":           model,
		"prompt":          prompt,
		"n":               1,
		"size":            size,
		"aspect_ratio":    aspect,
		"response_format": "b64_json",
	}
	if n := len(images); n == 1 {
		body["image"] = images[0]
		body["image_detail"] = "high"
	} else if n > 1 {
		body["image"] = images
		body["image_detail"] = "high"
	}
	// dall-e-3 only accepts standard|hd; gpt-image / cx image uses low|medium|high|auto
	q := strings.TrimSpace(quality)
	if q != "" {
		if strings.HasPrefix(model, "dall-e") {
			if q == "high" || q == "hd" {
				body["quality"] = "hd"
			} else {
				body["quality"] = "standard"
			}
		} else if strings.Contains(model, "gpt-image") || strings.HasPrefix(model, "dall-e") {
			body["quality"] = q
		}
		// cx/*-image & gemini flash-image: jangan kirim quality OpenAI-only kalau sering ditolak
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, imagesGenerationsURL(c.baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if res.StatusCode >= 400 {
		msg := strings.TrimSpace(string(raw))
		var er struct {
			Error struct {
				Message string `json:"message"`
				Code    any    `json:"code"`
			} `json:"error"`
			Detail string `json:"detail"`
		}
		if json.Unmarshal(raw, &er) == nil {
			if er.Error.Message != "" {
				msg = er.Error.Message
			} else if er.Detail != "" {
				msg = er.Detail
			}
		}
		low := strings.ToLower(msg)
		if strings.Contains(low, "codex") || strings.Contains(low, "gpt-5.6-sol") ||
			strings.Contains(low, "imageoutput") || strings.Contains(low, "does not support") ||
			strings.Contains(low, "not supported") || strings.Contains(low, "model_not_found") {
			return nil, fmt.Errorf("model %q bukan image model: %s", model, truncate(msg, 220))
		}
		return nil, fmt.Errorf("images API %d (%s): %s", res.StatusCode, model, truncate(msg, 400))
	}

	var out struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse OpenAI images: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("OpenAI images: response kosong")
	}
	if b64 := strings.TrimSpace(out.Data[0].B64JSON); b64 != "" {
		bin, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("decode b64 image: %w", err)
		}
		return bin, nil
	}
	u := strings.TrimSpace(out.Data[0].URL)
	if u == "" {
		return nil, fmt.Errorf("OpenAI images: tidak ada b64_json/url")
	}
	return downloadBytes(c.http, u)
}

func downloadBytes(client *http.Client, rawURL string) ([]byte, error) {
	res, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("download image %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 32<<20))
}

// coverCanvas mengisi penuh kanvas target (scale + center crop) — hasil selalu rasio tepat tanpa letterbox.
func coverCanvas(pngBytes []byte, outW, outH int) ([]byte, int, int, error) {
	if outW < 16 {
		outW = thumbOutW
	}
	if outH < 16 {
		outH = thumbOutH
	}
	img, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 16 || h < 16 {
		return nil, 0, 0, fmt.Errorf("image terlalu kecil")
	}

	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	scale := float64(outW) / float64(w)
	if s := float64(outH) / float64(h); s > scale {
		scale = s
	}
	dw := int(float64(w)*scale + 0.5)
	dh := int(float64(h)*scale + 0.5)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	x0 := (outW - dw) / 2
	y0 := (outH - dh) / 2
	if dh > outH {
		overflow := dh - outH
		if outH > outW {
			// Portrait 4:5: jangan potong zona judul di atas.
			y0 = 0
		} else {
			y0 = -(overflow * 30 / 100)
		}
	}
	target := image.Rect(x0, y0, x0+dw, y0+dh)
	xdraw.CatmullRom.Scale(dst, target, img, b, xdraw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), outW, outH, nil
}

// fitCanvas memuat seluruh gambar tanpa crop. Sisa gutter memakai versi
// full-bleed yang diredupkan agar tidak muncul bar putih mencolok.
func fitCanvas(pngBytes []byte, outW, outH int) ([]byte, int, int, error) {
	if outW < 16 {
		outW = thumbOutW
	}
	if outH < 16 {
		outH = thumbOutH
	}
	img, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 16 || h < 16 {
		return nil, 0, 0, fmt.Errorf("image terlalu kecil")
	}

	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	bgScale := float64(outW) / float64(w)
	if s := float64(outH) / float64(h); s > bgScale {
		bgScale = s
	}
	bgW := int(float64(w)*bgScale + 0.5)
	bgH := int(float64(h)*bgScale + 0.5)
	bgX := (outW - bgW) / 2
	bgY := (outH - bgH) / 2
	xdraw.CatmullRom.Scale(dst, image.Rect(bgX, bgY, bgX+bgW, bgY+bgH), img, b, xdraw.Over, nil)
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.NRGBA{A: 72}}, image.Point{}, draw.Over)

	// Scale contain: muat penuh, jaga aspek, center.
	scale := float64(outW) / float64(w)
	if s := float64(outH) / float64(h); s < scale {
		scale = s
	}
	dw := int(float64(w)*scale + 0.5)
	dh := int(float64(h)*scale + 0.5)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	x0 := (outW - dw) / 2
	y0 := (outH - dh) / 2
	target := image.Rect(x0, y0, x0+dw, y0+dh)
	xdraw.CatmullRom.Scale(dst, target, img, b, xdraw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), outW, outH, nil
}

// SaveThumbnailPNG writes PNG under dir and returns relative filename.
func SaveThumbnailPNG(dir string, pngBytes []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("thumb-%d.png", time.Now().UnixNano())
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pngBytes, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// DefaultThumbMediaDir is where generated Threads thumbnails are stored.
func DefaultThumbMediaDir() string {
	return filepath.Join(".data", "thumbs")
}
