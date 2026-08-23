package repliz

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ActiveStore struct {
	mu   sync.Mutex
	path string
}

type activeFile struct {
	ActiveID string `json:"active_id"`
}

func NewActiveStore(dir string) *ActiveStore {
	return &ActiveStore{path: filepath.Join(dir, "repliz.json")}
}

func (s *ActiveStore) Get() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		return ""
	}
	var f activeFile
	if json.Unmarshal(b, &f) != nil {
		return ""
	}
	return strings.TrimSpace(f.ActiveID)
}

func (s *ActiveStore) Set(id string) error {
	if s == nil {
		return nil
	}
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(activeFile{ActiveID: id}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(b, '\n'), 0o600)
}
