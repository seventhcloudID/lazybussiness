package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEditorialContextExcludesSavedBrand(t *testing.T) {
	ctx := buildEditorialContext(Memory{
		Brand:        "RAHASIA_BRAND",
		Niches:       []string{"edukasi AI"},
		Instructions: "Nada natural",
	}, GenerateRequest{Topic: "otomasi"}, nil)
	raw, err := json.Marshal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "RAHASIA_BRAND") {
		t.Fatalf("saved brand leaked into editorial pipeline: %s", raw)
	}
	if strings.Contains(string(raw), "edukasi AI") {
		t.Fatalf("filled brief must not be blended with account niche: %s", raw)
	}
	if ctx.Brief != "otomasi" {
		t.Fatalf("brief missing from editorial context: %+v", ctx)
	}
}

func TestEditorialContextUsesNicheOnlyWhenBriefEmpty(t *testing.T) {
	ctx := buildEditorialContext(Memory{Niches: []string{"keuangan UMKM"}}, GenerateRequest{}, nil)
	if len(ctx.Niches) != 1 || ctx.Niches[0] != "keuangan UMKM" {
		t.Fatalf("niche fallback missing: %+v", ctx.Niches)
	}
}

func TestEditorialContextUsesEditableBackendPromptWithoutLegacyTruncation(t *testing.T) {
	prompt := strings.Repeat("aturan kreatif panjang ", 100)
	ctx := buildEditorialContext(Memory{EditorialPrompt: prompt}, GenerateRequest{}, nil)
	if !strings.HasPrefix(ctx.Tone.Instructions, strings.TrimSpace(prompt)) {
		t.Fatalf("editable prompt changed or truncated: got=%d want-prefix=%d", len(ctx.Tone.Instructions), len(strings.TrimSpace(prompt)))
	}
	if !strings.Contains(ctx.Tone.Instructions, "BAHASA SANTAI") {
		t.Fatal("legacy custom prompt must receive conversational style guard")
	}
	if !strings.Contains(ctx.Tone.Instructions, "VISUAL HOOK AKTIF") {
		t.Fatal("legacy custom prompt must receive active visual hook guard")
	}
	if !strings.Contains(ctx.Tone.Instructions, "LANGKAH PRAKTIS") {
		t.Fatal("legacy custom prompt must receive practical output guard")
	}
	if !strings.Contains(ctx.Tone.Instructions, "BAHASA UMUM") {
		t.Fatal("legacy custom prompt must receive common-language guard")
	}
	if !strings.Contains(ctx.Tone.Instructions, "BATAS UTAS 8-10") {
		t.Fatal("legacy custom prompt must receive the current thread-length guard")
	}
	if !strings.Contains(ctx.Tone.Instructions, "HOOK TERASA HIDUP") {
		t.Fatal("legacy custom prompt must receive the active cover-copy guard")
	}
	if !strings.Contains(ctx.Tone.Instructions, "COVER SCORECARD 9/10") {
		t.Fatal("legacy custom prompt must receive the current cover scorecard")
	}
	if !strings.Contains(ctx.Tone.Instructions, "COVER VOICE SOSIAL V2") {
		t.Fatal("legacy custom prompt must receive the social cover voice")
	}
	if !strings.Contains(ctx.Tone.Instructions, "PRODUCT-LED SOFT SELLING") {
		t.Fatal("legacy custom prompt must receive product-led soft-selling rules")
	}
}

func TestEditorialContextCarriesProductProfile(t *testing.T) {
	product := ProductProfile{Name: "CatatCuan", Audience: "pemilik UMKM", Description: "merapikan arus kas", CTA: `Komen "CATAT"`}
	ctx := buildEditorialContext(Memory{Product: product}, GenerateRequest{}, nil)
	if ctx.Product.Name != product.Name || ctx.Product.Description != product.Description {
		t.Fatalf("product profile missing from editorial context: %+v", ctx.Product)
	}
	if len(ctx.CTAs) == 0 || ctx.CTAs[0] != product.CTA {
		t.Fatalf("product CTA must be prioritized: %+v", ctx.CTAs)
	}
}

