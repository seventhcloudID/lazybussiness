package ai

import "testing"

func TestMergeUsage(t *testing.T) {
	got := mergeUsage(
		&TokenUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14},
		&TokenUsage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10},
	)
	if got == nil || got.PromptTokens != 17 || got.CompletionTokens != 7 || got.TotalTokens != 24 {
		t.Fatalf("mergeUsage() = %#v", got)
	}
}
