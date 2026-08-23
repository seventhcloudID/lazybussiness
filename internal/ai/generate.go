package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type GenerateRequest struct {
	Topic        string `json:"topic"`
	Instructions string `json:"instructions"` // override one-shot; else use memory
	Count        int    `json:"count"`
	IgnoreNiche  bool   `json:"ignore_niche"` // true = brief-only, skip memory niche
}

type GenerateResult struct {
	Consideration string           `json:"consideration"`
	DailyFocus    *DailyFocus      `json:"daily_focus,omitempty"`
	Drafts        []GeneratedDraft `json:"drafts"`
	LessonsUsed   Lessons          `json:"lessons_used"`
	Model         string           `json:"model"`
	Provider      string           `json:"provider"`
	Usage         *TokenUsage      `json:"usage,omitempty"`
	Quota         *QuotaStatus     `json:"quota,omitempty"`

	// Pipeline v2 extras (optional for backward-compatible clients)
	Pipeline     *PipelineMeta        `json:"pipeline,omitempty"`
	Package      *GenEditorialPackage `json:"package,omitempty"`
	Research     *ResearchEvidence    `json:"research,omitempty"`
	CoverBrief   string               `json:"cover_brief,omitempty"`
	CoverTitle   string               `json:"cover_title,omitempty"`
	Sources      []string             `json:"sources,omitempty"`
	StrategyView *StrategyView        `json:"strategy,omitempty"`
	Critic       *GenCriticReport     `json:"critic,omitempty"`
}

// GenerateContent moved to generate_pipeline.go (v2 hybrid pipeline).

func (c *Client) generateUtasDrafts(snapshot map[string]any, mem Memory, req GenerateRequest) (*GenerateResult, error) {
	instructions := strings.TrimSpace(req.Instructions)
	if instructions == "" {
		instructions = strings.TrimSpace(mem.Instructions)
	}
	instructions = emptyFallback(instructions, DefaultGenerateInstructions)

	niches := NicheList(mem)
	nicheLine := strings.Join(niches, "\n")
	if nicheLine == "" {
		nicheLine = "(belum diisi user — minta topik umum sesuai instruksi, JANGAN menebak niche)"
	}
	if req.IgnoreNiche {
		niches = nil
		nicheLine = "(abaikan niche akun — ikuti topic_hint + user_instructions saja)"
	}

	payloadMap := map[string]any{
		"user_niches":         niches,
		"user_instructions":   instructions,
		"topic_hint":          strings.TrimSpace(req.Topic),
		"today":               time.Now().Format("2006-01-02"),
		"draft_count":         req.Count,
		"recent_drafts_avoid": recentDraftHints(mem.History, 6),
		"user_feedback":       trimFeedback(mem.Feedback, 8),
		"ignore_niche":        req.IgnoreNiche,
	}
	if compact := compactGenerateSnapshot(snapshot); compact != nil {
		payloadMap["connected_account"] = compact
	}

	payload, err := json.MarshalIndent(payloadMap, "", "  ")
	if err != nil {
		return nil, err
	}

	system := buildGenerateSystemPrompt(nicheLine, instructions, req.Count)
	userLead := "Generate utas Threads yang BERPOTENSI VIRAL (bukan filler). Niche + instruksi user; jangan mengulang draf lama:\n\n"
	if req.IgnoreNiche {
		userLead = "Generate utas Threads dari TOPIK + instruksi user saja. ABAIKAN niche akun. Jangan mengulang draf lama:\n\n"
	}
	user := userLead + string(payload)

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
	var out GenerateResult
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("gagal parse hasil generate: %w", err)
	}
	for i := range out.Drafts {
		normalizeThreadDraft(&out.Drafts[i], i)
	}
	if out.DailyFocus != nil && out.DailyFocus.Date == "" {
		out.DailyFocus.Date = time.Now().Format("2006-01-02")
	}
	out.LessonsUsed = Lessons{}
	out.Model = c.model
	out.Provider = c.provider
	out.Usage = usage
	if c.quota != nil {
		q := c.quota.status(c.provider, c.model)
		out.Quota = &q
	}
	return &out, nil
}