func TestOneColumnProductKnowledgeFeedsSoftSellGuard(t *testing.T) {
	product := ProductProfile{Knowledge: "Nama produk: CatatCuan\nTarget pengguna: pemilik UMKM\nCTA lembut: Komen CUAN"}
	ctx := buildEditorialContext(Memory{Product: product}, GenerateRequest{}, nil)
	if ctx.Product.Knowledge != product.Knowledge {
		t.Fatal("product knowledge missing from editorial context")
	}
	if got := product.EffectiveName(); got != "CatatCuan" {
		t.Fatalf("effective product name=%q", got)
	}
	if len(ctx.CTAs) == 0 || ctx.CTAs[0] != "Komen CUAN" {
		t.Fatalf("knowledge CTA must be prioritized: %+v", ctx.CTAs)
	}
	copy := GenCopy{Thread: []string{
		"hook", "Pakai CatatCuan sekarang.", "insight", "contoh", "langkah", "cara manual", "jembatan", "Komen CUAN kalau mau contohnya.",
	}}
	issues := strings.Join(productSoftSellIssues(copy, product), " | ")
	if !strings.Contains(issues, "menyebut identitas produk") {
		t.Fatalf("knowledge product name must be guarded: %s", issues)
	}
}

func TestEditorialContextDefaultsToVisibleBackendPrompt(t *testing.T) {
	ctx := buildEditorialContext(Memory{}, GenerateRequest{}, nil)
	if ctx.Tone.Instructions != DefaultEditableEditorialPrompt {
		t.Fatal("empty workspace must use the backend prompt shown in UI")
	}
}

func TestValidateEditorialPackageOK(t *testing.T) {
	pkg := GenEditorialPackage{
		Strategy: GenStrategy{Angle: "angle tajam"},
		Copy: GenCopy{
			Hook: "hook",
			Thread: []string{
				"hook",
				"Buka sheet pelanggan lalu cek kolom yang masih kosong.",
				"Buat template pesan dan simpan contoh output yang sudah benar.",
				"konteks", "penjelasan", "konsekuensi", "rangkuman", "penutup",
			},
		},
		VisualDirection: GenVisualDirection{CoverBrief: "cover"},
		Claims:          []GenClaim{{Text: "fakta", EvidenceIDs: []string{"src_1"}}},
	}
	ev := ResearchEvidence{Sources: []ResearchSource{{ID: "src_1", URL: "https://ex.com"}}}
	if errs := ValidateEditorialPackage(pkg, ev); len(errs) != 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
}

func TestEditorialCopyIssuesRejectsInsightWithoutPracticalSteps(t *testing.T) {
	copy := GenCopy{
		Hook: "Ada peluang yang sering lewat begitu saja.",
		Thread: []string{
			"konteks", "penjelasan", "masalah", "akibat", "peluang", "insight", "penutup",
		},
	}
	issues := strings.Join(editorialCopyIssues(copy), " | ")
	if !strings.Contains(issues, "utas belum praktis") {
		t.Fatalf("non-actionable thread should trigger repair, got %s", issues)
	}
}

func TestEditorialCopyIssuesRejectsSevenParts(t *testing.T) {
	copy := GenCopy{
		Hook: "Hook yang jelas.",
		Thread: []string{
			"Buka tools yang dipakai.", "Tulis prompt singkat.", "Simpan template dan contoh output.", "empat", "lima", "enam", "tujuh",
		},
	}
	issues := strings.Join(editorialCopyIssues(copy), " | ")
	if !strings.Contains(issues, "wajib 8–10") {
		t.Fatalf("seven-part thread must be rejected, got %s", issues)
	}
}

func TestEditorialCopyIssuesRejectsAIWritingPatterns(t *testing.T) {
	copy := GenCopy{
		Hook: "Masalahnya bukan waktu, tapi prioritas.",
		Thread: []string{
			"satu", "dua", "tiga", "empat", "lima", "enam", "tujuh — selesai",
		},
	}
	issues := strings.Join(editorialCopyIssues(copy), " | ")
	if !strings.Contains(issues, "bukan-X-tapi-Y") || !strings.Contains(issues, "em dash") {
		t.Fatalf("expected AI-pattern issues, got %s", issues)
	}
}

