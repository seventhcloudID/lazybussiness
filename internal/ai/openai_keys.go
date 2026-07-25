package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const openAIKeysPath = ".data/openai_keys.json"

type storedOpenAIKeysFile struct {
	Keys []string `json:"keys"`
}

var openAIKeysMu sync.Mutex

func loadStoredOpenAIKeys() []string {
	openAIKeysMu.Lock()
	defer openAIKeysMu.Unlock()
	raw, err := os.ReadFile(openAIKeysPath)
	if err != nil {
		return nil
	}
	var f storedOpenAIKeysFile
	if json.Unmarshal(raw, &f) != nil {
		return nil
	}
	return normalizeKeys(f.Keys)
}

func saveStoredOpenAIKeys(keys []string) error {
	openAIKeysMu.Lock()
	defer openAIKeysMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(openAIKeysPath), 0o700); err != nil {
		return err
	}
	clean := normalizeKeys(keys)
	if len(clean) == 0 {
		err := os.Remove(openAIKeysPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	payload, err := json.MarshalIndent(storedOpenAIKeysFile{Keys: clean}, "", "  ")
	if err != nil {
		return err
	}
	tmp := openAIKeysPath + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, openAIKeysPath)
}

func clearStoredOpenAIKeys() error {
	return saveStoredOpenAIKeys(nil)
}

func maskKey(k string) string {
	k = strings.TrimSpace(k)
	if len(k) <= 8 {
		return "****"
	}
	return k[:4] + "…" + k[len(k)-4:]
}
