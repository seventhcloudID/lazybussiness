package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	provider    string
	baseURL     string
	model       string
	chatModel   string // Chat UI — ChatGPT combo (GPTPlus), bukan Codex AI_MODEL
	searchModel string // Google Search grounding (sering beda kuota dari AI_MODEL)
	apiKeys     []string
	keyMu       sync.Mutex
	keyIdx      int
	http        *http.Client
	quota       *quotaTracker
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
	model := env("AI_MODEL", modelDefault)
	chatModel := strings.TrimSpace(env("AI_CHAT_MODEL", ""))
	if chatModel == "" && (provider == "openai" || provider == "chatgpt") {
		chatModel = "GPTPlus"
	}
	keys := collectAPIKeysFromEnv()
	// Key Gemini di UI (.data/ai_keys.json) hanya dipakai kalau provider Gemini.
	if provider == "gemini" || provider == "google" {
		keys = mergeAPIKeys(keys, LoadStoredAPIKeys())
	}
	c := &Client{
		provider:    provider,
		baseURL:     base,
		model:       model,
		chatModel:   chatModel,
		searchModel: resolveSearchModel(provider, model),
		apiKeys:     keys,
		http:        &http.Client{Timeout: 300 * time.Second},
		quota:       newQuotaTrackerFromEnv(len(keys)),
	}
	return c
}

