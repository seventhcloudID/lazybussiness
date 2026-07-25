package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Free-tier defaults for Gemini Flash-class models (estimasi; override via env).
// RPD resets at midnight Pacific Time (America/Los_Angeles).
const (
	defaultRPM = 10
	defaultRPD = 250
	defaultTPM = 250_000
)

type QuotaBucket struct {
	Used      int    `json:"used"`
	Limit     int    `json:"limit"`
	Remaining int    `json:"remaining"`
	ResetsAt  string `json:"resets_at,omitempty"`
}

type QuotaStatus struct {
	Tier        string      `json:"tier"`
	Provider    string      `json:"provider"`
	Model       string      `json:"model"`
	RPM         QuotaBucket `json:"rpm"`
	RPD         QuotaBucket `json:"rpd"`
	TPM         QuotaBucket `json:"tpm"`
	TokensToday int         `json:"tokens_today"`
	LastCall    *TokenUsage `json:"last_call,omitempty"`
	Note        string      `json:"note"`
}

type quotaState struct {
	DayKey               string `json:"day_key"`
	MinuteKey            string `json:"minute_key"`
	RequestsToday        int    `json:"requests_today"`
	TokensToday          int    `json:"tokens_today"`
	RequestsMinute       int    `json:"requests_minute"`
	TokensMinute         int    `json:"tokens_minute"`
	LastPromptTokens     int    `json:"last_prompt_tokens"`
	LastCompletionTokens int    `json:"last_completion_tokens"`
	UpdatedAt            string `json:"updated_at"`
}

type quotaTracker struct {
	mu       sync.Mutex
	path     string
	tier     string
	rpmLimit int
	rpdLimit int
	tpmLimit int
	state    quotaState
}

func newQuotaTrackerFromEnv(keyCount int) *quotaTracker {
	if keyCount < 1 {
		keyCount = 1
	}
	t := &quotaTracker{
		path:     filepath.Join(".data", "ai_quota.json"),
		tier:     env("AI_TIER", "free"),
		rpmLimit: envInt("AI_QUOTA_RPM", defaultRPM) * keyCount,
		rpdLimit: envInt("AI_QUOTA_RPD", defaultRPD) * keyCount,
		tpmLimit: envInt("AI_QUOTA_TPM", defaultTPM) * keyCount,
	}
	t.load()
	return t
}

// setKeyCount menyesuaikan limit kuota lokal saat jumlah API key berubah (UI).
func (t *quotaTracker) setKeyCount(keyCount int) {
	if t == nil {
		return
	}
	if keyCount < 1 {
		keyCount = 1
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rpmLimit = envInt("AI_QUOTA_RPM", defaultRPM) * keyCount
	t.rpdLimit = envInt("AI_QUOTA_RPD", defaultRPD) * keyCount
	t.tpmLimit = envInt("AI_QUOTA_TPM", defaultTPM) * keyCount
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func pacificNow() time.Time {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.Now().UTC().Add(-8 * time.Hour)
	}
	return time.Now().In(loc)
}

func (t *quotaTracker) load() {
	b, err := os.ReadFile(t.path)
	if err != nil {
		return
	}
	var st quotaState
	if json.Unmarshal(b, &st) == nil {
		t.state = st
	}
}

func (t *quotaTracker) persistLocked() {
	if t.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(t.path), 0o700)
	t.state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	b, err := json.MarshalIndent(t.state, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(t.path, b, 0o600)
}

func (t *quotaTracker) rollWindowsLocked(now time.Time) {
	day := now.Format("2006-01-02")
	minute := now.Format("2006-01-02T15:04")
	if t.state.DayKey != day {
		t.state.DayKey = day
		t.state.RequestsToday = 0
		t.state.TokensToday = 0
	}
	if t.state.MinuteKey != minute {
		t.state.MinuteKey = minute
		t.state.RequestsMinute = 0
		t.state.TokensMinute = 0
	}
}

func (t *quotaTracker) check() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := pacificNow()
	t.rollWindowsLocked(now)

	if t.rpdLimit > 0 && t.state.RequestsToday >= t.rpdLimit {
		reset := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		return &QuotaError{
			Kind:    "rpd",
			Message: "Kuota Gemini free tier harian (RPD) habis. Reset " + reset.Format(time.RFC3339),
		}
	}
	if t.rpmLimit > 0 && t.state.RequestsMinute >= t.rpmLimit {
		return &QuotaError{
			Kind:    "rpm",
			Message: "Kuota Gemini per menit (RPM) penuh. Tunggu ~1 menit lalu coba lagi.",
		}
	}
	return nil
}

func (t *quotaTracker) record(usage *TokenUsage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := pacificNow()
	t.rollWindowsLocked(now)

	prompt, completion, total := 0, 0, 0
	if usage != nil {
		prompt = usage.PromptTokens
		completion = usage.CompletionTokens
		total = usage.TotalTokens
		if total == 0 {
			total = prompt + completion
		}
	}

	t.state.RequestsToday++
	t.state.RequestsMinute++
	t.state.TokensToday += total
	t.state.TokensMinute += total
	t.state.LastPromptTokens = prompt
	t.state.LastCompletionTokens = completion
	t.persistLocked()
}

func (t *quotaTracker) status(provider, model string) QuotaStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := pacificNow()
	t.rollWindowsLocked(now)

	reset := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	bucket := func(used, limit int, resets string) QuotaBucket {
		rem := limit - used
		if rem < 0 {
			rem = 0
		}
		return QuotaBucket{Used: used, Limit: limit, Remaining: rem, ResetsAt: resets}
	}

	var last *TokenUsage
	if t.state.LastPromptTokens > 0 || t.state.LastCompletionTokens > 0 {
		last = &TokenUsage{
			PromptTokens:     t.state.LastPromptTokens,
			CompletionTokens: t.state.LastCompletionTokens,
			TotalTokens:      t.state.LastPromptTokens + t.state.LastCompletionTokens,
		}
	}

	return QuotaStatus{
		Tier:        t.tier,
		Provider:    provider,
		Model:       model,
		RPM:         bucket(t.state.RequestsMinute, t.rpmLimit, ""),
		RPD:         bucket(t.state.RequestsToday, t.rpdLimit, reset.Format(time.RFC3339)),
		TPM:         bucket(t.state.TokensMinute, t.tpmLimit, ""),
		TokensToday: t.state.TokensToday,
		LastCall:    last,
		Note:        "Estimasi lokal dari pemakaian app ini saja (bukan angka resmi AI Studio). RPD reset tengah malam Pacific.",
	}
}

type QuotaError struct {
	Kind    string
	Message string
}

func (e *QuotaError) Error() string { return e.Message }
