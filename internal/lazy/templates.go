package lazy

import "strings"

// Template IDs — pilihan desain carousel per akun.
const (
	TemplateNoir    = "noir"
	TemplateInk     = "ink"
	TemplateOcean   = "ocean"
	TemplateEmber   = "ember"
	TemplatePaper   = "paper"
	TemplateBloom   = "bloom"
	TemplateLilac   = "lilac"
	TemplatePeach   = "peach"
	TemplateBold    = "bold"
	TemplateFrame   = "frame"
	TemplateMeadow  = "meadow"
	TemplateMidnight = "midnight"
	TemplateCoral   = "coral"
	TemplateMint    = "mint"
	TemplateCherry  = "cherry"
	TemplateSand    = "sand"
	TemplateNeon    = "neon"
	TemplateSlate   = "slate"
	TemplateHoney   = "honey"
	TemplateMono    = "mono"
	DefaultTemplate = TemplateNoir
)

// TemplateInfo metadata untuk UI picker.
type TemplateInfo struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Desc   string   `json:"desc"`
	Swatch []string `json:"swatch"` // hex: bg1, bg2, accent
	Tag    string   `json:"tag,omitempty"`
}

var templateCatalog = []TemplateInfo{
	{ID: TemplateNoir, Name: "Noir", Desc: "Gelap editorial + bar emas", Swatch: []string{"#12161E", "#1C222E", "#E8A45A"}},
	{ID: TemplateInk, Name: "Ink", Desc: "Hitam pekat, kontras tajam", Swatch: []string{"#0A0A0A", "#141414", "#F5F5F5"}},
	{ID: TemplateOcean, Name: "Ocean", Desc: "Teal dalam + mint", Swatch: []string{"#0B2428", "#123A42", "#5EEAD4"}},
	{ID: TemplateEmber, Name: "Ember", Desc: "Charcoal + strip coral", Swatch: []string{"#1A1412", "#2A1E1A", "#E85D4C"}},
	{ID: TemplatePaper, Name: "Kertas", Desc: "Editorial terang klasik", Swatch: []string{"#F4F1EA", "#E8E2D6", "#1A1A1A"}},
	{ID: TemplateBloom, Name: "Bloom", Desc: "Blush pink lembut", Swatch: []string{"#FFF0F3", "#FFE0E8", "#E85A8C"}, Tag: "cewe"},
	{ID: TemplateLilac, Name: "Lilac", Desc: "Lavender dreamy", Swatch: []string{"#F6F1FF", "#E9DEFF", "#8B5CF6"}, Tag: "cewe"},
	{ID: TemplatePeach, Name: "Peach", Desc: "Peach soft + kartu putih", Swatch: []string{"#FFF5EE", "#FFE4D1", "#F0785A"}, Tag: "cewe"},
	{ID: TemplateBold, Name: "Bold", Desc: "Header aksen + tipe besar", Swatch: []string{"#111827", "#1F2937", "#F59E0B"}},
	{ID: TemplateFrame, Name: "Frame", Desc: "Bingkai majalah minimal", Swatch: []string{"#FAFAF8", "#F0EDE6", "#0F172A"}},
	{ID: TemplateMeadow, Name: "Meadow", Desc: "Sage hijau + kartu", Swatch: []string{"#E8EEE6", "#D5E0D2", "#2F4A3A"}},
	{ID: TemplateMidnight, Name: "Midnight", Desc: "Navy dalam + aksen biru", Swatch: []string{"#0B1220", "#152238", "#60A5FA"}},
	{ID: TemplateCoral, Name: "Coral", Desc: "Header coral cerah", Swatch: []string{"#1F1412", "#2C1C18", "#FF7A59"}},
	{ID: TemplateMint, Name: "Mint", Desc: "Mint segar lembut", Swatch: []string{"#ECFDF5", "#D1FAE5", "#059669"}, Tag: "cewe"},
	{ID: TemplateCherry, Name: "Cherry", Desc: "Wine soft feminine", Swatch: []string{"#FFF1F2", "#FFE4E6", "#BE123C"}, Tag: "cewe"},
	{ID: TemplateSand, Name: "Sand", Desc: "Pasir hangat editorial", Swatch: []string{"#FAF6F0", "#F0E6D8", "#8B6914"}},
	{ID: TemplateNeon, Name: "Neon", Desc: "Gelap cyber + hijau neon", Swatch: []string{"#05080A", "#0D1512", "#39FF14"}},
	{ID: TemplateSlate, Name: "Slate", Desc: "Abu dingin modern", Swatch: []string{"#F1F5F9", "#E2E8F0", "#334155"}},
	{ID: TemplateHoney, Name: "Honey", Desc: "Amber manis + quote", Swatch: []string{"#FFFBEB", "#FEF3C7", "#D97706"}, Tag: "cewe"},
	{ID: TemplateMono, Name: "Mono", Desc: "Putih bersih + bingkai hitam", Swatch: []string{"#FFFFFF", "#F4F4F5", "#09090B"}},
}

// ListCarouselTemplates daftar template yang bisa dipilih.
func ListCarouselTemplates() []TemplateInfo {
	out := make([]TemplateInfo, len(templateCatalog))
	copy(out, templateCatalog)
	return out
}

// NormalizeTemplate mengembalikan ID valid; kosong/unknown → default.
func NormalizeTemplate(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, t := range templateCatalog {
		if t.ID == id {
			return id
		}
	}
	return DefaultTemplate
}
