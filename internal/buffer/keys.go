package buffer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Legacy global path (pre multi-account). Migrated ke .data/accounts/{id}/buffer_key.json.
const LegacyKeyPath = ".data/buffer_key.json"

type storedKeyFile struct {
	APIKey string `json:"api_key"`
}

var keyFileMu sync.Mutex

// HasStoredAPIKey true kalau file keyPath berisi API key valid.
func HasStoredAPIKey(path string) bool {
	return loadStoredAPIKeyAt(path) != ""
}

func loadStoredAPIKeyAt(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	keyFileMu.Lock()
	defer keyFileMu.Unlock()
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var f storedKeyFile
	if json.Unmarshal(raw, &f) != nil {
		return ""
	}
	return normalizeKey(f.APIKey)
}

func saveStoredAPIKeyAt(path, key string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	keyFileMu.Lock()
	defer keyFileMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	key = normalizeKey(key)
	if key == "" {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	payload, err := json.MarshalIndent(storedKeyFile{APIKey: key}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func normalizeKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "bearer ")
	return strings.TrimSpace(key)
}
