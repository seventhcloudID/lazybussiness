package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	provider string
	baseURL  string
	model    string
	apiKeys  []string
	keyMu    sync.Mutex
	keyIdx   int
	http     *http.Client
	quota    *quotaTracker
}

func NewFromEnv() *Client {
	provider := strings.ToLower(env("AI_PROVIDER", "gemini"))
	baseDefault := "https://generativelanguage.googleapis.com"
	if provider == "deepseek" {
		baseDefault = "https://api.deepseek.com"
	}
	base := strings.TrimRight(env("AI_BASE_URL", baseDefault), "/")
	modelDefault := "gemini-3.6-flash"
	if provider == "deepseek" {
		modelDefault = "deepseek-v4-flash"
	}
	keys := mergeAPIKeys(collectAPIKeysFromEnv(), LoadStoredAPIKeys())
	c := &Client{
		provider: provider,
		baseURL:  base,
		model:    env("AI_MODEL", modelDefault),
		apiKeys:  keys,
		http:     &http.Client{Timeout: 120 * time.Second},
		quota:    newQuotaTrackerFromEnv(len(keys)),
	}
	return c
}

func (c *Client) Enabled() bool {
	return c != nil && len(c.apiKeys) > 0
}

func (c *Client) KeyCount() int {
	if c == nil {
		return 0
	}
	return len(c.apiKeys)
}

