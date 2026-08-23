package lazy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"threads-dashboard/internal/ai"
	"golang.org/x/image/webp"
)

const tiktokJPEGQuality = 90

func decodeImage(data []byte) (image.Image, error) {
	if img, err := jpeg.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if img, err := png.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	return webp.Decode(bytes.NewReader(data))
}

func writeJPEGFile(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: tiktokJPEGQuality})
}

func convertPNGFileToJPEG(pngPath string) (string, error) {
	jpgPath := strings.TrimSuffix(pngPath, filepath.Ext(pngPath)) + ".jpg"
	if st, err := os.Stat(jpgPath); err == nil && st.Size() > 0 {
		return jpgPath, nil
	}
	raw, err := os.ReadFile(pngPath)
	if err != nil {
		return "", err
	}
	img, err := decodeImage(raw)
	if err != nil {
		return "", err
	}
	if err := writeJPEGFile(jpgPath, img); err != nil {
		return "", err
	}
	return jpgPath, nil
}

func (d *Deps) resolveMediaFile(publicURL string) (string, error) {
	u := strings.TrimSpace(publicURL)
	if idx := strings.Index(u, "/media/lazy/"); idx >= 0 {
		rel := strings.ReplaceAll(u[idx+len("/media/lazy/"):], "..", "")
		p := filepath.Join(d.Store.MediaDir(), filepath.FromSlash(rel))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if idx := strings.Index(u, "/media/thumbs/"); idx >= 0 {
		rel := strings.ReplaceAll(u[idx+len("/media/thumbs/"):], "..", "")
		base := strings.TrimSpace(d.ThumbDir)
		if base == "" {
			base = ai.DefaultThumbMediaDir()
		}
		for _, root := range []string{base, ai.DefaultThumbMediaDir()} {
			p := filepath.Join(root, filepath.FromSlash(rel))
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("file media tidak ditemukan: %s", publicURL)
}

func (d *Deps) loadImageBytes(publicURL string) ([]byte, error) {
	if abs, err := d.resolveMediaFile(publicURL); err == nil {
		return os.ReadFile(abs)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, strings.TrimSpace(publicURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lazybussiness-tiktok/1.0")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d mengambil %s", res.StatusCode, publicURL)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 25<<20))
	if err != nil {
		return nil, err
	}
	if len(raw) < 32 {
		return nil, fmt.Errorf("file terlalu kecil")
	}
	return raw, nil
}

func tiktokCacheKey(urls []string) string {
	h := sha256.New()
	for _, u := range urls {
		_, _ = io.WriteString(h, strings.TrimSpace(u))
		_, _ = io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// prepareTikTokCarouselURLs re-encode semua slide ke JPEG baru di /media/lazy/_tiktok/.
// TikTok menolak PNG walau ekstensi URL .jpg — selalu tulis ulang binary JPEG.
func (d *Deps) prepareTikTokCarouselURLs(urls []string) ([]string, error) {
	if !d.publicOK() {
		return nil, fmt.Errorf("PUBLIC_BASE_URL belum valid")
	}
	cleaned := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u != "" {
			cleaned = append(cleaned, u)
		}
	}
	if len(cleaned) < 2 {
		return nil, fmt.Errorf("carousel butuh minimal 2 gambar")
	}

	subdir := filepath.Join("_tiktok", tiktokCacheKey(cleaned))
	dir := filepath.Join(d.Store.MediaDir(), filepath.FromSlash(subdir))
	base := strings.TrimRight(d.Public, "/")
	out := make([]string, 0, len(cleaned))

	for i, src := range cleaned {
		name := fmt.Sprintf("%02d.jpg", i+1)
		path := filepath.Join(dir, name)
		public := base + "/media/lazy/" + filepath.ToSlash(filepath.Join(subdir, name))

		if st, err := os.Stat(path); err == nil && st.Size() > 1024 && looksJPEGFile(path) {
			out = append(out, public)
			continue
		}

		raw, err := d.loadImageBytes(src)
		if err != nil {
			return nil, fmt.Errorf("slide %d: %w", i+1, err)
		}
		img, err := decodeImage(raw)
		if err != nil {
			return nil, fmt.Errorf("slide %d decode: %w", i+1, err)
		}
		if err := writeJPEGFile(path, img); err != nil {
			return nil, fmt.Errorf("slide %d encode: %w", i+1, err)
		}
		log.Printf("lazy tiktok export slide %d → %s", i+1, public)
		out = append(out, public)
	}
	return out, nil
}

func looksJPEGFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 3)
	n, _ := io.ReadFull(f, head)
	return n == 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF
}

func (d *Deps) ensureJPEGMediaURL(publicURL string) (string, error) {
	publicURL = strings.TrimSpace(publicURL)
	if publicURL == "" {
		return "", fmt.Errorf("url gambar kosong")
	}
	lower := strings.ToLower(publicURL)
	if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".webp") {
		return publicURL, nil
	}
	if !strings.HasSuffix(lower, ".png") {
		return publicURL, nil
	}
	abs, err := d.resolveMediaFile(publicURL)
	if err != nil {
		return "", err
	}
	jpgAbs, err := convertPNGFileToJPEG(abs)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(d.Public, "/")
	if strings.Contains(publicURL, "/media/lazy/") {
		rel, err := filepath.Rel(d.Store.MediaDir(), jpgAbs)
		if err != nil {
			return "", err
		}
		return base + "/media/lazy/" + filepath.ToSlash(rel), nil
	}
	if strings.Contains(publicURL, "/media/thumbs/") {
		thumbBase := strings.TrimSpace(d.ThumbDir)
		if thumbBase == "" {
			thumbBase = ai.DefaultThumbMediaDir()
		}
		rel, err := filepath.Rel(thumbBase, jpgAbs)
		if err != nil {
			rel, err = filepath.Rel(ai.DefaultThumbMediaDir(), jpgAbs)
			if err != nil {
				return "", err
			}
		}
		return base + "/media/thumbs/" + filepath.ToSlash(rel), nil
	}
	return publicURL, nil
}

func (d *Deps) ensureJPEGCarouselURLs(urls []string) ([]string, error) {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		ju, err := d.ensureJPEGMediaURL(u)
		if err != nil {
			return nil, err
		}
		out = append(out, ju)
	}
	return out, nil
}