func normalizeThreadDraft(d *GeneratedDraft, i int) {
	if d.Key == "" {
		d.Key = fmt.Sprintf("%d-%d", time.Now().Unix(), i)
	}
	// Clean each part: unescape literal \n, trim, drop empties.
	cleaned := make([]string, 0, len(d.Parts))
	for _, p := range d.Parts {
		p = normalizeNewlines(p)
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, spaceSentences(p))
		}
	}
	d.Parts = cleaned

	if len(d.Parts) == 0 && strings.TrimSpace(d.Draft) != "" {
		d.Parts = splitThreadChunks(normalizeNewlines(d.Draft))
	}
	// Model often dumps whole utas into 1 string — pecah biar ada space antar bagian.
	if len(d.Parts) == 1 {
		if split := splitThreadChunks(d.Parts[0]); len(split) > 1 {
			d.Parts = split
		}
	}
	// Always rebuild draft with blank line between parts (untuk salin / Buat Post).
	if len(d.Parts) > 0 {
		// Threads limit 500 karakter per post — pecah / potong biar valid.
		d.Parts = enforcePartLimit(d.Parts, 500)
		d.Draft = strings.Join(d.Parts, "\n\n")
	}
	if d.Format == "" || strings.EqualFold(d.Format, "TEXT") {
		if len(d.Parts) > 1 {
			d.Format = "THREAD"
		} else {
			d.Format = "TEXT"
		}
	}
	if d.Hook == "" && len(d.Parts) > 0 {
		d.Hook = clipRunes(d.Parts[0], 120)
	}
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\\n", "\n")
	return s
}

// spaceSentences sisipkan baris kosong antar kalimat kalau model kirim blok padat.
func spaceSentences(s string) string {
	s = strings.TrimSpace(normalizeNewlines(s))
	if s == "" {
		return s
	}
	// Sudah ada enter — rapikan jadi satu baris kosong antar paragraf.
	if strings.Contains(s, "\n") {
		var paras []string
		for _, p := range strings.Split(s, "\n") {
			p = strings.TrimSpace(p)
			if p != "" {
				paras = append(paras, p)
			}
		}
		return strings.Join(paras, "\n\n")
	}
	// Satu baris padat → pecah di akhir kalimat.
	var parts []string
	start := 0
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		// jangan pecah angka desimal kasar (1.5)
		if r == '.' && i > 0 && i+1 < len(runes) {
			prev, next := runes[i-1], runes[i+1]
			if prev >= '0' && prev <= '9' && next >= '0' && next <= '9' {
				continue
			}
		}
		end := i + 1
		for end < len(runes) && (runes[end] == '"' || runes[end] == '\'' || runes[end] == '”' || runes[end] == '’') {
			end++
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			parts = append(parts, chunk)
		}
		// skip spaces after punctuation
		j := end
		for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
			j++
		}
		start = j
		i = j - 1
	}
	tail := strings.TrimSpace(string(runes[start:]))
	if tail != "" {
		parts = append(parts, tail)
	}
	if len(parts) <= 1 {
		return s
	}
	return strings.Join(parts, "\n\n")
}

// splitThreadChunks memecah teks utas jadi bagian terpisah (blank line / penanda).
func splitThreadChunks(s string) []string {
	s = strings.TrimSpace(normalizeNewlines(s))
	if s == "" {
		return nil
	}
	var out []string
	for _, c := range strings.Split(s, "\n\n") {
		c = strings.TrimSpace(c)
		if c != "" {
			out = append(out, c)
		}
	}
	if len(out) > 1 {
		return out
	}
	// Satu blok padat — coba pecah per baris kalau cukup "bagian".
	lines := strings.Split(s, "\n")
	var nonempty []string
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			nonempty = append(nonempty, ln)
		}
	}
	if len(nonempty) >= 4 {
		return nonempty
	}
	return []string{s}
}

// enforcePartLimit memastikan tiap bagian ≤ max rune (batas Threads = 500).
func enforcePartLimit(parts []string, max int) []string {
	if max <= 0 {
		return parts
	}
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if utf8.RuneCountInString(p) <= max {
			out = append(out, p)
			continue
		}
		// Pecah di batas kalimat / spasi supaya tidak satu blok >500.
		out = append(out, splitByLimit(p, max)...)
	}
	return out
}

func splitByLimit(s string, max int) []string {
	var out []string
	rest := strings.TrimSpace(s)
	for rest != "" {
		if utf8.RuneCountInString(rest) <= max {
			out = append(out, rest)
			break
		}
		chunk := clipRunes(rest, max)
		// Mundur ke pemisah alami biar tidak potong di tengah kata.
		cut := len([]rune(chunk))
		runes := []rune(chunk)
		for i := len(runes) - 1; i > len(runes)*2/3; i-- {
			switch runes[i] {
			case '.', '!', '?', '\n', ';':
				cut = i + 1
				goto done
			case ' ', ',':
				cut = i
			}
		}
	done:
		if cut < 1 {
			cut = len(runes)
		}
		part := strings.TrimSpace(string(runes[:cut]))
		if part != "" {
			out = append(out, part)
		}
		rest = strings.TrimSpace(string([]rune(rest)[cut:]))
	}
	return out
}

