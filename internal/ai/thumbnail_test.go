package ai

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestThumbnailPromptRejectsDullColorGrade(t *testing.T) {
	prompt := strings.ToLower(BuildThumbnailPromptAspect("cara menata pembukuan", "4:5"))
	for _, required := range []string{"white balance netral", "warna natural hidup", "jangan sepia", "underexposed", "desaturated"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt missing color guard %q", required)
		}
	}
	if strings.Contains(prompt, "cinematic") {
		t.Fatal("default prompt must not invite dark cinematic grading")
	}
}

func TestPortraitNormalizationPreservesBottomArtwork(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 150)) // provider-style 2:3
	for y := 0; y < 150; y++ {
		c := color.RGBA{B: 220, A: 255}
		if y >= 130 {
			c = color.RGBA{R: 240, A: 255} // title/CTA near bottom edge
		}
		for x := 0; x < 100; x++ {
			src.SetRGBA(x, y, c)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}

	out, w, h, err := normalizeThumbnailCanvas(encoded.Bytes(), "4:5", true)
	if err != nil {
		t.Fatal(err)
	}
	if w != igOutW || h != igOutH {
		t.Fatalf("size=%dx%d", w, h)
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	r, _, b, _ := img.At(w/2, h-10).RGBA()
	if r <= b {
		t.Fatal("bottom artwork was cropped instead of preserved")
	}
}

func TestRenderWhiteTextPanelHasSquareCornersAndNoShadow(t *testing.T) {
	const w, h = 400, 500
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	drawColor := color.RGBA{R: 210, G: 40, B: 30, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.SetRGBA(x, y, drawColor)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}

	out, err := renderWhiteTextPanel(encoded.Bytes(), "Ke Jakarta cuma sehari? Pilih sedikit tempat yang benar-benar penting.", "bimosept", "Slide Deh →")
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	px := w * 55 / 1000
	py := h * 565 / 1000
	pr := w - px
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if got := color.RGBAModel.Convert(img.At(px, py)).(color.RGBA); got != white {
		t.Fatalf("top-left corner is not square white: %+v", got)
	}
	if got := color.RGBAModel.Convert(img.At(pr-1, py)).(color.RGBA); got != white {
		t.Fatalf("top-right corner is not square white: %+v", got)
	}
	if got := color.RGBAModel.Convert(img.At(px-1, py)).(color.RGBA); got != drawColor {
		t.Fatalf("pixel outside panel changed (shadow/border leaked): %+v", got)
	}
}

func TestCodedCoverTemplatesUseDifferentGeometry(t *testing.T) {
	const w, h = 400, 500
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	red := color.RGBA{R: 210, G: 40, B: 30, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.SetRGBA(x, y, red)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}
	edgeRaw, err := renderCodedCover(encoded.Bytes(), "Judul", "", "", "edge-clean")
	if err != nil {
		t.Fatal(err)
	}
	insetRaw, err := renderCodedCover(encoded.Bytes(), "Judul", "", "", "inset-editorial")
	if err != nil {
		t.Fatal(err)
	}
	edge, _, _ := image.Decode(bytes.NewReader(edgeRaw))
	inset, _, _ := image.Decode(bytes.NewReader(insetRaw))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if got := color.RGBAModel.Convert(edge.At(0, 320)).(color.RGBA); got != white {
		t.Fatalf("edge template must reach left edge: %+v", got)
	}
	if got := color.RGBAModel.Convert(inset.At(0, 320)).(color.RGBA); got != red {
		t.Fatalf("inset template must preserve photo gutter: %+v", got)
	}
}

func TestSplitCoverEmphasisUsesClosingStatement(t *testing.T) {
	words := splitCoverEmphasis("Ke Jakarta cuma sehari? Pilih sedikit tempat yang benar-benar penting.")
	if len(words) != 10 {
		t.Fatalf("unexpected words: %d", len(words))
	}
	for i, word := range words {
		wantBold := i >= 4
		if word.Bold != wantBold {
			t.Fatalf("word %q bold=%v, want %v", word.Text, word.Bold, wantBold)
		}
	}
}

func TestSplitCoverEmphasisHighlightsLastPhraseWithoutPunctuation(t *testing.T) {
	words := splitCoverEmphasis("Ubah satu konten menjadi banyak materi promosi")
	if len(words) != 7 {
		t.Fatalf("unexpected words: %d", len(words))
	}
	if words[3].Bold || !words[4].Bold || !words[6].Bold {
		t.Fatalf("unexpected emphasis split: %+v", words)
	}
}

func TestCompactCoverHeadlineRemovesCounterAndParagraph(t *testing.T) {
	got := compactCoverHeadline(`1/9 Serial 1 hari 1 tips cari cuan dari internet. Kalau belum punya portofolio, jangan menawarkan diri sebagai AI specialist.`)
	want := "Serial 1 hari 1 tips cari cuan dari internet. Kalau belum punya portofolio, jangan menawarkan diri sebagai AI specialist."
	if got != want {
		t.Fatalf("headline=%q, want %q", got, want)
	}
}

func TestThumbnailDeviceGeometryGuardIsAlwaysAppendedOnce(t *testing.T) {
	prompt := appendThumbnailDeviceGeometryGuard("foto editorial")
	if !strings.Contains(prompt, "screen panel behind a phone") || !strings.Contains(prompt, "physically enclosed inside the device bezel") {
		t.Fatalf("device geometry guard missing: %s", prompt)
	}
	again := appendThumbnailDeviceGeometryGuard(prompt)
	if strings.Count(again, "DEVICE GEOMETRY — MANDATORY") != 1 {
		t.Fatalf("device guard duplicated: %s", again)
	}
}

func TestBuildCoverBackgroundPromptForbidsOnImageText(t *testing.T) {
	prompt := BuildCoverBackgroundPrompt("Hook QRIS kecil", "pasangan baru nikah cek rekening")
	for _, want := range []string{"FOTO LATAR", "tanpa tulisan", "PHOTO BACKGROUND ONLY", "yellow keywords"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing %q in prompt: %s", want, prompt)
		}
	}
}

