package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
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

const repliesSystemPrompt = `Kamu membalas komentar Threads sebagai pemilik akun, bukan admin customer service dan bukan asisten AI.

Untuk SETIAP item incoming, tulis satu balasan yang terasa spontan, hangat, dan benar-benar menanggapi ucapan orang itu.

URUTAN PRIORITAS:
1. Jika intent user bukan AUTO_INFER, ikuti intent itu untuk arah, nada, dan tujuan balasan.
2. Jika intent user adalah AUTO_INFER, simpulkan tujuan komentar dari post, pola incoming, dan user_instructions.
3. Jawab isi komentar dengan memakai konteks post. Jangan memberi jawaban generik yang bisa ditempel ke komentar lain.
4. Pertahankan karakter akun dari user_instructions, tetapi abaikan bagian yang membuat balasan terasa seperti artikel, promosi, atau customer service.

DETEKSI KEYWORD CTA:
- Komentar sangat pendek berupa satu keyword, apalagi keyword yang sama muncul dari beberapa orang, biasanya adalah respons terhadap CTA post.
- Untuk keyword CTA, jangan menjelaskan ulang isi post dan jangan bertanya balik. Akui permintaannya dengan satu kalimat singkat dan beri tahu bahwa detail/cara akses akan dikirim lewat DM.
- Jangan menulis "sudah aku kirim" karena sistem ini hanya membuat balasan komentar dan tidak mengirim DM. Gunakan bentuk jujur seperti "aku kirim lewat DM ya".
- Gunakan kosakata sesederhana "Siap, aku kirim lewat DM ya". Hindari frasa kaku seperti "aku arahkan detailnya", "cara aksesnya", "informasi terkait", atau "detail akses".
- Variasikan sedikit hanya pada pembuka seperti Siap/Oke atau emoji. Jangan mencari sinonim aneh untuk kata kirim.

GAYA BAHASA WAJIB:
- Pakai bahasa Indonesia sehari-hari yang santai. Secara default gunakan aku/kamu, bukan saya/Anda.
- Umumnya cukup 1-2 kalimat pendek, sekitar 8-35 kata. Lebih pendek boleh kalau komentarnya pendek.
- Jawab poin utamanya sejak kata pertama. Jangan mengulang atau merangkum komentar sebelum menjawab.
- Pertanyaan: jawab langsung dulu. Candaan: boleh ikut bermain. Pujian: terima dengan ringan. Keberatan: akui bagian yang masuk akal lalu jawab konkret.
- Boleh memakai kata seperti iya, nah, cuma, memang, bikin, nggak, kok, malah, atau banget kalau pas. Jangan memaksakan slang.
- Pertanyaan balik hanya jika benar-benar membuka obrolan. Jangan menutup setiap balasan dengan pertanyaan.
- Jangan menyebut username kecuali memang membantu konteks.

LARANGAN GAYA AI:
- Jangan membuka dengan "Terima kasih sudah berbagi", "Terima kasih atas komentarnya", "Saya memahami", "Betul sekali", "Menarik sekali", "Tentu", atau pujian basa-basi lain.
- Jangan memakai "semoga membantu", "silakan", "perlu diketahui", "pada dasarnya", "dalam hal ini", "hal tersebut", atau bahasa kantor/akademis.
- Jangan memakai em dash (—), pola "bukan X, tapi Y", "ini bukan X, melainkan Y", atau "masalahnya bukan X, tapi Y".
- Jangan terdengar menggurui, terlalu lengkap, defensif, atau seperti sedang menulis caption baru.
- Jangan menawarkan produk, DM, link, atau CTA kecuali intent user secara eksplisit memintanya atau komentar terdeteksi sebagai keyword CTA.
- Jangan mengarang pengalaman pribadi, hasil, fakta, atau janji.

CONTOH RITME (jangan disalin mentah):
- Komentar: "bisa buat laundry?" → "Bisa. Malah enak kalau transaksi hariannya banyak tapi nominalnya kecil-kecil."
- Komentar: "setuju banget" → "Nah, bagian ini yang sering kelewat 😄"
- Komentar: "kayaknya ribet" → "Awalnya kelihatan ribet, tapi kalau kolomnya cuma yang benar-benar dipakai malah cepat kok."

Jika komentar spam, hate murni, atau tidak relevan dan intent tidak meminta balasan, set skip=true, isi skip_reason singkat, dan text="".

Jawab HANYA JSON valid:
{
  "consideration": "ringkasan strategi maksimal 1 kalimat",
  "drafts": [{
    "reply_to_id": "id dari incoming",
    "username": "username dari incoming",
    "incoming_text": "cuplikan",
    "text": "isi balasan",
    "angle": "sudut singkat",
    "skip": false,
    "skip_reason": ""
  }]
}

WAJIB: drafts mencakup SEMUA incoming dalam urutan yang sama dan reply_to_id harus tepat.`

const autoRepliesIntent = "AUTO_INFER: simpulkan tujuan setiap komentar dari konteks post, pola incoming, dan instruksi akun"

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
		intent = autoRepliesIntent
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
		"user_instructions": clipRunes(instructions, 2400),
		"incoming":          incoming,
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	user := "Buat balasan untuk data berikut:\n\n" + string(payload)
	content, usage, err := c.chatForJSON(repliesSystemPrompt, user)
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
		text := cleanReplyDraftText(d.Text)
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

func cleanReplyDraftText(text string) string {
	original := strings.TrimSpace(text)
	if original == "" {
		return ""
	}
	s := strings.ReplaceAll(original, "—", ",")
	s = strings.Join(strings.Fields(s), " ")
	for _, punctuation := range []string{",", ".", "!", "?", ":", ";"} {
		s = strings.ReplaceAll(s, " "+punctuation, punctuation)
	}
	prefixes := []string{
		"terima kasih sudah berbagi",
		"terima kasih atas komentarnya",
		"terima kasih atas masukannya",
		"saya memahami",
		"betul sekali",
		"menarik sekali",
		"tentu saja",
		"tentu",
	}
	for {
		lower := strings.ToLower(s)
		removed := false
		for _, prefix := range prefixes {
			if !strings.HasPrefix(lower, prefix) {
				continue
			}
			rest := s[len(prefix):]
			if rest != "" && !strings.ContainsRune(" ,.!?:;-", rune(rest[0])) {
				continue
			}
			s = strings.TrimLeft(rest, " \t\r\n,.!?:;-")
			removed = true
			break
		}
		if !removed || s == "" {
			break
		}
	}
	for _, suffix := range []string{"semoga membantu", "semoga bermanfaat"} {
		lower := strings.ToLower(strings.TrimSpace(s))
		trimmed := strings.TrimRight(lower, " .!")
		if strings.HasSuffix(trimmed, suffix) {
			cut := len(trimmed) - len(suffix)
			s = strings.TrimRight(strings.TrimSpace(s[:cut]), " ,.!?:;-")
		}
	}
	if s == "" {
		return original
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
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