func TestEditorialCopyIssuesRejectsStiffPassiveLanguage(t *testing.T) {
	copy := GenCopy{
		Hook: "Materi onboarding sering dikirim sebagai PDF, lalu dilupakan setelah hari pertama.",
		Thread: []string{
			"satu", "dua", "tiga", "empat", "lima", "enam", "tujuh",
		},
	}
	issues := strings.Join(editorialCopyIssues(copy), " | ")
	if !strings.Contains(issues, "formal, pasif") {
		t.Fatalf("stiff AI language should trigger repair, got %s", issues)
	}
}

func TestEditorialCopyIssuesRejectsForcedTranslation(t *testing.T) {
	copy := GenCopy{
		Hook: "Masukkan instruksi AI untuk mengoptimalkan hasil.",
		Thread: []string{
			"Buka tools yang dipakai.", "Tulis prompt singkat.", "Simpan template dan contoh output.", "empat", "lima", "enam", "tujuh",
		},
	}
	issues := strings.Join(editorialCopyIssues(copy), " | ")
	if !strings.Contains(issues, "istilah yang kaku") {
		t.Fatalf("forced translation should trigger repair, got %s", issues)
	}
}

func TestEditorialCopyIssuesRejectsGenericBusinessAudience(t *testing.T) {
	copy := GenCopy{Hook: "Pemilik UMKM sering kehilangan calon pelanggan di WhatsApp.", Thread: []string{
		"Pemilik UMKM sering kehilangan calon pelanggan di WhatsApp.", "Buka daftar chat.", "Catat statusnya.", "Buat kolom follow-up.", "Tulis contoh pesan.", "Cek balasan.", "Simpan output.", "Ketik FOLLOWUP kalau mau coba sistemnya.",
	}}
	issues := strings.Join(editorialCopyIssues(copy), " | ")
	if !strings.Contains(issues, "audiens umum") {
		t.Fatalf("generic UMKM target must trigger repair: %s", issues)
	}
}

func TestEditorialCopyIssuesAcceptsMicroBusinessAudience(t *testing.T) {
	copy := GenCopy{Hook: "Warung makan yang menerima pesanan lewat WhatsApp sering kehilangan chat pelanggan lama.", Thread: []string{
		"Warung makan yang menerima pesanan lewat WhatsApp sering kehilangan chat pelanggan lama.", "Buka daftar chat.", "Catat statusnya.", "Buat kolom follow-up.", "Tulis contoh pesan.", "Cek balasan.", "Simpan output.", "Ketik FOLLOWUP kalau mau coba sistemnya.",
	}}
	issues := strings.Join(editorialCopyIssues(copy), " | ")
	if strings.Contains(issues, "audiens umum") {
		t.Fatalf("micro segment was rejected: %s", issues)
	}
}

func TestProductSoftSellPlacement(t *testing.T) {
	product := ProductProfile{Name: "CatatCuan", Description: "mencatat arus kas"}
	good := GenCopy{Thread: []string{
		"hook", "masalah", "insight", "contoh", "langkah", "cara manual", "Sistem ini membantu merapikan catatan.", `Kalau mau coba sistem yang merapikan catatan ini, komen "CATAT". Nanti aku kirim detailnya lewat DM.`,
	}}
	if issues := productSoftSellIssues(good, product); len(issues) != 0 {
		t.Fatalf("natural late soft sell rejected: %v", issues)
	}
	early := good
	early.Thread = append([]string{}, good.Thread...)
	early.Thread[1] = "Pakai CatatCuan untuk menyelesaikannya."
	issues := strings.Join(productSoftSellIssues(early, product), " | ")
	if !strings.Contains(issues, "menyebut identitas produk") {
		t.Fatalf("public product mention must be rejected: %s", issues)
	}
}

func TestProductIdentifiersComeFromAllKnowledgeWebsites(t *testing.T) {
	product := ProductProfile{Knowledge: "produk: WhatsApp Marketing\nwebsite: ngumpulin.com\nproduk: keuangan lewat WhatsApp\nwebsite: wafin.id\nwebsite: tambahlaba.id"}
	got := strings.ToLower(strings.Join(product.PublicIdentifiers(), " | "))
	for _, want := range []string{"ngumpulin.com", "ngumpulin", "wafin.id", "wafin", "tambahlaba.id", "tambahlaba"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing identifier %q in %s", want, got)
		}
	}
}

