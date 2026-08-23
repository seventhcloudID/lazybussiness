package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu   sync.Mutex
	path string
	data Memory
}

type Memory struct {
	Instructions    string          `json:"instructions"` // legacy prompt
	EditorialPrompt string          `json:"editorial_prompt,omitempty"`
	Product         ProductProfile  `json:"product,omitempty"`
	Niche           string          `json:"niche"`  // legacy / joined display
	Niches          []string        `json:"niches"` // preferred: multi niche
	Brand           string          `json:"brand"`  // nama brand di carousel/IG
	UpdatedAt       string          `json:"updated_at"`
	Lessons         Lessons         `json:"lessons"`
	Daily           []DailyFocus    `json:"daily"`
	History         []GenHistory    `json:"history"`
	Feedback        []DraftFeedback `json:"feedback"`
}

// ProductProfile is user-owned context for product-led soft-selling. The AI
// may use these facts as positioning input, but must not invent missing claims.
type ProductProfile struct {
	Knowledge   string `json:"knowledge,omitempty"`
	Name        string `json:"name,omitempty"`
	Audience    string `json:"audience,omitempty"`
	Description string `json:"description,omitempty"`
	Proof       string `json:"proof,omitempty"`
	CTA         string `json:"cta,omitempty"`
}

func (p ProductProfile) Empty() bool {
	return strings.TrimSpace(p.Knowledge+p.Name+p.Description+p.Audience+p.Proof+p.CTA) == ""
}

// EffectiveName and EffectiveCTA keep one-column product knowledge compatible
// with the deterministic soft-selling guard. The legacy structured fields are
// still read so existing workspaces do not lose their settings.
func (p ProductProfile) EffectiveName() string {
	if name := strings.TrimSpace(p.Name); name != "" {
		return name
	}
	return productKnowledgeValue(p.Knowledge, "nama produk", "produk")
}

func (p ProductProfile) EffectiveCTA() string {
	if cta := strings.TrimSpace(p.CTA); cta != "" {
		return cta
	}
	return productKnowledgeValue(p.Knowledge, "cta lembut", "cta", "ajakan")
}