func buildGenerateSystemPrompt(nicheLine, instructions string, count int) string {
	sharedRules := fmt.Sprintf(`Jawab HANYA JSON valid:
{
  "consideration": "2-4 kalimat: kenapa angle ini punya peluang viral + niche mana yang dipakai + apa yang dihindari biar tidak ngulang",
  "daily_focus": {
    "date": "YYYY-MM-DD",
    "focus": "",
    "avoid_today": ["topik/angle yang dihindari biar tidak mengulang"],
    "notes": "catatan singkat"
  },
  "drafts": [{
    "title": "judul internal singkat",
    "hook": "ringkas hook bagian 1",
    "parts": [
      "teks bagian 1 (starter/hook)",
      "teks bagian 2",
      "teks bagian 3",
      "teks bagian 4"
    ],
    "draft": "seluruh utas digabung, dipisah baris kosong antar bagian (untuk salin cepat)",
    "angle": "sudut pandang yang bikin beda dari konten edukasi biasa",
    "format": "THREAD",
    "why": "kenapa berpotensi viral + niche mana yang dipakai",
    "based_on": "niche + instruksi + topik",
    "risk": "risiko terdengar dogeng/dirujak + cara dihindari"
  }]
}

Aturan ketat:
- Hasilkan tepat %d opsi utas (drafts) — tiap opsi HARUS beda angle viral, bukan parafrase.
- Tiap draft WAJIB "parts" = array 4–6 string TERPISAH (satu elemen = satu post di utas).
  JANGAN satukan semua bagian jadi satu paragraf / satu string.
  Field "draft" = join parts dengan "\\n\\n" (baris kosong antar bagian).
- BATAS KERAS: tiap elemen parts maksimal 500 karakter (batas Threads).
- Tiap bagian 2–3 kalimat saja. Pisah kalimat dengan baris baru (\\n).
- JANGAN mengulang topik/angle di recent_drafts_avoid.
- Hormati user_feedback jelek.
- NICHE hanya dari user_niches. daily_focus.focus biarkan string kosong.
- Bahasa Indonesia natural. Default: tanpa bullet/nummering, tanpa "lu/gue", tanpa list tips.
- Anti-dogeng: jangan frasa AI ("mari kita bahas", "di era digital", "banyak yang belum sadar").`, count)

	return fmt.Sprintf(`Kamu adalah penulis utas Threads yang nulis konten BERPOTENSI VIRAL — bukan filler, bukan "asal ada draf".
Utas = rantai post pendek yang nyambung (bukan 1 caption panjang).

Prioritas (urut):
1) Berpotensi viral di niche (stop scroll, bikin reply/share).
2) Bunyi RIL, bukan dogeng/AI cringe.
3) Spesifik & bisa dibela — bukan generic edukasi.

Filter WAJIB: kalau angle bisa diganti niche lain tanpa berubah artinya → BUANG.
Kalau hook cuma "penjelasan topik" tanpa tensi → BUANG.

NICHE AKUN (boleh multi; tiap draf pilih 1 niche/gabungan masuk akal):
"""
%s
"""

Instruksi user (WAJIB dipatuhi jika ada):
"""
%s
"""

%s

POTENSI VIRAL: kontradiksi, mekanisme tersembunyi, contoh konkret, pertanyaan yang memaksa reply, insight enak di-share.
HOOK: kalimat pertama nahan scroll. Tension dulu, penjelasan belakangan.`, nicheLine, instructions, sharedRules)
}

