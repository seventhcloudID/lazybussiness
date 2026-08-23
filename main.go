package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"threads-dashboard/internal/account"
	"threads-dashboard/internal/ai"
	"threads-dashboard/internal/auth"
	"threads-dashboard/internal/buffer"
	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/lazy"
	"threads-dashboard/internal/oauth"
	"threads-dashboard/internal/org"
	"threads-dashboard/internal/repliz"
	"threads-dashboard/internal/schedule"
	"threads-dashboard/internal/threads"
)

func main() {
	loadDotEnv(".env")

	addr := env("PORT", ":8080")
	if addr != "" && !strings.HasPrefix(addr, ":") && !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	var err error
	orgCtx, err = org.Bootstrap(".data")
	if err != nil {
		log.Fatal("org: ", err)
	}
	ai.ConfigureWorkspaceStore(orgCtx.WorkspaceDir)
	log.Printf("tenant=%s workspace=%s", orgCtx.Tenant.ID, orgCtx.Workspace.ID)

	aiClient := ai.NewFromEnv()
	thumbClient := ai.NewThumbnailFromEnv()
	replizCli = repliz.NewFromEnv()
	if aiClient.Enabled() {
		log.Printf("AI insight siap (%s / %s, chat %s, %d key)", aiClient.Provider(), aiClient.Model(), aiClient.ChatModel(), aiClient.KeyCount())
	} else {
		log.Println("AI insight nonaktif — set AI_API_KEY di .env atau API keys workspace")
	}
	if thumbClient.Enabled() {
		log.Printf("Thumbnail siap (%s @ %s)", thumbClient.Model(), os.Getenv("AI_BASE_URL"))
	} else {
		log.Println("Thumbnail nonaktif — set AI_API_KEY + OPENAI_IMAGE_MODEL (default cx/gpt-5.5-image)")
	}
	if replizCli.Ready() {
		log.Println("Repliz siap — insight memakai API Repliz")
	} else {
		log.Println("Repliz belum diset — set REPLIZ_ACCESS_KEY dan REPLIZ_SECRET_KEY")
	}

	publicBase := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")
	accountShared = account.Shared{
		AI: aiClient, Thumb: thumbClient, Public: publicBase, Publisher: replizPub{},
	}

	accounts, err = account.OpenAt(orgCtx.WorkspaceDir, accountShared)
	if err != nil {
		log.Fatal("accounts: ", err)
	}
	replizActive = repliz.NewActiveStore(orgCtx.WorkspaceDir)

	if b := buf(); b != nil && b.Enabled() {
		log.Printf("Buffer akun aktif (%s) siap — TikTok Notify Me + X shareNow", accounts.ActiveID())
	} else {
		log.Println("Buffer akun aktif belum ada key — isi di Akun → Kelola")
	}
	if tz := strings.TrimSpace(os.Getenv("LAZY_TIMEZONE")); tz != "" {
		cfg := lz().GetConfig()
		if cfg.Timezone == "Asia/Jakarta" || cfg.Timezone == "" {
			cfg.Timezone = tz
			_, _ = lz().SetConfig(cfg)
		}
	}
	accounts.StartSchedulers()
	log.Printf("akun aktif: %s (%d workspace lokal; identitas dashboard = Repliz)", accounts.ActiveID(), len(accounts.List()))

	users, err := auth.OpenUsers(".data")
	if err != nil {
		log.Fatal("users: ", err)
	}
	if seeded, serr := users.SeedAdminFromEnv(); serr != nil {
		log.Fatal("users seed: ", serr)
	} else if seeded {
		log.Println("users: admin di-seed dari AUTH_USER/AUTH_PASSWORD → .data/users.json")
	}

	gate := auth.NewFromEnv()
	gate.SetUsers(users)
	connectKeys, err := auth.OpenConnectKeys(orgCtx.WorkspaceDir)
	if err != nil {
		log.Fatal("connect keys: ", err)
	}
	gate.SetConnectKeys(connectKeys)
	if gate.Enabled() {
		log.Printf("login aktif (%d user)", users.Count())
	} else {
		log.Println("login NONAKTIF — set AUTH_USER + AUTH_PASSWORD di .env (seed admin) atau isi .data/users.json")
	}
	if strings.TrimSpace(os.Getenv("CONNECT_API_KEY")) != "" {
		log.Println("CONNECT_API_KEY aktif (Bearer OpenAPI)")
	}
	if n := len(connectKeys.List()); n > 0 {
		log.Printf("connect API keys: %d", n)
	}

	oa := oauth.FromEnv()
	if oa.ThreadsReady() {
		log.Printf("oauth threads → %s", oa.ThreadsRedirectURI())
	} else {
		log.Println("oauth threads off — set THREADS_APP_ID + THREADS_APP_SECRET + PUBLIC_BASE_URL")
	}
	if oa.InstagramReady() {
		log.Printf("oauth instagram → %s", oa.InstagramRedirectURI())
	} else {
		log.Println("oauth instagram off — set INSTAGRAM_APP_ID + INSTAGRAM_APP_SECRET + PUBLIC_BASE_URL")
	}

	mux := http.NewServeMux()
	serveLanding := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("web", "index.html"))
	}
	mux.HandleFunc("GET /{$}", serveLanding)
	mux.HandleFunc("GET /index.html", serveLanding)
	servePrivacy := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("web", "privacy.html"))
	}
	mux.HandleFunc("GET /privacy", servePrivacy)
	mux.HandleFunc("GET /privacy.html", servePrivacy)
	mux.HandleFunc("GET /privacy-policy", servePrivacy)
	mux.HandleFunc("GET /privacy-policy.html", servePrivacy)
	serveOpenAPI := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=120")
		http.ServeFile(w, r, filepath.Join("openapi", "openapi.yaml"))
	}
	mux.HandleFunc("GET /openapi", serveOpenAPI)
	mux.HandleFunc("GET /openapi.yaml", serveOpenAPI)
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/openapi.yaml", http.StatusFound)
	})
	serveAPIDocs := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("web", "docs.html"))
	}
	mux.HandleFunc("GET /docs", serveAPIDocs)
	mux.HandleFunc("GET /api-docs", serveAPIDocs)
	mux.HandleFunc("GET /docs.html", serveAPIDocs)
	mux.HandleFunc("GET /app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/ringkasan", http.StatusFound)
	})
	mux.HandleFunc("GET /app/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/ringkasan", http.StatusFound)
	})
	mux.HandleFunc("GET /auth/repliz/{platform}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("web", "akun.html"))
	})
	mux.HandleFunc("GET /auth/repliz/{platform}/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("web", "akun.html"))
	})
	mux.HandleFunc("GET /core", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/core/", http.StatusFound)
	})
	mux.HandleFunc("GET /admin.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/core/", http.StatusFound)
	})
	mux.HandleFunc("GET /login.html", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		target := "/app/login"
		if q != "" {
			target += "?" + q
		}
		http.Redirect(w, r, target, http.StatusFound)
	})
	// Customer dashboard — serve /app/<page> from web/<page>.html (no .html in URL)
	mux.HandleFunc("GET /app/{page...}", func(w http.ResponseWriter, r *http.Request) {
		page := r.PathValue("page")
		// Strip trailing slash
		page = strings.TrimSuffix(page, "/")
		if page == "" {
			http.Redirect(w, r, "/app/ringkasan", http.StatusFound)
			return
		}
		// If URL already ends in .html, strip it and redirect
		if strings.HasSuffix(page, ".html") {
			http.Redirect(w, r, "/app/"+strings.TrimSuffix(page, ".html"), http.StatusMovedPermanently)
			return
		}
		// Static assets (css, js, images inside /app/) — pass through
		if strings.Contains(page, ".") {
			http.ServeFile(w, r, "web/"+page)
			return
		}
		if page == "token" || page == "ig-token" {
			http.Redirect(w, r, "/app/akun", http.StatusFound)
			return
		}
		if page == "rapat" || page == "gambar" {
			http.Redirect(w, r, "/app/generate", http.StatusFound)
			return
		}
		// Serve the corresponding .html file
		http.ServeFile(w, r, "web/"+page+".html")
	})
	// Operator console (separate tree from customer dashboard)
	mux.Handle("GET /core/", http.StripPrefix("/core/", http.FileServer(http.Dir("core"))))
	// Shared assets (absolute /css /js used by both portals)
	mux.Handle("GET /css/", http.StripPrefix("/css/", http.FileServer(http.Dir("web/css"))))
	mux.Handle("GET /js/", http.StripPrefix("/js/", http.FileServer(http.Dir("web/js"))))
	// Legacy bookmarks: /ringkasan → /app/ringkasan
	mux.HandleFunc("GET /{page}", func(w http.ResponseWriter, r *http.Request) {
		page := r.PathValue("page")
		if strings.Contains(page, "/") {
			http.NotFound(w, r)
			return
		}
		// Strip .html if present
		page = strings.TrimSuffix(page, ".html")
		http.Redirect(w, r, "/app/"+page, http.StatusFound)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		thAcc, igAcc, thOK, igOK := replizConnectedPair(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"connected":    thOK,
			"ig_connected": igOK,
			"tenant_id":    orgCtx.Tenant.ID,
			"workspace_id": orgCtx.Workspace.ID,
			"account_id":   thAcc.AccountID(),
			"ig_user_id":   igAcc.AccountID(),
			"ai":           aiClient.Enabled(),
			"auth":         gate.Enabled(),
			"repliz":       replizCli != nil && replizCli.Ready(),
		})
	})
	mux.HandleFunc("GET /api/org", func(w http.ResponseWriter, r *http.Request) {
		list := accounts.List()
		writeJSON(w, http.StatusOK, map[string]any{
			"tenant": map[string]any{
				"id":   orgCtx.Tenant.ID,
				"name": orgCtx.Tenant.Name,
			},
			"workspace": map[string]any{
				"id":   orgCtx.Workspace.ID,
				"name": orgCtx.Workspace.Name,
			},
			"active_account_id": accounts.ActiveID(),
			"account_count":     len(list),
			"accounts":          list,
		})
	})

	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		sess := gate.SessionFromRequest(r)
		authed := sess != nil && (!gate.Enabled() || gate.Valid(r))
		out := map[string]any{
			"enabled":            gate.Enabled(),
			"authenticated":      authed,
			"active_tenant_id":   orgCtx.Tenant.ID,
			"active_tenant_name": orgCtx.Tenant.Name,
		}
		if sess != nil && authed {
			out["username"] = sess.Username
			out["role"] = sess.Role
			out["tenant_id"] = sess.TenantID
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if !gate.Enabled() {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "auth": false})
			return
		}
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		u, err := gate.Authenticate(body.Username, body.Password)
		if err != nil || u == nil {
			writeErr(w, http.StatusUnauthorized, "username/password salah")
			return
		}
		if u.Role == auth.RoleTenant && u.TenantID != "" {
			if t, terr := org.GetTenant(".data", u.TenantID); terr == nil {
				if t.Billing.Status == "suspended" {
					writeErr(w, http.StatusForbidden, "tenant ditangguhkan — hubungi admin")
					return
				}
			}
			if err := switchRuntimeTenant(u.TenantID, ""); err != nil {
				writeErr(w, http.StatusBadGateway, "gagal buka tenant: "+err.Error())
				return
			}
		}
		gate.IssueSession(w, u)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"username":  u.Username,
			"role":      u.Role,
			"tenant_id": u.TenantID,
		})
	})
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		gate.ClearCookie(w)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	requireAdmin := func(w http.ResponseWriter, r *http.Request) *auth.Session {
		sess := gate.SessionFromRequest(r)
		if sess == nil || !sess.IsAdmin() {
			writeErr(w, http.StatusForbidden, "admin only")
			return nil
		}
		return sess
	}

	mux.HandleFunc("GET /api/admin/tenants", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		list, err := org.ListTenants(".data")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		items := make([]map[string]any, 0, len(list))
		for _, t := range list {
			wsMeta, _ := org.ListWorkspaces(".data", t.ID)
			workspaces := make([]map[string]any, 0, len(wsMeta))
			var accountsN, threadsN, igN, bufN, geminiN, openaiN int
			for _, ws := range wsMeta {
				dir := filepath.Join(".data", "tenants", t.ID, "workspaces", ws.ID)
				peek := account.PeekWorkspaceDir(dir, ws.ID, ws.Name)
				workspaces = append(workspaces, map[string]any{
					"id":            peek.ID,
					"name":          peek.Name,
					"account_count": peek.AccountCount,
					"threads_n":     peek.ThreadsN,
					"instagram_n":   peek.InstagramN,
					"buffer_n":      peek.BufferN,
					"gemini_keys":   peek.GeminiKeys,
					"openai_keys":   peek.OpenAIKeys,
					"accounts":      peek.Accounts,
				})
				accountsN += peek.AccountCount
				threadsN += peek.ThreadsN
				igN += peek.InstagramN
				bufN += peek.BufferN
				geminiN += peek.GeminiKeys
				openaiN += peek.OpenAIKeys
			}
			items = append(items, map[string]any{
				"id":         t.ID,
				"name":       t.Name,
				"created_at": t.CreatedAt,
				"billing":    t.Billing,
				"workspaces": workspaces,
				"active":     t.ID == orgCtx.Tenant.ID,
				"connect": map[string]any{
					"accounts":  accountsN,
					"threads":   threadsN,
					"instagram": igN,
					"buffer":    bufN,
					"gemini":    geminiN,
					"openai":    openaiN,
				},
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tenants":             items,
			"active_tenant_id":    orgCtx.Tenant.ID,
			"active_workspace_id": orgCtx.Workspace.ID,
		})
	})
	mux.HandleFunc("POST /api/admin/tenants", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		var body struct {
			ID       string            `json:"id"`
			Name     string            `json:"name"`
			Billing  org.TenantBilling `json:"billing"`
			Username string            `json:"username"`
			Password string            `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		body.Password = strings.TrimSpace(body.Password)
		if body.Username == "" || body.Password == "" {
			writeErr(w, http.StatusBadRequest, "username + password login wajib (supaya customer bisa masuk /app)")
			return
		}
		t, err := org.CreateTenant(".data", body.ID, body.Name, body.Billing)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		u, err := users.Create(body.Username, body.Password, auth.RoleTenant, t.ID)
		if err != nil {
			_ = org.DeleteTenant(".data", t.ID)
			writeErr(w, http.StatusBadRequest, "tenant dibatalkan: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tenant": t, "user": u})
	})
	mux.HandleFunc("PATCH /api/admin/tenants/{id}", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		id := r.PathValue("id")
		var body struct {
			Name    *string            `json:"name"`
			Billing *org.TenantBilling `json:"billing"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		t, err := org.UpdateTenant(".data", id, func(m *org.TenantMeta) error {
			if body.Name != nil {
				m.Name = *body.Name
			}
			if body.Billing != nil {
				m.Billing = *body.Billing
			}
			return nil
		})
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tenant": t})
	})
	mux.HandleFunc("POST /api/admin/tenants/{id}/open", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		id := r.PathValue("id")
		var body struct {
			WorkspaceID string `json:"workspace_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := switchRuntimeTenant(id, body.WorkspaceID); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"tenant_id":    orgCtx.Tenant.ID,
			"tenant_name":  orgCtx.Tenant.Name,
			"workspace_id": orgCtx.Workspace.ID,
			"accounts":     len(accounts.List()),
		})
	})

	mux.HandleFunc("GET /api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users.List()})
	})
	mux.HandleFunc("POST /api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
			TenantID string `json:"tenant_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		wasEmpty := users.Count() == 0
		role := strings.TrimSpace(body.Role)
		if wasEmpty && role != auth.RoleAdmin {
			writeErr(w, http.StatusBadRequest, "user pertama harus role admin (atau seed lewat AUTH_USER/AUTH_PASSWORD)")
			return
		}
		u, err := users.Create(body.Username, body.Password, body.Role, body.TenantID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		// Creating the first user turns auth on — keep this browser session logged in.
		if wasEmpty && u.Role == auth.RoleAdmin {
			gate.IssueSession(w, u)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": u})
	})
	mux.HandleFunc("PATCH /api/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		id := r.PathValue("id")
		var body struct {
			Active   *bool         `json:"active"`
			Role     *string       `json:"role"`
			TenantID *string       `json:"tenant_id"`
			Billing  *auth.Billing `json:"billing"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		u, err := users.Update(id, func(u *auth.User) error {
			if body.Active != nil {
				u.Active = *body.Active
			}
			if body.Role != nil {
				role := strings.TrimSpace(*body.Role)
				if role != auth.RoleAdmin && role != auth.RoleTenant {
					return fmt.Errorf("role tidak valid")
				}
				u.Role = role
			}
			if body.TenantID != nil {
				u.TenantID = strings.TrimSpace(*body.TenantID)
			}
			if body.Billing != nil {
				u.Billing = *body.Billing
			}
			if u.Role == auth.RoleTenant && u.TenantID == "" {
				return fmt.Errorf("tenant_id wajib untuk role tenant")
			}
			return nil
		})
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": u})
	})
	mux.HandleFunc("POST /api/admin/users/{id}/password", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		id := r.PathValue("id")
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if err := users.SetPassword(id, body.Password); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /api/admin/pricing", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		p, err := org.LoadPricing(".data")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"pricing": p})
	})
	mux.HandleFunc("PUT /api/admin/pricing", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		var body org.Pricing
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		p, err := org.SavePricing(".data", body)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pricing": p})
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		thAcc, igAcc, thOK, igOK := replizConnectedPair(r.Context())
		rzID := ""
		if replizActive != nil {
			rzID = replizActive.Get()
		}
		if rzID == "" {
			rzID = thAcc.AccountID()
		}
		name := strings.TrimSpace(thAcc.Username)
		if name == "" {
			name = strings.TrimSpace(thAcc.Name)
		}
		if name == "" {
			name = strings.TrimSpace(igAcc.Username)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"connected":         thOK,
			"user_id":           thAcc.AccountID(),
			"ig_connected":      igOK,
			"ig_user_id":        igAcc.AccountID(),
			"tenant_id":         orgCtx.Tenant.ID,
			"tenant_name":       orgCtx.Tenant.Name,
			"workspace_id":      orgCtx.Workspace.ID,
			"workspace_name":    orgCtx.Workspace.Name,
			"account_id":        rzID,
			"account_name":      name,
			"repliz":            replizCli != nil && replizCli.Ready(),
			"repliz_account_id": rzID,
			"source":            "repliz",
		})
	})

	mux.HandleFunc("GET /api/accounts", func(w http.ResponseWriter, r *http.Request) {
		list := accounts.List()
		replizAccounts := []repliz.Account{}
		if replizCli != nil && replizCli.Ready() {
			replizAccounts, _ = replizCli.ListAccounts(r.Context())
		}
		items := make([]map[string]any, 0, len(list))
		for _, m := range list {
			ws, err := accounts.Get(m.ID)
			row := map[string]any{
				"id":                  m.ID,
				"name":                m.Name,
				"threads_username":    m.ThreadsUsername,
				"instagram_username":  m.InstagramUsername,
				"tiktok_username":     m.TikTokUsername,
				"repliz_threads_id":   m.ReplizThreadsID,
				"repliz_instagram_id": m.ReplizInstagramID,
				"repliz_tiktok_id":    m.ReplizTikTokID,
				"active":              m.ID == accounts.ActiveID(),
				"threads_connected":   false,
				"instagram_connected": false,
				"lazy_enabled":        false,
			}
			if err == nil && ws != nil {
				row["threads_connected"] = ws.Threads.Connected()
				row["instagram_connected"] = ws.IG.Connected()
				row["lazy_enabled"] = ws.Lazy.GetConfig().Enabled
				row["buffer_enabled"] = ws.Buffer != nil && ws.Buffer.Enabled()
			}
			items = append(items, row)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"active_id":       accounts.ActiveID(),
			"accounts":        items,
			"repliz_accounts": replizAccounts,
			"identity":        "workspace",
			"note":            "Satu workspace mengikat akun Threads, Instagram, dan TikTok dari Repliz.",
		})
	})

	mux.HandleFunc("POST /api/accounts", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		ws, err := accounts.Create(body.Name)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "account": ws.Meta})
	})

	mux.HandleFunc("POST /api/accounts/switch", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
			writeErr(w, http.StatusBadRequest, "id wajib")
			return
		}
		if err := accounts.Switch(body.ID); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		th().InvalidateReplyCaches()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"active_id":    accounts.ActiveID(),
			"account":      accounts.Active().Meta,
			"connected":    false,
			"ig_connected": false,
			"note":         "Workspace aktif dipakai oleh Generate, Insight, Balasan, dan Lazy Business.",
		})
	})

	mux.HandleFunc("PATCH /api/accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Name              string  `json:"name"`
			ReplizThreadsID   *string `json:"repliz_threads_id"`
			ReplizInstagramID *string `json:"repliz_instagram_id"`
			ReplizTikTokID    *string `json:"repliz_tiktok_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		var threadsName, instagramName, tiktokName string
		bindings := []struct {
			value    *string
			platform string
			username *string
		}{
			{body.ReplizThreadsID, "threads", &threadsName},
			{body.ReplizInstagramID, "instagram", &instagramName},
			{body.ReplizTikTokID, "tiktok", &tiktokName},
		}
		for _, binding := range bindings {
			if binding.value == nil || strings.TrimSpace(*binding.value) == "" {
				continue
			}
			acc, err := replizAccountForID(r.Context(), *binding.value, binding.platform)
			if err != nil || !replizAccountLive(acc) {
				if err == nil {
					err = fmt.Errorf("akun %s belum terhubung", binding.platform)
				}
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			*binding.username = strings.TrimSpace(acc.Username)
		}
		for _, other := range accounts.List() {
			if other.ID == id {
				continue
			}
			checks := []struct {
				value *string
				used  string
				label string
			}{
				{body.ReplizThreadsID, other.ReplizThreadsID, "Threads"},
				{body.ReplizInstagramID, other.ReplizInstagramID, "Instagram"},
				{body.ReplizTikTokID, other.ReplizTikTokID, "TikTok"},
			}
			for _, check := range checks {
				if check.value != nil && strings.TrimSpace(*check.value) != "" && strings.TrimSpace(*check.value) == strings.TrimSpace(check.used) {
					writeErr(w, http.StatusConflict, fmt.Sprintf("akun %s itu sudah dipakai workspace %s", check.label, other.Name))
					return
				}
			}
		}
		if err := accounts.UpdateMeta(id, func(m *account.Meta) {
			if strings.TrimSpace(body.Name) != "" {
				m.Name = strings.TrimSpace(body.Name)
			}
			if body.ReplizThreadsID != nil {
				m.ReplizThreadsID = strings.TrimSpace(*body.ReplizThreadsID)
				m.ThreadsUsername = threadsName
			}
			if body.ReplizInstagramID != nil {
				m.ReplizInstagramID = strings.TrimSpace(*body.ReplizInstagramID)
				m.InstagramUsername = instagramName
			}
			if body.ReplizTikTokID != nil {
				m.ReplizTikTokID = strings.TrimSpace(*body.ReplizTikTokID)
				m.TikTokUsername = tiktokName
			}
		}); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		ws, _ := accounts.Get(id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "account": ws.Meta})
	})

	mux.HandleFunc("DELETE /api/accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := accounts.Delete(r.PathValue("id")); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active_id": accounts.ActiveID()})
	})

	// Set tokens on a specific account (or active if id empty / "active").
	mux.HandleFunc("POST /api/accounts/{id}/threads-token", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Threads Meta tidak dipakai. Sambungkan akun lewat Repliz di /app/akun.")
	})

	mux.HandleFunc("POST /api/accounts/{id}/ig-token", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Instagram Meta tidak dipakai. Sambungkan akun lewat Repliz di /app/akun.")
	})

	mux.HandleFunc("DELETE /api/accounts/{id}/threads-token", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Threads Meta tidak dipakai. Kelola akun di /app/akun.")
	})

	mux.HandleFunc("DELETE /api/accounts/{id}/ig-token", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Instagram Meta tidak dipakai. Kelola akun di /app/akun.")
	})

	mux.HandleFunc("GET /api/oauth/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, oa.Status())
	})
	mux.HandleFunc("GET /api/connect/keys", func(w http.ResponseWriter, r *http.Request) {
		base := strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/")
		writeJSON(w, http.StatusOK, map[string]any{
			"keys":        gate.ConnectKeys().List(),
			"docs_url":    base + "/docs",
			"openapi_url": base + "/openapi.yaml",
			"env_key_set": strings.TrimSpace(os.Getenv("CONNECT_API_KEY")) != "",
			"auth_header": "Authorization: Bearer <key>",
		})
	})
	mux.HandleFunc("POST /api/connect/keys", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		plain, meta, err := gate.ConnectKeys().Create(body.Name)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"key":     plain,
			"meta":    meta,
			"note":    "Simpan key ini sekarang — tidak ditampilkan ulang.",
			"openapi": strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/") + "/openapi.yaml",
		})
	})
	mux.HandleFunc("DELETE /api/connect/keys", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
			writeErr(w, http.StatusBadRequest, "id wajib")
			return
		}
		if err := gate.ConnectKeys().Delete(body.ID); err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /api/oauth/threads/start", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "OAuth Meta tidak dipakai. Sambungkan Threads lewat Repliz di /app/akun.")
	})
	mux.HandleFunc("GET /api/oauth/instagram/start", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "OAuth Meta tidak dipakai. Sambungkan Instagram lewat Repliz di /app/akun.")
	})
	mux.HandleFunc("GET /auth/threads/callback", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthCallback(w, r, oa, "threads")
	})
	mux.HandleFunc("GET /auth/instagram/callback", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthCallback(w, r, oa, "instagram")
	})
	// Meta App Dashboard — Deauthorize + Data Deletion (wajib diisi di form Threads/IG)
	mux.HandleFunc("POST /auth/meta/deauthorize", func(w http.ResponseWriter, r *http.Request) {
		handleMetaDeauthorize(w, r, oa)
	})
	mux.HandleFunc("GET /auth/meta/deauthorize", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Meta deauthorize callback OK"))
	})
	mux.HandleFunc("POST /auth/meta/data-deletion", func(w http.ResponseWriter, r *http.Request) {
		handleMetaDataDeletion(w, r, oa)
	})
	mux.HandleFunc("GET /auth/meta/data-deletion", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Meta data deletion callback OK"))
	})
	mux.HandleFunc("GET /auth/meta/data-deletion-status", handleMetaDataDeletionStatus)

	mux.HandleFunc("POST /api/token", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Meta tidak dipakai. Sambungkan akun lewat Repliz di /app/akun.")
	})

	mux.HandleFunc("POST /api/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Meta tidak dipakai. Sambungkan akun lewat Repliz di /app/akun.")
	})

	mux.HandleFunc("DELETE /api/token", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Meta tidak dipakai. Kelola akun di /app/akun.")
	})

	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		acc, err := pickReplizByType(r.Context(), r.URL.Query().Get("account_id"), "threads")
		if err != nil {
			acc, err = pickReplizByType(r.Context(), r.URL.Query().Get("account_id"), "")
		}
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":                          acc.AccountID(),
			"username":                    acc.Username,
			"name":                        acc.Name,
			"threads_profile_picture_url": acc.Picture,
			"is_connected":                acc.IsConnected,
			"type":                        acc.Type,
			"source":                      "repliz",
		})
	})

	mux.HandleFunc("GET /api/threads", func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.URL.Query().Get("source"), "meta") {
			writeGoneMeta(w, "Feed Threads memakai Repliz, bukan Meta Graph.")
			return
		}
		accountID, err := resolveReplizID(r.Context(), r.URL.Query().Get("account_id"))
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		limit := 40
		if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		raw, err := replizCli.Feed(r.Context(), r.URL.Query().Get("since"), r.URL.Query().Get("until"), accountID, limit)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})

	mux.HandleFunc("GET /api/repliz/accounts", func(w http.ResponseWriter, r *http.Request) {
		if !replizCli.Ready() {
			writeErr(w, http.StatusBadRequest, "Repliz belum disambungkan")
			return
		}
		list, err := replizCli.ListAccounts(r.Context())
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		activeID := strings.TrimSpace(r.URL.Query().Get("account_id"))
		if activeID == "" {
			activeID = replizActive.Get()
		}
		if _, ok := repliz.FindAccount(list, activeID); !ok {
			picked, err := repliz.PickConnected(list)
			if err != nil {
				writeAPIErr(w, err)
				return
			}
			activeID = picked.AccountID()
			_ = replizActive.Set(activeID)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"accounts":  list,
			"active_id": activeID,
		})
	})

	mux.HandleFunc("POST /api/repliz/accounts/switch", func(w http.ResponseWriter, r *http.Request) {
		if !replizCli.Ready() {
			writeErr(w, http.StatusBadRequest, "Repliz belum disambungkan")
			return
		}
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
			writeErr(w, http.StatusBadRequest, "id akun Repliz wajib")
			return
		}
		acc, err := replizCli.GetAccount(r.Context(), body.ID)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		id := acc.AccountID()
		if err := replizActive.Set(id); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active_id": id, "account": acc})
	})

	mux.HandleFunc("GET /api/repliz/authorize", func(w http.ResponseWriter, r *http.Request) {
		if replizCli == nil || !replizCli.Ready() {
			writeErr(w, http.StatusBadRequest, "Repliz belum disambungkan")
			return
		}
		platform := r.URL.Query().Get("platform")
		redirect := r.URL.Query().Get("redirect")
		if !oauthRedirectOK(r, redirect) {
			writeErr(w, http.StatusBadRequest, "Redirect OAuth harus ke aplikasi ini")
			return
		}
		authURL, err := replizCli.AuthorizeURL(r.Context(), platform, redirect)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"url": authURL, "platform": platform})
	})

	mux.HandleFunc("POST /api/repliz/connect", func(w http.ResponseWriter, r *http.Request) {
		if replizCli == nil || !replizCli.Ready() {
			writeErr(w, http.StatusBadRequest, "Repliz belum disambungkan")
			return
		}
		var in struct {
			Platform  string `json:"platform"`
			Code      string `json:"code"`
			AccountID string `json:"account_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeErr(w, http.StatusBadRequest, "JSON tidak valid")
			return
		}
		id, err := replizCli.ConnectCode(r.Context(), in.Platform, in.Code, in.AccountID)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		if id != "" && replizActive != nil {
			_ = replizActive.Set(id)
		}
		list, err := replizCli.ListAccounts(r.Context())
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active_id": id, "accounts": list})
	})

	mux.HandleFunc("DELETE /api/repliz/accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if replizCli == nil || !replizCli.Ready() {
			writeErr(w, http.StatusBadRequest, "Repliz belum disambungkan")
			return
		}
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			writeErr(w, http.StatusBadRequest, "account id kosong")
			return
		}
		if err := replizCli.DeleteAccount(r.Context(), id); err != nil {
			writeAPIErr(w, err)
			return
		}
		if replizActive != nil && replizActive.Get() == id {
			_ = replizActive.Set("")
		}
		list, err := replizCli.ListAccounts(r.Context())
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		activeID := ""
		if replizActive != nil {
			activeID = replizActive.Get()
		}
		if _, ok := repliz.FindAccount(list, activeID); !ok {
			if picked, err := repliz.PickConnected(list); err == nil {
				activeID = picked.AccountID()
				_ = replizActive.Set(activeID)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active_id": activeID, "accounts": list})
	})

	mux.HandleFunc("GET /api/insights", func(w http.ResponseWriter, r *http.Request) {
		postLimit := -1
		if v := strings.TrimSpace(r.URL.Query().Get("posts")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				postLimit = n
			}
		}
		accountID, err := resolveReplizID(r.Context(), r.URL.Query().Get("account_id"))
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		raw, err := replizCli.Insights(r.Context(), r.URL.Query().Get("since"), r.URL.Query().Get("until"), postLimit, accountID)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})

	mux.HandleFunc("POST /api/insights/ai", func(w http.ResponseWriter, r *http.Request) {
		if replizCli == nil || !replizCli.Ready() {
			writeErr(w, http.StatusBadRequest, "AI Insight memakai API Repliz — set REPLIZ_ACCESS_KEY")
			return
		}
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		var body struct {
			AccountID string `json:"account_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		accountID, err := resolveReplizID(r.Context(), body.AccountID)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		snapshot, err := replizCli.SnapshotForAI(r.Context(), accountID, 24)
		if err != nil {
			log.Printf("ai insight snapshot: %v", err)
			writeAPIErr(w, err)
			return
		}
		if accountID != "" && replizActive != nil {
			_ = replizActive.Set(accountID)
		}
		if mstore := mem(); mstore != nil {
			m := mstore.Get()
			prof, _ := snapshot["profile"].(map[string]any)
			uname, _ := prof["username"].(string)
			name, _ := prof["name"].(string)
			if m.FitsAccount(uname, name) {
				instr := m.Instructions
				if rns := []rune(instr); len(rns) > 1200 {
					instr = string(rns[:1200]) + "…"
				}
				snapshot["brand_context"] = map[string]any{
					"brand":        m.Brand,
					"niche":        m.Niche,
					"niches":       m.Niches,
					"instructions": instr,
					"lessons":      m.Lessons,
				}
			}
		}
		result, err := aiClient.AnalyzeThreads(snapshot)
		if err != nil {
			log.Printf("ai insight analyze: %v", err)
			var qe *ai.QuotaError
			if errors.As(err, &qe) {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error": qe.Message,
					"kind":  qe.Kind,
					"quota": aiClient.Quota(),
				})
				return
			}
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		out := map[string]any{
			"insight":        result,
			"repliz_account": snapshot["profile"],
			"source":         "repliz",
		}
		// flatten InsightResult fields for existing UI
		b, _ := json.Marshal(result)
		_ = json.Unmarshal(b, &out)
		out["repliz_account"] = snapshot["profile"]
		out["source"] = "repliz"
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("GET /api/ai/status", func(w http.ResponseWriter, r *http.Request) {
		chatReasoning := strings.ToLower(strings.TrimSpace(os.Getenv("AI_CHAT_REASONING")))
		if chatReasoning == "" {
			chatReasoning = "high"
		}
		out := map[string]any{
			"enabled":      aiClient.Enabled(),
			"provider":     aiClient.Provider(),
			"model":        aiClient.Model(),
			"search_model": aiClient.SearchModel(),
			"keys":         aiClient.KeysStatus(),
			"thumbnail": map[string]any{
				"enabled":  thumbClient.Enabled(),
				"provider": aiClient.Provider(),
				"model":    thumbClient.Model(),
			},
		}
		if aiClient.Enabled() {
			out["quota"] = aiClient.Quota()
		}
		out["chat"] = map[string]any{
			"streaming": true,
			"vision":    true,
			"image":     thumbClient.Enabled(),
			"search":    true,
			"model":     aiClient.ChatModel(),
			"route":     "9router/responses",
			"reasoning": chatReasoning,
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /api/ai/chat", func(w http.ResponseWriter, r *http.Request) {
		handleAIChat(w, r, aiClient, thumbClient, wantsChatStream(r))
	})
	mux.HandleFunc("POST /api/ai/chat/stream", func(w http.ResponseWriter, r *http.Request) {
		handleAIChat(w, r, aiClient, thumbClient, true)
	})
	mux.HandleFunc("GET /api/ai/chat/context", handleChatContext)

	// Gemini / AI API keys — simpan di .data/ai_keys.json (bukan edit .env).
	mux.HandleFunc("GET /api/ai/keys", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, aiClient.KeysStatus())
	})

	mux.HandleFunc("PUT /api/ai/keys", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Keys     []string `json:"keys"`
			KeysText string   `json:"keys_text"`
			APIKey   string   `json:"api_key"`
			Key      string   `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		keys := body.Keys
		if len(keys) == 0 && strings.TrimSpace(body.KeysText) != "" {
			for _, line := range strings.Split(body.KeysText, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				keys = append(keys, line)
			}
		}
		if len(keys) == 0 {
			one := strings.TrimSpace(body.APIKey)
			if one == "" {
				one = strings.TrimSpace(body.Key)
			}
			if one != "" {
				keys = []string{one}
			}
		}
		if len(keys) == 0 {
			writeErr(w, http.StatusBadRequest, "isi minimal 1 API key")
			return
		}
		if err := aiClient.ApplyStoredAPIKeys(keys); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"keys": aiClient.KeysStatus(),
		})
	})

	mux.HandleFunc("DELETE /api/ai/keys", func(w http.ResponseWriter, r *http.Request) {
		if err := aiClient.ClearAndReloadStored(); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"keys": aiClient.KeysStatus(),
		})
	})

	mux.HandleFunc("GET /api/ai/quota", func(w http.ResponseWriter, r *http.Request) {
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi")
			return
		}
		writeJSON(w, http.StatusOK, aiClient.Quota())
	})

	mux.HandleFunc("GET /api/ai/memory", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, mem().Get())
	})

	saveAIInstructions := func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Instructions string `json:"instructions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if err := mem().SetInstructions(body.Instructions); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": mem().Get()})
	}
	mux.HandleFunc("PUT /api/ai/instructions", saveAIInstructions)
	mux.HandleFunc("POST /api/ai/instructions", saveAIInstructions)

	mux.HandleFunc("GET /api/ai/editorial-prompt", func(w http.ResponseWriter, r *http.Request) {
		m := mem().Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"prompt":  ai.EffectiveEditorialPrompt(m, ai.GenerateRequest{}),
			"custom":  strings.TrimSpace(m.EditorialPrompt) != "",
			"default": ai.DefaultEditableEditorialPrompt,
		})
	})
	mux.HandleFunc("PUT /api/ai/editorial-prompt", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if err := mem().SetEditorialPrompt(body.Prompt); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		m := mem().Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"prompt": ai.EffectiveEditorialPrompt(m, ai.GenerateRequest{}),
			"custom": strings.TrimSpace(m.EditorialPrompt) != "",
		})
	})

	mux.HandleFunc("PUT /api/ai/product", func(w http.ResponseWriter, r *http.Request) {
		var body ai.ProductProfile
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if err := mem().SetProductProfile(body); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": mem().Get()})
	})

	saveAINiche := func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Niche  string   `json:"niche"`
			Niches []string `json:"niches"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		var err error
		if len(body.Niches) > 0 {
			err = mem().SetNiches(body.Niches)
		} else {
			err = mem().SetNiche(body.Niche)
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": mem().Get()})
	}
	mux.HandleFunc("PUT /api/ai/niche", saveAINiche)
	mux.HandleFunc("POST /api/ai/niche", saveAINiche)

	saveAIBrand := func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Brand string `json:"brand"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if err := mem().SetBrand(body.Brand); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": mem().Get()})
	}
	mux.HandleFunc("PUT /api/ai/brand", saveAIBrand)
	mux.HandleFunc("POST /api/ai/brand", saveAIBrand)

	mux.HandleFunc("POST /api/ai/memory/refresh", func(w http.ResponseWriter, r *http.Request) {
		if replizCli == nil || !replizCli.Ready() {
			writeErr(w, http.StatusBadRequest, "memakai API Repliz")
			return
		}
		id, err := resolveReplizID(r.Context(), "")
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		snapshot, err := replizCli.SnapshotForAI(r.Context(), id, 12)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		lessons := ai.BuildLessonsFromSnapshot(snapshot)
		if err := mem().ApplyLessons(lessons); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": mem().Get()})
	})

	mux.HandleFunc("POST /api/ai/memory/reset", func(w http.ResponseWriter, r *http.Request) {
		if err := mem().ResetLearning(); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": mem().Get()})
	})

	mux.HandleFunc("POST /api/ai/feedback", func(w http.ResponseWriter, r *http.Request) {
		var body ai.DraftFeedback
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Verdict == "" {
			writeErr(w, http.StatusBadRequest, "verdict wajib (good|bad|used)")
			return
		}
		v := strings.ToLower(strings.TrimSpace(body.Verdict))
		if v != "good" && v != "bad" && v != "used" {
			writeErr(w, http.StatusBadRequest, "verdict harus good|bad|used")
			return
		}
		body.Verdict = v
		if err := mem().AddFeedback(body); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": mem().Get()})
	})

	// Thumbnail ChatGPT (OpenAI Images) untuk utas Threads saja — bukan IG carousel.
	mux.HandleFunc("GET /api/ai/thumbnail/defaults", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, thumbClient.Defaults())
	})

	mux.HandleFunc("GET /api/openai/keys", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, thumbClient.KeysStatus())
	})
	mux.HandleFunc("PUT /api/openai/keys", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Keys     []string `json:"keys"`
			KeysText string   `json:"keys_text"`
			APIKey   string   `json:"api_key"`
			Key      string   `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		keys := body.Keys
		if len(keys) == 0 && strings.TrimSpace(body.KeysText) != "" {
			for _, line := range strings.Split(body.KeysText, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				keys = append(keys, line)
			}
		}
		if len(keys) == 0 {
			one := strings.TrimSpace(body.APIKey)
			if one == "" {
				one = strings.TrimSpace(body.Key)
			}
			if one != "" {
				keys = []string{one}
			}
		}
		if len(keys) == 0 {
			writeErr(w, http.StatusBadRequest, "isi minimal 1 OpenAI API key")
			return
		}
		if err := thumbClient.ApplyStoredOpenAIKeys(keys); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "keys": thumbClient.KeysStatus()})
	})
	mux.HandleFunc("DELETE /api/openai/keys", func(w http.ResponseWriter, r *http.Request) {
		if err := thumbClient.ClearStoredOpenAIKeys(); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "keys": thumbClient.KeysStatus()})
	})

	mux.HandleFunc("POST /api/upload/image", handleUploadImage)

	mux.HandleFunc("POST /api/ai/thumbnail", func(w http.ResponseWriter, r *http.Request) {
		if !thumbClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "Thumbnail belum dikonfigurasi — set AI_API_KEY")
			return
		}
		var req ai.ThumbnailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		result, err := thumbClient.GenerateRequest(req)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		dir := ai.DefaultThumbMediaDir()
		if ws := accounts.Active(); ws != nil && strings.TrimSpace(ws.ThumbDir) != "" {
			dir = ws.ThumbDir
		}
		name, err := ai.SaveThumbnailPNG(dir, result.PNG)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		rel := "/media/thumbs/" + name
		// Preview UI harus path relatif (localhost). Absolute URL publik hanya untuk publish.
		out := map[string]any{
			"ok":         true,
			"image_url":  rel,
			"path":       rel,
			"local_path": rel,
			"width":      result.Width,
			"height":     result.Height,
			"model":      result.Model,
			"size":       result.Size,
			"prompt":     result.Prompt,
			"note":       "Untuk utas Threads (bagian 1 IMAGE). Bukan untuk carousel igc().",
		}
		if publicBase != "" {
			out["public_url"] = publicBase + rel
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /api/ai/generate", func(w http.ResponseWriter, r *http.Request) {
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		var req ai.GenerateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		snap := mem().Get()
		finish := func(result *ai.GenerateResult) {
			if result == nil {
				return
			}
			if result.DailyFocus != nil {
				if req.IgnoreNiche {
					result.DailyFocus.Focus = ""
				} else if niches := ai.NicheList(snap); len(niches) > 0 {
					result.DailyFocus.Focus = strings.Join(niches, " · ")
				} else {
					result.DailyFocus.Focus = ""
				}
				if !req.IgnoreNiche {
					_ = mem().SetDaily(*result.DailyFocus)
				}
			}
			if len(result.Drafts) > 0 {
				_ = mem().RecordGeneration(ai.GenHistory{
					Topic:         req.Topic,
					Instructions:  firstNonEmpty(req.Instructions, snap.EditorialPrompt, snap.Instructions),
					Drafts:        result.Drafts,
					Consideration: result.Consideration,
				})
			}
		}

		stream := wantsChatStream(r)
		if !stream {
			result, err := aiClient.GenerateContent(nil, snap, req)
			if err != nil {
				var qe *ai.QuotaError
				if errors.As(err, &qe) {
					writeJSON(w, http.StatusTooManyRequests, map[string]any{
						"error": qe.Message,
						"kind":  qe.Kind,
						"quota": aiClient.Quota(),
					})
					return
				}
				writeErr(w, http.StatusBadGateway, err.Error())
				return
			}
			finish(result)
			writeJSON(w, http.StatusOK, result)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeErr(w, http.StatusInternalServerError, "stream tidak didukung")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher.Flush()

		var sseMu sync.Mutex
		writeRaw := func(format string, args ...any) error {
			sseMu.Lock()
			defer sseMu.Unlock()
			if _, err := fmt.Fprintf(w, format, args...); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		}
		writeEv := func(ev ai.GenerateStreamEvent) error {
			raw, _ := json.Marshal(ev)
			return writeRaw("data: %s\n\n", raw)
		}

		streamCtx, stopKeepalive := context.WithCancel(r.Context())
		defer stopKeepalive()
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-streamCtx.Done():
					return
				case <-ticker.C:
					if err := writeRaw(": keepalive\n\n"); err != nil {
						stopKeepalive()
						return
					}
				}
			}
		}()

		result, err := aiClient.GenerateContentEmit(streamCtx, nil, snap, req, writeEv)
		if err != nil {
			var qe *ai.QuotaError
			if errors.As(err, &qe) {
				_ = writeEv(ai.GenerateStreamEvent{Type: "error", Error: qe.Message})
				return
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				log.Printf("generate aborted: %v", err)
				_ = writeEv(ai.GenerateStreamEvent{Type: "error", Error: "generate dibatalkan"})
				return
			}
			log.Printf("generate stream error: %v", err)
			_ = writeEv(ai.GenerateStreamEvent{Type: "error", Error: err.Error()})
			return
		}
		finish(result)
	})

	mux.HandleFunc("GET /api/quota", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"source": "repliz",
			"note":   "Kuota posting Meta Graph tidak tersedia. Publikasi memakai jadwal Repliz.",
			"data":   []any{},
			"repliz": replizCli != nil && replizCli.Ready(),
		})
	})

	mux.HandleFunc("GET /api/mentions", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Mention Threads (Graph) tidak tersedia di Repliz Public API. Tidak ada fallback Meta.")
	})

	mux.HandleFunc("GET /api/permissions", func(w http.ResponseWriter, r *http.Request) {
		_, _, thOK, _ := replizConnectedPair(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{
			"connected": thOK,
			"source":    "repliz",
			"scopes":    map[string]any{},
			"note":      "Izin Meta tidak dipakai. Sambungkan akun di /app/akun.",
		})
	})

	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Pencarian kata kunci Threads (Graph) tidak tersedia di Repliz Public API.")
	})

	mux.HandleFunc("GET /api/replies", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("media_id")
		if id == "" {
			writeErr(w, http.StatusBadRequest, "media_id wajib")
			return
		}
		acc, err := pickReplizByType(r.Context(), r.URL.Query().Get("account_id"), "threads")
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		raw, err := replizCli.RepliesAsFeed(r.Context(), id, acc.AccountID(), acc.Username)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})

	mux.HandleFunc("GET /api/replies/pending", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []any{},
			"source": "repliz",
			"note":   "Inbox pending Graph tidak tersedia. Buka Balasan dan pilih post Repliz.",
		})
	})

	mux.HandleFunc("POST /api/replies/manage", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Sembunyikan balasan (Graph) tidak tersedia di Repliz Public API.")
	})

	mux.HandleFunc("GET /api/calendar", func(w http.ResponseWriter, r *http.Request) {
		ws := accounts.Active()
		if ws == nil {
			writeErr(w, http.StatusBadRequest, "akun tidak siap")
			return
		}
		month := strings.TrimSpace(r.URL.Query().Get("month")) // YYYY-MM
		loc := time.FixedZone("WIB", 7*3600)
		if ws.Lazy != nil {
			loc = ws.Lazy.Location()
		}
		now := time.Now().In(loc)
		var start time.Time
		if month != "" {
			t, err := time.ParseInLocation("2006-01", month, loc)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "month format YYYY-MM")
				return
			}
			start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		} else {
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		}
		end := start.AddDate(0, 1, 0)
		monthKey := start.Format("2006-01")

		type calEvent struct {
			ID            string    `json:"id"`
			Source        string    `json:"source"` // lazy | manual
			Title         string    `json:"title"`
			Status        string    `json:"status"`
			At            time.Time `json:"at"`
			Day           string    `json:"day"`
			Preview       string    `json:"preview,omitempty"`
			Text          string    `json:"text,omitempty"`
			Parts         []string  `json:"parts,omitempty"`
			Caption       string    `json:"caption,omitempty"`
			MediaType     string    `json:"media_type,omitempty"`
			ImageURL      string    `json:"image_url,omitempty"`
			ThumbURL      string    `json:"thumb_url,omitempty"`
			Error         string    `json:"error,omitempty"`
			PartsN        int       `json:"parts_n,omitempty"`
			ThreadsIDs    []string  `json:"threads_ids,omitempty"`
			IGMediaID     string    `json:"ig_media_id,omitempty"`
			BufferPostID  string    `json:"buffer_post_id,omitempty"`
			BufferXPostID string    `json:"buffer_x_post_id,omitempty"`
		}
		var events []calEvent

		if ws.Lazy != nil {
			for _, j := range ws.Lazy.ListJobs() {
				at := j.ScheduledAt.In(loc)
				if at.Before(start) || !at.Before(end) {
					continue
				}
				title := strings.TrimSpace(j.Title)
				if title == "" {
					title = "Lazy"
				}
				preview := ""
				if len(j.Parts) > 0 {
					preview = j.Parts[0]
				} else if j.Caption != "" {
					preview = j.Caption
				}
				img := strings.TrimSpace(j.ThumbURL)
				if img == "" && len(j.ImageURLs) > 0 {
					img = j.ImageURLs[0]
				}
				events = append(events, calEvent{
					ID: j.ID, Source: "lazy", Title: title, Status: j.Status,
					At: at, Day: at.Format("2006-01-02"),
					Preview:  firstLine(preview, 120),
					Text:     preview,
					Parts:    append([]string(nil), j.Parts...),
					Caption:  strings.TrimSpace(j.Caption),
					ThumbURL: img,
					Error:    j.Error, PartsN: len(j.Parts),
					ThreadsIDs:    append([]string(nil), j.ThreadsIDs...),
					IGMediaID:     strings.TrimSpace(j.IGMediaID),
					BufferPostID:  strings.TrimSpace(j.BufferPostID),
					BufferXPostID: strings.TrimSpace(j.BufferXPostID),
				})
			}
		}
		if ws.Schedule != nil {
			for _, p := range ws.Schedule.List(true) {
				at := p.RunAt.In(loc)
				if at.Before(start) || !at.Before(end) {
					continue
				}
				preview := p.Text
				if preview == "" && len(p.Parts) > 0 {
					preview = p.Parts[0]
				}
				events = append(events, calEvent{
					ID: p.ID, Source: "manual", Title: "Manual", Status: p.Status,
					At: at, Day: at.Format("2006-01-02"),
					Preview:   firstLine(preview, 120),
					Text:      preview,
					Parts:     append([]string(nil), p.Parts...),
					MediaType: strings.TrimSpace(p.MediaType),
					ImageURL:  strings.TrimSpace(p.ImageURL),
					Error:     p.Error, PartsN: len(p.Parts),
					ThreadsIDs: append([]string(nil), p.ThreadsIDs...),
				})
			}
		}
		sort.Slice(events, func(i, j int) bool {
			return events[i].At.Before(events[j].At)
		})

		byDay := map[string][]calEvent{}
		for _, e := range events {
			byDay[e.Day] = append(byDay[e.Day], e)
		}
		tz := "Asia/Jakarta"
		if ws.Lazy != nil {
			if t := ws.Lazy.GetConfig().Timezone; t != "" {
				tz = t
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"month":      monthKey,
			"timezone":   tz,
			"account_id": ws.Meta.ID,
			"events":     events,
			"by_day":     byDay,
			"today":      now.Format("2006-01-02"),
		})
	})

	mux.HandleFunc("GET /api/schedule", func(w http.ResponseWriter, r *http.Request) {
		ws := accounts.Active()
		if ws == nil || ws.Schedule == nil {
			writeErr(w, http.StatusBadRequest, "akun tidak siap")
			return
		}
		all := r.URL.Query().Get("all") == "1" || r.URL.Query().Get("all") == "true"
		tz := "Asia/Jakarta"
		if lz() != nil {
			tz = lz().GetConfig().Timezone
			if tz == "" {
				tz = "Asia/Jakarta"
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"posts":      ws.Schedule.List(all),
			"account_id": ws.Meta.ID,
			"timezone":   tz,
		})
	})
	mux.HandleFunc("POST /api/schedule", func(w http.ResponseWriter, r *http.Request) {
		ws := accounts.Active()
		if ws == nil || ws.Schedule == nil {
			writeErr(w, http.StatusBadRequest, "akun tidak siap")
			return
		}
		var body struct {
			RunAt        string   `json:"run_at"`
			MediaType    string   `json:"media_type"`
			Text         string   `json:"text"`
			Parts        []string `json:"parts"`
			ImageURL     string   `json:"image_url"`
			VideoURL     string   `json:"video_url"`
			ReplyControl string   `json:"reply_control"`
			ReplyToID    string   `json:"reply_to_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		runAt, err := parseScheduleTime(body.RunAt)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		post, err := ws.Schedule.Create(schedule.CreateInput{
			RunAt:        runAt,
			MediaType:    body.MediaType,
			Text:         body.Text,
			Parts:        body.Parts,
			ImageURL:     body.ImageURL,
			VideoURL:     body.VideoURL,
			ReplyControl: body.ReplyControl,
			ReplyToID:    body.ReplyToID,
		})
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"post": post})
	})
	mux.HandleFunc("POST /api/schedule/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		ws := accounts.Active()
		if ws == nil || ws.Schedule == nil {
			writeErr(w, http.StatusBadRequest, "akun tidak siap")
			return
		}
		post, err := ws.Schedule.Cancel(r.PathValue("id"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"post": post})
	})
	mux.HandleFunc("DELETE /api/schedule/{id}", func(w http.ResponseWriter, r *http.Request) {
		ws := accounts.Active()
		if ws == nil || ws.Schedule == nil {
			writeErr(w, http.StatusBadRequest, "akun tidak siap")
			return
		}
		if err := ws.Schedule.Delete(r.PathValue("id")); err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("POST /api/publish", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MediaType string   `json:"media_type"`
			Text      string   `json:"text"`
			Parts     []string `json:"parts"`
			ImageURL  string   `json:"image_url"`
			VideoURL  string   `json:"video_url"`
			ReplyToID string   `json:"reply_to_id"`
			ParentID  string   `json:"parent_id"`
			Publish   bool     `json:"publish"`
			ContentID string   `json:"content_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		parts := body.Parts
		if len(parts) == 0 && strings.TrimSpace(body.Text) != "" {
			parts = []string{body.Text}
		}
		if len(parts) == 0 {
			writeErr(w, http.StatusBadRequest, "teks wajib")
			return
		}
		acc, err := pickReplizByType(r.Context(), "", "threads")
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		if !replizAccountLive(acc) {
			writeErr(w, http.StatusUnauthorized, "akun Repliz Threads belum terhubung — sambungkan di /app/akun")
			return
		}
		replyTo := strings.TrimSpace(body.ReplyToID)
		contentID := strings.TrimSpace(body.ContentID)
		if contentID == "" {
			contentID = strings.TrimSpace(body.ParentID)
		}
		if replyTo != "" {
			if contentID == "" {
				contentID = replyTo
			}
			parent := ""
			if replyTo != contentID {
				parent = replyTo
			}
			id, err := replizCli.CreateComment(r.Context(), contentID, acc.AccountID(), parts[0], parent)
			if err != nil {
				writeAPIErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":        true,
				"source":    "repliz",
				"published": map[string]any{"id": id},
			})
			return
		}
		ids, err := publishReplizParts(r.Context(), "threads", parts, body.ImageURL, body.VideoURL, time.Now().Add(25*time.Second))
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		id := ""
		if len(ids) > 0 {
			id = ids[0]
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"source":    "repliz",
			"schedule":  ids,
			"published": map[string]any{"id": id},
			"note":      "Dijadwalkan lewat Repliz (bukan Meta Graph).",
		})
	})

	mux.HandleFunc("GET /api/container", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Container Meta tidak dipakai. Publikasi langsung lewat jadwal Repliz.")
	})

	mux.HandleFunc("POST /api/repost", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Repost Threads (Graph) tidak tersedia di Repliz Public API.")
	})

	mux.HandleFunc("DELETE /api/media/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Hapus post terpublish tidak tersedia di Repliz Public API. Batalkan jadwal di Kalender jika belum terkirim.")
	})

	mux.HandleFunc("GET /api/media/{id}/insights", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		acc, err := pickReplizByType(r.Context(), r.URL.Query().Get("account_id"), "")
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		stats, err := replizCli.GetContentStatistic(r.Context(), id, acc.AccountID())
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": stats, "source": "repliz"})
	})

	mux.HandleFunc("GET /api/media/{id}/thread", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":     r.PathValue("id"),
			"parts":  []string{},
			"ids":    []string{},
			"source": "repliz",
			"note":   "Utas Graph tidak dibaca. Isi slide carousel secara manual atau dari teks post Repliz.",
		})
	})

	// ——— Instagram (token terpisah) ———
	mux.HandleFunc("GET /api/ig/status", func(w http.ResponseWriter, r *http.Request) {
		if replizCli != nil && replizCli.Ready() {
			acc, err := pickReplizByType(r.Context(), r.URL.Query().Get("account_id"), "instagram")
			if err != nil {
				writeJSON(w, http.StatusOK, map[string]any{
					"connected": false,
					"source":    "repliz",
					"error":     err.Error(),
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"connected":  acc.IsConnected || acc.Username != "",
				"user_id":    acc.AccountID(),
				"username":   acc.Username,
				"name":       acc.Name,
				"picture":    acc.Picture,
				"type":       acc.Type,
				"source":     "repliz",
				"account_id": acc.AccountID(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"connected": false,
			"source":    "repliz",
			"error":     "Repliz belum disambungkan",
		})
	})

	mux.HandleFunc("POST /api/ig/token", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Instagram Meta tidak dipakai. Sambungkan lewat Repliz di /app/akun.")
	})
	mux.HandleFunc("POST /api/ig/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Instagram Meta tidak dipakai. Sambungkan lewat Repliz di /app/akun.")
	})
	mux.HandleFunc("POST /api/ig/token/exchange", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Instagram Meta tidak dipakai. Sambungkan lewat Repliz di /app/akun.")
	})
	mux.HandleFunc("DELETE /api/ig/token", func(w http.ResponseWriter, r *http.Request) {
		writeGoneMeta(w, "Token Instagram Meta tidak dipakai. Kelola akun di /app/akun.")
	})

	mux.HandleFunc("GET /api/ig/me", func(w http.ResponseWriter, r *http.Request) {
		if replizCli != nil && replizCli.Ready() {
			acc, err := pickReplizByType(r.Context(), r.URL.Query().Get("account_id"), "instagram")
			if err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			stat, err := replizCli.GetAccountStatistic(r.Context(), acc.AccountID())
			if err != nil {
				stat = map[string]any{}
			}
			views := repliz.AsFloat(stat["views"])
			if views == 0 {
				views = repliz.AsFloat(stat["videoViews"])
			}
			if views == 0 {
				views = repliz.AsFloat(stat["impressions"])
			}
			likes := repliz.AsFloat(stat["likes"])
			if likes == 0 {
				likes = repliz.AsFloat(stat["totalLikes"])
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"id":                  acc.AccountID(),
				"user_id":             acc.AccountID(),
				"username":            acc.Username,
				"name":                acc.Name,
				"account_type":        acc.Type,
				"profile_picture_url": acc.Picture,
				"followers_count":     repliz.AsFloat(stat["followersCount"]),
				"follows_count":       repliz.AsFloat(stat["followingCount"]),
				"media_count":         repliz.AsFloat(stat["videosCount"]),
				"biography":           "",
				"website":             "",
				"source":              "repliz",
				"views":               views,
				"likes":               likes,
				"reach":               repliz.AsFloat(stat["reach"]),
				"comments":            repliz.AsFloat(stat["comments"]),
				"saves":               repliz.AsFloat(stat["saves"]),
				"shares":              repliz.AsFloat(stat["shares"]),
				"accounts_engaged":    repliz.AsFloat(stat["accountsEngaged"]),
			})
			return
		}
		writeGoneMeta(w, "Profil Instagram memakai Repliz. Set REPLIZ_ACCESS_KEY.")
	})

	mux.HandleFunc("GET /api/ig/media", func(w http.ResponseWriter, r *http.Request) {
		acc, err := pickReplizByType(r.Context(), r.URL.Query().Get("account_id"), "instagram")
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		limit := 24
		if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		raw, err := replizCli.Feed(r.Context(), "", "", acc.AccountID(), limit)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})

	mux.HandleFunc("POST /api/ai/carousel", func(w http.ResponseWriter, r *http.Request) {
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		var req ai.CarouselRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		snap := mem().Get()
		result, err := aiClient.GenerateCarousel(snap, req)
		if err != nil {
			var qe *ai.QuotaError
			if errors.As(err, &qe) {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error": qe.Message,
					"kind":  qe.Kind,
					"quota": aiClient.Quota(),
				})
				return
			}
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	// Caption IG saja (dari slide/utas yang sudah ada).
	mux.HandleFunc("POST /api/ai/caption", func(w http.ResponseWriter, r *http.Request) {
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		var body struct {
			Parts []string `json:"parts"`
			Brand string   `json:"brand"`
			Text  string   `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		parts := body.Parts
		if len(parts) == 0 && strings.TrimSpace(body.Text) != "" {
			parts = []string{body.Text}
		}
		snap := mem().Get()
		cap, usage, err := aiClient.GenerateIGCaption(snap, parts, body.Brand)
		if err != nil {
			var qe *ai.QuotaError
			if errors.As(err, &qe) {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error": qe.Message,
					"kind":  qe.Kind,
					"quota": aiClient.Quota(),
				})
				return
			}
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		out := map[string]any{"caption": cap}
		if usage != nil {
			out["usage"] = usage
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Auto-reply: generate draf balasan sesuai intent (preview di UI, kirim via /api/publish).
	mux.HandleFunc("POST /api/ai/replies", func(w http.ResponseWriter, r *http.Request) {
		if replizCli == nil || !replizCli.Ready() {
			writeErr(w, http.StatusUnauthorized, "akun Repliz belum terhubung — sambungkan di /app/akun")
			return
		}
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		var body struct {
			MediaID      string             `json:"media_id"`
			PostText     string             `json:"post_text"`
			Intent       string             `json:"intent"`
			Instructions string             `json:"instructions"`
			OnlyPending  *bool              `json:"only_pending"`
			Limit        int                `json:"limit"`
			Incoming     []ai.IncomingReply `json:"incoming"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		intent := strings.TrimSpace(body.Intent)
		mediaID := strings.TrimSpace(body.MediaID)
		postText := strings.TrimSpace(body.PostText)
		incoming := body.Incoming

		if mediaID != "" {
			if postText == "" {
				if accID, err := resolveReplizID(r.Context(), ""); err == nil {
					raw, err := replizCli.Feed(r.Context(), "", "", accID, 40)
					if err == nil {
						var list struct {
							Data []struct {
								ID   string `json:"id"`
								Text string `json:"text"`
							} `json:"data"`
						}
						if json.Unmarshal(raw, &list) == nil {
							for _, p := range list.Data {
								if p.ID == mediaID {
									postText = p.Text
									break
								}
							}
						}
					}
				}
			}
			if len(incoming) == 0 {
				acc, aerr := pickReplizByType(r.Context(), "", "threads")
				if aerr != nil {
					writeAPIErr(w, aerr)
					return
				}
				enriched, err := replizCli.RepliesAsFeed(r.Context(), mediaID, acc.AccountID(), acc.Username)
				if err != nil {
					writeAPIErr(w, err)
					return
				}
				var payload struct {
					Data []struct {
						ID       string `json:"id"`
						Username string `json:"username"`
						Text     string `json:"text"`
						IsMine   bool   `json:"is_mine"`
						Answered bool   `json:"answered"`
					} `json:"data"`
				}
				if err := json.Unmarshal(enriched, &payload); err != nil {
					writeErr(w, http.StatusBadGateway, "gagal baca balasan")
					return
				}
				onlyPending := true
				if body.OnlyPending != nil {
					onlyPending = *body.OnlyPending
				}
				for _, n := range payload.Data {
					if n.IsMine || strings.TrimSpace(n.ID) == "" {
						continue
					}
					if onlyPending && n.Answered {
						continue
					}
					incoming = append(incoming, ai.IncomingReply{
						ID: n.ID, Username: n.Username, Text: n.Text,
					})
				}
			}
		}

		if postText == "" {
			writeErr(w, http.StatusBadRequest, "post_text atau media_id wajib")
			return
		}
		if len(incoming) == 0 {
			writeErr(w, http.StatusBadRequest, "tidak ada komentar pending untuk dibalas")
			return
		}

		snap := mem().Get()
		result, err := aiClient.GenerateReplies(snap, ai.RepliesRequest{
			PostID:       mediaID,
			PostText:     postText,
			Intent:       intent,
			Instructions: body.Instructions,
			Incoming:     incoming,
			Limit:        body.Limit,
		})
		if err != nil {
			var qe *ai.QuotaError
			if errors.As(err, &qe) {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error": qe.Message,
					"kind":  qe.Kind,
					"quota": aiClient.Quota(),
				})
				return
			}
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/ai/lazy/daily", func(w http.ResponseWriter, r *http.Request) {
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		var req struct {
			Topic string `json:"topic"`
			Count int    `json:"count"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Count <= 0 {
			req.Count = 1
		}
		snap := mem().Get()
		gen, err := aiClient.GenerateContent(nil, snap, ai.GenerateRequest{
			Topic: req.Topic,
			Count: req.Count,
		})
		if err != nil {
			var qe *ai.QuotaError
			if errors.As(err, &qe) {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error": qe.Message,
					"kind":  qe.Kind,
					"quota": aiClient.Quota(),
				})
				return
			}
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		if gen.DailyFocus != nil {
			if niches := ai.NicheList(snap); len(niches) > 0 {
				gen.DailyFocus.Focus = strings.Join(niches, " · ")
			}
			_ = mem().SetDaily(*gen.DailyFocus)
		}
		_ = mem().RecordGeneration(ai.GenHistory{
			Topic:         req.Topic,
			Instructions:  snap.Instructions,
			Drafts:        gen.Drafts,
			Consideration: gen.Consideration,
		})

		var carousel *ai.CarouselResult
		if len(gen.Drafts) > 0 && len(gen.Drafts[0].Parts) >= 2 {
			carousel, _ = aiClient.GenerateCarousel(snap, ai.CarouselRequest{
				Parts: gen.Drafts[0].Parts,
				Brand: snap.Brand,
				Topic: req.Topic,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"date":     time.Now().Format("2006-01-02"),
			"brand":    snap.Brand,
			"threads":  gen,
			"carousel": carousel,
			"pipeline": []string{"generate_utas", "post_threads", "carousel_ig"},
		})
	})

	mux.HandleFunc("GET /api/ig/carousel/templates", func(w http.ResponseWriter, r *http.Request) {
		cfg := lz().GetConfig()
		writeJSON(w, http.StatusOK, map[string]any{
			"templates": lazy.ListCarouselTemplates(),
			"active":    lazy.NormalizeTemplate(cfg.CarouselTemplate),
		})
	})

	mux.HandleFunc("POST /api/ig/carousel/render", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text     string `json:"text"`
			Brand    string `json:"brand"`
			Template string `json:"template"`
			Index    int    `json:"index"` // 0-based; opsional
			Total    int    `json:"total"` // opsional — tampilkan 01/06 + dots
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		text := strings.TrimSpace(body.Text)
		if text == "" {
			text = "Isi slide muncul di sini."
		}
		brand := strings.TrimSpace(body.Brand)
		if brand == "" {
			brand = mem().Get().Brand
		}
		tpl := strings.TrimSpace(body.Template)
		if tpl == "" {
			tpl = lz().GetConfig().CarouselTemplate
		}
		slideNum, slideTotal := body.Index+1, body.Total
		if slideTotal < 0 {
			slideTotal = 0
		}
		if slideTotal > 10 {
			slideTotal = 10
		}
		if slideNum < 1 {
			slideNum = 1
		}
		if slideTotal > 0 && slideNum > slideTotal {
			slideNum = slideTotal
		}
		dir := filepath.Join(lz().MediaDir(), "_preview")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		path := filepath.Join(dir, fmt.Sprintf("%d.png", time.Now().UnixNano()))
		if err := lazy.RenderSlidePNG(path, brand, text, slideNum, slideTotal, tpl); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer os.Remove(path)
		b, err := os.ReadFile(path)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	})

	mux.HandleFunc("POST /api/ig/carousel/publish", func(w http.ResponseWriter, r *http.Request) {
		if _, err := pickReplizByType(r.Context(), "", "instagram"); err != nil {
			writeErr(w, http.StatusUnauthorized, "akun Repliz Instagram belum terhubung — sambungkan di /app/akun")
			return
		}
		var body struct {
			ImageURLs []string `json:"image_urls"`
			Parts     []string `json:"parts"`
			Brand     string   `json:"brand"`
			Caption   string   `json:"caption"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}

		urls := make([]string, 0, len(body.ImageURLs))
		for _, u := range body.ImageURLs {
			u = strings.TrimSpace(u)
			if u != "" {
				urls = append(urls, u)
			}
		}

		// Auto-render teks → PNG publik kalau parts dikirim / URL kurang
		if len(body.Parts) >= 2 && (len(urls) < 2 || len(urls) < len(body.Parts)) {
			brand := strings.TrimSpace(body.Brand)
			if brand == "" {
				brand = mem().Get().Brand
			}
			key := time.Now().Format("2006-01-02") + "/manual-" + fmt.Sprintf("%d", time.Now().Unix()%100000)
			tpl := lz().GetConfig().CarouselTemplate
			rendered, err := lazy.RenderPartsPublic(lz().MediaDir(), publicBase, brand, key, body.Parts, tpl)
			if err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			urls = rendered
		}

		if len(urls) < 2 {
			writeErr(w, http.StatusBadRequest, "butuh minimal 2 slide (teks akan di-render jadi gambar, atau isi image_urls)")
			return
		}

		accountID := accounts.PlatformAccountID(accounts.ActiveID(), "instagram")
		if accountID == "" {
			if picked, pickErr := pickReplizByType(r.Context(), "", "instagram"); pickErr == nil {
				accountID = picked.AccountID()
			}
		}
		id, err := (replizPub{}).PublishIGCarousel(accountID, urls, body.Caption)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"image_urls": urls,
			"result":     map[string]any{"id": id, "source": "repliz"},
			"container":  id,
		})
	})

	mux.HandleFunc("GET /api/lazy/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, lz().GetConfig())
	})
	mux.HandleFunc("PUT /api/lazy/config", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Enabled          bool     `json:"enabled"`
			PostsPerDay      int      `json:"posts_per_day"`
			Timezone         string   `json:"timezone"`
			TopicHint        string   `json:"topic_hint"`
			ThumbnailEnabled *bool    `json:"thumbnail_enabled"`
			CarouselTemplate *string  `json:"carousel_template"`
			Channels         []string `json:"channels"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if body.Enabled && len(body.Channels) == 0 {
			writeErr(w, http.StatusBadRequest, "pilih minimal satu akun tujuan")
			return
		}
		if body.Enabled {
			for _, channel := range body.Channels {
				channel = strings.ToLower(strings.TrimSpace(channel))
				if channel == "ig" {
					channel = "instagram"
				}
				if accounts.PlatformAccountID(accounts.ActiveID(), channel) == "" {
					writeErr(w, http.StatusBadRequest, fmt.Sprintf("pilih akun %s di workspace sebelum menyalakan automasi", channel))
					return
				}
			}
		}
		cur := lz().GetConfig()
		tz := strings.TrimSpace(body.Timezone)
		if tz == "" {
			tz = cur.Timezone
		}
		tpl := cur.CarouselTemplate
		if body.CarouselTemplate != nil {
			tpl = *body.CarouselTemplate
		}
		cfg, err := lz().SetConfig(lazy.Config{
			Enabled:          body.Enabled,
			PostsPerDay:      body.PostsPerDay,
			Timezone:         tz,
			TopicHint:        body.TopicHint,
			ThumbnailEnabled: true,
			CarouselTemplate: tpl,
			Channels:         body.Channels,
		})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		// If just enabled, plan today + tomorrow immediately
		if cfg.Enabled {
			loc := lz().Location()
			now := time.Now().In(loc)
			_ = lzs().EnsureDayPlan(now, cfg.PostsPerDay, nil)
			tom := time.Date(now.Year(), now.Month(), now.Day()+1, 12, 0, 0, 0, loc)
			_ = lzs().EnsureDayPlan(tom, cfg.PostsPerDay, nil)
		}
		writeJSON(w, http.StatusOK, cfg)
	})
	mux.HandleFunc("GET /api/lazy/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, lzs().Status())
	})
	mux.HandleFunc("GET /api/lazy/track", func(w http.ResponseWriter, r *http.Request) {
		withMetrics := r.URL.Query().Get("metrics") != "0"
		writeJSON(w, http.StatusOK, lazy.BuildTrackReport(lz(), nil, nil, withMetrics))
	})
	mux.HandleFunc("GET /api/lazy/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		job, ok := lzs().GetJob(id)
		if !ok {
			writeErr(w, http.StatusNotFound, "job tidak ditemukan")
			return
		}
		writeJSON(w, http.StatusOK, job)
	})
	mux.HandleFunc("POST /api/lazy/run-now", func(w http.ResponseWriter, r *http.Request) {
		job, err := lzs().RunNow()
		if err != nil {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"ok":      true,
			"started": true,
			"job":     job,
			"message": "Job jalan di background — pantau antrian (hindari 504)",
		})
	})
	mux.HandleFunc("POST /api/lazy/handoff", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Parts      []string `json:"parts"`
			Title      string   `json:"title"`
			CoverTitle string   `json:"cover_title"`
			ThumbURL   string   `json:"thumb_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		store := lz()
		if store == nil {
			writeErr(w, http.StatusServiceUnavailable, "workspace Lazy belum siap")
			return
		}
		attachedID, pending, err := store.EnqueueHandoff(lazy.ContentHandoff{
			Parts:      body.Parts,
			Title:      body.Title,
			CoverTitle: body.CoverTitle,
			ThumbURL:   body.ThumbURL,
		})
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		msg := "Utas + cover Edge Clean diantre ke slot Lazy berikutnya"
		if attachedID != "" {
			msg = "Utas + cover Edge Clean dilampirkan ke job " + attachedID
		} else if pending {
			msg = "Utas + cover Edge Clean menunggu slot Lazy berikutnya"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":               true,
			"attached_job_id":  attachedID,
			"pending":          pending,
			"pending_handoff":  store.PendingHandoff(),
			"message":          msg,
		})
	})
	mux.HandleFunc("GET /api/lazy/handoff", func(w http.ResponseWriter, r *http.Request) {
		store := lz()
		if store == nil {
			writeErr(w, http.StatusServiceUnavailable, "workspace Lazy belum siap")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":              true,
			"pending_handoff": store.PendingHandoff(),
		})
	})
	mux.HandleFunc("POST /api/lazy/replan", func(w http.ResponseWriter, r *http.Request) {
		if err := lzs().ReplanToday(); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, lzs().Status())
	})

	mux.HandleFunc("GET /api/buffer/status", func(w http.ResponseWriter, r *http.Request) {
		b := buf()
		if b == nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "enabled": false, "note": "Tidak ada akun aktif"})
			return
		}
		writeJSON(w, http.StatusOK, b.Status())
	})

	mux.HandleFunc("PUT /api/buffer/key", func(w http.ResponseWriter, r *http.Request) {
		writeBufferKey(w, r, accounts.Active())
	})

	mux.HandleFunc("DELETE /api/buffer/key", func(w http.ResponseWriter, r *http.Request) {
		clearBufferKey(w, accounts.Active())
	})

	mux.HandleFunc("GET /api/accounts/{id}/buffer", func(w http.ResponseWriter, r *http.Request) {
		ws, err := accountByParam(r.PathValue("id"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		st := ws.Buffer.Status()
		st["account_id"] = ws.Meta.ID
		writeJSON(w, http.StatusOK, st)
	})

	mux.HandleFunc("PUT /api/accounts/{id}/buffer", func(w http.ResponseWriter, r *http.Request) {
		ws, err := accountByParam(r.PathValue("id"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeBufferKey(w, r, ws)
	})

	mux.HandleFunc("DELETE /api/accounts/{id}/buffer", func(w http.ResponseWriter, r *http.Request) {
		ws, err := accountByParam(r.PathValue("id"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		clearBufferKey(w, ws)
	})

	// Manual: utas teks → Buffer X/Twitter thread (Notify Me).
	mux.HandleFunc("POST /api/buffer/twitter", func(w http.ResponseWriter, r *http.Request) {
		bc := buf()
		if bc == nil || !bc.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "Buffer key akun aktif belum di-set — isi di Workspace → Kelola")
			return
		}
		var body struct {
			Parts []string `json:"parts"`
			Text  string   `json:"text"` // fallback single post
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		parts := body.Parts
		if len(parts) == 0 && strings.TrimSpace(body.Text) != "" {
			parts = []string{body.Text}
		}
		res, err := bc.QueueTwitterThread(parts)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"parts":  len(parts),
			"buffer": res,
			"note":   "X shareNow — dipublish langsung via Buffer",
		})
	})

	// Manual: kirim image URLs + caption ke Buffer TikTok (Notify Me).
	mux.HandleFunc("POST /api/buffer/tiktok", func(w http.ResponseWriter, r *http.Request) {
		bc := buf()
		if bc == nil || !bc.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "Buffer key akun aktif belum di-set — isi di Workspace → Kelola")
			return
		}
		if publicBase == "" {
			writeErr(w, http.StatusBadRequest, "PUBLIC_BASE_URL wajib (gambar di-mirror ke URL publik)")
			return
		}
		var body struct {
			ImageURLs []string `json:"image_urls"`
			Caption   string   `json:"caption"`
			Title     string   `json:"title"`
			Mirror    *bool    `json:"mirror"` // default true
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		urls := body.ImageURLs
		doMirror := body.Mirror == nil || *body.Mirror
		if doMirror {
			key := time.Now().Format("2006-01-02") + "/buf-" + fmt.Sprintf("%d", time.Now().Unix()%100000)
			mirrored, err := buffer.MirrorPublicURLs(lz().MediaDir(), publicBase, key, urls)
			if err != nil {
				writeErr(w, http.StatusBadGateway, err.Error())
				return
			}
			urls = mirrored
		}
		res, err := bc.QueueTikTokPhotos(body.Caption, body.Title, urls)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"buffer":     res,
			"image_urls": urls,
			"note":       "TikTok Notify Me — selesai post dari notifikasi Buffer di HP",
		})
	})

	// Manual: carousel editor (teks slide) → render PNG → Buffer TikTok Notify Me.
	mux.HandleFunc("POST /api/buffer/from-carousel", func(w http.ResponseWriter, r *http.Request) {
		bc := buf()
		if bc == nil || !bc.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "Buffer key akun aktif belum di-set — isi di Workspace → Kelola")
			return
		}
		if publicBase == "" {
			writeErr(w, http.StatusBadRequest, "PUBLIC_BASE_URL wajib")
			return
		}
		var body struct {
			ImageURLs []string `json:"image_urls"`
			Parts     []string `json:"parts"`
			Brand     string   `json:"brand"`
			Caption   string   `json:"caption"`
			Title     string   `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		urls := make([]string, 0, len(body.ImageURLs))
		for _, u := range body.ImageURLs {
			u = strings.TrimSpace(u)
			if u != "" {
				urls = append(urls, u)
			}
		}
		if len(body.Parts) >= 1 && (len(urls) < 1 || len(urls) < len(body.Parts)) {
			brand := strings.TrimSpace(body.Brand)
			if brand == "" {
				brand = mem().Get().Brand
			}
			key := time.Now().Format("2006-01-02") + "/buf-car-" + fmt.Sprintf("%d", time.Now().Unix()%100000)
			tpl := lz().GetConfig().CarouselTemplate
			rendered, err := lazy.RenderPartsPublic(lz().MediaDir(), publicBase, brand, key, body.Parts, tpl)
			if err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			urls = rendered
		} else if len(urls) >= 1 {
			key := time.Now().Format("2006-01-02") + "/buf-car-" + fmt.Sprintf("%d", time.Now().Unix()%100000)
			mirrored, err := buffer.MirrorPublicURLs(lz().MediaDir(), publicBase, key, urls)
			if err != nil {
				writeErr(w, http.StatusBadGateway, "mirror: "+err.Error())
				return
			}
			urls = mirrored
		}
		if len(urls) < 1 {
			writeErr(w, http.StatusBadRequest, "butuh minimal 1 slide (teks atau image_urls)")
			return
		}
		if len(urls) > 10 {
			urls = urls[:10]
		}
		caption := strings.TrimSpace(body.Caption)
		title := strings.TrimSpace(body.Title)
		if title == "" {
			title = firstLine(caption, 80)
		}
		res, err := bc.QueueTikTokPhotos(caption, title, urls)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"slides":     len(urls),
			"buffer":     res,
			"image_urls": urls,
			"note":       "TikTok Notify Me — selesai post dari notifikasi Buffer di HP",
		})
	})

	// Manual: import post IG (IMAGE/CAROUSEL) → Buffer TikTok Notify Me.
	mux.HandleFunc("POST /api/buffer/from-ig", func(w http.ResponseWriter, r *http.Request) {
		bc := buf()
		if bc == nil || !bc.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "Buffer key akun aktif belum di-set — isi di Workspace → Kelola")
			return
		}
		if _, err := pickReplizByType(r.Context(), "", "instagram"); err != nil {
			writeErr(w, http.StatusUnauthorized, "akun Repliz Instagram belum terhubung — sambungkan di /app/akun")
			return
		}
		if publicBase == "" {
			writeErr(w, http.StatusBadRequest, "PUBLIC_BASE_URL wajib")
			return
		}
		var body struct {
			MediaID string `json:"media_id"`
			Caption string `json:"caption"` // override opsional
			Title   string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		acc, err := pickReplizByType(r.Context(), "", "instagram")
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		posts, err := replizCli.ListContentUpTo(r.Context(), acc.AccountID(), 40)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		var srcURLs []string
		caption := strings.TrimSpace(body.Caption)
		foundID := strings.TrimSpace(body.MediaID)
		for _, p := range posts {
			if p.PostID() != foundID {
				continue
			}
			if caption == "" {
				caption = strings.TrimSpace(p.Description)
				if caption == "" {
					caption = strings.TrimSpace(p.Title)
				}
			}
			for _, m := range p.Medias {
				u := strings.TrimSpace(m.URL)
				if u == "" {
					u = strings.TrimSpace(m.Thumbnail)
				}
				if u != "" {
					srcURLs = append(srcURLs, u)
				}
			}
			break
		}
		if len(srcURLs) == 0 {
			writeErr(w, http.StatusBadRequest, "media Repliz tidak ditemukan atau tanpa gambar")
			return
		}
		title := strings.TrimSpace(body.Title)
		if title == "" {
			title = firstLine(caption, 80)
		}
		key := "ig-import/" + foundID
		mirrored, err := buffer.MirrorPublicURLs(lz().MediaDir(), publicBase, key, srcURLs)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "mirror: "+err.Error())
			return
		}
		res, err := bc.QueueTikTokPhotos(caption, title, mirrored)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"ig_media_id": foundID,
			"media_type":  "IMAGE",
			"slides":      len(mirrored),
			"buffer":      res,
			"image_urls":  mirrored,
			"caption":     caption,
			"note":        "TikTok Notify Me — selesai post dari notifikasi Buffer di HP",
		})
	})

	mux.HandleFunc("GET /media/lazy/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/media/lazy/")
		if p := accounts.FindLazyMedia(rel); p != "" {
			http.ServeFile(w, r, p)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("GET /media/thumbs/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/media/thumbs/")
		if p := accounts.FindThumbMedia(rel); p != "" {
			http.ServeFile(w, r, p)
			return
		}
		http.NotFound(w, r)
	})

	log.Printf("Threads dashboard di http://localhost%s", addr)
	if publicBase != "" {
		log.Printf("PUBLIC_BASE_URL=%s (IG carousel media)", publicBase)
	} else {
		log.Println("PUBLIC_BASE_URL kosong — auto IG carousel akan di-skip")
	}
	if err := http.ListenAndServe(addr, withCORS(gate.Middleware(gate.RequireAdminHTML(mux)))); err != nil {
		log.Fatal(err)
	}
}