// PublicIdentifiers returns brand-like names that must stay out of public
// soft-selling copy. Product knowledge remains available to the model for
// factual benefits, while the brand/domain is revealed later through DM.
func (p ProductProfile) PublicIdentifiers() []string {
	values := make([]string, 0, 8)
	if name := strings.TrimSpace(p.Name); name != "" {
		values = append(values, name)
	}
	for _, line := range strings.Split(strings.ReplaceAll(p.Knowledge, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*• "))
		lower := strings.ToLower(line)
		for _, label := range []string{"nama produk", "nama brand", "brand"} {
			prefix := label + ":"
			if strings.HasPrefix(lower, prefix) {
				values = append(values, strings.TrimSpace(line[len(prefix):]))
			}
		}
		for _, label := range []string{"website", "situs", "domain", "url"} {
			prefix := label + ":"
			if !strings.HasPrefix(lower, prefix) {
				continue
			}
			domain := strings.ToLower(strings.TrimSpace(line[len(prefix):]))
			domain = strings.TrimPrefix(domain, "https://")
			domain = strings.TrimPrefix(domain, "http://")
			domain = strings.TrimPrefix(domain, "www.")
			if cut := strings.IndexAny(domain, "/?# "); cut >= 0 {
				domain = domain[:cut]
			}
			if domain == "" {
				continue
			}
			values = append(values, domain)
			if root := strings.TrimSpace(strings.Split(domain, ".")[0]); len([]rune(root)) >= 4 {
				values = append(values, root)
			}
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if len([]rune(key)) < 3 || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func productKnowledgeValue(knowledge string, labels ...string) string {
	for _, line := range strings.Split(strings.ReplaceAll(knowledge, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*• "))
		lower := strings.ToLower(line)
		for _, label := range labels {
			prefix := strings.ToLower(strings.TrimSpace(label)) + ":"
			if strings.HasPrefix(lower, prefix) {
				return strings.TrimSpace(line[len(prefix):])
			}
		}
	}
	return ""
}

type Lessons struct {
	DoMore    []LessonItem    `json:"do_more"`
	Avoid     []LessonItem    `json:"avoid"`
	Repurpose []RepurposeItem `json:"repurpose"`
}

type LessonItem struct {
	Pattern  string   `json:"pattern"`
	Evidence string   `json:"evidence"`
	PostIDs  []string `json:"post_ids,omitempty"`
}

type RepurposeItem struct {
	SourcePostID string   `json:"source_post_id"`
	Excerpt      string   `json:"excerpt"`
	Score        float64  `json:"score"`
	Why          string   `json:"why"`
	AngleIdeas   []string `json:"angle_ideas,omitempty"`
}

type DailyFocus struct {
	Date       string   `json:"date"`
	Focus      string   `json:"focus"`
	AvoidToday []string `json:"avoid_today"`
	Notes      string   `json:"notes"`
}

type GenHistory struct {
	At            string           `json:"at"`
	Topic         string           `json:"topic,omitempty"`
	Instructions  string           `json:"instructions,omitempty"`
	Drafts        []GeneratedDraft `json:"drafts,omitempty"`
	Consideration string           `json:"consideration,omitempty"`
}

type DraftFeedback struct {
	At       string `json:"at"`
	DraftKey string `json:"draft_key"`
	Verdict  string `json:"verdict"` // good | bad | used
	Note     string `json:"note,omitempty"`
	Text     string `json:"text,omitempty"`
}

type GeneratedDraft struct {
	Title   string   `json:"title"`
	Hook    string   `json:"hook"`
	Parts   []string `json:"parts,omitempty"`
	Draft   string   `json:"draft"`
	Angle   string   `json:"angle"`
	Format  string   `json:"format"`
	Why     string   `json:"why"`
	BasedOn string   `json:"based_on"`
	Risk    string   `json:"risk"`
	Key     string   `json:"key,omitempty"`
}

func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreAt(filepath.Join(".data", "ai_memory.json"))
}

func NewMemoryStoreAt(path string) *MemoryStore {
	s := &MemoryStore{path: path}
	s.load()
	return s
}

func (s *MemoryStore) load() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		s.data = Memory{Lessons: Lessons{}, Daily: []DailyFocus{}, History: []GenHistory{}, Feedback: []DraftFeedback{}}
		return
	}
	var m Memory
	if json.Unmarshal(b, &m) != nil {
		s.data = Memory{Lessons: Lessons{}}
		return
	}
	if len(m.Niches) == 0 && strings.TrimSpace(m.Niche) != "" {
		m.Niches = ParseNiches(m.Niche)
	}
	s.data = m
}

func (s *MemoryStore) persistLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	s.data.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func (s *MemoryStore) Get() Memory {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

// FitsAccount is true when saved brand/handle overlaps the Repliz account being analyzed.
func (m Memory) FitsAccount(username, name string) bool {
	brand := compactAccountKey(m.Brand)
	if brand == "" {
		return false
	}
	u := compactAccountKey(username)
	n := compactAccountKey(name)
	if u != "" && (strings.Contains(brand, u) || strings.Contains(u, brand)) {
		return true
	}
	if n != "" && len(n) >= 4 && (strings.Contains(brand, n) || strings.Contains(n, brand)) {
		return true
	}
	return false
}

func compactAccountKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "@")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *MemoryStore) SetInstructions(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Instructions = strings.TrimSpace(text)
	return s.persistLocked()
}

func (s *MemoryStore) SetEditorialPrompt(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.EditorialPrompt = strings.TrimSpace(text)
	return s.persistLocked()
}

func (s *MemoryStore) SetProductProfile(product ProductProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Product = ProductProfile{
		Knowledge:   clipRunes(strings.TrimSpace(product.Knowledge), 12000),
		Name:        clipRunes(strings.TrimSpace(product.Name), 160),
		Audience:    clipRunes(strings.TrimSpace(product.Audience), 500),
		Description: clipRunes(strings.TrimSpace(product.Description), 2000),
		Proof:       clipRunes(strings.TrimSpace(product.Proof), 1500),
		CTA:         clipRunes(strings.TrimSpace(product.CTA), 500),
	}
	return s.persistLocked()
}

func (s *MemoryStore) SetNiche(text string) error {
	return s.SetNiches(ParseNiches(text))
}

func (s *MemoryStore) SetNiches(list []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Niches = NormalizeNicheList(list)
	s.data.Niche = strings.Join(s.data.Niches, "\n")
	return s.persistLocked()
}

func (s *MemoryStore) SetBrand(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Brand = strings.TrimSpace(text)
	return s.persistLocked()
}

// NicheList returns niches from memory (supports legacy single niche string).
func NicheList(m Memory) []string {
	if len(m.Niches) > 0 {
		return NormalizeNicheList(m.Niches)
	}
	return ParseNiches(m.Niche)
}

func ParseNiches(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, ";", "\n")
	text = strings.ReplaceAll(text, "|", "\n")
	var raw []string
	for _, line := range strings.Split(text, "\n") {
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				raw = append(raw, part)
			}
		}
	}
	return NormalizeNicheList(raw)
}