// resolveSearchModel picks a model that still has free-tier Google Search grounding.
// Gemini 3.x free: grounding "Not available" → 429; 2.5 Flash free: up to ~500 RPD search.
func resolveSearchModel(provider, mainModel string) string {
	if override := strings.TrimSpace(os.Getenv("AI_SEARCH_MODEL")); override != "" {
		return override
	}
	if provider != "gemini" && provider != "google" {
		return mainModel
	}
	m := strings.ToLower(strings.TrimSpace(mainModel))
	if strings.Contains(m, "gemini-3") || strings.HasPrefix(m, "gemini-3") {
		return "gemini-2.5-flash"
	}
	return mainModel
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
func (c *Client) ChatModel() string {
	if c == nil {
		return ""
	}
	if s := strings.TrimSpace(c.chatModel); s != "" {
		return s
	}
	if c.provider == "openai" || c.provider == "chatgpt" {
		return "GPTPlus"
	}
	return c.model
}
func (c *Client) SearchModel() string { return c.searchModel }

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
	Headline          string             `json:"headline"`
	Summary           string             `json:"summary"`
	NorthStar         string             `json:"north_star,omitempty"`
	AccountRead       *AccountRead       `json:"account_read,omitempty"`
	Scorecard         *Scorecard         `json:"scorecard,omitempty"`
	EngagementProfile *EngagementProfile `json:"engagement_profile,omitempty"`
	HotContent        []ContentInsight   `json:"hot_content"`
	ColdContent       []ContentInsight   `json:"cold_content"`
	ContentDNA        *ContentDNA        `json:"content_dna,omitempty"`
	HookLab           *HookLab           `json:"hook_lab,omitempty"`
	Pillars           []ContentPillar    `json:"pillars,omitempty"`
	Patterns          []string           `json:"patterns"`
	Risks             []string           `json:"risks,omitempty"`
	Playbook          *Playbook          `json:"playbook,omitempty"`
	ReplyPlay         *ReplyPlay         `json:"reply_play,omitempty"`
	Experiments       []ExperimentIdea   `json:"experiments,omitempty"`
	ColdRewrites      []ColdRewrite      `json:"cold_rewrites,omitempty"`
	WeekPlan          []WeekSlot         `json:"week_plan,omitempty"`
	NextPosts         []NextPostIdea     `json:"next_posts,omitempty"`
	TimingRead        string             `json:"timing_read,omitempty"`
	Raw               string             `json:"raw,omitempty"`
	Model             string             `json:"model"`
	Provider          string             `json:"provider"`
	Usage             *TokenUsage        `json:"usage,omitempty"`
	Quota             *QuotaStatus       `json:"quota,omitempty"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type AccountRead struct {
	Niche          string `json:"niche"`
	Voice          string `json:"voice"`
	Audience       string `json:"audience"`
	Positioning    string `json:"positioning"`
	Differentiator string `json:"differentiator,omitempty"`
}

type Scorecard struct {
	Strength     string `json:"strength"`
	Weakness     string `json:"weakness"`
	Opportunity  string `json:"opportunity"`
	Reach        int    `json:"reach,omitempty"`
	Conversation int    `json:"conversation,omitempty"`
	Consistency  int    `json:"consistency,omitempty"`
	HookPower    int    `json:"hook_power,omitempty"`
	Originality  int    `json:"originality,omitempty"`
	ReplyMagnet  int    `json:"reply_magnet,omitempty"`
}

type EngagementProfile struct {
	WhatDrivesViews   string `json:"what_drives_views"`
	WhatDrivesReplies string `json:"what_drives_replies"`
	FormatBias        string `json:"format_bias"`
	LengthBias        string `json:"length_bias"`
	BestTime          string `json:"best_time,omitempty"`
	WorstTime         string `json:"worst_time,omitempty"`
}

type ContentInsight struct {
	PostID    string  `json:"post_id,omitempty"`
	Permalink string  `json:"permalink,omitempty"`
	Text      string  `json:"text,omitempty"`
	Label     string  `json:"label"`
	Excerpt   string  `json:"excerpt"`
	Why       string  `json:"why"`
	Proof     string  `json:"proof"`
	Pattern   string  `json:"pattern"`
	Views     float64 `json:"views,omitempty"`
	Likes     float64 `json:"likes,omitempty"`
	Replies   float64 `json:"replies,omitempty"`
	Reposts   float64 `json:"reposts,omitempty"`
	Quotes    float64 `json:"quotes,omitempty"`
	Score     float64 `json:"score,omitempty"`
	MediaType string  `json:"media_type,omitempty"`
	Timestamp string  `json:"timestamp,omitempty"`
}

type ContentDNA struct {
	RecurringThemes []string `json:"recurring_themes"`
	SignatureMoves  []string `json:"signature_moves"`
	BlindSpots      []string `json:"blind_spots"`
	HookFormulas    []string `json:"hook_formulas,omitempty"`
}

type HookLab struct {
	WinningOpeners []string `json:"winning_openers"`
	LosingOpeners  []string `json:"losing_openers"`
	DoMore         []string `json:"do_more"`
	StopDoing      []string `json:"stop_doing"`
}

type ContentPillar struct {
	Name          string   `json:"name"`
	WeightPct     int      `json:"weight_pct"`
	Why           string   `json:"why"`
	ExampleAngles []string `json:"example_angles,omitempty"`
}

type Playbook struct {
	Stop     []string `json:"stop"`
	Start    []string `json:"start"`
	Continue []string `json:"continue"`
	ThisWeek []string `json:"this_week"`
}

type ReplyPlay struct {
	WhenToAsk      string   `json:"when_to_ask"`
	QuestionStyles []string `json:"question_styles"`
	Avoid          []string `json:"avoid"`
}

type ExperimentIdea struct {
	Name          string `json:"name"`
	Hypothesis    string `json:"hypothesis"`
	HowToTest     string `json:"how_to_test"`
	SuccessMetric string `json:"success_metric"`
	Effort        string `json:"effort"`
}

type ColdRewrite struct {
	PostID    string `json:"post_id,omitempty"`
	Problem   string `json:"problem"`
	NewHook   string `json:"new_hook"`
	WhyBetter string `json:"why_better"`
}

type WeekSlot struct {
	Day     string `json:"day"`
	Daypart string `json:"daypart"`
	Angle   string `json:"angle"`
	Hook    string `json:"hook"`
	Format  string `json:"format"`
}

type NextPostIdea struct {
	Angle    string `json:"angle"`
	Hook     string `json:"hook"`
	Why      string `json:"why"`
	Format   string `json:"format"`
	Pillar   string `json:"pillar,omitempty"`
	CTA      string `json:"cta,omitempty"`
	Priority string `json:"priority,omitempty"`
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

	system := `Kamu adalah head of content + growth strategist (setingkat o1): tajam, agresif-akurat, berbasis bukti, anti-generic.

Kamu menerima snapshot SATU akun Repliz (profile.username, profile.name, profile.type, agregat, mix format/hari/daypart, sample post + metrik). Bedah akun itu saja. platform = profile.type (instagram/threads/tiktok) — jangan asumsi Threads.

Jawab HANYA JSON valid (tanpa markdown) dengan skema:
{
  "headline": "satu kalimat verdict berani + spesifik ke akun ini",
  "summary": "6-10 kalimat: niche, voice, kondisi performa vs persentil, apa yang bikin hidup/mati, leverage terbesar, risiko, arah 14 hari",
  "north_star": "1 metrik utama 14 hari ke depan + target realistis (contoh: naikkan median views / avg replies)",
  "account_read": {
    "niche": "niche nyata dari post+bio",
    "voice": "nada & persona + ciri kalimat",
    "audience": "siapa yang merespons, dari jenis engagement",
    "positioning": "posisi di percakapan platform ini (instagram/threads/tiktok sesuai profile.type)",
    "differentiator": "apa yang cuma akun ini punya, dari bukti"
  },
  "scorecard": {
    "strength": "kekuatan + bukti angka",
    "weakness": "kelemahan pola + bukti angka",
    "opportunity": "leverage terbesar 14 hari",
    "reach": 1, "conversation": 1, "consistency": 1,
    "hook_power": 1, "originality": 1, "reply_magnet": 1
  },
  "engagement_profile": {
    "what_drives_views": "angle/format + bukti",
    "what_drives_replies": "yang memancing balasan + bukti",
    "format_bias": "TEXT/IMAGE/VIDEO mana menang + angka",
    "length_bias": "panjang teks menang vs kalah (chars)",
    "best_time": "hari + daypart terkuat",
    "worst_time": "hari + daypart terlemah"
  },
  "hot_content": [{
    "post_id": "WAJIB id dari snapshot.posts[].id",
    "label": "judul singkat",
    "excerpt": "cuplikan 1 kalimat",
    "why": "kenapa rame — hook/timing/topik/format",
    "proof": "views/likes/replies/ER",
    "pattern": "pola yang bisa diulang"
  }],
  "cold_content": [{
    "post_id": "WAJIB id dari snapshot.posts[].id",
    "label": "judul singkat",
    "excerpt": "cuplikan",
    "why": "kenapa sepi — spesifik",
    "proof": "angka",
    "pattern": "pola yang harus dihentikan"
  }],
  "content_dna": {
    "recurring_themes": ["tema"],
    "signature_moves": ["ciri khas"],
    "blind_spots": ["yang jarang padahal niche minta"],
    "hook_formulas": ["3-6 rumus hook terbukti di data"]
  },
  "hook_lab": {
    "winning_openers": ["pola pembuka yang menang di hot posts"],
    "losing_openers": ["pola pembuka yang kalah di cold posts"],
    "do_more": ["latihan hook konkret"],
    "stop_doing": ["pembuka yang harus dibunuh"]
  },
  "pillars": [{
    "name": "nama pilar konten",
    "weight_pct": 30,
    "why": "kenapa pilar ini, pakai bukti",
    "example_angles": ["2-3 sudut"]
  }],
  "patterns": ["6-10 observasi lintas post, masing-masing pakai angka"],
  "risks": ["3-5 risiko konten/brand/algoritma spesifik akun ini"],
  "timing_read": "1-3 kalimat kapan akun paling hidup",
  "playbook": {
    "stop": ["hentikan — kenapa"],
    "start": ["mulai — kenapa + bukti hot"],
    "continue": ["pertahankan"],
    "this_week": ["5-7 langkah konkret minggu ini"]
  },
  "reply_play": {
    "when_to_ask": "kapan tutup dengan pertanyaan",
    "question_styles": ["3-5 gaya pertanyaan yang cocok voice akun"],
    "avoid": ["jenis CTA/pertanyaan yang jelek di data ini"]
  },
  "experiments": [{
    "name": "nama eksperimen",
    "hypothesis": "jika X maka Y",
    "how_to_test": "cara uji 3-5 post",
    "success_metric": "metrik sukses",
    "effort": "low|medium|high"
  }],
  "cold_rewrites": [{
    "post_id": "id cold post",
    "problem": "apa yang salah di hook/angle",
    "new_hook": "hook baru siap tempel, gaya akun",
    "why_better": "kenapa lebih kuat dari data"
  }],
  "week_plan": [{
    "day": "Senin",
    "daypart": "pagi|siang|sore|malam",
    "angle": "sudut",
    "hook": "hook siap tempel",
    "format": "TEXT|IMAGE"
  }],
  "next_posts": [{
    "angle": "sudut belum jenuh",
    "hook": "hook siap tempel 1-2 kalimat, voice akun",
    "why": "kenapa cocok dari data",
    "format": "TEXT|IMAGE",
    "pillar": "nama pilar",
    "cta": "ajakan reply singkat opsional",
    "priority": "P0|P1|P2"
  }]
}

Aturan:
- Klaim wajib nempel ke angka (views, likes, replies, ER, chars, format_mix, weekday_mix, daypart_mix, views_p25/p50/p75, top_score_ids/bottom_score_ids).
- Spesifik ke akun ini. Dilarang nasihat generik tanpa bukti.
- Minimal: 5 hot, 4 cold, 6 patterns, 3 pillars (weight_pct jumlah ~100), 4 experiments, 3 cold_rewrites, 7 week_plan (1 minggu), 6 next_posts (min 2 P0).
- hot/cold/cold_rewrites WAJIB post_id dari snapshot.posts.
- Semua skor integer 1-10.
- Hormati brand_context HANYA jika brand/handle-nya cocok dengan profile.username atau profile.name. Kalau brand_context tidak ada atau tidak cocok, abaikan total.
- Dilarang menyebut brand, persona, atau niche dari memori workspace lain (contoh "AI Personal", nama orang yang bukan pemilik akun di snapshot).
- account_read.niche dan headline wajib dari isi post + bio di snapshot, bukan dari brand_context yang tidak cocok.
- Bahasa Indonesia, tajam, seperti brief ke founder yang sibuk.`

	user := "Bedah HANYA akun di snapshot.profile. Jangan campur brand/niche workspace lain. Pakai mix, metrik, dan isi post. Untuk hot/cold/rewrites WAJIB rujuk post_id. Keluarkan sistem operasi konten + eksperimen + week plan:\n\n" + string(payload)

	var content string
	var usage *TokenUsage
	usedModel := c.model
	switch c.provider {
	case "gemini", "google":
		content, usage, err = c.chatGemini(system, user)
	default:
		anyMsgs := []map[string]any{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		}
		content, usage, usedModel, err = c.chatOpenAIMessagesAnyModels(c.chatUIModels(), anyMsgs, true, 0.35, 6000, isChatUIFallbackErr)
		if err != nil || strings.TrimSpace(content) == "" {
			content, usage, usedModel, err = c.chatOpenAIMessagesAnyModels(c.chatUIModels(), anyMsgs, false, 0.35, 6000, isChatUIFallbackErr)
		}
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("AI tidak mengembalikan jawaban")
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
	out.Model = usedModel
	if strings.TrimSpace(out.Model) == "" {
		out.Model = c.ChatModel()
	}
	out.Provider = c.provider
	out.Usage = usage
	if c.quota != nil {
		q := c.quota.status(c.provider, c.model)
		out.Quota = &q
	}
	EnrichInsightFromSnapshot(&out, snapshot)
	return &out, nil
}

// EnrichInsightFromSnapshot mengisi teks/permalink/metrik post asli ke hot/cold.
// Kalau AI lupa post_id, isi dari ranking score di snapshot.
func EnrichInsightFromSnapshot(out *InsightResult, snapshot any) {
	if out == nil || snapshot == nil {
		return
	}
	posts := snapshotPosts(snapshot)
	if len(posts) == 0 {
		return
	}
	byID := map[string]snapPost{}
	for _, p := range posts {
		if p.ID != "" {
			byID[p.ID] = p
		}
	}
	out.HotContent = hydrateContentList(out.HotContent, byID, posts, true, 6)
	out.ColdContent = hydrateContentList(out.ColdContent, byID, posts, false, 5)
}

type snapPost struct {
	ID        string
	Text      string
	Permalink string
	MediaType string
	Timestamp string
	Views     float64
	Likes     float64
	Replies   float64
	Reposts   float64
	Quotes    float64
	Score     float64
	ER        float64
}

func snapshotPosts(snapshot any) []snapPost {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil
	}
	var wrap struct {
		Posts []struct {
			ID             string  `json:"id"`
			Text           string  `json:"text"`
			Permalink      string  `json:"permalink"`
			MediaType      string  `json:"media_type"`
			Timestamp      string  `json:"timestamp"`
			Views          float64 `json:"views"`
			Likes          float64 `json:"likes"`
			Replies        float64 `json:"replies"`
			Reposts        float64 `json:"reposts"`
			Quotes         float64 `json:"quotes"`
			Score          float64 `json:"score"`
			EngagementRate float64 `json:"engagement_rate"`
		} `json:"posts"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return nil
	}
	out := make([]snapPost, 0, len(wrap.Posts))
	for _, p := range wrap.Posts {
		out = append(out, snapPost{
			ID: p.ID, Text: p.Text, Permalink: p.Permalink, MediaType: p.MediaType,
			Timestamp: p.Timestamp, Views: p.Views, Likes: p.Likes, Replies: p.Replies,
			Reposts: p.Reposts, Quotes: p.Quotes, Score: p.Score, ER: p.EngagementRate,
		})
	}
	return out
}