func TestProductSoftSellRejectsDetachedGenericCTA(t *testing.T) {
	product := ProductProfile{Knowledge: "Nama produk: Wafin\nCTA lembut: ketik CUAN"}
	copy := GenCopy{Thread: []string{
		"hook", "Pisahkan order dari uang masuk.", "Catat uang keluar.", "Cek selisihnya.", "Ulangi tiap malam.", "Lihat saldo usaha.", "Rapikan catatannya.",
		"Ketik CUAN untuk mendapatkan akses cara mendapatkan cuan dari internet GRATIS lewat sistemnya.",
	}}
	issues := strings.Join(productSoftSellIssues(copy, product), " | ")
	if !strings.Contains(issues, "tidak nyambung") {
		t.Fatalf("detached generic CTA must trigger repair: %s", issues)
	}
}

func TestHardSellDetectorAllowsPromoAsDiscussionTopic(t *testing.T) {
	allowed := []string{
		"Pesan promo yang sama dikirim ke pelanggan lama dan pelanggan baru.",
		"Warung makan ini menyimpan riwayat promo pelanggan di satu catatan.",
		"Cek apakah diskon sebelumnya membuat pelanggan kembali.",
	}
	for _, text := range allowed {
		if hardSellCopyRE.MatchString(text) {
			t.Fatalf("discussion text incorrectly marked hard selling: %q", text)
		}
	}
	blocked := []string{"Beli sekarang sebelum habis.", "Klaim promo hari ini.", "Langsung checkout sekarang."}
	for _, text := range blocked {
		if !hardSellCopyRE.MatchString(text) {
			t.Fatalf("transaction CTA was not detected: %q", text)
		}
	}
}

func TestProductSoftSellRejectsPublicPolicyLeakAndClunkyAccessLanguage(t *testing.T) {
	product := ProductProfile{Knowledge: "website: wafin.id"}
	copy := GenCopy{Thread: []string{
		"Laundry kiloan sering baru tahu kasnya meleset saat mau belanja deterjen.", "dua", "tiga", "empat", "lima", "enam", "tujuh",
		"Kalau kamu mau alur tutup kas laundry yang lebih rapi tanpa menyebut merek di sini, ketik LAUNDRY di komentar. Nanti aku kirim detail akses alat pencatatan lewat DM.",
	}}
	issues := strings.Join(productSoftSellIssues(copy, product), " | ")
	if !strings.Contains(issues, "aturan internal") || !strings.Contains(issues, "bahasa akses produk yang kaku") {
		t.Fatalf("meta/clunky CTA must trigger repair: %s", issues)
	}
}

func TestProductSoftSellRequiresCommentTrigger(t *testing.T) {
	product := ProductProfile{Knowledge: "Nama produk: Wafin"}
	copy := GenCopy{Thread: []string{
		"hook", "dua", "tiga", "empat", "lima", "enam", "tujuh",
		"Kalau mau template catatan kasnya, DM aku dan nanti aku kirim.",
	}}
	issues := strings.Join(productSoftSellIssues(copy, product), " | ")
	if !strings.Contains(issues, "memancing komentar") {
		t.Fatalf("DM-only CTA must not pass the comment trigger: %s", issues)
	}
}

func TestProductSoftSellRejectsMissingSentenceSpaces(t *testing.T) {
	product := ProductProfile{Knowledge: "Nama produk: Wafin"}
	copy := GenCopy{Thread: []string{
		"hook", "dua", "tiga", "empat", "lima", "enam", "tujuh",
		"Cek uang keluar.Mulai malam ini.Ketik CATAT kalau mau coba sistem catatannya.",
	}}
	issues := strings.Join(productSoftSellIssues(copy, product), " | ")
	if !strings.Contains(issues, "kehilangan spasi") {
		t.Fatalf("missing sentence spaces must trigger repair: %s", issues)
	}
}

func TestNormalizeGenPackageKeepsUpToTenPartsAndRemovesEmDash(t *testing.T) {
	pkg := GenEditorialPackage{Copy: GenCopy{Thread: []string{
		"1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
	}, Hook: "Jakarta — satu hari"}}
	normalizeGenPackage(&pkg)
	if len(pkg.Copy.Thread) != 10 {
		t.Fatalf("thread parts=%d", len(pkg.Copy.Thread))
	}
	if strings.Contains(pkg.Copy.Hook, "—") {
		t.Fatalf("em dash survived normalization: %q", pkg.Copy.Hook)
	}
}