func NormalizeNicheList(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, n := range list {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		key := strings.ToLower(n)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, n)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func (s *MemoryStore) AddFeedback(fb DraftFeedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fb.At == "" {
		fb.At = time.Now().UTC().Format(time.RFC3339)
	}
	s.data.Feedback = append([]DraftFeedback{fb}, s.data.Feedback...)
	if len(s.data.Feedback) > 100 {
		s.data.Feedback = s.data.Feedback[:100]
	}
	txt := strings.TrimSpace(fb.Text)
	if txt != "" {
		snip := clipRunes(txt, 120)
		switch fb.Verdict {
		case "bad":
			s.data.Lessons.Avoid = prependLesson(s.data.Lessons.Avoid, LessonItem{
				Pattern:  "Jangan ulangi gaya/draf mirip: " + snip,
				Evidence: "Feedback user: jelek" + feedbackNote(fb.Note),
			}, 12)
		case "good", "used":
			s.data.Lessons.DoMore = prependLesson(s.data.Lessons.DoMore, LessonItem{
				Pattern:  "Gaya yang disukai user: " + snip,
				Evidence: "Feedback user: " + fb.Verdict + feedbackNote(fb.Note),
			}, 12)
		}
	}
	return s.persistLocked()
}

func feedbackNote(n string) string {
	n = strings.TrimSpace(n)
	if n == "" {
		return ""
	}
	return " — " + n
}

func prependLesson(list []LessonItem, item LessonItem, max int) []LessonItem {
	out := append([]LessonItem{item}, list...)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func (s *MemoryStore) RecordGeneration(h GenHistory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h.At == "" {
		h.At = time.Now().UTC().Format(time.RFC3339)
	}
	s.data.History = append([]GenHistory{h}, s.data.History...)
	if len(s.data.History) > 30 {
		s.data.History = s.data.History[:30]
	}
	return s.persistLocked()
}

func (s *MemoryStore) SetDaily(d DailyFocus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.Date == "" {
		d.Date = time.Now().Format("2006-01-02")
	}
	found := false
	for i := range s.data.Daily {
		if s.data.Daily[i].Date == d.Date {
			s.data.Daily[i] = d
			found = true
			break
		}
	}
	if !found {
		s.data.Daily = append([]DailyFocus{d}, s.data.Daily...)
	}
	if len(s.data.Daily) > 14 {
		s.data.Daily = s.data.Daily[:14]
	}
	return s.persistLocked()
}

func (s *MemoryStore) ApplyLessons(lessons Lessons) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fbDo, fbAvoid := splitFeedbackLessons(s.data.Lessons)
	s.data.Lessons.DoMore = mergeLessons(fbDo, lessons.DoMore, 10)
	s.data.Lessons.Avoid = mergeLessons(fbAvoid, lessons.Avoid, 10)
	s.data.Lessons.Repurpose = lessons.Repurpose
	if len(s.data.Lessons.Repurpose) > 8 {
		s.data.Lessons.Repurpose = s.data.Lessons.Repurpose[:8]
	}
	return s.persistLocked()
}

// ResetLearning clears pelajaran dari data + history/feedback. Niche & instruksi tetap.
func (s *MemoryStore) ResetLearning() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Lessons = Lessons{
		DoMore:    []LessonItem{},
		Avoid:     []LessonItem{},
		Repurpose: []RepurposeItem{},
	}
	s.data.Daily = []DailyFocus{}
	s.data.History = []GenHistory{}
	s.data.Feedback = []DraftFeedback{}
	return s.persistLocked()
}

