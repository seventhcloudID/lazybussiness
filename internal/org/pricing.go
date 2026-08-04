package org

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Pricing is the single product plan shown in Core.
type Pricing struct {
	Name      string `json:"name"`
	Amount    int64  `json:"amount"` // IDR
	Currency  string `json:"currency"`
	Interval  string `json:"interval"` // month
	Features  []string `json:"features"`
	Notes     string `json:"notes,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func defaultPricing() Pricing {
	return Pricing{
		Name:     "Bulanan",
		Amount:   145000,
		Currency: "IDR",
		Interval: "month",
		Features: []string{
			"Multi akun brand (Threads + Instagram)",
			"Generate AI, Lazy scheduler, balasan",
			"Insight & kuota API workspace",
			"IG carousel + Buffer (TikTok / X)",
			"BYOK Gemini & OpenAI",
		},
		Notes: "Satu paket. Provisioned by admin.",
	}
}

func pricingPath(dataRoot string) string {
	if dataRoot == "" {
		dataRoot = ".data"
	}
	return filepath.Join(dataRoot, "pricing.json")
}

// LoadPricing reads .data/pricing.json or returns the default 145K plan.
func LoadPricing(dataRoot string) (Pricing, error) {
	path := pricingPath(dataRoot)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultPricing(), nil
		}
		return Pricing{}, err
	}
	var p Pricing
	if err := json.Unmarshal(b, &p); err != nil {
		return Pricing{}, err
	}
	if p.Amount <= 0 {
		p.Amount = 145000
	}
	if strings.TrimSpace(p.Currency) == "" {
		p.Currency = "IDR"
	}
	if strings.TrimSpace(p.Interval) == "" {
		p.Interval = "month"
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = "Bulanan"
	}
	if len(p.Features) == 0 {
		p.Features = defaultPricing().Features
	}
	return p, nil
}

// SavePricing writes pricing.json.
func SavePricing(dataRoot string, p Pricing) (Pricing, error) {
	if p.Amount <= 0 {
		p.Amount = 145000
	}
	if strings.TrimSpace(p.Currency) == "" {
		p.Currency = "IDR"
	}
	if strings.TrimSpace(p.Interval) == "" {
		p.Interval = "month"
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = "Bulanan"
	}
	if len(p.Features) == 0 {
		p.Features = defaultPricing().Features
	}
	clean := make([]string, 0, len(p.Features))
	for _, f := range p.Features {
		f = strings.TrimSpace(f)
		if f != "" {
			clean = append(clean, f)
		}
	}
	p.Features = clean
	p.Notes = strings.TrimSpace(p.Notes)
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeJSON(pricingPath(dataRoot), p); err != nil {
		return Pricing{}, err
	}
	return p, nil
}
