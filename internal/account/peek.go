package account

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"threads-dashboard/internal/buffer"
)

// PeekAccount is a lightweight connection snapshot (no live API calls).
type PeekAccount struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ThreadsUsername   string `json:"threads_username,omitempty"`
	InstagramUsername string `json:"instagram_username,omitempty"`
	Threads           bool   `json:"threads"`
	Instagram         bool   `json:"instagram"`
	Buffer            bool   `json:"buffer"`
}

// PeekWorkspace summarizes brand accounts + workspace-level API keys.
type PeekWorkspace struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	AccountCount int           `json:"account_count"`
	ThreadsN     int           `json:"threads_n"`
	InstagramN   int           `json:"instagram_n"`
	BufferN      int           `json:"buffer_n"`
	GeminiKeys   int           `json:"gemini_keys"`
	OpenAIKeys   int           `json:"openai_keys"`
	Accounts     []PeekAccount `json:"accounts"`
}

func fileNonEmpty(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) != ""
}

func countJSONKeys(path, field string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var raw map[string]any
	if json.Unmarshal(b, &raw) != nil {
		return 0
	}
	v, ok := raw[field]
	if !ok {
		// ai_keys.json often {"keys":["..."]} or {"api_keys":...}
		for _, alt := range []string{"keys", "api_keys", "items"} {
			if v, ok = raw[alt]; ok {
				break
			}
		}
	}
	switch t := v.(type) {
	case []any:
		n := 0
		for _, item := range t {
			switch s := item.(type) {
			case string:
				if strings.TrimSpace(s) != "" {
					n++
				}
			case map[string]any:
				if k, _ := s["key"].(string); strings.TrimSpace(k) != "" {
					n++
				} else if k, _ := s["api_key"].(string); strings.TrimSpace(k) != "" {
					n++
				}
			}
		}
		return n
	default:
		return 0
	}
}

// PeekWorkspaceDir reads accounts.json + token files under a workspace data dir.
func PeekWorkspaceDir(workspaceDir, id, name string) PeekWorkspace {
	out := PeekWorkspace{ID: id, Name: name, Accounts: []PeekAccount{}}
	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return out
	}

	out.GeminiKeys = countJSONKeys(filepath.Join(workspaceDir, "ai_keys.json"), "keys")
	out.OpenAIKeys = countJSONKeys(filepath.Join(workspaceDir, "openai_keys.json"), "keys")

	b, err := os.ReadFile(filepath.Join(workspaceDir, accountsFile))
	if err != nil {
		return out
	}
	var shape fileShape
	if json.Unmarshal(b, &shape) != nil {
		return out
	}
	for _, m := range shape.Accounts {
		dir := filepath.Join(workspaceDir, accountsSubdir, m.ID)
		row := PeekAccount{
			ID:                m.ID,
			Name:              m.Name,
			ThreadsUsername:   m.ThreadsUsername,
			InstagramUsername: m.InstagramUsername,
			Threads:           fileNonEmpty(filepath.Join(dir, "access_token")),
			Instagram:         fileNonEmpty(filepath.Join(dir, "ig_access_token")),
			Buffer:            buffer.HasStoredAPIKey(filepath.Join(dir, "buffer_key.json")),
		}
		if row.Threads {
			out.ThreadsN++
		}
		if row.Instagram {
			out.InstagramN++
		}
		if row.Buffer {
			out.BufferN++
		}
		out.Accounts = append(out.Accounts, row)
	}
	out.AccountCount = len(out.Accounts)
	return out
}
