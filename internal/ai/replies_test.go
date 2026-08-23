package ai

import (
	"strings"
	"testing"
)

func TestRepliesPromptCarriesNaturalConversationRules(t *testing.T) {
	wants := []string{
		"bukan admin customer service",
		"Jawab poin utamanya sejak kata pertama",
		"Terima kasih sudah berbagi",
		"bukan X, tapi Y",
		"Jangan menawarkan produk, DM, link, atau CTA",
		"Pertanyaan balik hanya jika benar-benar membuka obrolan",
		"DETEKSI KEYWORD CTA",
		"Jangan menulis \"sudah aku kirim\"",
		"Jangan mencari sinonim aneh untuk kata kirim",
	}
	for _, want := range wants {
		if !strings.Contains(repliesSystemPrompt, want) {
			t.Errorf("repliesSystemPrompt missing %q", want)
		}
	}
}

func TestEmptyIntentUsesAutomaticInference(t *testing.T) {
	intent := strings.TrimSpace("")
	if intent == "" {
		intent = autoRepliesIntent
	}
	if !strings.HasPrefix(intent, "AUTO_INFER:") {
		t.Fatalf("intent = %q, want AUTO_INFER", intent)
	}
}

func TestNormalizeIncomingKeepsRealNameWithoutAtPrefix(t *testing.T) {
	got := normalizeIncoming([]IncomingReply{{ID: " 123 ", Username: "@Shika", Text: "  bisa buat laundry? "}})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Username != "Shika" || got[0].Text != "bisa buat laundry?" {
		t.Fatalf("normalized = %#v", got[0])
	}
}

func TestCleanReplyDraftTextRemovesAICliches(t *testing.T) {
	got := cleanReplyDraftText("Terima kasih atas komentarnya. Tentu, bisa dipakai buat laundry — mulai dari catatan harian. Semoga membantu!")
	want := "Bisa dipakai buat laundry, mulai dari catatan harian"
	if got != want {
		t.Fatalf("cleanReplyDraftText() = %q, want %q", got, want)
	}
}
