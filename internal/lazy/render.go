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
	layEditorial layoutKind = iota // left brand + thin left rail + underline
	layBand                        // solid top header band, brand centered
	layCard                        // soft inset card + pill brand
	layQuote                       // large “ mark + small brand above
	layFrame                       // thin inset border, generous padding
	layCenter                      // centered brand + centered body
)

type slideTheme struct {
	bgTop, bgBot           color.RGBA
	blob1c, blob2c, blob3c color.RGBA
	blob1s, blob2s, blob3s float64
	blob1x, blob1y, blob1r int
	blob2x, blob2y, blob2r int
	blob3x, blob3y, blob3r int
	accent, accentDim      color.RGBA
	body, brandCol         color.RGBA
	muted, footerLine      color.RGBA
	leftBar                int // editorial thin rail (6–10)
	cardInset              bool
	cardBg                 color.RGBA
	underlineBrand         bool
	layout                 layoutKind
	bandBg                 color.RGBA
	pillBg                 color.RGBA
	framePad               int
	centerBody             bool
	maxBody                float64 // 0 = default 48
	minBody                float64 // 0 = default 28
	markBrand              bool    // 10×10 accent square before brand
	leadAccent             bool    // first paragraph in accent color
	topHairline            bool
}

func clampBlob(s float64) float64 {
	if s <= 0 {
		return 0
	}
	if s > 0.12 {
		return 0.12
	}
	return s
}

