package lazy

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	slideW = 1080
	slideH = 1350
)

//go:embed fonts/Inter-Regular.ttf
var interRegularTTF []byte

//go:embed fonts/Inter-SemiBold.ttf
var interSemiBoldTTF []byte

var (
	fontBody  *opentype.Font
	fontBrand *opentype.Font
)

func init() {
	if ft, err := opentype.Parse(interSemiBoldTTF); err == nil {
		fontBody = ft
		fontBrand = ft
	}
	if ft, err := opentype.Parse(interRegularTTF); err == nil && fontBrand == nil {
		fontBrand = ft
	}
	if fontBody == nil {
		if ft, err := opentype.Parse(goregular.TTF); err == nil {
			fontBody = ft
		}
	}
	if fontBrand == nil {
		fontBrand = fontBody
	}
}

type layoutKind int

const (
	layClassic layoutKind = iota // brand kiri + body
	layTopBand                   // pita warna di atas berisi brand
	layFrame                     // bingkai tepi
	layQuote                     // tanda kutip besar
	layPill                      // brand pill (feminine)
)

type slideTheme struct {
	bgTop, bgBot                      color.RGBA
	blob1c, blob2c, blob3c            color.RGBA
	blob1s, blob2s, blob3s            float64
	blob1x, blob1y, blob1r            int
	blob2x, blob2y, blob2r            int
	blob3x, blob3y, blob3r            int
	accent, accentDim                 color.RGBA
	body, brandCol, muted, footerLine color.RGBA
	leftBar                           int
	bottomBar                         int
	cardInset                         bool
	cardBg                            color.RGBA
	underlineBrand                    bool
	layout                            layoutKind
	bandBg                            color.RGBA
	pillBg                            color.RGBA
	framePad                          int
}