func hydrateContentList(items []ContentInsight, byID map[string]snapPost, ranked []snapPost, hot bool, want int) []ContentInsight {
	if want <= 0 {
		want = 4
	}
	seen := map[string]bool{}
	out := make([]ContentInsight, 0, want)

	apply := func(it ContentInsight, p snapPost) ContentInsight {
		it.PostID = p.ID
		it.Permalink = p.Permalink
		it.Text = p.Text
		it.Views = p.Views
		it.Likes = p.Likes
		it.Replies = p.Replies
		it.Reposts = p.Reposts
		it.Quotes = p.Quotes
		it.Score = p.Score
		it.MediaType = p.MediaType
		it.Timestamp = p.Timestamp
		if strings.TrimSpace(it.Excerpt) == "" {
			it.Excerpt = clipRunes(p.Text, 140)
		}
		if strings.TrimSpace(it.Label) == "" {
			it.Label = clipRunes(p.Text, 72)
		}
		if strings.TrimSpace(it.Proof) == "" {
			it.Proof = fmt.Sprintf("views %s · likes %s · replies %s · ER %.1f%%",
				fmtNum(p.Views), fmtNum(p.Likes), fmtNum(p.Replies), p.ER)
		}
		return it
	}

	for _, it := range items {
		id := strings.TrimSpace(it.PostID)
		if id == "" {
			// coba cocokkan excerpt/label ke teks post
			id = matchPostID(it, ranked)
		}
		p, ok := byID[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, apply(it, p))
		if len(out) >= want {
			return out
		}
	}

	// Isi sisa dari ranking score (hot = tertinggi, cold = terendah).
	ordered := append([]snapPost(nil), ranked...)
	if hot {
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].Score > ordered[j].Score })
	} else {
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].Score < ordered[j].Score })
	}
	for _, p := range ordered {
		if p.ID == "" || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		why := ""
		if hot {
			why = "Termasuk post dengan skor engagement tertinggi di sampel."
		} else {
			why = "Termasuk post dengan skor engagement terendah di sampel."
		}
		out = append(out, apply(ContentInsight{Why: why, Pattern: "ranking metrik"}, p))
		if len(out) >= want {
			break
		}
	}
	return out
}