func themeFor(id string) slideTheme {
	switch NormalizeTemplate(id) {
	case TemplatePaper:
		return slideTheme{
			bgTop: rgb(244, 241, 234), bgBot: rgb(232, 226, 214),
			blob1c: rgb(220, 210, 190), blob1s: 0.10, blob1x: 900, blob1y: 200, blob1r: 340,
			blob2c: rgb(210, 200, 180), blob2s: 0.08, blob2x: 160, blob2y: 1100, blob2r: 360,
			accent: rgb(26, 26, 26), accentDim: rgb(80, 80, 80),
			body: rgb(22, 22, 22), brandCol: rgb(70, 70, 70), muted: rgb(120, 120, 120),
			footerLine: rgb(210, 200, 185), underlineBrand: true, layout: layFrame, framePad: 48,
			markBrand: true, topHairline: true,
		}
	case TemplateOcean:
		return slideTheme{
			bgTop: rgb(11, 36, 40), bgBot: rgb(18, 58, 66),
			blob1c: rgb(30, 90, 100), blob1s: 0.12, blob1x: 960, blob1y: 160, blob1r: 360,
			blob2c: rgb(20, 70, 80), blob2s: 0.10, blob2x: 100, blob2y: 1200, blob2r: 400,
			accent: rgb(94, 234, 212), accentDim: rgb(45, 160, 145),
			body: rgb(236, 252, 248), brandCol: rgb(148, 196, 188), muted: rgb(120, 160, 155),
			footerLine: rgb(40, 80, 88), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateInk:
		return slideTheme{
			bgTop: rgb(10, 10, 10), bgBot: rgb(20, 20, 20),
			blob1c: rgb(40, 40, 40), blob1s: 0.10, blob1x: 950, blob1y: 220, blob1r: 320,
			blob2c: rgb(35, 35, 35), blob2s: 0.08, blob2x: 140, blob2y: 1150, blob2r: 360,
			accent: rgb(245, 245, 245), accentDim: rgb(180, 180, 180),
			body: rgb(245, 245, 245), brandCol: rgb(180, 180, 180), muted: rgb(130, 130, 130),
			footerLine: rgb(50, 50, 50), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateEmber:
		return slideTheme{
			bgTop: rgb(26, 20, 18), bgBot: rgb(42, 30, 26),
			blob1c: rgb(70, 40, 35), blob1s: 0.10, blob1x: 920, blob1y: 180, blob1r: 360,
			blob2c: rgb(55, 35, 30), blob2s: 0.08, blob2x: 140, blob2y: 1100, blob2r: 380,
			accent: rgb(232, 93, 76), accentDim: rgb(180, 70, 55),
			body: rgb(250, 244, 240), brandCol: rgb(26, 20, 18), muted: rgb(150, 120, 110),
			footerLine: rgb(60, 42, 38), layout: layBand, bandBg: rgb(232, 93, 76),
			leadAccent: true,
		}
	case TemplateBloom:
		return slideTheme{
			bgTop: rgb(255, 240, 243), bgBot: rgb(255, 224, 232),
			blob1c: rgb(255, 180, 200), blob1s: 0.10, blob1x: 920, blob1y: 160, blob1r: 360,
			blob2c: rgb(255, 200, 210), blob2s: 0.08, blob2x: 120, blob2y: 1180, blob2r: 380,
			accent: rgb(232, 90, 140), accentDim: rgb(200, 100, 140),
			body: rgb(70, 30, 48), brandCol: rgb(255, 255, 255), muted: rgb(160, 110, 130),
			footerLine: rgb(255, 200, 215), layout: layCard, pillBg: rgb(232, 90, 140),
			cardInset: true, cardBg: rgb(255, 250, 252), markBrand: false, leadAccent: true,
		}
	case TemplateLilac:
		return slideTheme{
			bgTop: rgb(246, 241, 255), bgBot: rgb(233, 222, 255),
			blob1c: rgb(200, 180, 255), blob1s: 0.10, blob1x: 940, blob1y: 180, blob1r: 360,
			blob2c: rgb(210, 190, 255), blob2s: 0.08, blob2x: 140, blob2y: 1150, blob2r: 380,
			accent: rgb(139, 92, 246), accentDim: rgb(110, 70, 200),
			body: rgb(45, 30, 80), brandCol: rgb(130, 100, 190), muted: rgb(140, 120, 180),
			footerLine: rgb(210, 195, 240), layout: layCenter, centerBody: true,
			markBrand: true, topHairline: true,
		}
	case TemplatePeach:
		return slideTheme{
			bgTop: rgb(255, 245, 238), bgBot: rgb(255, 228, 209),
			blob1c: rgb(255, 200, 170), blob1s: 0.10, blob1x: 900, blob1y: 200, blob1r: 340,
			blob2c: rgb(255, 190, 160), blob2s: 0.08, blob2x: 150, blob2y: 1120, blob2r: 360,
			accent: rgb(240, 120, 90), accentDim: rgb(210, 100, 75),
			body: rgb(70, 40, 30), brandCol: rgb(255, 255, 255), muted: rgb(160, 120, 100),
			footerLine: rgb(240, 210, 190), layout: layCard, pillBg: rgb(240, 120, 90),
			cardInset: true, cardBg: rgb(255, 252, 248), leadAccent: true,
		}
	case TemplateBold:
		return slideTheme{
			bgTop: rgb(17, 24, 39), bgBot: rgb(31, 41, 55),
			blob1c: rgb(55, 65, 85), blob1s: 0.10, blob1x: 960, blob1y: 400, blob1r: 340,
			blob2c: rgb(40, 50, 70), blob2s: 0.08, blob2x: 120, blob2y: 1100, blob2r: 360,
			accent: rgb(245, 158, 11), accentDim: rgb(200, 130, 20),
			body: rgb(248, 250, 252), brandCol: rgb(17, 24, 39), muted: rgb(148, 163, 184),
			footerLine: rgb(55, 65, 85), layout: layBand, bandBg: rgb(245, 158, 11),
			leadAccent: true,
		}
	case TemplateFrame:
		return slideTheme{
			bgTop: rgb(250, 250, 248), bgBot: rgb(240, 237, 230),
			blob1c: rgb(230, 225, 215), blob1s: 0.08, blob1x: 900, blob1y: 200, blob1r: 320,
			accent: rgb(15, 23, 42), accentDim: rgb(71, 85, 105),
			body: rgb(15, 23, 42), brandCol: rgb(15, 23, 42), muted: rgb(100, 116, 139),
			footerLine: rgb(210, 205, 195), layout: layFrame, framePad: 48, underlineBrand: true,
			markBrand: true, topHairline: true,
		}
	case TemplateMeadow:
		return slideTheme{
			bgTop: rgb(232, 238, 230), bgBot: rgb(213, 224, 210),
			blob1c: rgb(190, 210, 185), blob1s: 0.10, blob1x: 900, blob1y: 200, blob1r: 340,
			blob2c: rgb(180, 200, 175), blob2s: 0.08, blob2x: 150, blob2y: 1120, blob2r: 360,
			accent: rgb(47, 74, 58), accentDim: rgb(80, 110, 90),
			body: rgb(28, 48, 38), brandCol: rgb(255, 255, 255), muted: rgb(100, 120, 105),
			footerLine: rgb(190, 205, 185), layout: layCard, pillBg: rgb(47, 74, 58),
			cardInset: true, cardBg: rgb(248, 251, 246), leadAccent: true,
		}
	case TemplateMidnight:
		return slideTheme{
			bgTop: rgb(11, 18, 32), bgBot: rgb(21, 34, 56),
			blob1c: rgb(40, 70, 120), blob1s: 0.12, blob1x: 940, blob1y: 180, blob1r: 360,
			blob2c: rgb(30, 50, 90), blob2s: 0.10, blob2x: 120, blob2y: 1160, blob2r: 380,
			accent: rgb(96, 165, 250), accentDim: rgb(59, 130, 246),
			body: rgb(239, 246, 255), brandCol: rgb(147, 197, 253), muted: rgb(100, 140, 190),
			footerLine: rgb(40, 60, 90), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateCoral:
		return slideTheme{
			bgTop: rgb(31, 20, 18), bgBot: rgb(44, 28, 24),
			blob1c: rgb(80, 40, 35), blob1s: 0.10, blob1x: 920, blob1y: 400, blob1r: 320,
			accent: rgb(255, 122, 89), accentDim: rgb(220, 90, 60),
			body: rgb(255, 247, 244), brandCol: rgb(31, 20, 18), muted: rgb(180, 140, 130),
			footerLine: rgb(70, 45, 40), layout: layBand, bandBg: rgb(255, 122, 89),
			leadAccent: true,
		}
	case TemplateMint:
		return slideTheme{
			bgTop: rgb(236, 253, 245), bgBot: rgb(209, 250, 229),
			blob1c: rgb(167, 243, 208), blob1s: 0.10, blob1x: 900, blob1y: 180, blob1r: 360,
			blob2c: rgb(110, 231, 183), blob2s: 0.08, blob2x: 140, blob2y: 1140, blob2r: 360,
			accent: rgb(5, 150, 105), accentDim: rgb(4, 120, 87),
			body: rgb(6, 60, 45), brandCol: rgb(255, 255, 255), muted: rgb(80, 140, 120),
			footerLine: rgb(167, 243, 208), layout: layCard, pillBg: rgb(5, 150, 105),
			cardInset: true, cardBg: rgb(255, 255, 255), leadAccent: true,
		}
	case TemplateCherry:
		return slideTheme{
			bgTop: rgb(255, 241, 242), bgBot: rgb(255, 228, 230),
			blob1c: rgb(254, 180, 190), blob1s: 0.10, blob1x: 920, blob1y: 160, blob1r: 360,
			blob2c: rgb(252, 165, 165), blob2s: 0.08, blob2x: 130, blob2y: 1160, blob2r: 380,
			accent: rgb(190, 18, 60), accentDim: rgb(159, 18, 57),
			body: rgb(76, 10, 30), brandCol: rgb(170, 80, 100), muted: rgb(170, 100, 120),
			footerLine: rgb(254, 200, 210), layout: layCenter, centerBody: true,
			markBrand: true, topHairline: true,
		}
	case TemplateSand:
		return slideTheme{
			bgTop: rgb(250, 246, 240), bgBot: rgb(240, 230, 216),
			blob1c: rgb(230, 210, 180), blob1s: 0.10, blob1x: 900, blob1y: 200, blob1r: 340,
			blob2c: rgb(220, 195, 160), blob2s: 0.08, blob2x: 150, blob2y: 1100, blob2r: 360,
			accent: rgb(139, 105, 20), accentDim: rgb(120, 90, 20),
			body: rgb(55, 40, 20), brandCol: rgb(100, 75, 20), muted: rgb(140, 120, 90),
			footerLine: rgb(220, 200, 170), underlineBrand: true, layout: layQuote,
			markBrand: true, leadAccent: true,
		}
	case TemplateNeon:
		return slideTheme{
			bgTop: rgb(5, 8, 10), bgBot: rgb(13, 21, 18),
			blob1c: rgb(20, 50, 30), blob1s: 0.12, blob1x: 940, blob1y: 200, blob1r: 340,
			blob2c: rgb(15, 40, 25), blob2s: 0.08, blob2x: 120, blob2y: 1150, blob2r: 360,
			accent: rgb(57, 255, 20), accentDim: rgb(40, 200, 15),
			body: rgb(230, 255, 230), brandCol: rgb(120, 200, 120), muted: rgb(100, 150, 100),
			footerLine: rgb(30, 50, 35), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateSlate:
		return slideTheme{
			bgTop: rgb(241, 245, 249), bgBot: rgb(226, 232, 240),
			blob1c: rgb(200, 210, 225), blob1s: 0.08, blob1x: 900, blob1y: 180, blob1r: 340,
			blob2c: rgb(180, 195, 215), blob2s: 0.06, blob2x: 140, blob2y: 1120, blob2r: 360,
			accent: rgb(51, 65, 85), accentDim: rgb(71, 85, 105),
			body: rgb(15, 23, 42), brandCol: rgb(255, 255, 255), muted: rgb(100, 116, 139),
			footerLine: rgb(200, 210, 220), layout: layCard, pillBg: rgb(51, 65, 85),
			cardInset: true, cardBg: rgb(255, 255, 255), leadAccent: true,
		}
	case TemplateHoney:
		return slideTheme{
			bgTop: rgb(255, 251, 235), bgBot: rgb(254, 243, 199),
			blob1c: rgb(253, 224, 140), blob1s: 0.10, blob1x: 900, blob1y: 180, blob1r: 360,
			blob2c: rgb(252, 211, 77), blob2s: 0.08, blob2x: 140, blob2y: 1140, blob2r: 360,
			accent: rgb(217, 119, 6), accentDim: rgb(180, 95, 10),
			body: rgb(70, 40, 10), brandCol: rgb(217, 119, 6), muted: rgb(160, 120, 60),
			footerLine: rgb(250, 220, 140), layout: layQuote, underlineBrand: true,
			markBrand: true, leadAccent: true,
		}
	case TemplateMono:
		return slideTheme{
			bgTop: rgb(255, 255, 255), bgBot: rgb(244, 244, 245),
			blob1c: rgb(230, 230, 235), blob1s: 0.06, blob1x: 900, blob1y: 200, blob1r: 300,
			accent: rgb(9, 9, 11), accentDim: rgb(60, 60, 70),
			body: rgb(9, 9, 11), brandCol: rgb(9, 9, 11), muted: rgb(110, 110, 120),
			footerLine: rgb(220, 220, 225), layout: layFrame, framePad: 48, underlineBrand: true,
			markBrand: true, topHairline: true,
		}
	case TemplateAurora:
		return slideTheme{
			bgTop: rgb(10, 16, 32), bgBot: rgb(26, 20, 64),
			blob1c: rgb(34, 211, 238), blob1s: 0.10, blob1x: 920, blob1y: 180, blob1r: 360,
			blob2c: rgb(192, 38, 211), blob2s: 0.10, blob2x: 140, blob2y: 1180, blob2r: 400,
			accent: rgb(34, 211, 238), accentDim: rgb(8, 145, 178),
			body: rgb(240, 249, 255), brandCol: rgb(165, 243, 252), muted: rgb(125, 160, 200),
			footerLine: rgb(40, 50, 90), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateCocoa:
		return slideTheme{
			bgTop: rgb(247, 240, 232), bgBot: rgb(234, 217, 200),
			blob1c: rgb(210, 180, 150), blob1s: 0.10, blob1x: 900, blob1y: 200, blob1r: 340,
			blob2c: rgb(190, 150, 120), blob2s: 0.08, blob2x: 140, blob2y: 1120, blob2r: 360,
			accent: rgb(107, 63, 42), accentDim: rgb(140, 90, 60),
			body: rgb(55, 32, 20), brandCol: rgb(255, 255, 255), muted: rgb(140, 110, 90),
			footerLine: rgb(210, 185, 160), layout: layCard, pillBg: rgb(107, 63, 42),
			cardInset: true, cardBg: rgb(255, 250, 245), leadAccent: true,
		}
	case TemplateIvory:
		return slideTheme{
			bgTop: rgb(255, 254, 247), bgBot: rgb(245, 240, 225),
			blob1c: rgb(230, 210, 160), blob1s: 0.08, blob1x: 920, blob1y: 160, blob1r: 320,
			blob2c: rgb(220, 195, 140), blob2s: 0.06, blob2x: 150, blob2y: 1140, blob2r: 340,
			accent: rgb(180, 83, 9), accentDim: rgb(146, 64, 14),
			body: rgb(40, 30, 15), brandCol: rgb(120, 70, 20), muted: rgb(140, 120, 90),
			footerLine: rgb(220, 200, 160), layout: layFrame, framePad: 48, underlineBrand: true,
			markBrand: true, topHairline: true,
		}
	case TemplateForest:
		return slideTheme{
			bgTop: rgb(12, 22, 16), bgBot: rgb(21, 36, 26),
			blob1c: rgb(34, 80, 50), blob1s: 0.12, blob1x: 940, blob1y: 180, blob1r: 360,
			blob2c: rgb(25, 60, 40), blob2s: 0.10, blob2x: 120, blob2y: 1160, blob2r: 380,
			accent: rgb(134, 239, 172), accentDim: rgb(74, 180, 120),
			body: rgb(236, 253, 245), brandCol: rgb(160, 200, 170), muted: rgb(110, 160, 130),
			footerLine: rgb(35, 60, 45), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateRose:
		return slideTheme{
			bgTop: rgb(255, 245, 245), bgBot: rgb(245, 208, 208),
			blob1c: rgb(240, 180, 190), blob1s: 0.10, blob1x: 900, blob1y: 180, blob1r: 360,
			blob2c: rgb(230, 160, 170), blob2s: 0.08, blob2x: 140, blob2y: 1140, blob2r: 360,
			accent: rgb(159, 74, 92), accentDim: rgb(130, 60, 75),
			body: rgb(70, 30, 40), brandCol: rgb(159, 74, 92), muted: rgb(160, 110, 120),
			footerLine: rgb(235, 190, 200), layout: layCenter, centerBody: true,
			markBrand: true, topHairline: true,
		}
	case TemplateGraphite:
		return slideTheme{
			bgTop: rgb(20, 22, 24), bgBot: rgb(35, 38, 43),
			blob1c: rgb(60, 64, 72), blob1s: 0.10, blob1x: 940, blob1y: 200, blob1r: 340,
			blob2c: rgb(50, 54, 60), blob2s: 0.08, blob2x: 130, blob2y: 1150, blob2r: 360,
			accent: rgb(161, 161, 170), accentDim: rgb(113, 113, 122),
			body: rgb(244, 244, 245), brandCol: rgb(212, 212, 216), muted: rgb(140, 140, 150),
			footerLine: rgb(55, 58, 64), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateCitrus:
		return slideTheme{
			bgTop: rgb(254, 252, 232), bgBot: rgb(254, 240, 138),
			blob1c: rgb(250, 204, 21), blob1s: 0.10, blob1x: 900, blob1y: 180, blob1r: 360,
			blob2c: rgb(234, 179, 8), blob2s: 0.08, blob2x: 140, blob2y: 1140, blob2r: 360,
			accent: rgb(161, 98, 7), accentDim: rgb(133, 77, 14),
			body: rgb(50, 35, 5), brandCol: rgb(50, 35, 5), muted: rgb(120, 95, 40),
			footerLine: rgb(230, 200, 80), layout: layBand, bandBg: rgb(250, 204, 21),
			leadAccent: true,
		}
	case TemplateClay:
		return slideTheme{
			bgTop: rgb(255, 247, 243), bgBot: rgb(254, 215, 170),
			blob1c: rgb(253, 186, 116), blob1s: 0.10, blob1x: 900, blob1y: 200, blob1r: 340,
			blob2c: rgb(251, 146, 60), blob2s: 0.08, blob2x: 150, blob2y: 1120, blob2r: 360,
			accent: rgb(154, 52, 18), accentDim: rgb(124, 45, 18),
			body: rgb(70, 25, 10), brandCol: rgb(154, 52, 18), muted: rgb(160, 100, 70),
			footerLine: rgb(240, 190, 150), layout: layQuote, underlineBrand: true,
			markBrand: true, leadAccent: true,
		}
	case TemplateGlacier:
		return slideTheme{
			bgTop: rgb(240, 249, 255), bgBot: rgb(224, 242, 254),
			blob1c: rgb(186, 230, 253), blob1s: 0.10, blob1x: 920, blob1y: 160, blob1r: 360,
			blob2c: rgb(125, 211, 252), blob2s: 0.08, blob2x: 130, blob2y: 1160, blob2r: 380,
			accent: rgb(3, 105, 161), accentDim: rgb(7, 89, 133),
			body: rgb(12, 40, 70), brandCol: rgb(255, 255, 255), muted: rgb(90, 140, 180),
			footerLine: rgb(186, 230, 253), layout: layCard, pillBg: rgb(3, 105, 161),
			cardInset: true, cardBg: rgb(255, 255, 255), leadAccent: true,
		}
	case TemplateMatcha:
		return slideTheme{
			bgTop: rgb(244, 247, 239), bgBot: rgb(226, 235, 211),
			blob1c: rgb(190, 210, 160), blob1s: 0.10, blob1x: 900, blob1y: 180, blob1r: 360,
			blob2c: rgb(170, 195, 140), blob2s: 0.08, blob2x: 140, blob2y: 1140, blob2r: 360,
			accent: rgb(91, 112, 64), accentDim: rgb(70, 90, 50),
			body: rgb(35, 50, 25), brandCol: rgb(255, 255, 255), muted: rgb(110, 130, 90),
			footerLine: rgb(200, 215, 175), layout: layCard, pillBg: rgb(91, 112, 64),
			cardInset: true, cardBg: rgb(255, 255, 252), leadAccent: true,
		}
	case TemplateSignal:
		return slideTheme{
			bgTop: rgb(255, 255, 255), bgBot: rgb(254, 242, 242),
			blob1c: rgb(254, 202, 202), blob1s: 0.08, blob1x: 920, blob1y: 400, blob1r: 320,
			accent: rgb(220, 38, 38), accentDim: rgb(185, 28, 28),
			body: rgb(20, 10, 10), brandCol: rgb(255, 255, 255), muted: rgb(140, 100, 100),
			footerLine: rgb(254, 202, 202), layout: layBand, bandBg: rgb(220, 38, 38),
			leadAccent: true,
		}
	case TemplateEspresso:
		return slideTheme{
			bgTop: rgb(26, 18, 14), bgBot: rgb(42, 28, 22),
			blob1c: rgb(80, 50, 35), blob1s: 0.12, blob1x: 920, blob1y: 180, blob1r: 360,
			blob2c: rgb(60, 40, 28), blob2s: 0.10, blob2x: 140, blob2y: 1140, blob2r: 380,
			accent: rgb(231, 200, 160), accentDim: rgb(190, 150, 110),
			body: rgb(250, 240, 225), brandCol: rgb(200, 170, 140), muted: rgb(160, 130, 105),
			footerLine: rgb(60, 42, 34), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateSky:
		return slideTheme{
			bgTop: rgb(240, 247, 255), bgBot: rgb(219, 234, 254),
			blob1c: rgb(147, 197, 253), blob1s: 0.10, blob1x: 900, blob1y: 160, blob1r: 360,
			blob2c: rgb(96, 165, 250), blob2s: 0.08, blob2x: 140, blob2y: 1160, blob2r: 360,
			accent: rgb(29, 78, 216), accentDim: rgb(30, 64, 175),
			body: rgb(20, 40, 90), brandCol: rgb(80, 110, 180), muted: rgb(100, 130, 180),
			footerLine: rgb(190, 210, 240), layout: layCenter, centerBody: true,
			markBrand: true, topHairline: true,
		}
	case TemplateDusk:
		return slideTheme{
			bgTop: rgb(26, 18, 36), bgBot: rgb(42, 24, 48),
			blob1c: rgb(120, 50, 80), blob1s: 0.12, blob1x: 920, blob1y: 180, blob1r: 360,
			blob2c: rgb(80, 40, 60), blob2s: 0.10, blob2x: 130, blob2y: 1150, blob2r: 380,
			accent: rgb(251, 146, 60), accentDim: rgb(234, 88, 12),
			body: rgb(255, 247, 237), brandCol: rgb(253, 186, 116), muted: rgb(180, 140, 160),
			footerLine: rgb(60, 40, 70), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplatePearl:
		return slideTheme{
			bgTop: rgb(251, 251, 254), bgBot: rgb(241, 240, 247),
			blob1c: rgb(220, 220, 235), blob1s: 0.08, blob1x: 900, blob1y: 200, blob1r: 320,
			blob2c: rgb(200, 205, 225), blob2s: 0.06, blob2x: 150, blob2y: 1120, blob2r: 340,
			accent: rgb(71, 85, 105), accentDim: rgb(100, 116, 139),
			body: rgb(30, 41, 59), brandCol: rgb(71, 85, 105), muted: rgb(120, 130, 150),
			footerLine: rgb(220, 220, 230), layout: layFrame, framePad: 48, underlineBrand: true,
			markBrand: true, topHairline: true,
		}
	case TemplateOlive:
		return slideTheme{
			bgTop: rgb(244, 241, 232), bgBot: rgb(229, 224, 208),
			blob1c: rgb(190, 195, 160), blob1s: 0.10, blob1x: 900, blob1y: 200, blob1r: 340,
			blob2c: rgb(170, 180, 140), blob2s: 0.08, blob2x: 140, blob2y: 1120, blob2r: 360,
			accent: rgb(77, 92, 58), accentDim: rgb(60, 75, 45),
			body: rgb(35, 45, 25), brandCol: rgb(255, 255, 255), muted: rgb(110, 120, 90),
			footerLine: rgb(200, 195, 170), layout: layCard, pillBg: rgb(77, 92, 58),
			cardInset: true, cardBg: rgb(252, 250, 244), leadAccent: true,
		}
	case TemplateInkred:
		return slideTheme{
			bgTop: rgb(12, 12, 12), bgBot: rgb(24, 24, 24),
			blob1c: rgb(50, 20, 20), blob1s: 0.10, blob1x: 940, blob1y: 200, blob1r: 340,
			blob2c: rgb(40, 20, 20), blob2s: 0.08, blob2x: 120, blob2y: 1150, blob2r: 360,
			accent: rgb(239, 68, 68), accentDim: rgb(185, 28, 28),
			body: rgb(250, 250, 250), brandCol: rgb(200, 200, 200), muted: rgb(140, 140, 140),
			footerLine: rgb(50, 50, 50), underlineBrand: true, layout: layEditorial, leftBar: 8,
			markBrand: true, leadAccent: true,
		}
	case TemplateOrchid:
		return slideTheme{
			bgTop: rgb(251, 247, 255), bgBot: rgb(239, 228, 255),
			blob1c: rgb(216, 180, 254), blob1s: 0.10, blob1x: 920, blob1y: 160, blob1r: 360,
			blob2c: rgb(192, 132, 252), blob2s: 0.08, blob2x: 140, blob2y: 1160, blob2r: 360,
			accent: rgb(124, 58, 237), accentDim: rgb(109, 40, 217),
			body: rgb(45, 20, 80), brandCol: rgb(140, 100, 200), muted: rgb(140, 110, 180),
			footerLine: rgb(220, 200, 250), layout: layCenter, centerBody: true,
			markBrand: true, topHairline: true,
		}
	default: // noir
		return slideTheme{
			bgTop: rgb(18, 22, 30), bgBot: rgb(28, 34, 46),
			blob1c: rgb(55, 72, 88), blob1s: 0.12, blob1x: 980, blob1y: 180, blob1r: 360,
			blob2c: rgb(42, 52, 68), blob2s: 0.10, blob2x: 120, blob2y: 1180, blob2r: 400,
			accent: rgb(232, 164, 90), accentDim: rgb(180, 128, 72),
			body: rgb(245, 246, 248), brandCol: rgb(168, 176, 188), muted: rgb(140, 148, 160),
			footerLine: rgb(48, 56, 70), leftBar: 8, underlineBrand: true, layout: layEditorial,
			markBrand: true, leadAccent: true, topHairline: true,
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
	lead   bool // first paragraph (for leadAccent)
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
				out = append(out, drawLine{text: cur, lead: pi == 0})
				cur = w
			}
		}
		out = append(out, drawLine{text: cur, lead: pi == 0})
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
	strength = clampBlob(strength)
	if strength <= 0 || radius <= 0 {
		return
	}
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

func underlineBrandBar(img *image.RGBA, x, y, hw int, c color.RGBA) int {
	barW := hw
	if barW < 48 {
		barW = 48
	}
	if barW > 180 {
		barW = 180
	}
	for dy := 0; dy < 2; dy++ {
		drawHLine(img, x, x+barW, y+dy, c)
	}
	return 14
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

	const (
		padDefault = 100
		bandH      = 106
		cardInset  = 48
		brandSize  = 26
	)

	// Structural chrome (rails, bands, frames, cards) — no gimmick shapes.
	switch th.layout {
	case layEditorial:
		bar := th.leftBar
		if bar <= 0 {
			bar = 8
		}
		if bar < 6 {
			bar = 6
		}
		if bar > 10 {
			bar = 10
		}
		fillRect(img, 0, 0, bar, slideH, th.accent)
	case layBand:
		fillRect(img, 0, 0, slideW, bandH, th.bandBg)
	case layFrame:
		fp := th.framePad
		if fp < 40 {
			fp = 48
		}
		bw := 3
		fillRect(img, fp, fp, fp+bw, slideH-fp, th.accent)
		fillRect(img, slideW-fp-bw, fp, slideW-fp, slideH-fp, th.accent)
		fillRect(img, fp, fp, slideW-fp, fp+bw, th.accent)
		fillRect(img, fp, slideH-fp-bw, slideW-fp, slideH-fp, th.accent)
	case layCard:
		fillRect(img, cardInset, cardInset, slideW-cardInset, slideH-cardInset, th.cardBg)
	}

	if th.topHairline {
		hy := 36
		if th.layout == layBand {
			hy = bandH + 12
		}
		if th.layout == layCard {
			hy = cardInset + 20
		}
		if th.layout == layFrame {
			fp := th.framePad
			if fp < 40 {
				fp = 48
			}
			hy = fp + 18
		}
		drawHLine(img, padDefault, slideW-padDefault, hy, th.footerLine)
	}

	padX := padDefault
	padTop := 96
	padBottom := 108

	switch th.layout {
	case layEditorial:
		if th.leftBar > 0 {
			padX = padDefault + 8
		}
		padTop = 100
	case layBand:
		padTop = bandH + 48
	case layCard:
		padX = cardInset + 48
		padTop = cardInset + 44
		padBottom = cardInset + 88
	case layFrame:
		fp := th.framePad
		if fp < 40 {
			fp = 48
		}
		padX = fp + 44
		padTop = fp + 44
		padBottom = fp + 88
	case layQuote:
		padTop = 92
	case layCenter:
		padX = 110
		padTop = 120
	}

	contentLeft := padX
	contentRight := slideW - padX
	if th.layout == layCenter {
		contentLeft = 110
		contentRight = slideW - 110
		padX = contentLeft
	}
	maxW := contentRight - contentLeft

	handle := brandHandle(brand)
	y := padTop
	centerBody := th.centerBody || th.layout == layCenter

	drawBrandRow := func(size float64, x int, centered bool) {
		fh := makeFace(fontBrand, size)
		if fh == nil {
			return
		}
		defer closeFace(fh)
		hw := font.MeasureString(fh, handle).Ceil()
		bx := x
		markGap := 0
		if th.markBrand && th.layout != layBand && th.layout != layCard {
			markGap = 18
		}
		totalW := hw + markGap
		if centered {
			bx = (slideW - totalW) / 2
		}
		ascent := fh.Metrics().Ascent.Ceil()
		lineH := fh.Metrics().Height.Ceil()
		baseline := y + ascent

		if markGap > 0 {
			my := y + (lineH-10)/2
			fillRect(img, bx, my, bx+10, my+10, th.accent)
			bx += markGap
		}
		drawString(img, fh, handle, bx, baseline, th.brandCol)
		y += lineH + 12
		if th.underlineBrand {
			ux := bx
			if centered {
				uw := hw
				if uw < 48 {
					uw = 48
				}
				if uw > 160 {
					uw = 160
				}
				ux = (slideW - uw) / 2
				y += underlineBrandBar(img, ux, y, uw, th.accent)
			} else {
				y += underlineBrandBar(img, ux, y, hw, th.accent)
			}
		} else {
			y += 6
		}
	}

	switch th.layout {
	case layBand:
		fh := makeFace(fontBrand, 27)
		if fh != nil {
			hw := font.MeasureString(fh, handle).Ceil()
			bx := (slideW - hw) / 2
			by := (bandH-fh.Metrics().Height.Ceil())/2 + fh.Metrics().Ascent.Ceil()
			drawString(img, fh, handle, bx, by, th.brandCol)
			closeFace(fh)
		}
		y = padTop
	case layCard:
		fh := makeFace(fontBrand, 24)
		if fh != nil {
			hw := font.MeasureString(fh, handle).Ceil()
			ph, pv := 20, 12
			pillW := hw + ph*2
			pillH := fh.Metrics().Height.Ceil() + pv*2
			bx := padX
			fillRect(img, bx, y, bx+pillW, y+pillH, th.pillBg)
			drawString(img, fh, handle, bx+ph, y+pv+fh.Metrics().Ascent.Ceil()-2, th.brandCol)
			y += pillH + 28
			closeFace(fh)
		}
	case layQuote:
		fh := makeFace(fontBrand, 22)
		if fh != nil {
			markGap := 0
			bx := padX
			if th.markBrand {
				my := y + (fh.Metrics().Height.Ceil()-10)/2
				fillRect(img, bx, my, bx+10, my+10, th.accent)
				markGap = 16
			}
			drawString(img, fh, handle, bx+markGap, y+fh.Metrics().Ascent.Ceil(), th.brandCol)
			y += fh.Metrics().Height.Ceil() + 10
			closeFace(fh)
			if th.underlineBrand {
				y += underlineBrandBar(img, padX, y, 72, th.accent)
			}
		}
		fq := makeFace(fontBrand, 88)
		if fq != nil {
			drawString(img, fq, "“", padX-6, y+fq.Metrics().Ascent.Ceil()-8, th.accent)
			y += 28
			closeFace(fq)
		}
	case layCenter:
		drawBrandRow(24, padX, true)
	case layEditorial, layFrame:
		drawBrandRow(brandSize, padX, false)
	default:
		drawBrandRow(brandSize, padX, false)
	}

	// Breathing room between brand block and body.
	switch th.layout {
	case layBand:
		y += 36
	case layCard:
		y += 8
	case layQuote:
		y += 20
	case layCenter:
		y += 56
	default:
		y += 44
	}

	bodyTop := y
	body := normalizeBody(text)
	if body == "" {
		body = "Isi slide muncul di sini."
	}
	bodyLimit := 500
	isCover := slideNum == 1 && slideTotal > 1
	if isCover {
		// Slide pertama adalah cover: headline lebih padat dan lebih tebal.
		bodyLimit = 280
		centerBody = true
	}
	if utf8.RuneCountInString(body) > bodyLimit {
		runes := []rune(body)
		body = string(runes[:bodyLimit])
	}

	footerReserve := 72
	bodyAvail := slideH - padBottom - bodyTop - footerReserve
	if bodyAvail < 200 {
		bodyAvail = 200
	}

	maxSz := th.maxBody
	if maxSz <= 0 {
		maxSz = 52
	}
	minSz := th.minBody
	if minSz <= 0 {
		minSz = 28
	}
	if maxSz > 54 {
		maxSz = 54
	}
	if minSz < 28 {
		minSz = 28
	}
	if isCover {
		maxSz = 68
		minSz = 36
	}

	var faceBody font.Face
	var lines []drawLine
	var lineGap, paraGap int

	for size := maxSz; size >= minSz; size -= 1 {
		if faceBody != nil {
			closeFace(faceBody)
		}
		bodyFont := fontBody
		if isCover {
			bodyFont = fontBrand
		}
		faceBody = makeFace(bodyFont, size)
		if faceBody == nil {
			continue
		}
		lineGap = int(size * 0.32)
		if lineGap < 4 {
			lineGap = 4
		}
		paraGap = lineGap + 12
		lines = layoutLines(faceBody, body, maxW)
		if blockHeight(faceBody, lines, lineGap, paraGap) <= bodyAvail {
			break
		}
	}
	if faceBody == nil {
		return fmt.Errorf("gagal siapkan font body")
	}
	defer closeFace(faceBody)

	bh := blockHeight(faceBody, lines, lineGap, paraGap)
	contentBottom := slideH - padBottom - footerReserve
	if leftover := contentBottom - bodyTop - bh; leftover > 80 {
		shift := leftover / 2
		switch th.layout {
		case layBand, layEditorial, layFrame, layCard:
			shift = leftover * 2 / 5
		case layCenter, layQuote:
			shift = leftover / 2
		}
		if shift > leftover-32 {
			shift = leftover - 32
		}
		if shift > 0 {
			bodyTop += shift
		}
	}

	lineH := faceBody.Metrics().Height.Ceil() + lineGap
	ascent := faceBody.Metrics().Ascent.Ceil()
	maxY := contentBottom
	y = bodyTop
	contentMid := (contentLeft + contentRight) / 2
	for _, ln := range lines {
		if ln.spacer {
			y += paraGap
			continue
		}
		if y+lineH > maxY {
			break
		}
		x := padX
		if centerBody {
			mw := font.MeasureString(faceBody, ln.text).Ceil()
			x = contentMid - mw/2
		}
		col := th.body
		if th.leadAccent && ln.lead {
			col = th.accent
		}
		drawString(img, faceBody, ln.text, x, y+ascent, col)
		y += lineH
	}

	// Quiet footer: thin rule + dots + 02/05 + geser
	fy := slideH - 70
	if th.layout == layCard || th.layout == layFrame {
		fy = slideH - padBottom + 20
		if fy > slideH-56 {
			fy = slideH - 56
		}
	}

	footerLeft := padX
	footerRight := contentRight

	drawHLine(img, footerLeft, footerRight, fy-22, th.footerLine)
	drawHLine(img, footerLeft, footerLeft+48, fy-22, th.accentDim)

	if slideTotal >= 2 {
		label := fmt.Sprintf("%02d  /  %02d", slideNum, slideTotal)
		ff := makeFace(fontBrand, 18)
		if ff != nil {
			drawString(img, ff, label, footerLeft, fy+ff.Metrics().Ascent.Ceil()-4, th.muted)
			closeFace(ff)
		}
		dotActive := slideNum - 1
		if dotActive < 0 {
			dotActive = 0
		}
		if dotActive >= slideTotal {
			dotActive = slideTotal - 1
		}
		drawProgressDots(img, slideTotal, dotActive, slideW/2, fy+5, 20, 4, 3, th.accent, th.footerLine)

		nudge := "geser >"
		if slideNum >= slideTotal {
			nudge = "selesai"
		}
		fn := makeFace(fontBrand, 16)
		if fn != nil {
			nw := font.MeasureString(fn, nudge).Ceil()
			drawString(img, fn, nudge, footerRight-nw, fy+fn.Metrics().Ascent.Ceil()-4, th.accentDim)
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