func themeFor(id string) slideTheme {
	switch NormalizeTemplate(id) {
	case TemplatePaper:
		return slideTheme{
			bgTop: rgb(244, 241, 234), bgBot: rgb(232, 226, 214),
			blob1c: rgb(220, 210, 190), blob1s: 0.16, blob1x: 900, blob1y: 200, blob1r: 380,
			blob2c: rgb(210, 200, 180), blob2s: 0.12, blob2x: 160, blob2y: 1100, blob2r: 420,
			accent: rgb(26, 26, 26), accentDim: rgb(80, 80, 80),
			body: rgb(22, 22, 22), brandCol: rgb(70, 70, 70), muted: rgb(110, 110, 110),
			footerLine: rgb(200, 190, 175), underlineBrand: true, layout: layClassic,
		}
	case TemplateOcean:
		return slideTheme{
			bgTop: rgb(11, 36, 40), bgBot: rgb(18, 58, 66),
			blob1c: rgb(30, 90, 100), blob1s: 0.28, blob1x: 960, blob1y: 160, blob1r: 400,
			blob2c: rgb(20, 70, 80), blob2s: 0.24, blob2x: 100, blob2y: 1200, blob2r: 460,
			blob3c: rgb(94, 234, 212), blob3s: 0.07, blob3x: 880, blob3y: 1050, blob3r: 260,
			accent: rgb(94, 234, 212), accentDim: rgb(45, 160, 145),
			body: rgb(236, 252, 248), brandCol: rgb(148, 196, 188), muted: rgb(120, 160, 155),
			footerLine: rgb(40, 80, 88), leftBar: 8, underlineBrand: true, layout: layClassic,
		}
	case TemplateInk:
		return slideTheme{
			bgTop: rgb(10, 10, 10), bgBot: rgb(20, 20, 20),
			blob1c: rgb(40, 40, 40), blob1s: 0.2, blob1x: 950, blob1y: 220, blob1r: 360,
			blob2c: rgb(35, 35, 35), blob2s: 0.18, blob2x: 140, blob2y: 1150, blob2r: 400,
			accent: rgb(245, 245, 245), accentDim: rgb(180, 180, 180),
			body: rgb(245, 245, 245), brandCol: rgb(160, 160, 160), muted: rgb(130, 130, 130),
			footerLine: rgb(50, 50, 50), leftBar: 16, layout: layClassic,
		}
	case TemplateEmber:
		return slideTheme{
			bgTop: rgb(26, 20, 18), bgBot: rgb(42, 30, 26),
			blob1c: rgb(70, 40, 35), blob1s: 0.22, blob1x: 920, blob1y: 180, blob1r: 400,
			blob2c: rgb(55, 35, 30), blob2s: 0.2, blob2x: 140, blob2y: 1100, blob2r: 440,
			blob3c: rgb(232, 93, 76), blob3s: 0.08, blob3x: 860, blob3y: 980, blob3r: 280,
			accent: rgb(232, 93, 76), accentDim: rgb(180, 70, 55),
			body: rgb(250, 244, 240), brandCol: rgb(190, 160, 150), muted: rgb(150, 120, 110),
			footerLine: rgb(60, 42, 38), bottomBar: 14, underlineBrand: true, layout: layClassic,
		}
	case TemplateBloom:
		return slideTheme{
			bgTop: rgb(255, 240, 243), bgBot: rgb(255, 224, 232),
			blob1c: rgb(255, 180, 200), blob1s: 0.22, blob1x: 920, blob1y: 160, blob1r: 420,
			blob2c: rgb(255, 200, 210), blob2s: 0.2, blob2x: 120, blob2y: 1180, blob2r: 460,
			blob3c: rgb(232, 90, 140), blob3s: 0.08, blob3x: 780, blob3y: 980, blob3r: 240,
			accent: rgb(232, 90, 140), accentDim: rgb(200, 100, 140),
			body: rgb(70, 30, 48), brandCol: rgb(255, 255, 255), muted: rgb(160, 110, 130),
			footerLine: rgb(255, 200, 215), layout: layPill, pillBg: rgb(232, 90, 140),
			cardInset: true, cardBg: rgb(255, 250, 252),
		}
	case TemplateLilac:
		return slideTheme{
			bgTop: rgb(246, 241, 255), bgBot: rgb(233, 222, 255),
			blob1c: rgb(200, 180, 255), blob1s: 0.24, blob1x: 940, blob1y: 180, blob1r: 400,
			blob2c: rgb(210, 190, 255), blob2s: 0.18, blob2x: 140, blob2y: 1150, blob2r: 440,
			blob3c: rgb(139, 92, 246), blob3s: 0.07, blob3x: 820, blob3y: 1000, blob3r: 260,
			accent: rgb(139, 92, 246), accentDim: rgb(110, 70, 200),
			body: rgb(45, 30, 80), brandCol: rgb(255, 255, 255), muted: rgb(130, 110, 170),
			footerLine: rgb(210, 195, 240), layout: layPill, pillBg: rgb(139, 92, 246),
		}
	case TemplatePeach:
		return slideTheme{
			bgTop: rgb(255, 245, 238), bgBot: rgb(255, 228, 209),
			blob1c: rgb(255, 200, 170), blob1s: 0.2, blob1x: 900, blob1y: 200, blob1r: 380,
			blob2c: rgb(255, 190, 160), blob2s: 0.16, blob2x: 150, blob2y: 1120, blob2r: 420,
			accent: rgb(240, 120, 90), accentDim: rgb(210, 100, 75),
			body: rgb(70, 40, 30), brandCol: rgb(240, 120, 90), muted: rgb(160, 120, 100),
			footerLine: rgb(240, 210, 190), layout: layQuote, underlineBrand: true,
			cardInset: true, cardBg: rgb(255, 252, 248),
		}
	case TemplateBold:
		return slideTheme{
			bgTop: rgb(17, 24, 39), bgBot: rgb(31, 41, 55),
			blob1c: rgb(55, 65, 85), blob1s: 0.18, blob1x: 960, blob1y: 400, blob1r: 380,
			blob2c: rgb(40, 50, 70), blob2s: 0.16, blob2x: 120, blob2y: 1100, blob2r: 400,
			accent: rgb(245, 158, 11), accentDim: rgb(200, 130, 20),
			body: rgb(248, 250, 252), brandCol: rgb(17, 24, 39), muted: rgb(148, 163, 184),
			footerLine: rgb(55, 65, 85), layout: layTopBand, bandBg: rgb(245, 158, 11),
		}
	case TemplateFrame:
		return slideTheme{
			bgTop: rgb(250, 250, 248), bgBot: rgb(240, 237, 230),
			blob1c: rgb(230, 225, 215), blob1s: 0.12, blob1x: 900, blob1y: 200, blob1r: 360,
			accent: rgb(15, 23, 42), accentDim: rgb(71, 85, 105),
			body: rgb(15, 23, 42), brandCol: rgb(15, 23, 42), muted: rgb(100, 116, 139),
			footerLine: rgb(210, 205, 195), layout: layFrame, framePad: 40, underlineBrand: true,
		}
	case TemplateMeadow:
		return slideTheme{
			bgTop: rgb(232, 238, 230), bgBot: rgb(213, 224, 210),
			blob1c: rgb(190, 210, 185), blob1s: 0.2, blob1x: 900, blob1y: 200, blob1r: 380,
			blob2c: rgb(180, 200, 175), blob2s: 0.16, blob2x: 150, blob2y: 1120, blob2r: 420,
			accent: rgb(47, 74, 58), accentDim: rgb(80, 110, 90),
			body: rgb(28, 48, 38), brandCol: rgb(47, 74, 58), muted: rgb(100, 120, 105),
			footerLine: rgb(190, 205, 185), layout: layClassic, underlineBrand: true,
			cardInset: true, cardBg: rgb(248, 251, 246),
		}
	case TemplateMidnight:
		return slideTheme{
			bgTop: rgb(11, 18, 32), bgBot: rgb(21, 34, 56),
			blob1c: rgb(40, 70, 120), blob1s: 0.24, blob1x: 940, blob1y: 180, blob1r: 400,
			blob2c: rgb(30, 50, 90), blob2s: 0.2, blob2x: 120, blob2y: 1160, blob2r: 440,
			blob3c: rgb(96, 165, 250), blob3s: 0.07, blob3x: 860, blob3y: 1000, blob3r: 240,
			accent: rgb(96, 165, 250), accentDim: rgb(59, 130, 246),
			body: rgb(239, 246, 255), brandCol: rgb(147, 197, 253), muted: rgb(100, 140, 190),
			footerLine: rgb(40, 60, 90), leftBar: 10, underlineBrand: true, layout: layClassic,
		}
	case TemplateCoral:
		return slideTheme{
			bgTop: rgb(31, 20, 18), bgBot: rgb(44, 28, 24),
			blob1c: rgb(80, 40, 35), blob1s: 0.2, blob1x: 920, blob1y: 400, blob1r: 360,
			accent: rgb(255, 122, 89), accentDim: rgb(220, 90, 60),
			body: rgb(255, 247, 244), brandCol: rgb(31, 20, 18), muted: rgb(180, 140, 130),
			footerLine: rgb(70, 45, 40), layout: layTopBand, bandBg: rgb(255, 122, 89),
		}
	case TemplateMint:
		return slideTheme{
			bgTop: rgb(236, 253, 245), bgBot: rgb(209, 250, 229),
			blob1c: rgb(167, 243, 208), blob1s: 0.22, blob1x: 900, blob1y: 180, blob1r: 400,
			blob2c: rgb(110, 231, 183), blob2s: 0.14, blob2x: 140, blob2y: 1140, blob2r: 420,
			accent: rgb(5, 150, 105), accentDim: rgb(4, 120, 87),
			body: rgb(6, 60, 45), brandCol: rgb(255, 255, 255), muted: rgb(80, 140, 120),
			footerLine: rgb(167, 243, 208), layout: layPill, pillBg: rgb(5, 150, 105),
			cardInset: true, cardBg: rgb(255, 255, 255),
		}
	case TemplateCherry:
		return slideTheme{
			bgTop: rgb(255, 241, 242), bgBot: rgb(255, 228, 230),
			blob1c: rgb(254, 180, 190), blob1s: 0.2, blob1x: 920, blob1y: 160, blob1r: 400,
			blob2c: rgb(252, 165, 165), blob2s: 0.16, blob2x: 130, blob2y: 1160, blob2r: 440,
			accent: rgb(190, 18, 60), accentDim: rgb(159, 18, 57),
			body: rgb(76, 10, 30), brandCol: rgb(255, 255, 255), muted: rgb(170, 100, 120),
			footerLine: rgb(254, 200, 210), layout: layPill, pillBg: rgb(190, 18, 60),
		}
	case TemplateSand:
		return slideTheme{
			bgTop: rgb(250, 246, 240), bgBot: rgb(240, 230, 216),
			blob1c: rgb(230, 210, 180), blob1s: 0.16, blob1x: 900, blob1y: 200, blob1r: 380,
			blob2c: rgb(220, 195, 160), blob2s: 0.12, blob2x: 150, blob2y: 1100, blob2r: 400,
			accent: rgb(139, 105, 20), accentDim: rgb(120, 90, 20),
			body: rgb(55, 40, 20), brandCol: rgb(100, 75, 20), muted: rgb(140, 120, 90),
			footerLine: rgb(220, 200, 170), underlineBrand: true, layout: layClassic,
		}
	case TemplateNeon:
		return slideTheme{
			bgTop: rgb(5, 8, 10), bgBot: rgb(13, 21, 18),
			blob1c: rgb(20, 50, 30), blob1s: 0.22, blob1x: 940, blob1y: 200, blob1r: 380,
			blob2c: rgb(15, 40, 25), blob2s: 0.18, blob2x: 120, blob2y: 1150, blob2r: 420,
			blob3c: rgb(57, 255, 20), blob3s: 0.06, blob3x: 860, blob3y: 1000, blob3r: 220,
			accent: rgb(57, 255, 20), accentDim: rgb(40, 200, 15),
			body: rgb(230, 255, 230), brandCol: rgb(160, 220, 160), muted: rgb(100, 150, 100),
			footerLine: rgb(30, 50, 35), leftBar: 8, underlineBrand: true, layout: layClassic,
		}
	case TemplateSlate:
		return slideTheme{
			bgTop: rgb(241, 245, 249), bgBot: rgb(226, 232, 240),
			blob1c: rgb(200, 210, 225), blob1s: 0.16, blob1x: 900, blob1y: 180, blob1r: 380,
			blob2c: rgb(180, 195, 215), blob2s: 0.12, blob2x: 140, blob2y: 1120, blob2r: 400,
			accent: rgb(51, 65, 85), accentDim: rgb(71, 85, 105),
			body: rgb(15, 23, 42), brandCol: rgb(51, 65, 85), muted: rgb(100, 116, 139),
			footerLine: rgb(200, 210, 220), underlineBrand: true, layout: layClassic,
			cardInset: true, cardBg: rgb(255, 255, 255),
		}
	case TemplateHoney:
		return slideTheme{
			bgTop: rgb(255, 251, 235), bgBot: rgb(254, 243, 199),
			blob1c: rgb(253, 224, 140), blob1s: 0.2, blob1x: 900, blob1y: 180, blob1r: 400,
			blob2c: rgb(252, 211, 77), blob2s: 0.12, blob2x: 140, blob2y: 1140, blob2r: 420,
			accent: rgb(217, 119, 6), accentDim: rgb(180, 95, 10),
			body: rgb(70, 40, 10), brandCol: rgb(217, 119, 6), muted: rgb(160, 120, 60),
			footerLine: rgb(250, 220, 140), layout: layQuote, underlineBrand: true,
			cardInset: true, cardBg: rgb(255, 253, 245),
		}
	case TemplateMono:
		return slideTheme{
			bgTop: rgb(255, 255, 255), bgBot: rgb(244, 244, 245),
			blob1c: rgb(230, 230, 235), blob1s: 0.1, blob1x: 900, blob1y: 200, blob1r: 340,
			accent: rgb(9, 9, 11), accentDim: rgb(60, 60, 70),
			body: rgb(9, 9, 11), brandCol: rgb(9, 9, 11), muted: rgb(110, 110, 120),
			footerLine: rgb(220, 220, 225), layout: layFrame, framePad: 36, underlineBrand: true,
		}
	default: // noir
		return slideTheme{
			bgTop: rgb(18, 22, 30), bgBot: rgb(28, 34, 46),
			blob1c: rgb(55, 72, 88), blob1s: 0.22, blob1x: 980, blob1y: 180, blob1r: 420,
			blob2c: rgb(42, 52, 68), blob2s: 0.28, blob2x: 120, blob2y: 1180, blob2r: 480,
			blob3c: rgb(232, 164, 90), blob3s: 0.06, blob3x: 900, blob3y: 1100, blob3r: 280,
			accent: rgb(232, 164, 90), accentDim: rgb(180, 128, 72),
			body: rgb(245, 246, 248), brandCol: rgb(168, 176, 188), muted: rgb(140, 148, 160),
			footerLine: rgb(48, 56, 70), leftBar: 10, underlineBrand: true, layout: layClassic,
		}
	}
}

