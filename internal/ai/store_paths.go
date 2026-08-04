package ai

import (
	"path/filepath"
	"sync"
)

// Workspace-scoped store paths for Gemini / OpenAI / quota files.
var (
	storeMu   sync.RWMutex
	storeRoot = ".data"
)

// ConfigureWorkspaceStore sets the directory for ai_keys.json, openai_keys.json, ai_quota.json.
// Call before NewFromEnv / NewThumbnailFromEnv.
func ConfigureWorkspaceStore(dir string) {
	storeMu.Lock()
	defer storeMu.Unlock()
	if dir == "" {
		dir = ".data"
	}
	storeRoot = dir
}

func workspaceStoreRoot() string {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return storeRoot
}

func geminiKeysFile() string {
	return filepath.Join(workspaceStoreRoot(), "ai_keys.json")
}

func openAIKeysFile() string {
	return filepath.Join(workspaceStoreRoot(), "openai_keys.json")
}

func quotaFile() string {
	return filepath.Join(workspaceStoreRoot(), "ai_quota.json")
}
