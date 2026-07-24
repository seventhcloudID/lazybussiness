package lazy

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
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

var (
	faceHandle font.Face
	faceBody   font.Face
)

func init() {
	ft, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return
	}
	faceHandle, _ = opentype.NewFace(ft, &opentype.FaceOptions{
		Size: 36, DPI: 72, Hinting: font.HintingFull,
	})
	faceBody, _ = opentype.NewFace(ft, &opentype.FaceOptions{
		Size: 48, DPI: 72, Hinting: font.HintingFull,
	})
}

func brandHandle(brand string) string {
	b := strings.TrimSpace(brand)
	b = strings.TrimLeft(b, "@")
	if b == "" {
		return ""
	}
	return "@" + b
}

// RenderSlidePNG writes a 4:5 dark slide PNG with @brand, divider, top-aligned text.
func RenderSlidePNG(path, brand, text string) error {
	if faceHandle == nil || faceBody == nil {
		return fmt.Errorf("font slide belum siap")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	img := image.NewRGBA(image.Rect(0, 0, slideW, slideH))
	// dark gradient-ish: fill + top/bottom washes
	bg := color.RGBA{R: 11, G: 16, B: 24, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	for y := 0; y < 420; y++ {
		alpha := uint8(40 * (420 - y) / 420)
		c := color.RGBA{R: 37, G: 99, B: 235, A: alpha}
		draw.Draw(img, image.Rect(0, y, slideW, y+1), &image.Uniform{C: c}, image.Point{}, draw.Over)
	}

	padX := 72
	y := 80

	handle := brandHandle(brand)
	if handle != "" {
		col := color.RGBA{R: 248, G: 250, B: 252, A: 160}
		drawString(img, faceHandle, handle, padX, y+36, col)
		y += 70
		// divider
		divY := y
		for x := padX; x < slideW-padX; x++ {
			t := float64(x-padX) / float64(slideW-2*padX)
			a := uint8(140 * (1 - t*0.85))
			img.Set(x, divY, color.RGBA{255, 255, 255, a})
		}
		for x := padX; x < padX+56; x++ {
			for dy := -1; dy <= 1; dy++ {
				img.Set(x, divY+dy, color.RGBA{248, 250, 252, 220})
			}
		}
		y += 48
	}

	body := strings.TrimSpace(text)
	if utf8.RuneCountInString(body) > 500 {
		runes := []rune(body)
		body = string(runes[:500])
	}
	lines := wrapText(faceBody, body, slideW-2*padX)
	lineH := faceBody.Metrics().Height.Ceil() + 10
	maxY := slideH - 100
	col := color.RGBA{R: 248, G: 250, B: 252, A: 235}
	for _, line := range lines {
		if y+lineH > maxY {
			break
		}
		drawString(img, faceBody, line, padX, y+faceBody.Metrics().Ascent.Ceil(), col)
		y += lineH
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func drawString(img *image.RGBA, face font.Face, s string, x, y int, col color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

func wrapText(face font.Face, text string, maxWidth int) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimRight(para, " ")
		if para == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := words[0]
		for _, w := range words[1:] {
			trial := cur + " " + w
			if font.MeasureString(face, trial).Ceil() <= maxWidth {
				cur = trial
			} else {
				lines = append(lines, cur)
				cur = w
			}
		}
		lines = append(lines, cur)
	}
	return lines
}
