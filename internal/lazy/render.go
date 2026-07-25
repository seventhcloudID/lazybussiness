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
	fontBody  *opentype.Font // isi slide — seragam
	fontBrand *opentype.Font // @handle + footer
)

func init() {
	// Body pakai SemiBold — Regular terlalu tipis di background gelap (susah dibaca di HP).
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

func brandHandle(brand string) string {
	b := strings.TrimSpace(brand)
	b = strings.TrimLeft(b, "@")
	if b == "" {
		return ""
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
		// DPI 96 + tanpa hinting = lebih mulus di ukuran besar (slide IG)
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

// RenderSlidePNG — tipografi seragam (Inter), semua paragraf sama ukuran/warna.
// slideNum 1-based; slideTotal 0 = sembunyikan indikator.
func RenderSlidePNG(path, brand, text string, slideNum, slideTotal int) error {
	if fontBody == nil {
		return fmt.Errorf("font slide belum siap")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	img := image.NewRGBA(image.Rect(0, 0, slideW, slideH))

	top := rgb(18, 22, 30)
	bot := rgb(28, 34, 46)
	fillGradient(img, top, bot)
	softBlob(img, 980, 180, 420, rgb(55, 72, 88), 0.22)
	softBlob(img, 120, 1180, 480, rgb(42, 52, 68), 0.28)
	softBlob(img, 900, 1100, 280, rgb(232, 164, 90), 0.06)

	accent := rgb(232, 164, 90)
	accentDim := rgb(180, 128, 72)
	fillRect(img, 0, 0, 10, slideH, accent)

	padX := 88
	padTop := 88
	padBottom := 110
	maxW := slideW - padX - 88

	y := padTop
	handle := brandHandle(brand)
	if handle != "" {
		fh := makeFace(fontBrand, 24)
		if fh != nil {
			drawString(img, fh, handle, padX, y+fh.Metrics().Ascent.Ceil(), rgb(168, 176, 188))
			hw := font.MeasureString(fh, handle).Ceil()
			y += fh.Metrics().Height.Ceil() + 16
			closeFace(fh)
			barW := hw
			if barW < 56 {
				barW = 56
			}
			if barW > 220 {
				barW = 220
			}
			for dy := 0; dy < 3; dy++ {
				drawHLine(img, padX, padX+barW, y+dy, accent)
			}
			y += 44
		}
	} else {
		y += 12
	}

	bodyTop := y
	body := normalizeBody(text)
	if utf8.RuneCountInString(body) > 500 {
		runes := []rune(body)
		body = string(runes[:500])
	}

	footerReserve := 72
	bodyAvail := slideH - padBottom - bodyTop - footerReserve

	var faceBody font.Face
	var lines []drawLine
	var lineGap, paraGap int

	// Satu face + satu warna untuk SEMUA paragraf (tidak ada hook vs body).
	// Size di DPI 96: ~36pt ≈ nyaman dibaca di feed HP.
	for size := 36.0; size >= 26.0; size -= 1 {
		if faceBody != nil {
			closeFace(faceBody)
		}
		faceBody = makeFace(fontBody, size)
		if faceBody == nil {
			continue
		}
		lineGap = int(size*0.55) + 4
		// Hampir sama dengan jarak antar baris — biar tidak terlihat seperti judul vs body
		paraGap = lineGap + 8
		lines = layoutLines(faceBody, body, maxW)
		if blockHeight(faceBody, lines, lineGap, paraGap) <= bodyAvail {
			break
		}
	}
	if faceBody == nil {
		return fmt.Errorf("gagal siapkan font body")
	}
	defer closeFace(faceBody)

	colBody := rgb(245, 246, 248) // kontras tinggi, semua paragraf sama
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
		drawString(img, faceBody, ln.text, padX, y+ascent, colBody)
		y += lineH
	}

	fy := slideH - 78
	drawHLine(img, padX, slideW-padX, fy-28, rgb(48, 56, 70))
	drawHLine(img, padX, padX+72, fy-28, accentDim)

	if slideTotal >= 2 {
		label := fmt.Sprintf("%02d  /  %02d", slideNum, slideTotal)
		ff := makeFace(fontBrand, 20)
		if ff != nil {
			drawString(img, ff, label, padX, fy+ff.Metrics().Ascent.Ceil()-4, rgb(140, 148, 160))
			closeFace(ff)
		}
		dotActive := slideNum - 1
		if dotActive < 0 {
			dotActive = 0
		}
		if dotActive >= slideTotal {
			dotActive = slideTotal - 1
		}
		drawProgressDots(img, slideTotal, dotActive, slideW/2, fy+6, 22, 5, 3, accent, rgb(70, 78, 92))

		nudge := "geser >"
		if slideNum >= slideTotal {
			nudge = "selesai"
		}
		fn := makeFace(fontBrand, 18)
		if fn != nil {
			nw := font.MeasureString(fn, nudge).Ceil()
			drawString(img, fn, nudge, slideW-padX-nw, fy+fn.Metrics().Ascent.Ceil()-4, accent)
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