func matchPostID(it ContentInsight, posts []snapPost) string {
	needle := strings.ToLower(strings.TrimSpace(it.Excerpt))
	if needle == "" {
		needle = strings.ToLower(strings.TrimSpace(it.Label))
	}
	if needle == "" {
		return ""
	}
	for _, p := range posts {
		t := strings.ToLower(p.Text)
		if t == "" {
			continue
		}
		if strings.Contains(t, needle) || (len([]rune(t)) >= 12 && strings.Contains(needle, string([]rune(t)[:min(40, len([]rune(t)))]))) {
			return p.ID
		}
	}
	return ""
}

func fmtNum(v float64) string {
	if v >= 1_000_000 {
		return fmt.Sprintf("%.1fj", v/1_000_000)
	}
	if v >= 1_000 {
		return fmt.Sprintf("%.1frb", v/1_000)
	}
	return fmt.Sprintf("%.0f", v)
}

func (c *Client) chatGemini(system, user string) (string, *TokenUsage, error) {
	return c.chatGeminiRequest(c.model, map[string]any{
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
	})
}

// chatGeminiSearch enables Google Search grounding (no responseMimeType — extract JSON from text).
// Uses searchModel (default gemini-2.5-flash) because Gemini 3 free tier has no search grounding.
func (c *Client) chatGeminiSearch(system, user string) (string, *TokenUsage, error) {
	model := c.searchModel
	if strings.TrimSpace(model) == "" {
		model = c.model
	}
	content, usage, err := c.chatGeminiRequest(model, map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": system}},
		},
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": user}},
			},
		},
		"tools": []map[string]any{
			{"google_search": map[string]any{}},
		},
		"generationConfig": map[string]any{
			"temperature":     0.3,
			"maxOutputTokens": 4096,
		},
	})
	if err != nil && isGeminiQuotaError(0, []byte(err.Error())) {
		return "", usage, fmt.Errorf("kuota Google Search grounding habis/tidak tersedia (model %s). Umum tetap bisa jalan. Set AI_SEARCH_MODEL=gemini-2.5-flash atau aktifkan billing. Detail: %w", model, err)
	}
	return content, usage, err
}

func (c *Client) chatGeminiRequest(model string, reqBody map[string]any) (string, *TokenUsage, error) {
	if strings.TrimSpace(model) == "" {
		model = c.model
	}
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", c.baseURL, url.PathEscape(model))
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
			lastErr = fmt.Errorf("Gemini API status %d (%s): %s", res.StatusCode, model, truncate(string(body), 500))
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
		return "", nil, fmt.Errorf("semua API key kena limit (search/model %s): %w", model, lastErr)
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
	return c.chatOpenAICompatContext(context.Background(), system, user)
}

func (c *Client) chatOpenAICompatContext(ctx context.Context, system, user string) (string, *TokenUsage, error) {
	msgs := []map[string]string{
		{"role": "system", "content": system},
		{"role": "user", "content": user},
	}
	anyMsgs := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		anyMsgs = append(anyMsgs, map[string]any{"role": m["role"], "content": m["content"]})
	}
	content, usage, _, _, _, err := c.chatOpenAIResponsesModels(ctx, chatModelFallbacks(c.model), anyMsgs, false, true, nil)
	if err == nil {
		return content, usage, nil
	}
	content, usage, _, fallbackErr := c.chatOpenAIMessagesAnyModelsCtx(ctx, chatModelFallbacks(c.model), anyMsgs, true, 0.45, 6000, isBrokenCodexRouteErr)
	if fallbackErr != nil {
		return "", nil, err
	}
	return content, usage, nil
}

// chatWithWebSearch: Researcher — Gemini grounding atau OpenAI web_search (GPTPlus / Responses).
func (c *Client) chatWithWebSearch(system, user string) (string, *TokenUsage, error) {
	return c.chatWithWebSearchContext(context.Background(), system, user)
}

func (c *Client) chatWithWebSearchContext(ctx context.Context, system, user string) (string, *TokenUsage, error) {
	switch strings.ToLower(strings.TrimSpace(c.provider)) {
	case "gemini", "google":
		return c.chatGeminiSearch(system, user)
	default:
		msgs := []map[string]any{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		}
		// Codex/9router rejects web_search combined with JSON mode. The prompt still
		// requires JSON and callers validate/parse the returned text locally.
		content, usage, _, _, searched, err := c.chatOpenAIResponsesModels(ctx, c.chatUIModels(), msgs, true, false, nil)
		if err != nil {
			return "", usage, err
		}
		if !searched {
			return "", usage, fmt.Errorf("web search tidak dijalankan oleh upstream")
		}
		return content, usage, nil
	}
}

