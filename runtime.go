package main

import (
	"context"
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
	"threads-dashboard/internal/repliz"
	"threads-dashboard/internal/threads"
)

// Global org + brand-account registry.
// Interim: one active tenant/workspace per process (admin "open tenant" / tenant login switches it).
var (
	orgCtx   *org.Context
	accounts *account.Registry
	runtimeMu sync.Mutex
	accountShared account.Shared
	replizCli    *repliz.Client
	replizActive *repliz.ActiveStore
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

// resolveReplizID memakai account_id request, lalu active_id di repliz.json, lalu ListAccounts.
func resolveReplizID(ctx context.Context, requested string) (string, error) {
	if replizCli == nil || !replizCli.Ready() {
		return "", fmt.Errorf("Repliz belum disambungkan — set REPLIZ_ACCESS_KEY dan REPLIZ_SECRET_KEY")
	}
	id := strings.TrimSpace(requested)
	if id == "" && replizActive != nil {
		id = replizActive.Get()
	}
	if id != "" {
		acc, err := replizCli.GetAccount(ctx, id)
		if err == nil && acc.AccountID() != "" {
			got := acc.AccountID()
			if replizActive != nil {
				_ = replizActive.Set(got)
			}
			return got, nil
		}
	}
	list, err := replizCli.ListAccounts(ctx)
	if err != nil {
		return "", err
	}
	if acc, ok := repliz.FindAccount(list, id); ok {
		got := acc.AccountID()
		if replizActive != nil {
			_ = replizActive.Set(got)
		}
		return got, nil
	}
	picked, err := repliz.PickConnected(list)
	if err != nil {
		return "", err
	}
	got := picked.AccountID()
	if replizActive != nil {
		_ = replizActive.Set(got)
	}
	return got, nil
}

func pickReplizByType(ctx context.Context, requested, wantType string) (repliz.Account, error) {
	if replizCli == nil || !replizCli.Ready() {
		return repliz.Account{}, fmt.Errorf("Repliz belum disambungkan")
	}
	wantType = strings.ToLower(strings.TrimSpace(wantType))
	list, err := replizCli.ListAccounts(ctx)
	if err != nil {
		return repliz.Account{}, err
	}
	id := strings.TrimSpace(requested)
	if id == "" && replizActive != nil {
		id = replizActive.Get()
	}
	if acc, ok := repliz.FindAccount(list, id); ok {
		if wantType == "" || strings.EqualFold(acc.Type, wantType) {
			return acc, nil
		}
	}
	if wantType == "" {
		return repliz.PickConnected(list)
	}
	var fallback repliz.Account
	for _, a := range list {
		if !strings.EqualFold(a.Type, wantType) {
			continue
		}
		if a.IsConnected {
			return a, nil
		}
		if fallback.AccountID() == "" {
			fallback = a
		}
	}
	if fallback.AccountID() != "" {
		return fallback, nil
	}
	return repliz.Account{}, fmt.Errorf("tidak ada akun Repliz %s", wantType)
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
	replizActive = repliz.NewActiveStore(ctx.WorkspaceDir)
	accounts.StartSchedulers()
	log.Printf("runtime: switch → tenant=%s workspace=%s (%d akun)", ctx.Tenant.ID, ctx.Workspace.ID, len(accounts.List()))
	return nil
}
