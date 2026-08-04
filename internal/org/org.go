// Package org models Tenant → Workspace → brand accounts.
//
// On disk (after migration):
//
//	.data/
//	  org.json                          // active tenant + workspace pointers
//	  tenants/{tenantID}/meta.json
//	  tenants/{tenantID}/workspaces/{workspaceID}/
//	    meta.json
//	    ai_keys.json / openai_keys.json / ai_quota.json
//	    accounts.json
//	    accounts/{accountID}/…
package org

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultTenantID    = "default"
	defaultWorkspaceID = "default"
)

type TenantBilling struct {
	Status string `json:"status"` // trial | active | past_due | suspended
	Plan   string `json:"plan,omitempty"`
	Notes  string `json:"notes,omitempty"`
	DueAt  string `json:"due_at,omitempty"`
}

type TenantMeta struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	CreatedAt string        `json:"created_at"`
	Billing   TenantBilling `json:"billing"`
}

type WorkspaceMeta struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type pointerFile struct {
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
}

// Context is the active tenant + workspace for this process.
type Context struct {
	Root          string
	Tenant        TenantMeta
	Workspace     WorkspaceMeta
	WorkspaceDir  string // absolute/relative path to workspace data root
	TenantDir     string
}

func (c *Context) AccountsDir() string {
	return filepath.Join(c.WorkspaceDir, "accounts")
}

