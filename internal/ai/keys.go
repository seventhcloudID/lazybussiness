package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type storedKeysFile struct {
	Keys []string `json:"keys"`
}

var (
	keysFileMu sync.Mutex
)

// LoadStoredAPIKeys membaca key Gemini dari store workspace (UI).
func LoadStoredAPIKeys() []string {
	keysFileMu.Lock()
	defer keysFileMu.Unlock()
	raw, err := os.ReadFile(geminiKeysFile())
	if err != nil {
		return nil
	}
	var f storedKeysFile
	if json.Unmarshal(raw, &f) != nil {
		return nil
	}
	return normalizeKeys(f.Keys)
}

// SaveStoredAPIKeys menulis key ke store workspace (mode 0600).
func SaveStoredAPIKeys(keys []string) error {
	keysFileMu.Lock()
	defer keysFileMu.Unlock()
	path := geminiKeysFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	clean := normalizeKeys(keys)
	payload, err := json.MarshalIndent(storedKeysFile{Keys: clean}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ClearStoredAPIKeys menghapus file key UI.
func ClearStoredAPIKeys() error {
	keysFileMu.Lock()
	defer keysFileMu.Unlock()
	err := os.Remove(geminiKeysFile())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func normalizeKeys(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range in {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func mergeAPIKeys(parts ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range parts {
		for _, k := range list {
			k = strings.TrimSpace(k)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// MaskAPIKey menyembunyikan tengah key untuk ditampilkan di UI.
func MaskAPIKey(k string) string {
	k = strings.TrimSpace(k)
	if k == "" {
		return ""
	}
	r := []rune(k)
	n := len(r)
	if n <= 10 {
		return strings.Repeat("•", n)
	}
	return string(r[:6]) + "…" + string(r[n-4:])
}

// KeysStatus ringkasan untuk API/UI.
type KeysStatus struct {
	Enabled      bool     `json:"enabled"`
	Provider     string   `json:"provider"`
	Model        string   `json:"model"`
	Total        int      `json:"total"`
	FromEnv      int      `json:"from_env"`
	FromStore    int      `json:"from_store"`
	StoredMasked []string `json:"stored_masked"`
	Note         string   `json:"note"`
}

func (c *Client) KeysStatus() KeysStatus {
	envKeys := collectAPIKeysFromEnv()
	storeKeys := LoadStoredAPIKeys()
	st := KeysStatus{
		Enabled:   c.Enabled(),
		Provider:  c.Provider(),
		Model:     c.Model(),
		Total:     c.KeyCount(),
		FromEnv:   len(envKeys),
		FromStore: len(storeKeys),
		Note:      "API key dari .env (AI_API_KEY). Key Gemini/OpenAI di UI tidak dipakai.",
	}
	for _, k := range storeKeys {
		st.StoredMasked = append(st.StoredMasked, MaskAPIKey(k))
	}
	if st.Total == 0 {
		st.Note = "Belum ada API key workspace — isi di halaman ini atau set AI_API_KEY di .env"
	}
	return st
}

// SetAPIKeys mengganti key aktif di memori (hasil merge env + store biasanya).
func (c *Client) SetAPIKeys(keys []string) {
	if c == nil {
		return
	}
	clean := normalizeKeys(keys)
	c.keyMu.Lock()
	c.apiKeys = clean
	c.keyIdx = 0
	c.keyMu.Unlock()
	if c.quota != nil {
		c.quota.setKeyCount(len(clean))
	}
}

// ReloadAPIKeys memuat ulang dari .env + file UI.
func (c *Client) ReloadAPIKeys() {
	if c == nil {
		return
	}
	keys := collectAPIKeysFromEnv()
	if c.provider == "gemini" || c.provider == "google" {
		keys = mergeAPIKeys(keys, LoadStoredAPIKeys())
	}
	c.SetAPIKeys(keys)
}

// ApplyStoredAPIKeys menyimpan ke file lalu reload ke client.
func (c *Client) ApplyStoredAPIKeys(keys []string) error {
	if c == nil {
		return fmt.Errorf("AI client nil")
	}
	if err := SaveStoredAPIKeys(keys); err != nil {
		return err
	}
	c.ReloadAPIKeys()
	return nil
}

// ClearAndReloadStored menghapus key UI lalu reload (sisakan .env jika ada).
func (c *Client) ClearAndReloadStored() error {
	if c == nil {
		return fmt.Errorf("AI client nil")
	}
	if err := ClearStoredAPIKeys(); err != nil {
		return err
	}
	c.ReloadAPIKeys()
	return nil
}

func collectAPIKeysFromEnv() []string {
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	add(os.Getenv("AI_API_KEY"))
	add(os.Getenv("AI_API_KEYS"))
	for i := 2; i <= 8; i++ {
		add(os.Getenv(fmt.Sprintf("AI_API_KEY_%d", i)))
	}
	return out
}