func TestBimoseptCoverDoesNotInjectLegacySeriesHook(t *testing.T) {
	got := coverHeadlineForHandle("Jangan tawarkan WEBSITE dulu. Satu pertanyaan awal menentukan apakah pemilik toko mau lanjut.", "@bimosept")
	want := "Jangan tawarkan WEBSITE dulu. Satu pertanyaan awal menentukan apakah pemilik toko mau lanjut."
	if got != want {
		t.Fatalf("headline=%q, want %q", got, want)
	}
}

func TestExplicitSeriesHookIsPreserved(t *testing.T) {
	got := coverHeadlineForHandle("Serial 1 hari 1 tips cari cuan dari internet. WEBSITE bukan tawaran pertama yang membuat pemilik toko tertarik.", "bimosept")
	want := "Serial 1 hari 1 tips cari cuan dari internet. WEBSITE bukan tawaran pertama yang membuat pemilik toko tertarik."
	if got != want {
		t.Fatalf("headline=%q, want %q", got, want)
	}
}

func TestBimoseptCoverDoesNotTruncateTopicHook(t *testing.T) {
	const topic = "Katalog PDF supplier sering bikin reseller kerja dua kali. Ada jasa kecil dari sheet produk siap pakai yang jarang ditawarkan."
	got := coverHeadlineForHandle(topic, "bimosept")
	want := topic
	if got != want {
		t.Fatalf("headline dipotong: %q, want %q", got, want)
	}
	if strings.Contains(got, "…") {
		t.Fatalf("headline tidak boleh diberi ellipsis: %q", got)
	}
}

func TestBimoseptSeriesIsBoldAccentAndSeparated(t *testing.T) {
	words := splitCoverEmphasis("1 hari 1 tips cuan dari internet. Tawaran pertama menentukan respons pemilik toko.")
	if len(words) < 8 {
		t.Fatalf("unexpected words: %+v", words)
	}
	for i := 0; i <= 6; i++ {
		if !words[i].Bold || !words[i].Accent {
			t.Fatalf("series word must be bold accent: %+v", words[i])
		}
	}
	if !words[6].BreakAfter || words[7].Accent {
		t.Fatalf("series must end with a forced line break: %+v", words)
	}
}

