package lazy

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareTikTokCarouselURLs(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreAt("", "", filepath.Join(dir, "media"))
	pngPath := filepath.Join(dir, "media", "cover.png")
	if err := os.MkdirAll(filepath.Dir(pngPath), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1080, 1350))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pngPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	d := &Deps{
		Store:  store,
		Public: "https://flowa.example.com",
	}
	cover := "https://flowa.example.com/media/lazy/cover.png"
	slide := cover // duplicate OK for test count

	urls, err := d.prepareTikTokCarouselURLs([]string{cover, slide})
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 2 {
		t.Fatalf("want 2 urls got %d", len(urls))
	}
	for i, u := range urls {
		if !stringsHasSuffix(u, ".jpg") {
			t.Fatalf("url %d not jpg: %s", i, u)
		}
		rel := stringsAfter(u, "/media/lazy/")
		abs := filepath.Join(dir, "media", filepath.FromSlash(rel))
		if !looksJPEGFile(abs) {
			t.Fatalf("file %s not valid jpeg", abs)
		}
	}
}

func stringsHasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

func stringsAfter(s, sep string) string {
	if i := indexOf(s, sep); i >= 0 {
		return s[i+len(sep):]
	}
	return ""
}

func indexOf(s, sub string) int {
	return bytes.Index([]byte(s), []byte(sub))
}