func TestValidateEditorialPackageBadEvidenceID(t *testing.T) {
	pkg := GenEditorialPackage{
		Strategy:        GenStrategy{Angle: "a"},
		Copy:            GenCopy{Hook: "h", Thread: []string{"1", "2", "3", "4"}},
		VisualDirection: GenVisualDirection{CoverBrief: "c"},
		Claims:          []GenClaim{{Text: "x", EvidenceIDs: []string{"src_99"}}},
	}
	ev := ResearchEvidence{Sources: []ResearchSource{{ID: "src_1", URL: "https://ex.com"}}}
	errs := ValidateEditorialPackage(pkg, ev)
	if len(errs) == 0 {
		t.Fatal("expected evidence_id error")
	}
}

func TestPackageResultDoesNotRecycleCaptionAsSecondDraft(t *testing.T) {
	pkg := GenEditorialPackage{
		Strategy: GenStrategy{Angle: "angle utama"},
		Copy: GenCopy{
			Hook:    "hook utama",
			Thread:  []string{"satu", "dua", "tiga", "empat"},
			Caption: "caption bukan alternatif thread",
		},
	}
	out := packageToGenerateResult(GenerateRequest{Count: 2}, Memory{}, pkg, ResearchEvidence{}, nil, PipelineMeta{})
	if len(out.Drafts) != 1 {
		t.Fatalf("package conversion must return only its independent draft, got %d", len(out.Drafts))
	}
}

func TestNormalizeResearchDropsUnverifiableSources(t *testing.T) {
	ev := ResearchEvidence{Sources: []ResearchSource{
		{ID: "bad", Title: "tanpa URL"},
		{ID: "also-bad", URL: "javascript:alert(1)"},
		{ID: "good", URL: "https://example.com/source"},
	}}
	normalizeResearch(&ev)
	if len(ev.Sources) != 1 || ev.Sources[0].ID != "good" {
		t.Fatalf("sources=%+v", ev.Sources)
	}
}

func TestEvaluateCriticGateFactuality(t *testing.T) {
	g := evaluateCriticGate(GenCriticReport{
		Scores: map[string]float64{"factuality": 0.7, "hook": 0.9},
	})
	if g.Go || !g.NeedsRevision {
		t.Fatalf("expected revision, got %+v", g)
	}
}

func TestEvaluateCriticGateEvidenceInsufficient(t *testing.T) {
	g := evaluateCriticGate(GenCriticReport{
		Issues: []CriticIssue{{Code: "EVIDENCE_INSUFFICIENT", Severity: "blocking", Instruction: "kurang data"}},
	})
	if !g.EvidenceInsufficient {
		t.Fatalf("%+v", g)
	}
}

func TestGenerateCriticIsOptIn(t *testing.T) {
	t.Setenv("AI_GENERATE_CRITIC", "")
	if generateCriticEnabled() {
		t.Fatal("AI critic must default off for interactive latency")
	}
	t.Setenv("AI_GENERATE_CRITIC", "true")
	if !generateCriticEnabled() {
		t.Fatal("AI critic opt-in was ignored")
	}
}

func TestIntegratedGenerateDefaultsOn(t *testing.T) {
	t.Setenv("AI_GENERATE_INTEGRATED", "")
	if !generateIntegratedEnabled() {
		t.Fatal("integrated ChatGPT generation should be the default")
	}
	t.Setenv("AI_GENERATE_INTEGRATED", "false")
	if generateIntegratedEnabled() {
		t.Fatal("integrated generation opt-out was ignored")
	}
}