func TestCoverSeriesHookOnlyAppliesToBimosept(t *testing.T) {
	const title = "Satu pertanyaan awal menentukan apakah pemilik toko mau lanjut."
	if got := coverHeadlineForHandle(title, "akunlain"); got != title {
		t.Fatalf("headline akun lain berubah: %q", got)
	}
}

func TestCoverHeadlineFromPackagePrefersCoverSlide(t *testing.T) {
	pkg := GenEditorialPackage{
		Copy:  GenCopy{Hook: "1/9 Ini isi utas pertama yang sangat panjang dan bukan judul cover."},
		Story: GenStory{Slides: []GenStorySlide{{Index: 1, Role: "cover", Headline: "Cari Cuan dari Internet Tanpa Portofolio?"}}},
	}
	if got := coverHeadlineFromPackage(pkg); got != "Cari cuan dari internet tanpa portofolio?" {
		t.Fatalf("headline=%q", got)
	}
}

func TestCoverEmphasisUsesWeightAtSameEditorialScale(t *testing.T) {
	if coverEmphasisScale != 1 {
		t.Fatalf("emphasis scale=%v, want same-size editorial weight", coverEmphasisScale)
	}
	if coverTextColor == (color.RGBA{}) {
		t.Fatal("cover text color is empty")
	}
}

func TestBimoseptSeriesUsesLargerDisplayScale(t *testing.T) {
	if coverSeriesScale < 1.15 {
		t.Fatalf("series scale=%v, want a clearly larger series label", coverSeriesScale)
	}
}

func TestCoverEmphasisPreservesEditorialCasing(t *testing.T) {
	if got := coverDisplayWord(coverTextWord{Text: "bisa jadi jasa", Bold: true}); got != "bisa jadi jasa" {
		t.Fatalf("display emphasis=%q", got)
	}
}

func TestCompactCoverHeadlineNormalizesTitleCase(t *testing.T) {
	got := compactCoverHeadline("Pilih AI Dari Pekerjaannya")
	if got != "Pilih AI sesuai jenis pekerjaan" {
		t.Fatalf("headline=%q", got)
	}
}

func TestShortCoverEmphasisUsesPhraseNotSingleWord(t *testing.T) {
	words := splitCoverEmphasis("Pilih AI sesuai jenis pekerjaan")
	if len(words) != 5 || words[2].Bold || !words[3].Bold || !words[4].Bold {
		t.Fatalf("unexpected emphasis: %+v", words)
	}
}

func TestCoverHeadlineCriticRejectsGenericLabel(t *testing.T) {
	issues := coverHeadlineTextIssues("Pilih AI sesuai jenis pekerjaan")
	if len(issues) == 0 {
		t.Fatal("generic cover label should trigger repair")
	}
}

func TestCoverHeadlineCriticAcceptsSpecificCuriosityGap(t *testing.T) {
	issues := coverHeadlineTextIssues("ChatGPT menjawab cepat di awal. Pengguna masih menemukan kelemahannya saat dokumen panjang masuk bertubi-tubi.")
	if len(issues) != 0 {
		t.Fatalf("strong cover headline rejected: %v", issues)
	}
}

func TestCoverHeadlineCriticRejectsLeakedAnswer(t *testing.T) {
	issues := coverHeadlineTextIssues("Chat calon pembeli mati karena hitungannya terlalu lama")
	if len(issues) < 2 {
		t.Fatalf("answer-leaking awkward hook should trigger repair: %v", issues)
	}
}

func TestCoverHeadlineCriticRejectsAwkwardWhatsAppShopPhrase(t *testing.T) {
	issues := coverHeadlineTextIssues("Jangan tawarkan WEBSITE dulu ke toko WhatsApp. Ada langkah awal yang jauh lebih menentukan keputusan pemiliknya.")
	if len(issues) == 0 {
		t.Fatal("awkward WhatsApp shop phrase should trigger repair")
	}
}
