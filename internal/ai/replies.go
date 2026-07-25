package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

type IncomingReply struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Text     string `json:"text"`
}

type RepliesRequest struct {
	PostID       string          `json:"post_id"`
	PostText     string          `json:"post_text"`
	Intent       string          `json:"intent"`                 // arah balasan dari user
	Instructions string          `json:"instructions,omitempty"` // override opsional
	Incoming     []IncomingReply `json:"incoming"`
	Limit        int             `json:"limit"` // max balasan digenerate
}

type ReplyDraft struct {
	ReplyToID    string `json:"reply_to_id"`
	Username     string `json:"username"`
	IncomingText string `json:"incoming_text"`
	Text         string `json:"text"`
	Angle        string `json:"angle,omitempty"`
	Skip         bool   `json:"skip,omitempty"`
	SkipReason   string `json:"skip_reason,omitempty"`
}

type RepliesResult struct {
	PostID        string       `json:"post_id"`
	Intent        string       `json:"intent"`
	Consideration string       `json:"consideration,omitempty"`
	Drafts        []ReplyDraft `json:"drafts"`
	Model         string       `json:"model"`
	Provider      string       `json:"provider"`
	Usage         *TokenUsage  `json:"usage,omitempty"`
	Quota         *QuotaStatus `json:"quota,omitempty"`
}

func (c *Client) GenerateReplies(mem Memory, req RepliesRequest) (*RepliesResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("AI belum dikonfigurasi — set AI_API_KEY di .env")
	}
	if c.quota != nil {
		if err := c.quota.check(); err != nil {
			return nil, err
		}
	}

	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		return nil, fmt.Errorf("arah balasan (intent) wajib diisi")
	}
	postText := strings.TrimSpace(req.PostText)
	if postText == "" {
		return nil, fmt.Errorf("teks post kosong")
	}

	incoming := normalizeIncoming(req.Incoming)
	if len(incoming) == 0 {
		return nil, fmt.Errorf("tidak ada komentar untuk dibalas")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 12
	}
	if limit > 20 {
		limit = 20
	}
	if len(incoming) > limit {
		incoming = incoming[:limit]
	}

	instructions := strings.TrimSpace(req.Instructions)
	if instructions == "" {
		instructions = strings.TrimSpace(mem.Instructions)
	}
	niches := NicheList(mem)
	nicheLine := strings.Join(niches, ", ")
	if nicheLine == "" {
		nicheLine = "(belum diisi)"
	}
	brand := strings.TrimSpace(mem.Brand)

	payload, err := json.MarshalIndent(map[string]any{
		"post_id":           strings.TrimSpace(req.PostID),
		"post_text":         clipRunes(postText, 900),
		"intent":            intent,
		"brand":             brand,
		"niche":             nicheLine,
		"user_instructions": instructions,
		"incoming":          incoming,
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	system := `Kamu membalas komentar di Threads atas nama pemilik akun.

Tugas: untuk SETIAP item incoming, tulis 1 balasan pendek yang:
1) Sesuai INTENT user (arah/tone/tujuan balasan) — ini prioritas utama.
2) Nyambung ke isi POST dan komentar orang itu (jangan generic).
3) Bunyi manusia, santai, spesifik — bukan AI cringe / basabasi.
4) Panjang ideal 1–3 kalimat (maks ~280 karakter). Boleh 1 pertanyaan kalau intent minta engagement.
5) Jangan spam link, jangan janji palsu, jangan menyerang.
6) Kalau komentar spam/hate/tidak relevan dan intent tidak minta dibalas: set "skip": true + skip_reason singkat, text boleh "".

Jawab HANYA JSON valid:
{
  "consideration": "1-2 kalimat strategi balasan",
  "drafts": [{
    "reply_to_id": "id dari incoming",
    "username": "@user",
    "incoming_text": "cuplikan",
    "text": "isi balasan",
    "angle": "sudut singkat",
    "skip": false,
    "skip_reason": ""
  }]
}

WAJIB: drafts harus cover SEMUA incoming (urutan sama). reply_to_id harus tepat.`

	user := "Buat balasan untuk data berikut:\n\n" + string(payload)
	content, usage, err := c.chatForJSON(system, user)
	if err != nil {
		return nil, err
	}
	if c.quota != nil {
		c.quota.record(usage)
	}

	var parsed struct {
		Consideration string `json:"consideration"`
		Drafts        []struct {
			ReplyToID    string `json:"reply_to_id"`
			Username     string `json:"username"`
			IncomingText string `json:"incoming_text"`
			Text         string `json:"text"`
			Angle        string `json:"angle"`
			Skip         bool   `json:"skip"`
			SkipReason   string `json:"skip_reason"`
		} `json:"drafts"`
	}
	raw := extractJSON(content)
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("AI balasan tidak valid JSON: %w", err)
	}

	byID := map[string]IncomingReply{}
	for _, in := range incoming {
		byID[in.ID] = in
	}

	drafts := make([]ReplyDraft, 0, len(incoming))
	seen := map[string]bool{}
	for _, d := range parsed.Drafts {
		id := strings.TrimSpace(d.ReplyToID)
		in, ok := byID[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		text := strings.TrimSpace(d.Text)
		if utf8.RuneCountInString(text) > 480 {
			text = string([]rune(text)[:480])
		}
		drafts = append(drafts, ReplyDraft{
			ReplyToID:    id,
			Username:     emptyFallback(strings.TrimSpace(d.Username), in.Username),
			IncomingText: emptyFallback(strings.TrimSpace(d.IncomingText), clipRunes(in.Text, 160)),
			Text:         text,
			Angle:        strings.TrimSpace(d.Angle),
			Skip:         d.Skip || text == "",
			SkipReason:   strings.TrimSpace(d.SkipReason),
		})
	}
	// Pastikan semua incoming punya slot (fallback kalau AI miss)
	for _, in := range incoming {
		if seen[in.ID] {
			continue
		}
		drafts = append(drafts, ReplyDraft{
			ReplyToID:    in.ID,
			Username:     in.Username,
			IncomingText: clipRunes(in.Text, 160),
			Skip:         true,
			SkipReason:   "AI tidak menghasilkan draf",
		})
	}

	out := &RepliesResult{
		PostID:        strings.TrimSpace(req.PostID),
		Intent:        intent,
		Consideration: strings.TrimSpace(parsed.Consideration),
		Drafts:        drafts,
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

func normalizeIncoming(in []IncomingReply) []IncomingReply {
	var out []IncomingReply
	seen := map[string]bool{}
	for _, r := range in {
		id := strings.TrimSpace(r.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, IncomingReply{
			ID:       id,
			Username: strings.TrimSpace(strings.TrimLeft(r.Username, "@")),
			Text:     strings.TrimSpace(r.Text),
		})
	}
	return out
}