var slugRe = regexp.MustCompile(`[^a-z0-9_-]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.TrimPrefix(s, "@")
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return defaultTenantID
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// Bootstrap opens (and migrates) the org hierarchy under dataRoot (usually ".data").
func Bootstrap(dataRoot string) (*Context, error) {
	if dataRoot == "" {
		dataRoot = ".data"
	}
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return nil, err
	}

	ptrPath := filepath.Join(dataRoot, "org.json")
	if _, err := os.Stat(ptrPath); err == nil {
		return load(dataRoot)
	}

	// Prefer AUTH_USER as tenant slug when present.
	tenantID := defaultTenantID
	tenantName := "Admin"
	if u := strings.TrimSpace(os.Getenv("AUTH_USER")); u != "" {
		tenantID = slugify(u)
		tenantName = u
	}
	wsID := defaultWorkspaceID
	wsName := "Main"

	if err := migrateLegacy(dataRoot, tenantID, tenantName, wsID, wsName); err != nil {
		return nil, fmt.Errorf("migrasi org: %w", err)
	}
	return load(dataRoot)
}

func load(dataRoot string) (*Context, error) {
	var ptr pointerFile
	b, err := os.ReadFile(filepath.Join(dataRoot, "org.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &ptr); err != nil {
		return nil, err
	}
	if ptr.TenantID == "" {
		ptr.TenantID = defaultTenantID
	}
	if ptr.WorkspaceID == "" {
		ptr.WorkspaceID = defaultWorkspaceID
	}

	tenantDir := filepath.Join(dataRoot, "tenants", ptr.TenantID)
	wsDir := filepath.Join(tenantDir, "workspaces", ptr.WorkspaceID)

	tenant, err := readTenantMeta(tenantDir, ptr.TenantID)
	if err != nil {
		return nil, err
	}
	ws, err := readWorkspaceMeta(wsDir, ptr.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &Context{
		Root:         dataRoot,
		Tenant:       tenant,
		Workspace:    ws,
		WorkspaceDir: wsDir,
		TenantDir:    tenantDir,
	}, nil
}

func readTenantMeta(dir, id string) (TenantMeta, error) {
	var m TenantMeta
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.ID == "" {
		m.ID = id
	}
	return m, nil
}

func readWorkspaceMeta(dir, id string) (WorkspaceMeta, error) {
	var m WorkspaceMeta
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.ID == "" {
		m.ID = id
	}
	return m, nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func migrateLegacy(dataRoot, tenantID, tenantName, wsID, wsName string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tenantDir := filepath.Join(dataRoot, "tenants", tenantID)
	wsDir := filepath.Join(tenantDir, "workspaces", wsID)
	if err := os.MkdirAll(wsDir, 0o700); err != nil {
		return err
	}

	tenant := TenantMeta{
		ID: tenantID, Name: tenantName, CreatedAt: now,
		Billing: TenantBilling{Status: "active", Plan: "default"},
	}
	ws := WorkspaceMeta{ID: wsID, Name: wsName, CreatedAt: now}
	if err := writeJSON(filepath.Join(tenantDir, "meta.json"), tenant); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(wsDir, "meta.json"), ws); err != nil {
		return err
	}

	// Move brand-account index + dirs.
	moveIfExists(filepath.Join(dataRoot, "accounts.json"), filepath.Join(wsDir, "accounts.json"))
	moveDirIfExists(filepath.Join(dataRoot, "accounts"), filepath.Join(wsDir, "accounts"))

	// Workspace-level AI keys / quota.
	for _, name := range []string{"ai_keys.json", "openai_keys.json", "ai_quota.json"} {
		moveIfExists(filepath.Join(dataRoot, name), filepath.Join(wsDir, name))
	}

	// Ensure accounts index exists (empty → create default main later via account.Open).
	if _, err := os.Stat(filepath.Join(wsDir, "accounts.json")); err != nil {
		_ = os.MkdirAll(filepath.Join(wsDir, "accounts"), 0o700)
	}

	ptr := pointerFile{TenantID: tenantID, WorkspaceID: wsID}
	if err := writeJSON(filepath.Join(dataRoot, "org.json"), ptr); err != nil {
		return err
	}

	log.Printf("org: migrasi → tenant=%s workspace=%s (%s)", tenantID, wsID, wsDir)
	return nil
}

func moveIfExists(src, dst string) {
	if _, err := os.Stat(src); err != nil {
		return
	}
	if _, err := os.Stat(dst); err == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o700)
	if err := os.Rename(src, dst); err != nil {
		b, rerr := os.ReadFile(src)
		if rerr != nil {
			return
		}
		if werr := os.WriteFile(dst, b, 0o600); werr == nil {
			_ = os.Remove(src)
		}
	}
}

func moveDirIfExists(src, dst string) {
	if _, err := os.Stat(src); err != nil {
		return
	}
	if _, err := os.Stat(dst); err == nil {
		return
	}
	if err := os.Rename(src, dst); err != nil {
		_ = copyDir(src, dst)
		_ = os.RemoveAll(src)
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_ = os.MkdirAll(filepath.Dir(target), 0o700)
		return os.WriteFile(target, b, info.Mode())
	})
}

func normalizeBilling(b TenantBilling) TenantBilling {
	b.Status = strings.ToLower(strings.TrimSpace(b.Status))
	switch b.Status {
	case "trial", "active", "past_due", "suspended":
	default:
		if b.Status == "" {
			b.Status = "trial"
		} else {
			b.Status = "active"
		}
	}
	b.Plan = strings.TrimSpace(b.Plan)
	b.Notes = strings.TrimSpace(b.Notes)
	b.DueAt = strings.TrimSpace(b.DueAt)
	return b
}

// ListTenants returns all tenant meta under dataRoot/tenants.
func ListTenants(dataRoot string) ([]TenantMeta, error) {
	if dataRoot == "" {
		dataRoot = ".data"
	}
	dir := filepath.Join(dataRoot, "tenants")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]TenantMeta, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := readTenantMeta(filepath.Join(dir, e.Name()), e.Name())
		if err != nil {
			continue
		}
		if m.Billing.Status == "" {
			m.Billing.Status = "active"
		}
		out = append(out, m)
	}
	return out, nil
}

// CreateTenant creates tenant + default workspace "Main".
func CreateTenant(dataRoot, id, name string, billing TenantBilling) (TenantMeta, error) {
	var zero TenantMeta
	if dataRoot == "" {
		dataRoot = ".data"
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return zero, fmt.Errorf("nama tenant wajib")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		id = slugify(name)
	} else {
		id = slugify(id)
	}
	tenantDir := filepath.Join(dataRoot, "tenants", id)
	if _, err := os.Stat(tenantDir); err == nil {
		return zero, fmt.Errorf("tenant %s sudah ada", id)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	wsID := defaultWorkspaceID
	wsDir := filepath.Join(tenantDir, "workspaces", wsID)
	if err := os.MkdirAll(filepath.Join(wsDir, "accounts"), 0o700); err != nil {
		return zero, err
	}
	billing = normalizeBilling(billing)
	if billing.Plan == "" {
		billing.Plan = "starter"
	}
	tenant := TenantMeta{ID: id, Name: name, CreatedAt: now, Billing: billing}
	ws := WorkspaceMeta{ID: wsID, Name: "Main", CreatedAt: now}
	if err := writeJSON(filepath.Join(tenantDir, "meta.json"), tenant); err != nil {
		return zero, err
	}
	if err := writeJSON(filepath.Join(wsDir, "meta.json"), ws); err != nil {
		return zero, err
	}
	// accounts.json dibuat otomatis oleh account.OpenAt (default akun main).
	return tenant, nil
}

// DeleteTenant removes a tenant directory (used to roll back failed provisioning).
func DeleteTenant(dataRoot, id string) error {
	if dataRoot == "" {
		dataRoot = ".data"
	}
	id = slugify(strings.TrimSpace(id))
	if id == "" || id == defaultTenantID {
		return fmt.Errorf("tenant tidak boleh dihapus")
	}
	dir := filepath.Join(dataRoot, "tenants", id)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("tenant tidak ditemukan")
	}
	return os.RemoveAll(dir)
}

// UpdateTenant mutates tenant meta.json.
func UpdateTenant(dataRoot, id string, mutate func(*TenantMeta) error) (TenantMeta, error) {
	var zero TenantMeta
	if dataRoot == "" {
		dataRoot = ".data"
	}
	id = slugify(strings.TrimSpace(id))
	tenantDir := filepath.Join(dataRoot, "tenants", id)
	m, err := readTenantMeta(tenantDir, id)
	if err != nil {
		return zero, fmt.Errorf("tenant tidak ditemukan")
	}
	if err := mutate(&m); err != nil {
		return zero, err
	}
	m.ID = id
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return zero, fmt.Errorf("nama tenant wajib")
	}
	m.Billing = normalizeBilling(m.Billing)
	if err := writeJSON(filepath.Join(tenantDir, "meta.json"), m); err != nil {
		return zero, err
	}
	return m, nil
}

// GetTenant loads one tenant meta.
func GetTenant(dataRoot, id string) (TenantMeta, error) {
	if dataRoot == "" {
		dataRoot = ".data"
	}
	id = slugify(strings.TrimSpace(id))
	return readTenantMeta(filepath.Join(dataRoot, "tenants", id), id)
}

// ListWorkspaces lists workspaces under a tenant.
func ListWorkspaces(dataRoot, tenantID string) ([]WorkspaceMeta, error) {
	if dataRoot == "" {
		dataRoot = ".data"
	}
	tenantID = slugify(strings.TrimSpace(tenantID))
	dir := filepath.Join(dataRoot, "tenants", tenantID, "workspaces")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]WorkspaceMeta, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := readWorkspaceMeta(filepath.Join(dir, e.Name()), e.Name())
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// SwitchActive writes org.json and returns the loaded Context.
func SwitchActive(dataRoot, tenantID, workspaceID string) (*Context, error) {
	if dataRoot == "" {
		dataRoot = ".data"
	}
	tenantID = slugify(strings.TrimSpace(tenantID))
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id wajib")
	}
	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = defaultWorkspaceID
	} else {
		workspaceID = slugify(workspaceID)
	}
	tenantDir := filepath.Join(dataRoot, "tenants", tenantID)
	wsDir := filepath.Join(tenantDir, "workspaces", workspaceID)
	if _, err := os.Stat(filepath.Join(tenantDir, "meta.json")); err != nil {
		return nil, fmt.Errorf("tenant tidak ditemukan")
	}
	if _, err := os.Stat(filepath.Join(wsDir, "meta.json")); err != nil {
		return nil, fmt.Errorf("workspace tidak ditemukan")
	}
	ptr := pointerFile{TenantID: tenantID, WorkspaceID: workspaceID}
	if err := writeJSON(filepath.Join(dataRoot, "org.json"), ptr); err != nil {
		return nil, err
	}
	return load(dataRoot)
}
