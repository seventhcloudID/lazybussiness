package buffer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MirrorPublicURLs unduh gambar ke mediaDir lalu kembalikan URL publik (PUBLIC_BASE_URL).
// Buffer butuh URL yang stabil; CDN IG sering kedaluwarsa.
func MirrorPublicURLs(mediaDir, publicBase, subdir string, sourceURLs []string) ([]string, error) {
	publicBase = strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if publicBase == "" {
		return nil, fmt.Errorf("PUBLIC_BASE_URL kosong")
	}
	urls := cleanURLs(sourceURLs, 10)
	if len(urls) == 0 {
		return nil, fmt.Errorf("tidak ada URL gambar")
	}
	dir := filepath.Join(mediaDir, filepath.Clean(subdir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	hc := &http.Client{Timeout: 60 * time.Second}
	out := make([]string, 0, len(urls))
	for i, src := range urls {
		ext, body, err := fetchImage(hc, src)
		if err != nil {
			return nil, fmt.Errorf("gambar %d: %w", i+1, err)
		}
		name := fmt.Sprintf("slide-%02d%s", i+1, ext)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return nil, err
		}
		rel := strings.ReplaceAll(filepath.ToSlash(filepath.Join(subdir, name)), "\\", "/")
		out = append(out, publicBase+"/media/lazy/"+rel)
	}
	return out, nil
}

func fetchImage(hc *http.Client, src string) (ext string, body []byte, err error) {
	req, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "lazybussiness-buffer/1.0")
	res, err := hc.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return "", nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 25<<20))
	if err != nil {
		return "", nil, err
	}
	if len(raw) < 32 {
		return "", nil, fmt.Errorf("file terlalu kecil")
	}
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	switch {
	case strings.Contains(ct, "png") || looksPNG(raw):
		ext = ".png"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	default:
		ext = ".jpg"
	}
	return ext, raw, nil
}

func looksPNG(b []byte) bool {
	return len(b) >= 8 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4e && b[3] == 0x47
}
