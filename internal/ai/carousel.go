package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CarouselRequest struct {
	Parts        []string `json:"parts"` // utas Threads → tiap bagian = 1 slide
	Brand        string   `json:"brand"`
	Topic        string   `json:"topic"`
	Niche        string   `json:"niche"`
	Instructions string   `json:"instructions"`
	Count        int      `json:"count"` // unused if parts set
}

type CarouselSlide struct {
	Text string `json:"text"`
	Note string `json:"note,omitempty"`
}

type CarouselResult struct {
	Title         string          `json:"title"`
	Brand         string          `json:"brand"`
	Caption       string          `json:"caption"`
	Slides        []CarouselSlide `json:"slides"`
	Parts         []string        `json:"parts"` // mirror utas untuk Threads
	Angle         string          `json:"angle,omitempty"`
	Consideration string          `json:"consideration,omitempty"`
	Source        string          `json:"source"` // utas | generated
	Model         string          `json:"model"`
	Provider      string          `json:"provider"`
	Usage         *TokenUsage     `json:"usage,omitempty"`
	Quota         *QuotaStatus    `json:"quota,omitempty"`
}

func (c *Client) GenerateCarousel(mem Memory, req CarouselRequest) (*CarouselResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("AI belum dikonfigurasi — set AI_API_KEY di .env")
	}
	if c.quota != nil {
		if err := c.quota.check(); err != nil {
			return nil, err
		}
	}

	brand := strings.TrimSpace(req.Brand)
	if brand == "" {
		brand = strings.TrimSpace(mem.Brand)
	}

	parts := cleanParts(req.Parts)
	if len(parts) >= 2 {
		return c.carouselFromUtas(mem, parts, brand, req)
	}
	return c.carouselFromScratch(mem, brand, req)
}

func cleanParts(in []string) []string {
	var out []string
	for _, p := range in {
		p = strings.TrimSpace(normalizeNewlines(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// carouselFromUtas: utas Threads → slide IG (1 bagian = 1 slide) + caption.
func (c *Client) carouselFromUtas(mem Memory, parts []string, brand string, req CarouselRequest) (*CarouselResult, error) {
	if len(parts) > 10 {
		parts = parts[:10]
	}
	payload, _ := json.MarshalIndent(map[string]any{
		"brand": brand,
		"parts": parts,
		"topic": strings.TrimSpace(req.Topic),
		"today": time.Now().Format("2006-01-02"),
	}, "", "  ")

	system := fmt.Sprintf(`Kamu menyiapkan CAPTION Instagram dari utas Threads yang sudah jadi.
Jangan tulis ulang isi slide — slides sudah fixed dari utas.

Brand (tampil di kartu): %s

Jawab HANYA JSON valid:
{
  "title": "judul internal singkat",
  "caption": "caption IG (hook singkat + ajakan reply alami, max ~1200 karakter, 0-3 hashtag max)",
  "consideration": "1 kalimat kenapa utas ini cocok jadi carousel"
}

Aturan: bahasa Indonesia natural, anti-dogeng, jangan spam hashtag.`, emptyFallback(brand, "(tanpa brand)"))

	user := "Buat caption untuk carousel dari utas ini:\n\n" + string(payload)
	content, usage, err := c.chatForJSON(system, user)
	if err != nil {
		// fallback tanpa AI caption
		slides := make([]CarouselSlide, 0, len(parts))
		for _, p := range parts {
			slides = append(slides, CarouselSlide{Text: p})
		}
		out := &CarouselResult{
			Title:   "Utas → Carousel",
			Brand:   brand,
			Caption: clipRunes(parts[0], 200),
			Slides:  slides,
			Parts:   parts,
			Source:  "utas",
			Model:   c.model,
			Provider: c.provider,
		}
		return out, nil
	}
	if c.quota != nil {
		c.quota.record(usage)
	}

	var meta struct {
		Title         string `json:"title"`
		Caption       string `json:"caption"`
		Consideration string `json:"consideration"`
	}
	_ = json.Unmarshal([]byte(extractJSON(content)), &meta)

	slides := make([]CarouselSlide, 0, len(parts))
	for _, p := range parts {
		slides = append(slides, CarouselSlide{Text: p})
	}
	out := &CarouselResult{
		Title:         emptyFallback(meta.Title, "Utas → Carousel"),
		Brand:         brand,
		Caption:       meta.Caption,
		Slides:        slides,
		Parts:         parts,
		Consideration: meta.Consideration,
		Source:        "utas",
		Model:         c.model,
		Provider:      c.provider,
		Usage:         usage,
	}
	if c.quota != nil {
		q := c.quota.status(c.provider, c.model)
		out.Quota = &q
	}
	return out, nil
}

// carouselFromScratch: generate utas dulu lalu map ke slides (lazy business path).
func (c *Client) carouselFromScratch(mem Memory, brand string, req CarouselRequest) (*CarouselResult, error) {
	count := req.Count
	if count <= 0 {
		count = 6
	}
	if count < 4 {
		count = 4
	}
	if count > 10 {
		count = 10
	}

	gen, err := c.GenerateContent(nil, mem, GenerateRequest{
		Topic:        req.Topic,
		Instructions: req.Instructions,
		Count:        1,
	})
	if err != nil {
		return nil, err
	}
	if len(gen.Drafts) == 0 || len(gen.Drafts[0].Parts) < 2 {
		return nil, fmt.Errorf("gagal generate utas untuk carousel")
	}
	parts := gen.Drafts[0].Parts
	if len(parts) > count {
		parts = parts[:count]
	}
	out, err := c.carouselFromUtas(mem, parts, brand, req)
	if err != nil {
		return nil, err
	}
	out.Source = "generated"
	out.Angle = gen.Drafts[0].Angle
	if out.Title == "" || out.Title == "Utas → Carousel" {
		out.Title = gen.Drafts[0].Title
	}
	if gen.Usage != nil && out.Usage != nil {
		out.Usage.PromptTokens += gen.Usage.PromptTokens
		out.Usage.CompletionTokens += gen.Usage.CompletionTokens
		out.Usage.TotalTokens += gen.Usage.TotalTokens
	} else if gen.Usage != nil {
		out.Usage = gen.Usage
	}
	return out, nil
}

func (c *Client) chatForJSON(system, user string) (string, *TokenUsage, error) {
	switch c.provider {
	case "gemini", "google":
		return c.chatGemini(system, user)
	default:
		return c.chatOpenAICompat(system, user)
	}
}