func (c *Client) chatOpenAIWebSearch(system, user string) (string, *TokenUsage, error) {
	model := c.ChatModel()
	if strings.TrimSpace(model) == "" {
		model = "GPTPlus"
	}
	msgs := []map[string]any{}
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": system})
	}
	msgs = append(msgs, map[string]any{"role": "user", "content": user})

	body := buildOpenAIChatBody(model, msgs, false, 0.4, 4500, false)
	body["tools"] = []map[string]any{{"type": "web_search"}}

	rawReq, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/chat/completions", bytes.NewReader(rawReq))
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
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", nil, err
	}
	if res.StatusCode >= 400 {
		// Coba Responses API (beberapa gateway hanya expose web_search di /responses).
		if alt, usage, aerr := c.chatOpenAIResponsesWebSearch(model, system, user); aerr == nil {
			return alt, usage, nil
		}
		return "", nil, fmt.Errorf("web search API status %d: %s", res.StatusCode, truncate(string(raw), 400))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *TokenUsage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", nil, fmt.Errorf("decode web search: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", parsed.Usage, fmt.Errorf("web search: jawaban kosong")
	}
	text := strings.TrimSpace(extractStreamContent(parsed.Choices[0].Message.Content))
	if text == "" {
		return "", parsed.Usage, fmt.Errorf("web search: content kosong")
	}
	return text, parsed.Usage, nil
}

func (c *Client) chatOpenAIResponsesWebSearch(model, system, user string) (string, *TokenUsage, error) {
	body := map[string]any{
		"model": model,
		"input": user,
		"tools": []map[string]any{{"type": "web_search"}},
	}
	if strings.TrimSpace(system) != "" {
		body["instructions"] = system
	}
	rawReq, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/responses", bytes.NewReader(rawReq))
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
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return "", nil, err
	}
	if res.StatusCode >= 400 {
		return "", nil, fmt.Errorf("responses web search status %d: %s", res.StatusCode, truncate(string(raw), 400))
	}
	text := extractResponsesOutputText(raw)
	if strings.TrimSpace(text) == "" {
		return "", nil, fmt.Errorf("responses web search: output kosong")
	}
	var usageWrap struct {
		Usage *TokenUsage `json:"usage"`
	}
	_ = json.Unmarshal(raw, &usageWrap)
	return text, usageWrap.Usage, nil
}

type responsesEnvelope struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Status string `json:"status"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
	Output []struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		Action *struct {
			Sources []struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"sources"`
		} `json:"action"`
		Content []struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			Annotations []struct {
				Type  string `json:"type"`
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"annotations"`
		} `json:"content"`
	} `json:"output"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

type responsesRequestOptions struct {
	PreviousResponseID string
	Instructions       string
	PromptCacheKey     string
	Reasoning          string
	Verbosity          string
	Store              bool
	IncludeSources     bool
}

type responsesMeta struct {
	ID      string
	Sources []ChatSource
}

func (r responsesEnvelope) sources() []ChatSource {
	seen := map[string]bool{}
	out := make([]ChatSource, 0, 8)
	add := func(url, title string) {
		url = strings.TrimSpace(url)
		if url == "" || seen[url] {
			return
		}
		seen[url] = true
		out = append(out, ChatSource{URL: url, Title: strings.TrimSpace(title)})
	}
	for _, item := range r.Output {
		if item.Action != nil {
			for _, source := range item.Action.Sources {
				add(source.URL, source.Title)
			}
		}
		for _, part := range item.Content {
			for _, annotation := range part.Annotations {
				if strings.Contains(strings.ToLower(annotation.Type), "citation") {
					add(annotation.URL, annotation.Title)
				}
			}
		}
	}
	return out
}

func (r responsesEnvelope) textAndSearch() (string, bool) {
	var b strings.Builder
	searched := false
	for _, item := range r.Output {
		if strings.Contains(strings.ToLower(item.Type), "web_search") {
			searched = true
		}
		for _, part := range item.Content {
			if part.Type != "output_text" && part.Type != "text" && part.Text == "" {
				continue
			}
			if part.Text != "" {
				b.WriteString(part.Text)
			}
		}
	}
	return strings.TrimSpace(b.String()), searched
}

func responseUsage(r responsesEnvelope) *TokenUsage {
	if r.Usage == nil {
		return nil
	}
	total := r.Usage.TotalTokens
	if total == 0 {
		total = r.Usage.InputTokens + r.Usage.OutputTokens
	}
	return &TokenUsage{PromptTokens: r.Usage.InputTokens, CompletionTokens: r.Usage.OutputTokens, TotalTokens: total}
}

func responsesInput(messages []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content := msg["content"]
		parts, ok := content.([]map[string]any)
		if !ok {
			out = append(out, map[string]any{"role": role, "content": content})
			continue
		}
		converted := make([]map[string]any, 0, len(parts))
		for _, part := range parts {
			typ, _ := part["type"].(string)
			switch typ {
			case "text":
				converted = append(converted, map[string]any{"type": "input_text", "text": part["text"]})
			case "image_url":
				if image, ok := part["image_url"].(map[string]any); ok {
					converted = append(converted, map[string]any{"type": "input_image", "image_url": image["url"]})
				}
			default:
				converted = append(converted, part)
			}
		}
		out = append(out, map[string]any{"role": role, "content": converted})
	}
	return out
}

func (c *Client) responsesBody(model string, messages []map[string]any, useSearch, jsonMode, stream bool) map[string]any {
	return c.responsesBodyWithOptions(model, messages, useSearch, jsonMode, stream, responsesRequestOptions{})
}

