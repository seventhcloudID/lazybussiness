package lazy

import "strings"

// Template IDs — pilihan desain carousel per akun.
const (
	TemplateNoir     = "noir"
	TemplateInk      = "ink"
	TemplateOcean    = "ocean"
	TemplateEmber    = "ember"
	TemplatePaper    = "paper"
	TemplateBloom    = "bloom"
	TemplateLilac    = "lilac"
	TemplatePeach    = "peach"
	TemplateBold     = "bold"
	TemplateFrame    = "frame"
	TemplateMeadow   = "meadow"
	TemplateMidnight = "midnight"
	TemplateCoral    = "coral"
	TemplateMint     = "mint"
	TemplateCherry   = "cherry"
	TemplateSand     = "sand"
	TemplateNeon     = "neon"
	TemplateSlate    = "slate"
	TemplateHoney    = "honey"
	TemplateMono     = "mono"
	TemplateAurora   = "aurora"
	TemplateCocoa    = "cocoa"
	TemplateIvory    = "ivory"
	TemplateForest   = "forest"
	TemplateRose     = "rose"
	TemplateGraphite = "graphite"
	TemplateCitrus   = "citrus"
	TemplateClay     = "clay"
	TemplateGlacier  = "glacier"
	TemplateMatcha   = "matcha"
	TemplateSignal   = "signal"
	TemplateEspresso = "espresso"
	TemplateSky      = "sky"
	TemplateDusk     = "dusk"
	TemplatePearl    = "pearl"
	TemplateOlive    = "olive"
	TemplateInkred   = "inkred"
	TemplateOrchid   = "orchid"
	DefaultTemplate  = TemplateNoir
)

// TemplateInfo metadata untuk UI picker.
type TemplateInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Desc     string   `json:"desc"`
	Swatch   []string `json:"swatch"` // hex: bg1, bg2, accent
	Tag      string   `json:"tag,omitempty"`
	Category string   `json:"category,omitempty"` // gelap|terang|soft|bold
}