func brandHandle(brand string) string {
	b := strings.TrimSpace(brand)
	b = strings.TrimLeft(b, "@")
	if b == "" {
		return "@brand"
	}
	return "@" + b
}

func rgb(r, g, b uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func lerpRGB(a, b color.RGBA, t float64) color.RGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return rgb(
		uint8(float64(a.R)+(float64(b.R)-float64(a.R))*t),
		uint8(float64(a.G)+(float64(b.G)-float64(a.G))*t),
		uint8(float64(a.B)+(float64(b.B)-float64(a.B))*t),
	)
}

func makeFace(ft *opentype.Font, size float64) font.Face {
	if ft == nil {
		return nil
	}
	f, err := opentype.NewFace(ft, &opentype.FaceOptions{
		Size: size, DPI: 96, Hinting: font.HintingNone,
	})
	if err != nil {
		return nil
	}
	return f
}

func closeFace(f font.Face) {
	if c, ok := f.(interface{ Close() error }); ok {
		_ = c.Close()
	}
}

func normalizeBody(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	var paras []string
	var buf []string
	flush := func() {
		if len(buf) == 0 {
			return
		}
		paras = append(paras, strings.Join(buf, " "))
		buf = nil
	}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			flush()
			continue
		}
		buf = append(buf, ln)
	}
	flush()
	return strings.Join(paras, "\n\n")
}

