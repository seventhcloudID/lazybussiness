package main

import (
	"fmt"
	"strings"

	"threads-dashboard/internal/account"
	"threads-dashboard/internal/ai"
	"threads-dashboard/internal/buffer"
	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/lazy"
	"threads-dashboard/internal/threads"
)

// Global account registry — HTTP handlers resolve the active workspace via helpers.
var accounts *account.Registry

func th() *threads.Client {
	ws := accounts.Active()
	if ws == nil {
		return nil
	}
	return ws.Threads
}

func igc() *instagram.Client {
	ws := accounts.Active()
	if ws == nil {
		return nil
	}
	return ws.IG
}

func buf() *buffer.Client {
	ws := accounts.Active()
	if ws == nil {
		return nil
	}
	return ws.Buffer
}

func mem() *ai.MemoryStore {
	ws := accounts.Active()
	if ws == nil {
		return nil
	}
	return ws.Memory
}

func lz() *lazy.Store {
	ws := accounts.Active()
	if ws == nil {
		return nil
	}
	return ws.Lazy
}

func lzs() *lazy.Scheduler {
	ws := accounts.Active()
	if ws == nil {
		return nil
	}
	return ws.Sched
}

func accountByParam(id string) (*account.Workspace, error) {
	id = strings.TrimSpace(id)
	if id == "" || id == "active" || id == "_" {
		ws := accounts.Active()
		if ws == nil {
			return nil, fmt.Errorf("tidak ada akun aktif")
		}
		return ws, nil
	}
	return accounts.Get(id)
}