func proxy(w http.ResponseWriter, fn func() (json.RawMessage, error)) {
	raw, err := fn()
	if err != nil {
		writeAPIErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func writeAPIErr(w http.ResponseWriter, err error) {
	var apiErr *threads.APIError
	if errors.As(err, &apiErr) {
		w.Header().Set("Content-Type", "application/json")
		status := apiErr.Status
		if status < 400 {
			status = http.StatusBadGateway
		}
		body := strings.TrimSpace(string(apiErr.Body))
		if body == "" || body == "{}" || body == "null" {
			writeJSON(w, status, map[string]any{"error": apiErr.Error()})
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write(apiErr.Body)
		return
	}
	var igErr *instagram.APIError
	if errors.As(err, &igErr) {
		w.Header().Set("Content-Type", "application/json")
		status := igErr.Status
		if status < 400 {
			status = http.StatusBadGateway
		}
		body := strings.TrimSpace(string(igErr.Body))
		if body == "" || body == "{}" || body == "null" {
			writeJSON(w, status, map[string]any{"error": igErr.Error()})
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write(igErr.Body)
		return
	}
	writeErr(w, http.StatusBadGateway, err.Error())
}

func oauthRedirect(w http.ResponseWriter, r *http.Request, provider, status, msg string) {
	q := url.Values{"oauth": {status}, "provider": {provider}}
	if msg != "" {
		q.Set("msg", msg)
	}
	http.Redirect(w, r, "/app/akun.html?"+q.Encode(), http.StatusFound)
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, oa *oauth.Config, provider string) {
	if errMsg := strings.TrimSpace(r.URL.Query().Get("error")); errMsg != "" {
		desc := strings.TrimSpace(r.URL.Query().Get("error_description"))
		if desc == "" {
			desc = errMsg
		}
		oauthRedirect(w, r, provider, "err", desc)
		return
	}
	code := oauth.CleanCode(r.URL.Query().Get("code"))
	state := r.URL.Query().Get("state")
	accountID, err := oa.ParseState(state, provider)
	if err != nil {
		oauthRedirect(w, r, provider, "err", err.Error())
		return
	}
	ws, err := accountByParam(accountID)
	if err != nil {
		oauthRedirect(w, r, provider, "err", err.Error())
		return
	}

	var tok *oauth.TokenResult
	switch provider {
	case "threads":
		tok, err = oa.ExchangeThreadsCode(code)
	case "instagram":
		tok, err = oa.ExchangeInstagramCode(code)
	default:
		err = fmt.Errorf("provider tidak dikenal")
	}
	if err != nil {
		oauthRedirect(w, r, provider, "err", err.Error())
		return
	}

	switch provider {
	case "threads":
		ws.Threads.SetToken(tok.AccessToken)
		if uid := strings.TrimSpace(tok.UserID); uid != "" && uid != "<nil>" {
			ws.Threads.SetUserID(uid)
		}
		me, merr := ws.Threads.GetMe()
		if merr != nil && strings.TrimSpace(tok.UserID) != "" && strings.TrimSpace(tok.UserID) != "<nil>" {
			me, merr = ws.Threads.GetUser(tok.UserID)
		}
		if merr != nil {
			// Token dari OAuth tetap disimpan — Meta kadang 500 kosong sementara di /me.
			log.Printf("threads oauth: GetMe gagal setelah token tersimpan: %v", merr)
			_ = accounts.UpdateMeta(ws.Meta.ID, func(m *account.Meta) {
				if uid := strings.TrimSpace(tok.UserID); uid != "" && uid != "<nil>" && m.ThreadsUsername == "" {
					m.ThreadsUsername = uid
				}
			})
			oauthRedirect(w, r, provider, "ok", "token tersimpan; profil Threads belum terbaca ("+merr.Error()+")")
			return
		}
		var profile struct {
			Username string `json:"username"`
		}
		_ = json.Unmarshal(me, &profile)
		_ = accounts.UpdateMeta(ws.Meta.ID, func(m *account.Meta) {
			if profile.Username != "" {
				m.ThreadsUsername = profile.Username
				if m.Name == "" || m.Name == m.ID || m.Name == "main" {
					m.Name = profile.Username
				}
			}
		})
	case "instagram":
		ws.IG.SetToken(tok.AccessToken)
		if uid := strings.TrimSpace(tok.UserID); uid != "" && uid != "<nil>" {
			ws.IG.SetUserID(uid)
		}
		me, merr := ws.IG.GetMe()
		if merr != nil && strings.TrimSpace(tok.UserID) != "" && strings.TrimSpace(tok.UserID) != "<nil>" {
			me, merr = ws.IG.GetUser(tok.UserID)
		}
		if merr != nil {
			ws.IG.ClearToken()
			oauthRedirect(w, r, provider, "err", merr.Error())
			return
		}
		var profile struct {
			Username string `json:"username"`
		}
		_ = json.Unmarshal(me, &profile)
		_ = accounts.UpdateMeta(ws.Meta.ID, func(m *account.Meta) {
			if profile.Username != "" {
				m.InstagramUsername = profile.Username
			}
		})
	}
	oauthRedirect(w, r, provider, "ok", "")
}

func handleMetaDeauthorize(w http.ResponseWriter, r *http.Request, oa *oauth.Config) {
	_ = r.ParseForm()
	signed := strings.TrimSpace(r.FormValue("signed_request"))
	if signed == "" {
		writeErr(w, http.StatusBadRequest, "signed_request wajib")
		return
	}
	userID, err := oa.ParseSignedRequest(signed)
	if err != nil {
		log.Printf("meta deauthorize: %v", err)
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	log.Printf("meta deauthorize user_id=%s", userID)
	w.WriteHeader(http.StatusOK)
}

func handleMetaDataDeletion(w http.ResponseWriter, r *http.Request, oa *oauth.Config) {
	_ = r.ParseForm()
	signed := strings.TrimSpace(r.FormValue("signed_request"))
	if signed == "" {
		writeErr(w, http.StatusBadRequest, "signed_request wajib")
		return
	}
	userID, err := oa.ParseSignedRequest(signed)
	if err != nil {
		log.Printf("meta data-deletion: %v", err)
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	code := oauth.ConfirmationCode(userID)
	log.Printf("meta data-deletion user_id=%s code=%s", userID, code)
	writeJSON(w, http.StatusOK, map[string]any{
		"url":               oa.DataDeletionStatusURI(code),
		"confirmation_code": code,
	})
}

func handleMetaDataDeletionStatus(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		code = "—"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="id"><head><meta charset="utf-8"><title>Status hapus data</title>
<style>body{font-family:system-ui,sans-serif;max-width:36rem;margin:3rem auto;padding:0 1rem;line-height:1.5;color:#111}
code{background:#f3f3f3;padding:.15rem .4rem;border-radius:4px}</style></head><body>
<h1>Permintaan hapus data</h1>
<p>Kode konfirmasi: <code>%s</code></p>
<p>Permintaan penghapusan data yang terkait akun Meta untuk aplikasi malesngonten sudah kami terima dan diproses.</p>
</body></html>`, html.EscapeString(code))
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func wantsChatStream(r *http.Request) bool {
	if r.URL.Query().Get("stream") == "1" {
		return true
	}
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

func handleAIChat(w http.ResponseWriter, r *http.Request, aiClient *ai.Client, thumbClient *ai.ThumbnailClient, stream bool) {
	if !aiClient.Enabled() {
		writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY")
		return
	}
	var req ai.ChatRequest
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	if len(req.Messages) == 0 || len(req.Messages) > 200 {
		writeErr(w, http.StatusBadRequest, "jumlah pesan harus 1–200")
		return
	}
	if len([]rune(req.System)) > 12000 {
		writeErr(w, http.StatusBadRequest, "instruksi sistem terlalu panjang")
		return
	}
	if len(req.PreviousResponseID) > 512 || len(req.ConversationKey) > 64 {
		writeErr(w, http.StatusBadRequest, "state percakapan tidak valid")
		return
	}
	switch strings.ToLower(strings.TrimSpace(req.Reasoning)) {
	case "", "auto", "fast", "low", "medium", "deep", "high":
	default:
		writeErr(w, http.StatusBadRequest, "mode reasoning tidak valid")
		return
	}
	for _, message := range req.Messages {
		if len([]rune(message.Text)) > 200000 {
			writeErr(w, http.StatusBadRequest, "isi pesan terlalu panjang")
			return
		}
		if len(message.Images) > 4 {
			writeErr(w, http.StatusBadRequest, "maksimal 4 gambar per pesan")
			return
		}
		for _, image := range message.Images {
			if len(image) > 12<<20 {
				writeErr(w, http.StatusBadRequest, "ukuran gambar terlalu besar")
				return
			}
		}
	}
	hydrateChatImages(&req)

	if !stream {
		attachAccountToChat(&req)
		result, err := aiClient.ChatContext(r.Context(), req)
		if err != nil {
			var qe *ai.QuotaError
			if errors.As(err, &qe) {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error": qe.Message,
					"kind":  qe.Kind,
					"quota": aiClient.Quota(),
				})
				return
			}
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		if urls := generateChatImages(thumbClient, req); len(urls) > 0 {
			result.Images = urls
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "stream tidak didukung")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	writeEv := func(ev ai.ChatStreamEvent) error {
		raw, _ := json.Marshal(ev)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	if chatNeedsAccountContext(req.Messages) {
		_ = writeEv(ai.ChatStreamEvent{Type: "status", Delta: "Membaca data akun…"})
		attachAccountToChat(&req)
	}

	err := aiClient.ChatStream(r.Context(), req, writeEv)
	if err != nil {
		var qe *ai.QuotaError
		if errors.As(err, &qe) {
			_ = writeEv(ai.ChatStreamEvent{Type: "error", Error: qe.Message})
			return
		}
		_ = writeEv(ai.ChatStreamEvent{Type: "error", Error: err.Error()})
		return
	}
	if req.WantsImage() && thumbClient.Enabled() {
		_ = writeEv(ai.ChatStreamEvent{Type: "status", Delta: "Menggambar…"})
		for _, u := range generateChatImages(thumbClient, req) {
			_ = writeEv(ai.ChatStreamEvent{Type: "image", Image: u})
		}
	}
}

func hydrateChatImages(req *ai.ChatRequest) {
	if req == nil {
		return
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		role := strings.ToLower(strings.TrimSpace(req.Messages[i].Role))
		if role != "user" && role != "" {
			continue
		}
		for j, img := range req.Messages[i].Images {
			if resolved := chatImageToDataURL(img); resolved != "" {
				req.Messages[i].Images[j] = resolved
			}
		}
		break
	}
}

func chatImageToDataURL(img string) string {
	img = strings.TrimSpace(img)
	if img == "" {
		return ""
	}
	if strings.HasPrefix(img, "data:") {
		return img
	}
	rel := img
	if i := strings.Index(rel, "/media/thumbs/"); i >= 0 {
		rel = rel[i:]
	}
	rel = strings.TrimPrefix(rel, "/media/thumbs/")
	rel = strings.TrimPrefix(rel, "/")
	path := accounts.FindThumbMedia(rel)
	if path == "" {
		return img
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return img
	}
	mime := http.DetectContentType(b)
	if !strings.HasPrefix(mime, "image/") {
		mime = "image/png"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
}

func generateChatImages(thumb *ai.ThumbnailClient, req ai.ChatRequest) []string {
	if thumb == nil || !thumb.Enabled() || !req.WantsImage() {
		return nil
	}
	ref := ""
	if imgs := lastChatUserImages(req.Messages); len(imgs) > 0 {
		ref = imgs[0]
	}
	result, err := thumb.GenerateFreeform(req.ImagePrompt(), ref)
	if err != nil {
		log.Printf("chat image: %v", err)
		return nil
	}
	dir := ai.DefaultThumbMediaDir()
	if ws := accounts.Active(); ws != nil && strings.TrimSpace(ws.ThumbDir) != "" {
		dir = ws.ThumbDir
	}
	name, err := ai.SaveThumbnailPNG(dir, result.PNG)
	if err != nil {
		log.Printf("chat image save: %v", err)
		return nil
	}
	return []string{"/media/thumbs/" + name}
}

func lastChatUserImages(msgs []ai.ChatMessage) []string {
	for i := len(msgs) - 1; i >= 0; i-- {
		role := strings.ToLower(strings.TrimSpace(msgs[i].Role))
		if role == "user" || role == "" {
			return msgs[i].Images
		}
	}
	return nil
}

func handleUploadImage(w http.ResponseWriter, r *http.Request) {
	ws := accounts.Active()
	if ws == nil {
		writeErr(w, http.StatusBadRequest, "akun tidak siap")
		return
	}
	const maxBytes = 12 << 20 // 12 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+512)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "file terlalu besar atau form tidak valid (maks 12MB)")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		file, hdr, err = r.FormFile("image")
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, "field file wajib (multipart)")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		// sniff content-type
		ct := strings.ToLower(hdr.Header.Get("Content-Type"))
		switch {
		case strings.Contains(ct, "jpeg"):
			ext = ".jpg"
		case strings.Contains(ct, "png"):
			ext = ".png"
		case strings.Contains(ct, "webp"):
			ext = ".webp"
		case strings.Contains(ct, "gif"):
			ext = ".gif"
		default:
			writeErr(w, http.StatusBadRequest, "format harus jpg/png/webp/gif")
			return
		}
	}

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	head := buf[:n]
	ct := http.DetectContentType(head)
	if !strings.HasPrefix(ct, "image/") {
		writeErr(w, http.StatusBadRequest, "file bukan gambar")
		return
	}
	rest, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "gagal baca file")
		return
	}
	data := append(head, rest...)
	if len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "file kosong")
		return
	}

	day := time.Now().Format("20060102")
	name := fmt.Sprintf("up-%d%s", time.Now().UnixNano(), ext)
	rel := filepath.ToSlash(filepath.Join("uploads", day, name))
	dir := filepath.Join(ws.ThumbDir, "uploads", day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	mediaPath := "/media/thumbs/" + rel
	imageURL := mediaPath
	if base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/"); base != "" {
		imageURL = base + mediaPath
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"image_url": imageURL,
		"path":      mediaPath,
		"bytes":     len(data),
		"filename":  hdr.Filename,
	})
}

// parseScheduleTime accepts RFC3339 or "YYYY-MM-DDTHH:MM" in Asia/Jakarta.
func parseScheduleTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("run_at wajib")
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	for _, layout := range []string{
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("run_at format tidak dikenal (pakai RFC3339 atau 2006-01-02T15:04 WIB)")
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	r := []rune(s)
	if max > 0 && len(r) > max {
		return string(r[:max])
	}
	return s
}

func writeBufferKey(w http.ResponseWriter, r *http.Request, ws *account.Workspace) {
	if ws == nil || ws.Buffer == nil {
		writeErr(w, http.StatusBadRequest, "akun tidak valid")
		return
	}
	var body struct {
		APIKey string `json:"api_key"`
		Key    string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	key := strings.TrimSpace(body.APIKey)
	if key == "" {
		key = strings.TrimSpace(body.Key)
	}
	if key == "" {
		writeErr(w, http.StatusBadRequest, "api_key wajib")
		return
	}
	if err := ws.Buffer.SetAPIKey(key); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	st := ws.Buffer.Status()
	st["account_id"] = ws.Meta.ID
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"enabled": ws.Buffer.Enabled(),
		"status":  st,
	})
}

func clearBufferKey(w http.ResponseWriter, ws *account.Workspace) {
	if ws == nil || ws.Buffer == nil {
		writeErr(w, http.StatusBadRequest, "akun tidak valid")
		return
	}
	if err := ws.Buffer.ClearAPIKey(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	st := ws.Buffer.Status()
	st["account_id"] = ws.Meta.ID
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"enabled": ws.Buffer.Enabled(),
		"status":  st,
	})
}

func oauthRedirectOK(r *http.Request, redirect string) bool {
	u, err := url.Parse(strings.TrimSpace(redirect))
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	reqHost := r.Host
	if u.Host == reqHost {
		return true
	}
	if pub := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")); pub != "" {
		if p, e := url.Parse(pub); e == nil && p.Host != "" && u.Host == p.Host {
			return true
		}
	}
	h := strings.ToLower(u.Hostname())
	if h == "localhost" || h == "127.0.0.1" || strings.HasSuffix(h, ".sslip.io") || strings.HasSuffix(h, ".nip.io") {
		return true
	}
	return false
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	// Baris .env bisa sangat panjang (API key); naikkan buffer default Scanner.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// Dukung "export KEY=val" ala shell.
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		// Strip BOM / karakter aneh di nama key.
		k = strings.TrimPrefix(k, "\ufeff")
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		if k == "" || v == "" {
			continue
		}
		// File .env menang (dotenv): timpa nilai lama dari systemd EnvironmentFile
		// yang sering kosong, terpotong, atau stale — termasuk BUFFER_API_KEY.
		_ = os.Setenv(k, v)
	}
}