var templateCatalog = []TemplateInfo{
	// Gelap
	{ID: TemplateNoir, Name: "Noir", Desc: "Editorial gelap + rail emas", Swatch: []string{"#12161E", "#1C222E", "#E8A45A"}, Category: "gelap"},
	{ID: TemplateInk, Name: "Ink", Desc: "Editorial hitam + rail putih", Swatch: []string{"#0A0A0A", "#141414", "#F5F5F5"}, Category: "gelap"},
	{ID: TemplateOcean, Name: "Ocean", Desc: "Editorial teal + mint", Swatch: []string{"#0B2428", "#123A42", "#5EEAD4"}, Category: "gelap"},
	{ID: TemplateEmber, Name: "Ember", Desc: "Band coral di charcoal", Swatch: []string{"#1A1412", "#2A1E1A", "#E85D4C"}, Category: "gelap"},
	{ID: TemplateMidnight, Name: "Midnight", Desc: "Editorial navy + biru", Swatch: []string{"#0B1220", "#152238", "#60A5FA"}, Category: "gelap"},
	{ID: TemplateCoral, Name: "Coral", Desc: "Band coral bold", Swatch: []string{"#1F1412", "#2C1C18", "#FF7A59"}, Category: "gelap"},
	{ID: TemplateNeon, Name: "Neon", Desc: "Editorial cyber + neon", Swatch: []string{"#05080A", "#0D1512", "#39FF14"}, Category: "gelap"},
	{ID: TemplateBold, Name: "Bold", Desc: "Band amber tegas", Swatch: []string{"#111827", "#1F2937", "#F59E0B"}, Category: "bold"},
	{ID: TemplateAurora, Name: "Aurora", Desc: "Editorial aurora cyan", Swatch: []string{"#0A1020", "#1A1440", "#22D3EE"}, Category: "gelap"},
	{ID: TemplateForest, Name: "Forest", Desc: "Editorial hutan + hijau", Swatch: []string{"#0C1610", "#15241A", "#86EFAC"}, Category: "gelap"},
	{ID: TemplateGraphite, Name: "Graphite", Desc: "Editorial abu modern", Swatch: []string{"#141618", "#23262B", "#A1A1AA"}, Category: "gelap"},
	{ID: TemplateEspresso, Name: "Espresso", Desc: "Editorial kopi + cream", Swatch: []string{"#1A120E", "#2A1C16", "#E7C8A0"}, Category: "gelap"},
	{ID: TemplateDusk, Name: "Dusk", Desc: "Editorial senja oranye", Swatch: []string{"#1A1224", "#2A1830", "#FB923C"}, Category: "gelap"},
	{ID: TemplateInkred, Name: "Ink Red", Desc: "Editorial hitam + merah", Swatch: []string{"#0C0C0C", "#181818", "#EF4444"}, Category: "bold"},

	// Terang / editorial
	{ID: TemplatePaper, Name: "Kertas", Desc: "Frame cream klasik", Swatch: []string{"#F4F1EA", "#E8E2D6", "#1A1A1A"}, Category: "terang"},
	{ID: TemplateFrame, Name: "Frame", Desc: "Bingkai tipis majalah", Swatch: []string{"#FAFAF8", "#F0EDE6", "#0F172A"}, Category: "terang"},
	{ID: TemplateMeadow, Name: "Meadow", Desc: "Kartu sage lembut", Swatch: []string{"#E8EEE6", "#D5E0D2", "#2F4A3A"}, Category: "terang"},
	{ID: TemplateSand, Name: "Sand", Desc: "Quote pasir hangat", Swatch: []string{"#FAF6F0", "#F0E6D8", "#8B6914"}, Category: "terang"},
	{ID: TemplateSlate, Name: "Slate", Desc: "Kartu abu dingin", Swatch: []string{"#F1F5F9", "#E2E8F0", "#334155"}, Category: "terang"},
	{ID: TemplateMono, Name: "Mono", Desc: "Frame hitam minimal", Swatch: []string{"#FFFFFF", "#F4F4F5", "#09090B"}, Category: "terang"},
	{ID: TemplateIvory, Name: "Ivory", Desc: "Frame gading + emas", Swatch: []string{"#FFFEF7", "#F5F0E1", "#B45309"}, Category: "terang"},
	{ID: TemplateGlacier, Name: "Glacier", Desc: "Kartu es biru", Swatch: []string{"#F0F9FF", "#E0F2FE", "#0369A1"}, Category: "terang"},
	{ID: TemplateSky, Name: "Sky", Desc: "Center langit airy", Swatch: []string{"#F0F7FF", "#DBEAFE", "#1D4ED8"}, Category: "terang"},
	{ID: TemplatePearl, Name: "Pearl", Desc: "Frame mutiara soft", Swatch: []string{"#FBFBFE", "#F1F0F7", "#475569"}, Category: "terang"},
	{ID: TemplateOlive, Name: "Olive", Desc: "Kartu zaitun chic", Swatch: []string{"#F4F1E8", "#E5E0D0", "#4D5C3A"}, Category: "terang"},
	{ID: TemplateCitrus, Name: "Citrus", Desc: "Band lemon energik", Swatch: []string{"#FEFCE8", "#FEF08A", "#A16207"}, Category: "bold"},
	{ID: TemplateClay, Name: "Clay", Desc: "Quote terracotta", Swatch: []string{"#FFF7F3", "#FED7AA", "#9A3412"}, Category: "terang"},
	{ID: TemplateSignal, Name: "Signal", Desc: "Band merah high-contrast", Swatch: []string{"#FFFFFF", "#FEF2F2", "#DC2626"}, Category: "bold"},
	{ID: TemplateCocoa, Name: "Cocoa", Desc: "Kartu cokelat lembut", Swatch: []string{"#F7F0E8", "#EAD9C8", "#6B3F2A"}, Category: "terang"},

	// Soft
	{ID: TemplateBloom, Name: "Bloom", Desc: "Kartu blush + pill", Swatch: []string{"#FFF0F3", "#FFE0E8", "#E85A8C"}, Tag: "soft", Category: "soft"},
	{ID: TemplateLilac, Name: "Lilac", Desc: "Center lavender", Swatch: []string{"#F6F1FF", "#E9DEFF", "#8B5CF6"}, Tag: "soft", Category: "soft"},
	{ID: TemplatePeach, Name: "Peach", Desc: "Kartu peach + pill", Swatch: []string{"#FFF5EE", "#FFE4D1", "#F0785A"}, Tag: "soft", Category: "soft"},
	{ID: TemplateMint, Name: "Mint", Desc: "Kartu mint + pill", Swatch: []string{"#ECFDF5", "#D1FAE5", "#059669"}, Tag: "soft", Category: "soft"},
	{ID: TemplateCherry, Name: "Cherry", Desc: "Center wine soft", Swatch: []string{"#FFF1F2", "#FFE4E6", "#BE123C"}, Tag: "soft", Category: "soft"},
	{ID: TemplateHoney, Name: "Honey", Desc: "Quote amber manis", Swatch: []string{"#FFFBEB", "#FEF3C7", "#D97706"}, Tag: "soft", Category: "soft"},
	{ID: TemplateRose, Name: "Rose", Desc: "Center dusty rose", Swatch: []string{"#FFF5F5", "#F5D0D0", "#9F4A5C"}, Tag: "soft", Category: "soft"},
	{ID: TemplateMatcha, Name: "Matcha", Desc: "Kartu matcha tenang", Swatch: []string{"#F4F7EF", "#E2EBD3", "#5B7040"}, Tag: "soft", Category: "soft"},
	{ID: TemplateOrchid, Name: "Orchid", Desc: "Center orchid lembut", Swatch: []string{"#FBF7FF", "#EFE4FF", "#7C3AED"}, Tag: "soft", Category: "soft"},
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
