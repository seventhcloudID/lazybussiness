package main

import (
	"testing"

	"threads-dashboard/internal/ai"
)

func TestChatNeedsAccountContext(t *testing.T) {
	if !chatNeedsAccountContext([]ai.ChatMessage{{Role: "user", Text: "Analisa performa akun Threads aku"}}) {
		t.Fatal("account question should load account context")
	}
	if chatNeedsAccountContext([]ai.ChatMessage{{Role: "user", Text: "Jelaskan hukum Newton"}}) {
		t.Fatal("unrelated question must not load account context")
	}
}
