package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
)

// Output thumbnail 4:3 yang cukup untuk Threads — tidak perlu resolusi raksasa.
const (
	thumbOutW = 1024
	thumbOutH = 768 // 4:3
)

// ThumbnailClient calls OpenAI Images API (ChatGPT image models) for Threads utas thumbnails.
// Separate from the Gemini text client.
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
	keys := collectOpenAIKeys()
	return &ThumbnailClient{
		apiKeys: keys,
		baseURL: strings.TrimRight(env("OPENAI_BASE_URL", "https://api.openai.com"), "/"),
		model: env("OPENAI_IMAGE_MODEL", "gpt-image-1"),
		// Request kecil (1024²), lalu crop+scale ke 1024×768 (4:3).
		size:    env("OPENAI_IMAGE_SIZE", "1024x1024"),
		quality: env("OPENAI_IMAGE_QUALITY", "low"),
		http:    &http.Client{Timeout: 300 * time.Second},
	}
}

func collectOpenAIKeys() []string {
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	add(os.Getenv("OPENAI_API_KEY"))
	add(os.Getenv("OPENAI_API_KEYS"))
	for i := 2; i <= 8; i++ {
		add(os.Getenv(fmt.Sprintf("OPENAI_API_KEY_%d", i)))
	}
	return out
}

func (c *ThumbnailClient) Enabled() bool {
	return c != nil && len(c.apiKeys) > 0
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
	lazyModel := env("LAZY_THUMB_MODEL", "gpt-image-2")
	lazySize := env("LAZY_THUMB_SIZE", "1024x768")
	lazyQuality := env("LAZY_THUMB_QUALITY", "high")
	return map[string]any{
		"enabled":  c.Enabled(),
		"provider": "openai",
		"model":    c.Model(),
		"size":     c.Size(),
		"quality":  c.Quality(),
		// Preset yang dipakai Lazy Business + Generate page (bukan lab).
		"preset": map[string]any{
			"model":    lazyModel,
			"size":     lazySize,
			"quality":  lazyQuality,
			"crop_4_3": true,
		},
		"models": []string{
			"gpt-image-1",
			"gpt-image-1-mini",
			"gpt-image-1.5",
			"gpt-image-2",
			"dall-e-3",
		},
		"sizes": []string{
			"1024x1024",
			"1024x768",
			"1280x960",
			"1536x1152",
			"1536x1024",
			"1024x1536",
		},
		"qualities":   []string{"low", "medium", "high", "auto"},
		"output_size": fmt.Sprintf("%dx%d", thumbOutW, thumbOutH),
	}
}

// ThumbnailRequest allows per-call overrides (lab / A-B testing).
type ThumbnailRequest struct {
	Hook       string `json:"hook"`
	Text       string `json:"text"`
	Model      string `json:"model"`
	Size       string `json:"size"`
	Quality    string `json:"quality"`
	Extra      string `json:"extra"`       // catatan tambahan ke prompt
	Crop43     *bool  `json:"crop_4_3"`   // default true
	CustomOnly bool   `json:"custom_only"` // pakai Extra sebagai prompt penuh (ignore template)
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

// BuildThumbnailPrompt — brief tetap dari user, tanpa tambahan aturan.
// Konteks = hook bagian 1 utas.
func BuildThumbnailPrompt(hook string) string {
	hook = strings.TrimSpace(hook)
	if hook == "" {
		hook = "(hook kosong)"
	}
	if len(hook) > 2000 {
		hook = hook[:2000] + "…"
	}
	return fmt.Sprintf(`buatkan thumbnail untuk utas threads 4:3 konteksnya

%s

buat design clean, minimalis.
dont: 
- jangan menulis ulang narasinya`, hook)
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

func (c *ThumbnailClient) Generate(hook string) (*ThumbnailResult, error) {
	return c.GenerateRequest(ThumbnailRequest{Hook: hook})
}

func (c *ThumbnailClient) GenerateRequest(req ThumbnailRequest) (*ThumbnailResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("thumbnail ChatGPT belum dikonfigurasi — set OPENAI_API_KEY di .env")
	}
	hook := strings.TrimSpace(req.Hook)
	if hook == "" {
		hook = strings.TrimSpace(req.Text)
	}
	if hook == "" && !req.CustomOnly {
		return nil, fmt.Errorf("hook utas (bagian 1) wajib diisi")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	size := strings.TrimSpace(req.Size)
	if size == "" {
		size = c.size
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
		prompt = BuildThumbnailPrompt(hook)
		if extra != "" {
			prompt += "\n\nAdditional direction from editor:\n" + extra
		}
	}

	rawPNG, err := c.generatePNG(prompt, model, size, quality)
	if err != nil {
		return nil, err
	}

	crop := true
	if req.Crop43 != nil {
		crop = *req.Crop43
	}

	outPNG := rawPNG
	w, h := 0, 0
	if crop {
		cropped, cw, ch, err := toThumb43(rawPNG, thumbOutW, thumbOutH)
		if err != nil {
			return nil, err
		}
		outPNG, w, h = cropped, cw, ch
	} else {
		img, _, err := image.Decode(bytes.NewReader(rawPNG))
		if err != nil {
			return nil, fmt.Errorf("decode image: %w", err)
		}
		b := img.Bounds()
		w, h = b.Dx(), b.Dy()
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		outPNG = buf.Bytes()
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

func (c *ThumbnailClient) generatePNG(prompt, model, size, quality string) ([]byte, error) {
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
		png, err := c.generatePNGOnce(key, prompt, model, size, quality)
		if err == nil {
			return png, nil
		}
		lastErr = err
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

func (c *ThumbnailClient) generatePNGOnce(apiKey, prompt, model, size, quality string) ([]byte, error) {
	body := map[string]any{
		"model":  model,
		"prompt": prompt,
		"n":      1,
		"size":   size,
	}
	// dall-e-3 only accepts standard|hd; gpt-image uses low|medium|high|auto
	q := strings.TrimSpace(quality)
	if q != "" {
		if strings.HasPrefix(model, "dall-e") {
			if q == "high" || q == "hd" {
				body["quality"] = "hd"
			} else {
				body["quality"] = "standard"
			}
		} else {
			body["quality"] = q
		}
	}
	// gpt-image models return b64 by default; be explicit for older dall-e too.
	if strings.HasPrefix(model, "dall-e") {
		body["response_format"] = "b64_json"
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/images/generations", bytes.NewReader(payload))
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
		}
		if json.Unmarshal(raw, &er) == nil && er.Error.Message != "" {
			msg = er.Error.Message
		}
		return nil, fmt.Errorf("OpenAI images %d: %s", res.StatusCode, truncate(msg, 400))
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

// toThumb43 center-crops to 4:3 lalu scale ke outW×outH (default 1024×768).
func toThumb43(pngBytes []byte, outW, outH int) ([]byte, int, int, error) {
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

	cropW, cropH := w, h
	if w*3 == h*4 {
		// already 4:3
	} else if w*3 > h*4 {
		cropW = h * 4 / 3
		if cropW < 1 {
			cropW = 1
		}
		if cropW > w {
			cropW = w
		}
	} else {
		cropH = w * 3 / 4
		if cropH < 1 {
			cropH = 1
		}
		if cropH > h {
			cropH = h
		}
	}

	x0 := b.Min.X + (w-cropW)/2
	y0 := b.Min.Y + (h-cropH)/2
	cropped := image.NewRGBA(image.Rect(0, 0, cropW, cropH))
	draw.Draw(cropped, cropped.Bounds(), img, image.Pt(x0, y0), draw.Src)

	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), cropped, cropped.Bounds(), xdraw.Over, nil)

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
