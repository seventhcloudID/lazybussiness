package lazy

import (
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

var parsedFont *opentype.Font

func init() {
	ft, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return
	}
	parsedFont = ft
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

func makeFace(size float64) font.Face {
	if parsedFont == nil {
		return nil
	}
	f, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size: size, DPI: 72, Hinting: font.HintingFull,
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

// RenderSlidePNG — @brand + garis fixed di atas; body di bawahnya (tidak ikut naik/turun).
func RenderSlidePNG(path, brand, text string) error {
	if parsedFont == nil {
		return fmt.Errorf("font slide belum siap")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	img := image.NewRGBA(image.Rect(0, 0, slideW, slideH))
	bg := rgb(14, 18, 26)
	for y := 0; y < slideH; y++ {
		for x := 0; x < slideW; x++ {
			img.SetRGBA(x, y, bg)
		}
	}

	padX := 100
	padTop := 88
	padBottom := 110
	maxW := slideW - 2*padX

	// Header fixed di atas
	y := padTop
	handle := brandHandle(brand)
	if handle != "" {
		fh := makeFace(26)
		if fh != nil {
			drawString(img, fh, handle, padX, y+fh.Metrics().Ascent.Ceil(), rgb(150, 158, 170))
			y += fh.Metrics().Height.Ceil() + 22
			closeFace(fh)
		}
		divY := y
		for x := padX; x < slideW-padX; x++ {
			img.SetRGBA(x, divY, rgb(58, 66, 80))
		}
		y += 36
	}

	bodyTop := y
	body := normalizeBody(text)
	if utf8.RuneCountInString(body) > 500 {
		runes := []rune(body)
		body = string(runes[:500])
	}

	bodyAvail := slideH - padBottom - bodyTop
	var faceBody font.Face
	var lines []drawLine
	var lineGap, paraGap int
	for size := 46.0; size >= 32.0; size -= 1 {
		if faceBody != nil {
			closeFace(faceBody)
		}
		faceBody = makeFace(size)
		if faceBody == nil {
			continue
		}
		lineGap = 14
		paraGap = int(size*0.9) + 12
		if size <= 36 {
			lineGap = 11
			paraGap = int(size*0.75) + 10
		}
		lines = layoutLines(faceBody, body, maxW)
		if blockHeight(faceBody, lines, lineGap, paraGap) <= bodyAvail {
			break
		}
	}
	if faceBody == nil {
		return fmt.Errorf("gagal siapkan font body")
	}
	defer closeFace(faceBody)

	col := rgb(242, 244, 248)
	lineH := faceBody.Metrics().Height.Ceil() + lineGap
	ascent := faceBody.Metrics().Ascent.Ceil()
	maxY := slideH - padBottom
	y = bodyTop
	for _, ln := range lines {
		if ln.spacer {
			y += paraGap
			continue
		}
		if y+lineH > maxY {
			break
		}
		drawString(img, faceBody, ln.text, padX, y+ascent, col)
		y += lineH
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

func drawString(img *image.RGBA, face font.Face, s string, x, y int, col color.RGBA) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}
