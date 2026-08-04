package main

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"threads-dashboard/internal/account"
	"threads-dashboard/internal/ai"
	"threads-dashboard/internal/buffer"
	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/lazy"
	"threads-dashboard/internal/org"
	"threads-dashboard/internal/threads"
)

// Global org + brand-account registry.
// Interim: one active tenant/workspace per process (admin "open tenant" / tenant login switches it).
var (
	orgCtx   *org.Context
	accounts *account.Registry
	runtimeMu sync.Mutex
	accountShared account.Shared
)

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

// switchRuntimeTenant reloads org pointer + accounts registry for a tenant/workspace.
func switchRuntimeTenant(tenantID, workspaceID string) error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	root := ".data"
	if orgCtx != nil && orgCtx.Root != "" {
		root = orgCtx.Root
	}
	ctx, err := org.SwitchActive(root, tenantID, workspaceID)
	if err != nil {
		return err
	}
	if accounts != nil {
		accounts.StopSchedulers()
	}
	reg, err := account.OpenAt(ctx.WorkspaceDir, accountShared)
	if err != nil {
		return err
	}
	ai.ConfigureWorkspaceStore(ctx.WorkspaceDir)
	orgCtx = ctx
	accounts = reg
	accounts.StartSchedulers()
	log.Printf("runtime: switch → tenant=%s workspace=%s (%d akun)", ctx.Tenant.ID, ctx.Workspace.ID, len(accounts.List()))
	return nil
}