func (c *Client) responsesBodyWithOptions(model string, messages []map[string]any, useSearch, jsonMode, stream bool, opts responsesRequestOptions) map[string]any {
	body := map[string]any{
		"model":  model,
		"input":  responsesInput(messages),
		"store":  opts.Store,
		"stream": stream,
	}
	if v := strings.TrimSpace(opts.PreviousResponseID); v != "" {
		body["previous_response_id"] = v
	}
	if v := strings.TrimSpace(opts.Instructions); v != "" {
		body["instructions"] = v
	}
	if v := strings.TrimSpace(opts.PromptCacheKey); v != "" {
		body["prompt_cache_key"] = v
	}
	maxOutput := 8192
	if raw := strings.TrimSpace(os.Getenv("AI_CHAT_MAX_OUTPUT_TOKENS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1024 {
			maxOutput = parsed
		}
	}
	body["max_output_tokens"] = maxOutput
	// Medium matches GPT-5.5's balanced default. Forcing high on every request
	// adds substantial latency, especially in the multi-stage generate pipeline.
	effort := strings.ToLower(strings.TrimSpace(opts.Reasoning))
	if effort == "" {
		effort = strings.ToLower(strings.TrimSpace(env("AI_CHAT_REASONING", "medium")))
	}
	if effort == "none" || effort == "minimal" || effort == "low" || effort == "medium" || effort == "high" || effort == "xhigh" {
		body["reasoning"] = map[string]any{"effort": effort}
	}
	verbosity := strings.ToLower(strings.TrimSpace(opts.Verbosity))
	if verbosity == "" {
		verbosity = strings.ToLower(strings.TrimSpace(env("AI_CHAT_VERBOSITY", "medium")))
	}
	if verbosity == "low" || verbosity == "medium" || verbosity == "high" {
		body["text"] = map[string]any{"verbosity": verbosity}
	}
	if jsonMode && !useSearch {
		textCfg, _ := body["text"].(map[string]any)
		if textCfg == nil {
			textCfg = map[string]any{}
		}
		textCfg["format"] = map[string]any{"type": "json_object"}
		body["text"] = textCfg
	}
	if useSearch {
		body["tools"] = []map[string]any{{"type": "web_search"}}
		if opts.IncludeSources {
			body["include"] = []string{"web_search_call.action.sources"}
		}
	}
	return body
}

func (c *Client) chatOpenAIResponsesModels(ctx context.Context, models []string, messages []map[string]any, useSearch, jsonMode bool, onDelta func(string) error) (string, *TokenUsage, string, string, bool, error) {
	text, usage, requested, actual, searched, _, err := c.chatOpenAIResponsesModelsWithOptions(ctx, models, messages, useSearch, jsonMode, onDelta, responsesRequestOptions{})
	return text, usage, requested, actual, searched, err
}

func (c *Client) chatOpenAIResponsesModelsWithOptions(ctx context.Context, models []string, messages []map[string]any, useSearch, jsonMode bool, onDelta func(string) error, opts responsesRequestOptions) (string, *TokenUsage, string, string, bool, *responsesMeta, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var last error
	requested := ""
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		requested = model
		text, usage, actual, searched, meta, err := c.chatOpenAIResponsesTryWithOptions(ctx, model, messages, useSearch, jsonMode, onDelta, 0, opts)
		if err == nil {
			if actual == "" {
				actual = model
			}
			return text, usage, model, actual, searched, meta, nil
		}
		last = err
		if ctx.Err() != nil {
			return "", nil, model, "", false, nil, ctx.Err()
		}
		if !isChatUIFallbackErr(err) {
			break
		}
	}
	return "", nil, requested, "", false, nil, last
}

func (c *Client) chatOpenAIResponsesTry(ctx context.Context, model string, messages []map[string]any, useSearch, jsonMode bool, onDelta func(string) error, attempt int) (string, *TokenUsage, string, bool, error) {
	text, usage, actual, searched, _, err := c.chatOpenAIResponsesTryWithOptions(ctx, model, messages, useSearch, jsonMode, onDelta, attempt, responsesRequestOptions{})
	return text, usage, actual, searched, err
}

func (c *Client) chatOpenAIResponsesTryWithOptions(ctx context.Context, model string, messages []map[string]any, useSearch, jsonMode bool, onDelta func(string) error, attempt int, opts responsesRequestOptions) (string, *TokenUsage, string, bool, *responsesMeta, error) {
	stream := onDelta != nil
	rawReq, err := json.Marshal(c.responsesBodyWithOptions(model, messages, useSearch, jsonMode, stream, opts))
	if err != nil {
		return "", nil, "", false, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/responses", bytes.NewReader(rawReq))
	if err != nil {
		return "", nil, "", false, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.currentKey())
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", nil, "", false, nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 8000))
		errMsg := fmt.Errorf("Responses API status %d: %s", res.StatusCode, truncate(string(raw), 500))
		if res.StatusCode == 429 && attempt+1 < len(c.apiKeys) {
			if _, ok := c.rotateKey(); ok {
				return c.chatOpenAIResponsesTryWithOptions(ctx, model, messages, useSearch, jsonMode, onDelta, attempt+1, opts)
			}
		}
		return "", nil, "", false, nil, errMsg
	}
	if !stream {
		raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
		if err != nil {
			return "", nil, "", false, nil, err
		}
		var parsed responsesEnvelope
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", nil, "", false, nil, err
		}
		if parsed.Error != nil || (parsed.Status != "" && parsed.Status != "completed") {
			message := "response tidak selesai"
			if parsed.Error != nil && parsed.Error.Message != "" {
				message = parsed.Error.Message
			}
			return "", responseUsage(parsed), parsed.Model, false, &responsesMeta{ID: parsed.ID, Sources: parsed.sources()}, fmt.Errorf("Responses API: %s", message)
		}
		text, searched := parsed.textAndSearch()
		if text == "" {
			return "", responseUsage(parsed), parsed.Model, searched, &responsesMeta{ID: parsed.ID, Sources: parsed.sources()}, fmt.Errorf("Responses API tidak mengembalikan jawaban")
		}
		if useSearch && !searched {
			return "", responseUsage(parsed), parsed.Model, false, &responsesMeta{ID: parsed.ID, Sources: parsed.sources()}, fmt.Errorf("Responses API tidak menjalankan web search")
		}
		return text, responseUsage(parsed), parsed.Model, searched, &responsesMeta{ID: parsed.ID, Sources: parsed.sources()}, nil
	}
	return parseResponsesStreamWithMeta(ctx, res.Body, useSearch, onDelta)
}

func parseResponsesStream(ctx context.Context, body io.Reader, requireSearch bool, onDelta func(string) error) (string, *TokenUsage, string, bool, error) {
	text, usage, actual, searched, _, err := parseResponsesStreamWithMeta(ctx, body, requireSearch, onDelta)
	return text, usage, actual, searched, err
}

