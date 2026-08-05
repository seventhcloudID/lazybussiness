package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const APIKeyPrefix = "mn_"

// ConnectKeyStore holds hashed API keys for OpenAPI / GPT Actions / Hermes.
type ConnectKeyStore struct {
	mu   sync.Mutex
	path string
	keys []ConnectKey
}

type ConnectKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"` // mn_xxxxxxxx…
	Hash      string `json:"hash"`
	CreatedAt int64  `json:"created_at"`
	LastUsed  int64  `json:"last_used,omitempty"`
}

type connectKeyFile struct {
	Keys []ConnectKey `json:"keys"`
}

func OpenConnectKeys(dir string) (*ConnectKeyStore, error) {
	if dir == "" {
		dir = ".data"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &ConnectKeyStore{path: filepath.Join(dir, "connect_api_keys.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *ConnectKeyStore) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.keys = nil
			return nil
		}
		return err
	}
	var f connectKeyFile
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	s.keys = f.Keys
	return nil
}

func (s *ConnectKeyStore) saveLocked() error {
	raw, err := json.MarshalIndent(connectKeyFile{Keys: s.keys}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *ConnectKeyStore) List() []ConnectKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ConnectKey, len(s.keys))
	for i, k := range s.keys {
		cp := k
		cp.Hash = "" // never expose hash
		out[i] = cp
	}
	return out
}

// Create returns the plaintext key once.
func (s *ConnectKeyStore) Create(name string) (plain string, meta ConnectKey, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	var buf [24]byte
	if _, err = rand.Read(buf[:]); err != nil {
		return "", ConnectKey{}, err
	}
	plain = APIKeyPrefix + hex.EncodeToString(buf[:])
	meta = ConnectKey{
		ID:        hex.EncodeToString(buf[:8]),
		Name:      name,
		Prefix:    plain[:min(12, len(plain))] + "…",
		Hash:      hashAPIKey(plain),
		CreatedAt: time.Now().Unix(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, meta)
	if err = s.saveLocked(); err != nil {
		s.keys = s.keys[:len(s.keys)-1]
		return "", ConnectKey{}, err
	}
	meta.Hash = ""
	return plain, meta, nil
}

func (s *ConnectKeyStore) Delete(id string) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, k := range s.keys {
		if k.ID == id {
			s.keys = append(s.keys[:i], s.keys[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("key tidak ditemukan")
}

// Validate returns true if raw key matches a stored hash or env CONNECT_API_KEY.
func (s *ConnectKeyStore) Validate(raw string) (name string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if env := strings.TrimSpace(os.Getenv("CONNECT_API_KEY")); env != "" && subtleConstEq(raw, env) {
		return "env", true
	}
	if s == nil {
		return "", false
	}
	h := hashAPIKey(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.keys {
		if subtleConstEq(s.keys[i].Hash, h) {
			s.keys[i].LastUsed = time.Now().Unix()
			_ = s.saveLocked()
			return s.keys[i].Name, true
		}
	}
	return "", false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
