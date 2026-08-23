package repliz

import "testing"

func TestTikTokAdditionalInfoDraft(t *testing.T) {
	got := TikTokAdditionalInfo(true, true)
	if got["isDraft"] != true {
		t.Fatalf("isDraft = %v, want true", got["isDraft"])
	}
	if got["isAiGenerated"] != true {
		t.Fatalf("isAiGenerated = %v, want true", got["isAiGenerated"])
	}
	if got["isAutoAddMusic"] != false {
		t.Fatalf("isAutoAddMusic = %v, want false", got["isAutoAddMusic"])
	}
}