func parseResponsesStreamWithMeta(ctx context.Context, body io.Reader, requireSearch bool, onDelta func(string) error) (string, *TokenUsage, string, bool, *responsesMeta, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	var full strings.Builder
	var usage *TokenUsage
	actual := ""
	searched := false
	terminal := false
	meta := &responsesMeta{}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return full.String(), usage, actual, searched, meta, err
		}
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			terminal = true
			continue
		}
		var event struct {
			Type     string            `json:"type"`
			Delta    string            `json:"delta"`
			Message  string            `json:"message"`
			Response responsesEnvelope `json:"response"`
		}
		if json.Unmarshal([]byte(payload), &event) != nil {
			continue
		}
		if strings.Contains(event.Type, "web_search") {
			searched = true
		}
		switch event.Type {
		case "response.output_text.delta":
			if event.Delta != "" {
				full.WriteString(event.Delta)
				if onDelta != nil {
					if err := onDelta(event.Delta); err != nil {
						return full.String(), usage, actual, searched, meta, err
					}
				}
			}
		case "response.completed":
			terminal = true
			usage = responseUsage(event.Response)
			actual = event.Response.Model
			meta.ID = event.Response.ID
			meta.Sources = event.Response.sources()
			_, responseSearched := event.Response.textAndSearch()
			searched = searched || responseSearched
		case "response.failed", "response.incomplete", "error":
			msg := strings.TrimSpace(event.Message)
			if msg == "" && event.Response.Error != nil {
				msg = event.Response.Error.Message
			}
			if msg == "" {
				msg = event.Type
			}
			return full.String(), usage, actual, searched, meta, fmt.Errorf("Responses stream: %s", msg)
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), usage, actual, searched, meta, err
	}
	if !terminal {
		return full.String(), usage, actual, searched, meta, fmt.Errorf("Responses stream terputus sebelum event selesai")
	}
	text := strings.TrimSpace(full.String())
	if text == "" {
		return "", usage, actual, searched, meta, fmt.Errorf("Responses stream tidak mengembalikan jawaban")
	}
	if requireSearch && !searched {
		return text, usage, actual, false, meta, fmt.Errorf("Responses API tidak menjalankan web search")
	}
	return text, usage, actual, searched, meta, nil
}

func extractResponsesOutputText(raw []byte) string {
	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return ""
	}
	if s := strings.TrimSpace(parsed.OutputText); s != "" {
		return s
	}
	var b strings.Builder
	for _, item := range parsed.Output {
		for _, part := range item.Content {
			if part.Type == "output_text" || part.Type == "text" || part.Text != "" {
				if t := strings.TrimSpace(part.Text); t != "" {
					if b.Len() > 0 {
						b.WriteByte('\n')
					}
					b.WriteString(t)
				}
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func (c *Client) chatOpenAIMessages(messages []map[string]string, jsonMode bool, temp float64, maxTokens int) (string, *TokenUsage, error) {
	anyMsgs := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		anyMsgs = append(anyMsgs, map[string]any{"role": m["role"], "content": m["content"]})
	}
	return c.chatOpenAIMessagesAny(anyMsgs, jsonMode, temp, maxTokens)
}

func (c *Client) chatOpenAIMessagesAny(messages []map[string]any, jsonMode bool, temp float64, maxTokens int) (string, *TokenUsage, error) {
	content, usage, _, err := c.chatOpenAIMessagesAnyModels(chatModelFallbacks(c.model), messages, jsonMode, temp, maxTokens, isBrokenCodexRouteErr)
	return content, usage, err
}

func (c *Client) chatOpenAIMessagesAnyModels(models []string, messages []map[string]any, jsonMode bool, temp float64, maxTokens int, retryNext func(error) bool) (string, *TokenUsage, string, error) {
	return c.chatOpenAIMessagesAnyModelsCtx(context.Background(), models, messages, jsonMode, temp, maxTokens, retryNext)
}

func (c *Client) chatOpenAIMessagesAnyModelsCtx(ctx context.Context, models []string, messages []map[string]any, jsonMode bool, temp float64, maxTokens int, retryNext func(error) bool) (string, *TokenUsage, string, error) {
	if retryNext == nil {
		retryNext = isBrokenCodexRouteErr
	}
	var last error
	used := ""
	for _, model := range models {
		used = model
		content, usage, err := c.chatOpenAIMessagesTryCtx(ctx, model, messages, jsonMode, temp, maxTokens, 0)
		if err == nil {
			return content, usage, model, nil
		}
		last = err
		if !retryNext(err) {
			return "", nil, model, err
		}
	}
	return "", nil, used, last
}

func chatModelFallbacks(primary string) []string {
	primary = strings.TrimSpace(primary)
	if primary == "" {
		primary = "cx/gpt-5.5"
	}
	out := []string{primary}
	seen := map[string]bool{strings.ToLower(primary): true}
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m == "" || seen[strings.ToLower(m)] {
			return
		}
		seen[strings.ToLower(m)] = true
		out = append(out, m)
	}
	// GPTPlus kadang di-route ke Codex gpt-5.6-sol yang error "newer version of Codex".
	add("cx/gpt-5.5")
	add("cx/gpt-5.6-terra")
	add("cx/gpt-5.4")
	return out
}

// chatUIModels: Chat UI memakai ChatGPT combo (GPTPlus / tiga-awan), bukan Codex cx/*.
func (c *Client) chatUIModels() []string {
	primary := ""
	if c != nil {
		primary = strings.TrimSpace(c.chatModel)
		if primary == "" {
			primary = c.ChatModel()
		}
	}
	if primary == "" {
		primary = "GPTPlus"
	}
	out := make([]string, 0, 4)
	seen := map[string]bool{}
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m == "" || seen[strings.ToLower(m)] {
			return
		}
		seen[strings.ToLower(m)] = true
		out = append(out, m)
	}
	add(primary)
	add("GPTPlus")
	add("tiga-awan")
	if c != nil {
		add(c.model)
	}
	return out
}

func isReasoningChatModel(m string) bool {
	s := strings.ToLower(strings.TrimSpace(m))
	if s == "" {
		return false
	}
	if isChatComboModel(s) {
		return false
	}
	return strings.Contains(s, "gpt-5") ||
		strings.Contains(s, "o1") ||
		strings.Contains(s, "o3") ||
		strings.Contains(s, "o4") ||
		strings.Contains(s, "terra") ||
		strings.Contains(s, "luna") ||
		strings.Contains(s, "sol") ||
		strings.Contains(s, "codex")
}