// DefaultGenerateInstructions is a solid baseline for edukasi + utas RAW anti-dogeng.
const DefaultGenerateInstructions = `FORMAT
- Output: utas berantai 4–6 bagian (bukan 1 post panjang).
- Tiap bagian maksimal 500 karakter (batas Threads) — hitung ketat.
- Tiap bagian 2–3 kalimat saja. Pisah tiap kalimat dengan enter (baris kosong) biar enak dibaca — jangan digabung jadi satu paragraf padat.
- Bagian 1 = HOOK keras (bukan soft open / definisi / latar belakang).
- Bagian tengah = daging: mekanisme, sebab-akibat, konteks nyata.
- Bagian akhir = penutup tajam atau pertanyaan terbuka (bukan jualan).

POTENSI VIRAL (wajib)
- Jangan bikin konten "aman tapi sepi". Harus ada alasan orang stop scroll / reply / share.
- Angle wajib salah satu: kontradiksi, mekanisme tersembunyi, contoh konkret mengejutkan, insight yang enak di-share.
- Kalau bisa diganti niche lain tanpa berubah artinya → terlalu generic, ganti angle.
- Harus ada 1 poin yang masih diingat 3 detik setelah baca.

HOOK (wajib kuat)
- Kalimat pertama harus nahan scroll: kontradiksi, klaim tajam, kejadian konkret, atau "yang dikira X padahal Y".
- Dilarang buka dengan: definisi, "ada yang menarik", "banyak orang", ringkasan ensiklopedia, nada laporan pasif.
- Tension dulu → penjelasan belakangan.

ENERGI & BAHASA
- Kalimat aktif, tajam, spesifik. Bukan pasif/lembek ("dapat dikatakan", "perlu dipahami", "hal ini menunjukkan").
- Kayak orang yang paham lagi cerita ke timeline — bukan guru, bukan copywriter, bukan Wikipedia.
- Hindari basabasi ("hari ini kita bahas", "penting diketahui", "mari kita").

NICHE & ISI
- Ikuti niche yang user tentukan di app (boleh lebih dari satu; pilih/kombinasi yang relevan per draf).
- Konten RAW/daging: padat informasi + ada tensi, bukan ceramah tenang.

ANTI-DOGENG / ANTI-RUJAK
- Jangan terdengar AI, guru kehidupan, atau influencer yang lagi "membuka mata orang".
- Jangan overclaim. Kalau belum pasti, tulis sebagai observasi/dugaan.
- Jangan edgy palsu, jangan kasar murahan, jangan drama kosong.
- Hindari frasa: "banyak yang belum sadar", "di era digital", "faktanya mengejutkan", "thread ini penting".
- Tiap kalimat harus nambah informasi; kalau bisa dihapus tanpa rugi, hapus.

BAHASA
- Bahasa Indonesia natural.
- Dilarang: bullet point, numbering, "lu/gue", emoji berlebihan, hashtag, "thread 🧵".
- Boleh "kamu" atau kalimat netral tanpa sapaan kasual berlebihan.

LARANGAN
- Jangan ringkas jadi list tips.
- Jangan clickbait kosong: setiap bagian harus nambah isi.
- Jangan terdengar AI/LinkedIn.
- Jangan pasif: kalau hook lemah, utas gagal.
- Jangan lewat 500 karakter per bagian.
- Jangan filler: kalau tidak berpotensi viral di niche, jangan dikeluarkan.`

func emptyFallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func trimFeedback(list []DraftFeedback, n int) []DraftFeedback {
	if len(list) <= n {
		return list
	}
	return list[:n]
}

func trimDaily(list []DailyFocus, n int) []DailyFocus {
	if len(list) <= n {
		return list
	}
	return list[:n]
}

func recentDraftHints(history []GenHistory, n int) []map[string]string {
	var out []map[string]string
	for _, h := range history {
		for _, d := range h.Drafts {
			title := strings.TrimSpace(d.Title)
			hook := strings.TrimSpace(d.Hook)
			angle := strings.TrimSpace(d.Angle)
			if title == "" && hook == "" {
				continue
			}
			out = append(out, map[string]string{
				"title": title,
				"hook":  clipRunes(hook, 100),
				"angle": angle,
			})
			if len(out) >= n {
				return out
			}
		}
	}
	return out
}

func compactGenerateSnapshot(snapshot map[string]any) map[string]any {
	if snapshot == nil {
		return nil
	}
	out := map[string]any{
		"source":  snapshot["source"],
		"profile": snapshot["profile"],
		"metrics": snapshot["account_metrics"],
		"sample":  snapshot["sample"],
	}
	var rows []map[string]any
	switch t := snapshot["posts"].(type) {
	case []map[string]any:
		rows = t
	case []any:
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				rows = append(rows, m)
			}
		}
	}
	const max = 8
	if len(rows) > max {
		rows = rows[:max]
	}
	posts := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		posts = append(posts, map[string]any{
			"text":    p["text"],
			"views":   p["views"],
			"likes":   p["likes"],
			"replies": p["replies"],
			"score":   p["score"],
		})
	}
	if len(posts) > 0 {
		out["recent_posts"] = posts
	}
	return out
}