func (c *Client) currentKey() string {
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

func (c *Client) rotateKey() (string, bool) {
	c.keyMu.Lock()
	defer c.keyMu.Unlock()
	if len(c.apiKeys) <= 1 {
		return "", false
	}
	c.keyIdx = (c.keyIdx + 1) % len(c.apiKeys)
	return c.apiKeys[c.keyIdx], true
}

func (c *Client) Provider() string { return c.provider }
func (c *Client) Model() string    { return c.model }

func (c *Client) Quota() QuotaStatus {
	if c == nil || c.quota == nil {
		return QuotaStatus{Note: "quota tracker tidak aktif"}
	}
	q := c.quota.status(c.provider, c.model)
	if n := c.KeyCount(); n > 1 {
		q.Note = strings.TrimSpace(q.Note + fmt.Sprintf(" · %d API key (rotasi otomatis jika limit)", n))
	}
	return q
}

type InsightResult struct {
	Headline           string              `json:"headline"`
	Summary            string              `json:"summary"`
	AccountRead        *AccountRead        `json:"account_read,omitempty"`
	Scorecard          *Scorecard          `json:"scorecard,omitempty"`
	EngagementProfile  *EngagementProfile  `json:"engagement_profile,omitempty"`
	HotContent         []ContentInsight    `json:"hot_content"`
	ColdContent        []ContentInsight    `json:"cold_content"`
	ContentDNA         *ContentDNA         `json:"content_dna,omitempty"`
	Patterns           []string            `json:"patterns"`
	Raw                string              `json:"raw,omitempty"`
	Model              string              `json:"model"`
	Provider           string              `json:"provider"`
	Usage              *TokenUsage         `json:"usage,omitempty"`
	Quota              *QuotaStatus        `json:"quota,omitempty"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type AccountRead struct {
	Niche        string `json:"niche"`
	Voice        string `json:"voice"`
	Audience     string `json:"audience"`
	Positioning  string `json:"positioning"`
}

type Scorecard struct {
	Strength    string `json:"strength"`
	Weakness    string `json:"weakness"`
	Opportunity string `json:"opportunity"`
}

type EngagementProfile struct {
	WhatDrivesViews   string `json:"what_drives_views"`
	WhatDrivesReplies string `json:"what_drives_replies"`
	FormatBias        string `json:"format_bias"`
	LengthBias        string `json:"length_bias"`
}

type ContentInsight struct {
	Label   string `json:"label"`
	Excerpt string `json:"excerpt"`
	Why     string `json:"why"`
	Proof   string `json:"proof"`
	Pattern string `json:"pattern"`
}

type ContentDNA struct {
	RecurringThemes []string `json:"recurring_themes"`
	SignatureMoves  []string `json:"signature_moves"`
	BlindSpots      []string `json:"blind_spots"`
}

func (c *Client) AnalyzeThreads(snapshot any) (*InsightResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("AI belum dikonfigurasi — set AI_API_KEY di .env")
	}
	if c.quota != nil {
		if err := c.quota.check(); err != nil {
			return nil, err
		}
	}

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}

	system := `Kamu adalah analis akun Threads. Tugasmu HANYA membaca dan membedah akun ini dari data yang diberi.
Jangan buat draf post, jangan buat rencana aksi, jangan kasih to-do list. Itu bukan bagian dari tugasmu.

Fokus: siapa akun ini, bagaimana performanya, apa yang bikin kontennya hidup/mati, pola & DNA kontennya.

Jawab HANYA JSON valid (tanpa markdown) dengan skema:
{
  "headline": "satu kalimat verdict tentang kondisi akun (bukan saran)",
  "summary": "3-5 kalimat: baca akun — niche, tone, dan kondisi performa terkini berdasarkan data",
  "account_read": {
    "niche": "niche/topik yang terlihat dari post + bio",
    "voice": "nada & persona (contoh: sinis-analitis, hangat-edukatif)",
    "audience": "siapa yang kelihatannya merespons (dari jenis balasan/engagement)",
    "positioning": "posisi akun di Threads menurut bukti konten"
  },
  "scorecard": {
    "strength": "kekuatan akun + bukti angka",
    "weakness": "kelemahan pola performa + bukti angka",
    "opportunity": "karakteristik/signal yang menonjol (bukan saran aksi)"
  },
  "engagement_profile": {
    "what_drives_views": "tipe konten/angle yang mendorong views",
    "what_drives_replies": "tipe konten yang memancing balasan",
    "format_bias": "format yang lebih kuat di akun ini (TEXT/IMAGE/dll) + bukti",
    "length_bias": "kecenderungan panjang/pendek teks yang terlihat dari data"
  },
  "hot_content": [{
    "label": "judul singkat post yang rame",
    "excerpt": "cuplikan isi",
    "why": "analisis kenapa rame di konteks akun ini",
    "proof": "angka metrik",
    "pattern": "pola yang terlihat dari post ini"
  }],
  "cold_content": [{
    "label": "judul singkat post yang sepi",
    "excerpt": "cuplikan",
    "why": "analisis kenapa sepi",
    "proof": "angka",
    "pattern": "pola yang terlihat"
  }],
  "content_dna": {
    "recurring_themes": ["tema yang sering muncul"],
    "signature_moves": ["ciri khas gaya/struktur yang berulang"],
    "blind_spots": ["area/tema yang jarang atau lemah di data"]
  },
  "patterns": ["pola observasi 1", "pola 2", "pola 3"]
}

Aturan ketat:
- Semua klaim harus merujuk metrik & isi post di data (views/likes/replies/reposts/quotes/score/ER/bio).
- Spesifik ke akun ini. Dilarang generic.
- Minimal 2 hot_content, 2 cold_content, 3 patterns.
- DILARANG menulis draf post, hook siap tempel, action plan, "hari ini lakukan X", atau rekomendasi eksekusi.
- Bahasa Indonesia, tajam, observasional.`

	user := "Bedah akun Threads ini. Fokus breakdown & observasi — tanpa saran eksekusi atau draf:\n\n" + string(payload)

	var content string
	var usage *TokenUsage
	switch c.provider {
	case "gemini", "google":
		content, usage, err = c.chatGemini(system, user)
	default:
		content, usage, err = c.chatOpenAICompat(system, user)
	}
	if err != nil {
		return nil, err
	}
	if c.quota != nil {
		c.quota.record(usage)
	}

	content = extractJSON(content)

	var out InsightResult
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		out = InsightResult{
			Headline: "Analisis tersedia (format bebas)",
			Summary:  content,
			Raw:      content,
		}
	}
	if strings.HasPrefix(strings.TrimSpace(out.Summary), "{") && out.Headline == "Analisis tersedia (format bebas)" {
		var retry InsightResult
		if json.Unmarshal([]byte(extractJSON(out.Summary)), &retry) == nil && retry.Summary != "" {
			out = retry
		}
	}
	out.Model = c.model
	out.Provider = c.provider
	out.Usage = usage
	if c.quota != nil {
		q := c.quota.status(c.provider, c.model)
		out.Quota = &q
	}
	return &out, nil
}

func (c *Client) chatGemini(system, user string) (string, *TokenUsage, error) {
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", c.baseURL, url.PathEscape(c.model))
	reqBody := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": system}},
		},
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": user}},
			},
		},
		"generationConfig": map[string]any{
			"temperature":      0.45,
			"maxOutputTokens":  8192,
			"responseMimeType": "application/json",
		},
	}
	rawReq, _ := json.Marshal(reqBody)

	var lastErr error
	tried := 0
	maxTries := len(c.apiKeys)
	if maxTries < 1 {
		maxTries = 1
	}
	for tried < maxTries {
		tried++
		key := c.currentKey()
		if key == "" {
			return "", nil, fmt.Errorf("AI_API_KEY kosong")
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(rawReq))
		if err != nil {
			return "", nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", key)

		res, err := c.http.Do(req)
		if err != nil {
			return "", nil, err
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return "", nil, err
		}
		if res.StatusCode >= 400 {
			lastErr = fmt.Errorf("Gemini API status %d: %s", res.StatusCode, truncate(string(body), 500))
			if isGeminiQuotaError(res.StatusCode, body) {
				if _, ok := c.rotateKey(); ok {
					continue
				}
			}
			return "", nil, lastErr
		}

		var parsed struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata *struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
				TotalTokenCount      int `json:"totalTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return "", nil, err
		}
		if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
			return "", nil, fmt.Errorf("Gemini tidak mengembalikan jawaban: %s", truncate(string(body), 300))
		}

		var b strings.Builder
		for _, p := range parsed.Candidates[0].Content.Parts {
			b.WriteString(p.Text)
		}
		var usage *TokenUsage
		if parsed.UsageMetadata != nil {
			usage = &TokenUsage{
				PromptTokens:     parsed.UsageMetadata.PromptTokenCount,
				CompletionTokens: parsed.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      parsed.UsageMetadata.TotalTokenCount,
			}
		}
		return strings.TrimSpace(b.String()), usage, nil
	}
	if lastErr != nil {
		return "", nil, fmt.Errorf("semua API key kena limit: %w", lastErr)
	}
	return "", nil, fmt.Errorf("Gemini gagal setelah rotasi key")
}