func TestFilterRelevantLessons(t *testing.T) {
	lessons := Lessons{
		DoMore: []LessonItem{
			{Pattern: "pajak UMKM", Evidence: "relatable"},
			{Pattern: "desain dark Twitter", Evidence: "banner"},
		},
		Avoid: []LessonItem{
			{Pattern: "omzet vs laba", Evidence: "sering dipakai"},
		},
	}
	out := filterRelevantLessons(lessons, "cara hitung laba UMKM", []string{"keuangan"}, 5)
	joined := ""
	for _, it := range out {
		joined += it.Pattern + " "
	}
	if !strings.Contains(strings.ToLower(joined), "umkm") && !strings.Contains(strings.ToLower(joined), "laba") && !strings.Contains(strings.ToLower(joined), "omzet") {
		t.Fatalf("expected relevant lessons, got %q", joined)
	}
	if strings.Contains(joined, "Twitter") && !strings.Contains(joined, "UMKM") {
		t.Fatalf("irrelevant twitter lesson leaked alone: %q", joined)
	}
}

func TestBuildContentHistory(t *testing.T) {
	mem := Memory{
		History: []GenHistory{
			{Topic: "pajak", Drafts: []GeneratedDraft{{Angle: "omzet vs laba", Hook: "Omzet naik..."}}},
			{Topic: "pajak", Drafts: []GeneratedDraft{{Angle: "nota berantakan", Hook: "Nota..."}}},
		},
	}
	h := buildContentHistory(mem)
	if len(h.RecentTopics) == 0 || len(h.RecentAngles) < 2 {
		t.Fatalf("%+v", h)
	}
}

func TestParseGenEditorialPackage(t *testing.T) {
	raw := `{"intent":{"primary_goal":"awareness","format":"thread"},"strategy":{"angle":"A","why_this_angle":"w","core_problem":"p","content_promise":"c"},"story":{"arc":"x","slides":[]},"copy":{"hook":"H","thread":["1","2","3","4"]},"visual_direction":{"cover_brief":"brief"},"claims":[],"creative_reasoning":{"why_this_angle":"wa"}}`
	pkg, err := parseGenEditorialPackage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Copy.Thread) != 4 || pkg.VisualDirection.CoverBrief != "brief" {
		t.Fatalf("%+v", pkg)
	}
}

func TestRemoveInternalSourceRefsFromPublicCopy(t *testing.T) {
	cases := map[string]string{
		"Data ini naik 20% [src_1, src_2].":       "Data ini naik 20%.",
		"Temuan resminya sama (src_3).":           "Temuan resminya sama.",
		`Angkanya terverifikasi [src\_4; src\_5]`: "Angkanya terverifikasi",
		"Sumber tetap internal [Sumber: src_6]":   "Sumber tetap internal",
	}
	for input, want := range cases {
		if got := removeInternalSourceRefs(input); got != want {
			t.Errorf("removeInternalSourceRefs(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestCoverHeadlineRejectsPassiveAbstractCopy(t *testing.T) {
	headline := "Riset KOMPETITOR tiap bulan sering cuma jadi tab browser. Ubah jadi change log yang siap dipakai pemilik usaha."
	issues := strings.Join(coverHeadlineTextIssues(headline), " | ")
	if !strings.Contains(issues, "abstrak atau pasif") {
		t.Fatalf("passive cover must be rejected, got %s", issues)
	}
	if !strings.Contains(issues, "perintah generik") {
		t.Fatalf("generic command must be rejected, got %s", issues)
	}
}

func TestCoverHeadlinePositiveSignals(t *testing.T) {
	headline := "Kompetitor bergerak diam-diam. Pemilik usaha masih membongkar tab lama satu-satu sebelum berani mengganti harga."
	issues := coverHeadlineTextIssues(headline)
	for _, issue := range issues {
		if strings.Contains(issue, "pelaku") || strings.Contains(issue, "aksi") || strings.Contains(issue, "ketegangan") || strings.Contains(issue, "dua kalimat") {
			t.Fatalf("alive cover incorrectly rejected: %v", issues)
		}
	}
}

func TestCoverHeadlineAcceptsConversationalCreatorVoice(t *testing.T) {
	cases := []string{
		"Serius guys, minimal paham basic Excel. Mau kuliah, magang, atau kerja bakal terus kepake.",
		"POV: Atasan heran lihat anak magang lebih cepat olah data daripada senior. Ternyata ini rahasianya.",
	}
	for _, headline := range cases {
		if issues := coverHeadlineTextIssues(headline); len(issues) != 0 {
			t.Errorf("conversational cover rejected %q: %v", headline, issues)
		}
	}
}