type drawLine struct {
	text   string
	spacer bool
}

func layoutLines(face font.Face, text string, maxWidth int) []drawLine {
	var out []drawLine
	paras := strings.Split(text, "\n\n")
	for pi, para := range paras {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		words := strings.Fields(para)
		if len(words) == 0 {
			continue
		}
		cur := words[0]
		for _, w := range words[1:] {
			trial := cur + " " + w
			if font.MeasureString(face, trial).Ceil() <= maxWidth {
				cur = trial
			} else {
				out = append(out, drawLine{text: cur})
				cur = w
			}
		}
		out = append(out, drawLine{text: cur})
		if pi < len(paras)-1 {
			out = append(out, drawLine{spacer: true})
		}
	}
	return out
}

func blockHeight(face font.Face, lines []drawLine, lineGap, paraGap int) int {
	h := 0
	lineH := face.Metrics().Height.Ceil() + lineGap
	for _, ln := range lines {
		if ln.spacer {
			h += paraGap
			continue
		}
		h += lineH
	}
	return h
}


func fillGradient(img *image.RGBA, top, bottom color.RGBA) {
	for y := 0; y < slideH; y++ {
		t := float64(y) / float64(slideH-1)
		c := lerpRGB(top, bottom, t)
		for x := 0; x < slideW; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func softBlob(img *image.RGBA, cx, cy, radius int, tint color.RGBA, strength float64) {
	r2 := radius * radius
	for y := cy - radius; y <= cy+radius; y++ {
		if y < 0 || y >= slideH {
			continue
		}
		dy := y - cy
		for x := cx - radius; x <= cx+radius; x++ {
			if x < 0 || x >= slideW {
				continue
			}
			dx := x - cx
			d2 := dx*dx + dy*dy
			if d2 > r2 {
				continue
			}
			fall := 1 - float64(d2)/float64(r2)
			fall *= fall
			t := fall * strength
			if t <= 0 {
				continue
			}
			base := img.RGBAAt(x, y)
			img.SetRGBA(x, y, lerpRGB(base, tint, t))
		}
	}
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > slideW {
		x1 = slideW
	}
	if y1 > slideH {
		y1 = slideH
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawString(img *image.RGBA, face font.Face, s string, x, y int, col color.RGBA) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

func drawHLine(img *image.RGBA, x0, x1, y int, c color.RGBA) {
	if y < 0 || y >= slideH {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 > slideW {
		x1 = slideW
	}
	for x := x0; x < x1; x++ {
		img.SetRGBA(x, y, c)
	}
}

func drawProgressDots(img *image.RGBA, total, active, cx, y, gap, rOn, rOff int, on, off color.RGBA) {
	if total < 2 || total > 10 {
		return
	}
	w := (total - 1) * gap
	x0 := cx - w/2
	for i := 0; i < total; i++ {
		x := x0 + i*gap
		rr := rOff
		c := off
		if i == active {
			rr = rOn
			c = on
		}
		for dy := -rr; dy <= rr; dy++ {
			for dx := -rr; dx <= rr; dx++ {
				if dx*dx+dy*dy <= rr*rr {
					px, py := x+dx, y+dy
					if px >= 0 && px < slideW && py >= 0 && py < slideH {
						img.SetRGBA(px, py, c)
					}
				}
			}
		}
	}
}

// RenderSlidePNG menulis PNG 1080×1350 dengan template pilihan akun.
// Brand selalu ditampilkan (fallback @brand). Teks di-scale ke ruang tersedia.
// slideNum 1-based; slideTotal 0 = sembunyikan indikator.
func RenderSlidePNG(path, brand, text string, slideNum, slideTotal int, template string) error {
	if fontBody == nil {
		return fmt.Errorf("font slide belum siap")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	th := themeFor(template)
	img := image.NewRGBA(image.Rect(0, 0, slideW, slideH))

	fillGradient(img, th.bgTop, th.bgBot)
	if th.blob1s > 0 {
		softBlob(img, th.blob1x, th.blob1y, th.blob1r, th.blob1c, th.blob1s)
	}
	if th.blob2s > 0 {
		softBlob(img, th.blob2x, th.blob2y, th.blob2r, th.blob2c, th.blob2s)
	}
	if th.blob3s > 0 {
		softBlob(img, th.blob3x, th.blob3y, th.blob3r, th.blob3c, th.blob3s)
	}
	if th.leftBar > 0 {
		fillRect(img, 0, 0, th.leftBar, slideH, th.accent)
	}
	if th.bottomBar > 0 {
		fillRect(img, 0, slideH-th.bottomBar, slideW, slideH, th.accent)
	}
	if th.layout == layFrame {
		fp := th.framePad
		if fp < 28 {
			fp = 28
		}
		fillRect(img, fp, fp, fp+4, slideH-fp, th.accent)
		fillRect(img, slideW-fp-4, fp, slideW-fp, slideH-fp, th.accent)
		fillRect(img, fp, fp, slideW-fp, fp+4, th.accent)
		fillRect(img, fp, slideH-fp-4, slideW-fp, slideH-fp, th.accent)
	}
	if th.layout == layTopBand {
		fillRect(img, 0, 0, slideW, 132, th.bandBg)
	}

	padX := 88
	if th.leftBar > 10 {
		padX = 100
	}
	padTop := 72
	padBottom := 110
	if th.bottomBar > 0 {
		padBottom += th.bottomBar
	}
	if th.layout == layFrame {
		padX = th.framePad + 48
		padTop = th.framePad + 48
		padBottom = th.framePad + 90
	}
	if th.layout == layTopBand {
		padTop = 160
	}

	contentLeft := padX
	contentRight := slideW - padX
	maxW := contentRight - contentLeft

	if th.cardInset {
		inset := 44
		topInset := inset + 16
		if th.layout == layTopBand {
			topInset = 148
		}
		fillRect(img, inset, topInset, slideW-inset, slideH-inset-16, th.cardBg)
		contentLeft = inset + 44
		contentRight = slideW - inset - 44
		padX = contentLeft
		maxW = contentRight - contentLeft
		padTop = topInset + 40
		padBottom = inset + 88
	}

	handle := brandHandle(brand)
	y := padTop

	switch th.layout {
	case layTopBand:
		fh := makeFace(fontBrand, 30)
		if fh != nil {
			hw := font.MeasureString(fh, handle).Ceil()
			bx := (slideW - hw) / 2
			drawString(img, fh, handle, bx, 48+fh.Metrics().Ascent.Ceil(), th.brandCol)
			closeFace(fh)
		}
		y = padTop
	case layPill:
		fh := makeFace(fontBrand, 26)
		if fh != nil {
			hw := font.MeasureString(fh, handle).Ceil()
			ph, pv := 22, 14
			pillW, pillH := hw+ph*2, fh.Metrics().Height.Ceil()+pv*2
			fillRect(img, padX, y, padX+pillW, y+pillH, th.pillBg)
			drawString(img, fh, handle, padX+ph, y+pv+fh.Metrics().Ascent.Ceil()-2, th.brandCol)
			y += pillH + 20
			closeFace(fh)
		}
	default:
		fh := makeFace(fontBrand, 28)
		if fh != nil {
			drawString(img, fh, handle, padX, y+fh.Metrics().Ascent.Ceil(), th.brandCol)
			hw := font.MeasureString(fh, handle).Ceil()
			y += fh.Metrics().Height.Ceil() + 14
			closeFace(fh)
			if th.underlineBrand {
				barW := hw
				if barW < 64 {
					barW = 64
				}
				if barW > 260 {
					barW = 260
				}
				for dy := 0; dy < 4; dy++ {
					drawHLine(img, padX, padX+barW, y+dy, th.accent)
				}
				y += 18
			} else {
				y += 8
			}
		}
	}

	// Jarak jelas antara brand dan isi slide
	y += 48

	if th.layout == layQuote {
		fq := makeFace(fontBrand, 96)
		if fq != nil {
			drawString(img, fq, "“", padX-8, y+fq.Metrics().Ascent.Ceil()-10, th.accent)
			y += 36
			closeFace(fq)
		}
	}

	bodyTop := y
	body := normalizeBody(text)
	if body == "" {
		body = "Isi slide muncul di sini."
	}
	if utf8.RuneCountInString(body) > 500 {
		runes := []rune(body)
		body = string(runes[:500])
	}

	footerReserve := 80
	bodyAvail := slideH - padBottom - bodyTop - footerReserve
	if bodyAvail < 200 {
		bodyAvail = 200
	}

	var faceBody font.Face
	var lines []drawLine
	var lineGap, paraGap int

	// Pakai ukuran terbesar yang masih muat di bodyAvail — teks pendek (meski 3 paragraf)
	// otomatis membesar supaya whitespace berkurang. Bukan melebar jarak paragraf.
	for size := 50.0; size >= 26.0; size -= 1 {
		if faceBody != nil {
			closeFace(faceBody)
		}
		faceBody = makeFace(fontBody, size)
		if faceBody == nil {
			continue
		}
		lineGap = int(size * 0.34)
		if lineGap < 4 {
			lineGap = 4
		}
		paraGap = lineGap + 10
		lines = layoutLines(faceBody, body, maxW)
		if blockHeight(faceBody, lines, lineGap, paraGap) <= bodyAvail {
			break
		}
	}
	if faceBody == nil {
		return fmt.Errorf("gagal siapkan font body")
	}
	defer closeFace(faceBody)

	lineH := faceBody.Metrics().Height.Ceil() + lineGap
	ascent := faceBody.Metrics().Ascent.Ceil()
	maxY := slideH - padBottom - footerReserve
	y = bodyTop
	for _, ln := range lines {
		if ln.spacer {
			y += paraGap
			continue
		}
		if y+lineH > maxY {
			break
		}
		drawString(img, faceBody, ln.text, padX, y+ascent, th.body)
		y += lineH
	}

	fy := slideH - 78
	if th.bottomBar > 0 {
		fy -= th.bottomBar
	}
	if th.cardInset || th.layout == layFrame {
		fy = slideH - padBottom + 18
		if fy > slideH-60 {
			fy = slideH - 60
		}
	}
	drawHLine(img, padX, contentRight, fy-28, th.footerLine)
	drawHLine(img, padX, padX+72, fy-28, th.accentDim)

	if slideTotal >= 2 {
		label := fmt.Sprintf("%02d  /  %02d", slideNum, slideTotal)
		ff := makeFace(fontBrand, 20)
		if ff != nil {
			drawString(img, ff, label, padX, fy+ff.Metrics().Ascent.Ceil()-4, th.muted)
			closeFace(ff)
		}
		dotActive := slideNum - 1
		if dotActive < 0 {
			dotActive = 0
		}
		if dotActive >= slideTotal {
			dotActive = slideTotal - 1
		}
		drawProgressDots(img, slideTotal, dotActive, slideW/2, fy+6, 22, 5, 3, th.accent, th.footerLine)

		nudge := "geser >"
		if slideNum >= slideTotal {
			nudge = "selesai"
		}
		fn := makeFace(fontBrand, 18)
		if fn != nil {
			nw := font.MeasureString(fn, nudge).Ceil()
			drawString(img, fn, nudge, contentRight-nw, fy+fn.Metrics().Ascent.Ceil()-4, th.accent)
			closeFace(fn)
		}
	}

	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+3] = 255
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