func isGeminiQuotaError(status int, body []byte) bool {
	if status == 429 {
		return true
	}
	s := strings.ToLower(string(body))
	return strings.Contains(s, "resource_exhausted") ||
		strings.Contains(s, "quota") ||
		strings.Contains(s, "rate limit") ||
		strings.Contains(s, "exceeded")
}

func (c *Client) chatOpenAICompat(system, user string) (string, *TokenUsage, error) {
	return c.chatOpenAICompatTry(system, user, 0)
}

func (c *Client) chatOpenAICompatTry(system, user string, attempt int) (string, *TokenUsage, error) {
	reqBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature":     0.45,
		"max_tokens":      4500,
		"response_format": map[string]string{"type": "json_object"},
	}
	rawReq, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(rawReq))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.currentKey())

	res, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", nil, err
	}
	if res.StatusCode >= 400 {
		errMsg := fmt.Errorf("AI API status %d: %s", res.StatusCode, truncate(string(body), 400))
		if res.StatusCode == 429 && attempt+1 < len(c.apiKeys) {
			if _, ok := c.rotateKey(); ok {
				return c.chatOpenAICompatTry(system, user, attempt+1)
			}
		}
		return "", nil, errMsg
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *TokenUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", nil, err
	}
	if len(parsed.Choices) == 0 {
		return "", nil, fmt.Errorf("AI tidak mengembalikan jawaban")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), parsed.Usage, nil
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = stripFence(s)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			return strings.TrimSpace(s[i : j+1])
		}
	}
	return s
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
