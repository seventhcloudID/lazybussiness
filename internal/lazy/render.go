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

// RenderSlidePNG writes a 4:5 dark slide PNG with @brand, divider, top-aligned text.
// Uses only opaque pixels — semi-transparent draw.Over on image.RGBA breaks colors (pink/green).
func RenderSlidePNG(path, brand, text string) error {
	if faceHandle == nil || faceBody == nil {
		return fmt.Errorf("font slide belum siap")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	img := image.NewRGBA(image.Rect(0, 0, slideW, slideH))
	base := rgb(11, 16, 24)
	top := rgb(28, 48, 86) // soft blue wash, opaque blend
	for y := 0; y < slideH; y++ {
		t := 0.0
		if y < 480 {
			t = 1 - float64(y)/480
			t *= 0.55
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
	y := 80

	handle := brandHandle(brand)
	if handle != "" {
		drawString(img, faceHandle, handle, padX, y+36, rgb(160, 170, 185))
		y += 70
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
		y += 48
	}

	body := strings.TrimSpace(text)
	if utf8.RuneCountInString(body) > 500 {
		runes := []rune(body)
		body = string(runes[:500])
	}
	lines := wrapText(faceBody, body, slideW-2*padX)
	lineH := faceBody.Metrics().Height.Ceil() + 12
	maxY := slideH - 100
	col := rgb(245, 247, 250)
	for _, line := range lines {
		if y+lineH > maxY {
			break
		}
		drawString(img, faceBody, line, padX, y+faceBody.Metrics().Ascent.Ceil(), col)
		y += lineH
	}

	// Flatten: ensure every pixel is opaque before encode (IG-safe).
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+3] = 255
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	return enc.Encode(f, img)
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
