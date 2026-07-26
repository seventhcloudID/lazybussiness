package lazy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderAllTemplates(t *testing.T) {
	dir := t.TempDir()
	for _, tpl := range ListCarouselTemplates() {
		path := filepath.Join(dir, tpl.ID+".png")
		err := RenderSlidePNG(path, "bimosept", "Ini contoh isi slide.\n\nParagraf kedua.", 2, 5, tpl.ID)
		if err != nil {
			t.Fatalf("%s: %v", tpl.ID, err)
		}
		st, err := os.Stat(path)
		if err != nil || st.Size() < 1000 {
			t.Fatalf("%s: bad output file", tpl.ID)
		}
	}
}