func buildOpenAIChatBody(model string, messages []map[string]any, jsonMode bool, temp float64, maxTokens int, stream bool) map[string]any {
	body := map[string]any{
		"model":    model,
		"messages": messages,
	}
	combo := isChatComboModel(model)
	reasoning := isReasoningChatModel(model)
	if jsonMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	if stream {
		body["stream"] = true
		if !combo {
			body["stream_options"] = map[string]any{"include_usage": true}
		}
	}
	if combo {
		return body
	}
	if !reasoning && temp > 0 {
		body["temperature"] = temp
	}
	if maxTokens > 0 {
		if reasoning {
			body["max_completion_tokens"] = maxTokens
		} else {
			body["max_tokens"] = maxTokens
		}
	}
	return body
}

func isChatUIFallbackErr(err error) bool {
	if isBrokenCodexRouteErr(err) {
		return true
	}
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "does not exist") ||
		strings.Contains(s, "model_not_found") ||
		strings.Contains(s, "unknown model") ||
		strings.Contains(s, "invalid model") ||
		strings.Contains(s, "unsupported") ||
		strings.Contains(s, "max_tokens") ||
		strings.Contains(s, "max_completion_tokens") ||
		strings.Contains(s, "stream_options") ||
		strings.Contains(s, "temperature") ||
		strings.Contains(s, "tidak menjalankan web search") ||
		strings.Contains(s, "tidak mengembalikan jawaban") ||
		strings.Contains(s, "response_format") ||
		strings.Contains(s, "json_object")
}

func isBrokenCodexRouteErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "gpt-5.6-sol") ||
		strings.Contains(s, "newer version of codex") ||
		strings.Contains(s, "codex/gpt-5.6-sol") ||
		(strings.Contains(s, "codex") && strings.Contains(s, "upgrade"))
}

func (c *Client) chatOpenAIMessagesTry(model string, messages []map[string]any, jsonMode bool, temp float64, maxTokens int, attempt int) (string, *TokenUsage, error) {
	return c.chatOpenAIMessagesTryCtx(context.Background(), model, messages, jsonMode, temp, maxTokens, attempt)
}

func (c *Client) chatOpenAIMessagesTryCtx(ctx context.Context, model string, messages []map[string]any, jsonMode bool, temp float64, maxTokens int, attempt int) (string, *TokenUsage, error) {
	if strings.TrimSpace(model) == "" {
		model = c.model
	}
	rawReq, _ := json.Marshal(buildOpenAIChatBody(model, messages, jsonMode, temp, maxTokens, false))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/chat/completions", bytes.NewReader(rawReq))
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
				return c.chatOpenAIMessagesTryCtx(ctx, model, messages, jsonMode, temp, maxTokens, attempt+1)
			}
		}
		return "", nil, errMsg
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content          json.RawMessage `json:"content"`
				ReasoningContent json.RawMessage `json:"reasoning_content"`
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
	msg := parsed.Choices[0].Message
	content := strings.TrimSpace(extractStreamContent(msg.Content))
	if content == "" {
		content = strings.TrimSpace(extractStreamContent(msg.ReasoningContent))
	}
	if content == "" {
		return "", nil, fmt.Errorf("AI tidak mengembalikan jawaban")
	}
	return content, parsed.Usage, nil
}

func (c *Client) chatOpenAIStream(ctx context.Context, messages []map[string]any, temp float64, maxTokens int, onDelta func(string) error) (string, *TokenUsage, string, error) {
	return c.chatOpenAIStreamModels(ctx, c.chatUIModels(), messages, temp, maxTokens, onDelta, isChatUIFallbackErr)
}

func (c *Client) chatOpenAIStreamModels(ctx context.Context, models []string, messages []map[string]any, temp float64, maxTokens int, onDelta func(string) error, retryNext func(error) bool) (string, *TokenUsage, string, error) {
	if retryNext == nil {
		retryNext = isBrokenCodexRouteErr
	}
	var last error
	used := ""
	for _, model := range models {
		used = model
		full, usage, err := c.chatOpenAIStreamTry(ctx, model, messages, temp, maxTokens, 0, onDelta)
		if err == nil {
			return full, usage, model, nil
		}
		last = err
		if ctx.Err() != nil {
			return "", nil, model, ctx.Err()
		}
		if !retryNext(err) {
			return "", nil, model, err
		}
	}
	if used == "" && c != nil {
		used = c.ChatModel()
	}
	return "", nil, used, last
}

func (c *Client) chatOpenAIStreamTry(ctx context.Context, model string, messages []map[string]any, temp float64, maxTokens int, attempt int, onDelta func(string) error) (string, *TokenUsage, error) {
	if strings.TrimSpace(model) == "" {
		model = c.model
	}
	rawReq, _ := json.Marshal(buildOpenAIChatBody(model, messages, false, temp, maxTokens, true))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/chat/completions", bytes.NewReader(rawReq))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.currentKey())
	req.Header.Set("Accept", "text/event-stream")

	res, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 8000))
		errMsg := fmt.Errorf("AI API status %d: %s", res.StatusCode, truncate(string(body), 400))
		if res.StatusCode == 429 && attempt+1 < len(c.apiKeys) {
			if _, ok := c.rotateKey(); ok {
				return c.chatOpenAIStreamTry(ctx, model, messages, temp, maxTokens, attempt+1, onDelta)
			}
		}
		return "", nil, errMsg
	}

	scanner := bufio.NewScanner(res.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	var full strings.Builder
	var usage *TokenUsage
	for scanner.Scan() {
		if ctx.Err() != nil {
			return full.String(), usage, ctx.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		delta, chunkUsage := parseOpenAIStreamChunk(payload)
		if chunkUsage != nil {
			usage = chunkUsage
		}
		if delta == "" {
			continue
		}
		full.WriteString(delta)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return full.String(), usage, err
			}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		if full.Len() == 0 {
			return "", usage, err
		}
	}
	out := strings.TrimSpace(full.String())
	if out == "" {
		return "", usage, fmt.Errorf("AI tidak mengembalikan jawaban")
	}
	return out, usage, nil
}

func parseOpenAIStreamChunk(payload string) (string, *TokenUsage) {
	var parsed struct {
		Choices []struct {
			Delta struct {
				Content json.RawMessage `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *TokenUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return "", nil
	}
	var delta string
	if len(parsed.Choices) > 0 {
		delta = extractStreamContent(parsed.Choices[0].Delta.Content)
	}
	return delta, parsed.Usage
}

func extractStreamContent(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Text != "" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	var obj struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Text != "" {
		return obj.Text
	}
	return ""
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
