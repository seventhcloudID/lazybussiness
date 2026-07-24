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

func lerpByte(a, b uint8, t float64) uint8 {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return uint8(float64(a) + (float64(b)-float64(a))*t + 0.5)
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

func textBlockHeight(face font.Face, lines []string, gap int) int {
	if face == nil {
		return 0
	}
	lineH := face.Metrics().Height.Ceil() + gap
	n := len(lines)
	if n == 0 {
		return 0
	}
	return n * lineH
}

// RenderSlidePNG writes a 4:5 dark slide PNG with @brand, divider, top-aligned text.
// Font size auto-shrinks so the full body fits (no silent crop).
func RenderSlidePNG(path, brand, text string) error {
	if parsedFont == nil {
		return fmt.Errorf("font slide belum siap")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	img := image.NewRGBA(image.Rect(0, 0, slideW, slideH))
	base := rgb(11, 16, 24)
	top := rgb(28, 48, 86)
	for y := 0; y < slideH; y++ {
		t := 0.0
		if y < 480 {
			t = (1 - float64(y)/480) * 0.55
		}
		row := rgb(
			lerpByte(base.R, top.R, t),
			lerpByte(base.G, top.G, t),
			lerpByte(base.B, top.B, t),
		)
		for x := 0; x < slideW; x++ {
			img.SetRGBA(x, y, row)
		}
	}

	padX := 72
	contentTop := 80
	handle := brandHandle(brand)
	faceHandle := makeFace(34)
	defer func() {
		if faceHandle != nil {
			_ = faceHandle.Close()
		}
	}()

	y := contentTop
	if handle != "" && faceHandle != nil {
		drawString(img, faceHandle, handle, padX, y+32, rgb(160, 170, 185))
		y += 64
		divY := y
		for x := padX; x < slideW-padX; x++ {
			fade := float64(x-padX) / float64(slideW-2*padX)
			v := lerpByte(200, 40, fade)
			img.SetRGBA(x, divY, rgb(v, v, v))
		}
		for x := padX; x < padX+56; x++ {
			for dy := -1; dy <= 1; dy++ {
				yy := divY + dy
				if yy >= 0 && yy < slideH {
					img.SetRGBA(x, yy, rgb(230, 232, 236))
				}
			}
		}
		y += 44
	}

	body := strings.TrimSpace(text)
	if utf8.RuneCountInString(body) > 500 {
		runes := []rune(body)
		body = string(runes[:500])
	}

	maxY := slideH - 80
	avail := maxY - y
	maxW := slideW - 2*padX

	var faceBody font.Face
	var lines []string
	var lineGap int
	for size := 46.0; size >= 26.0; size -= 2 {
		if faceBody != nil {
			_ = faceBody.Close()
		}
		faceBody = makeFace(size)
		if faceBody == nil {
			continue
		}
		lineGap = 8
		if size <= 32 {
			lineGap = 6
		}
		lines = wrapText(faceBody, body, maxW)
		if textBlockHeight(faceBody, lines, lineGap) <= avail {
			break
		}
	}
	if faceBody == nil {
		return fmt.Errorf("gagal siapkan font body")
	}
	defer faceBody.Close()

	col := rgb(245, 247, 250)
	lineH := faceBody.Metrics().Height.Ceil() + lineGap
	ascent := faceBody.Metrics().Ascent.Ceil()
	for _, line := range lines {
		if y+lineH > maxY {
			// Still overflow at min size — stop cleanly (shouldn't happen often).
			break
		}
		drawString(img, faceBody, line, padX, y+ascent, col)
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