func splitFeedbackLessons(l Lessons) (doMore, avoid []LessonItem) {
	for _, x := range l.DoMore {
		if strings.Contains(x.Evidence, "Feedback user") {
			doMore = append(doMore, x)
		}
	}
	for _, x := range l.Avoid {
		if strings.Contains(x.Evidence, "Feedback user") {
			avoid = append(avoid, x)
		}
	}
	return
}

func mergeLessons(prefer, rest []LessonItem, max int) []LessonItem {
	seen := map[string]bool{}
	out := make([]LessonItem, 0, max)
	add := func(items []LessonItem) {
		for _, it := range items {
			k := strings.ToLower(strings.TrimSpace(it.Pattern))
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, it)
			if len(out) >= max {
				return
			}
		}
	}
	add(prefer)
	add(rest)
	return out
}

// BuildLessonsFromSnapshot classifies posts into winners/losers for learning signals.
func BuildLessonsFromSnapshot(snapshot map[string]any) Lessons {
	type row struct {
		ID, Text, MediaType string
		Score, Views, ER    float64
		Replies             float64
	}
	posts := make([]row, 0)
	add := func(m map[string]any) {
		if m == nil {
			return
		}
		posts = append(posts, row{
			ID:        strAny(m["id"]),
			Text:      strAny(m["text"]),
			MediaType: strAny(m["media_type"]),
			Score:     floatAny(m["score"]),
			Views:     floatAny(m["views"]),
			ER:        floatAny(m["engagement_rate"]),
			Replies:   floatAny(m["replies"]),
		})
	}
	switch raw := snapshot["posts"].(type) {
	case []map[string]any:
		for _, m := range raw {
			add(m)
		}
	case []any:
		for _, p := range raw {
			if m, ok := p.(map[string]any); ok {
				add(m)
			}
		}
	}
	if len(posts) == 0 {
		return Lessons{}
	}
	sort.Slice(posts, func(i, j int) bool { return posts[i].Score > posts[j].Score })

	n := len(posts)
	topN := maxInt(1, n/3)
	botN := maxInt(1, n/3)
	winners := posts[:topN]
	losers := posts[n-botN:]

	lessons := Lessons{}
	for _, w := range winners {
		lessons.DoMore = append(lessons.DoMore, LessonItem{
			Pattern:  "Performa kuat: " + clipRunes(w.Text, 90),
			Evidence: fmtEvidence(w.Views, w.Replies, w.ER, w.Score, w.MediaType),
			PostIDs:  []string{w.ID},
		})
		lessons.Repurpose = append(lessons.Repurpose, RepurposeItem{
			SourcePostID: w.ID,
			Excerpt:      clipRunes(w.Text, 140),
			Score:        w.Score,
			Why:          "Skor tinggi — cocok diangkat ulang dengan sudut baru",
		})
	}
	for _, l := range losers {
		lessons.Avoid = append(lessons.Avoid, LessonItem{
			Pattern:  "Performa lemah: " + clipRunes(l.Text, 90),
			Evidence: fmtEvidence(l.Views, l.Replies, l.ER, l.Score, l.MediaType),
			PostIDs:  []string{l.ID},
		})
	}
	return lessons
}

func strAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func floatAny(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}

func clipRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func fmtEvidence(views, replies, er, score float64, media string) string {
	return fmt.Sprintf("format %s · views %.0f · replies %.0f · ER %.2f%% · score %.0f", media, views, replies, er, score)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
