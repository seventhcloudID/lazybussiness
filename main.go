package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"threads-dashboard/internal/ai"
	"threads-dashboard/internal/auth"
	"threads-dashboard/internal/buffer"
	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/lazy"
	"threads-dashboard/internal/threads"
)

func main() {
	loadDotEnv(".env")

	addr := env("PORT", ":8080")
	if addr != "" && !strings.HasPrefix(addr, ":") && !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	client := threads.New()
	ig := instagram.New()
	aiClient := ai.NewFromEnv()
	thumbClient := ai.NewThumbnailFromEnv()
	aiMemory := ai.NewMemoryStore()
	if t := os.Getenv("THREADS_ACCESS_TOKEN"); t != "" {
		client.SetToken(t)
		log.Println("token dimuat dari THREADS_ACCESS_TOKEN")
	} else if client.Connected() {
		log.Println("token dimuat dari .data/access_token")
	}
	if t := os.Getenv("INSTAGRAM_ACCESS_TOKEN"); t != "" {
		ig.SetToken(t)
		log.Println("token Instagram dimuat dari INSTAGRAM_ACCESS_TOKEN")
	} else if ig.Connected() {
		log.Println("token Instagram dimuat dari .data/ig_access_token")
	}
	if aiClient.Enabled() {
		log.Printf("AI insight siap (%s / %s, %d key)", aiClient.Provider(), aiClient.Model(), aiClient.KeyCount())
	} else {
		log.Println("AI insight nonaktif — set AI_API_KEY di .env")
	}
	if thumbClient.Enabled() {
		log.Printf("Thumbnail ChatGPT siap (%s)", thumbClient.Model())
	} else {
		log.Println("Thumbnail ChatGPT nonaktif — set OPENAI_API_KEY di .env")
	}

	lazyStore := lazy.NewStore()
	publicBase := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")
	if tz := strings.TrimSpace(os.Getenv("LAZY_TIMEZONE")); tz != "" {
		cfg := lazyStore.GetConfig()
		if cfg.Timezone == "Asia/Jakarta" || cfg.Timezone == "" {
			cfg.Timezone = tz
			_, _ = lazyStore.SetConfig(cfg)
		}
	}
	buffClient := buffer.NewFromEnv()
	if buffClient != nil && buffClient.Enabled() {
		log.Println("Buffer aktif (TikTok carousel + X thread, Notify Me)")
	} else {
		log.Println("Buffer nonaktif — set BUFFER_API_KEY di .env")
	}

	lazyDeps := &lazy.Deps{
		Store:   lazyStore,
		Threads: client,
		IG:      ig,
		AI:      aiClient,
		Thumb:   thumbClient,
		Buffer:  buffClient,
		Memory:  aiMemory,
		Public:  publicBase,
	}
	lazySched := lazy.NewScheduler(lazyDeps)
	lazySched.Start()

	gate := auth.NewFromEnv()
	if gate.Enabled() {
		log.Println("login aktif (AUTH_USER/AUTH_PASSWORD)")
	} else {
		log.Println("login NONAKTIF — set AUTH_USER + AUTH_PASSWORD di .env (wajib di VPS)")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ringkasan.html", http.StatusFound)
	})
	mux.Handle("GET /", http.FileServer(http.Dir("web")))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"connected":    client.Connected(),
			"ig_connected": ig.Connected(),
			"ai":           aiClient.Enabled(),
			"auth":         gate.Enabled(),
		})
	})

	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":       gate.Enabled(),
			"authenticated": !gate.Enabled() || gate.Valid(r),
		})
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
		if !gate.Check(body.Username, body.Password) {
			writeErr(w, http.StatusUnauthorized, "username/password salah")
			return
		}
		gate.IssueCookie(w, body.Username)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		gate.ClearCookie(w)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"connected":    client.Connected(),
			"user_id":      client.UserID(),
			"ig_connected": ig.Connected(),
			"ig_user_id":   ig.UserID(),
		})
	})

	mux.HandleFunc("POST /api/token", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
			writeErr(w, http.StatusBadRequest, "token wajib diisi")
			return
		}
		client.SetToken(body.Token)
		me, err := client.GetMe()
		if err != nil {
			client.ClearToken()
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "me": json.RawMessage(me)})
	})

	mux.HandleFunc("POST /api/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		raw, err := client.RefreshToken()
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, json.RawMessage(raw))
	})

	mux.HandleFunc("DELETE /api/token", func(w http.ResponseWriter, r *http.Request) {
		client.ClearToken()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		proxy(w, func() (json.RawMessage, error) { return client.GetMe() })
	})

	mux.HandleFunc("GET /api/threads", func(w http.ResponseWriter, r *http.Request) {
		proxy(w, func() (json.RawMessage, error) {
			return client.GetThreads(r.URL.Query().Get("since"), r.URL.Query().Get("until"))
		})
	})

	mux.HandleFunc("GET /api/insights", func(w http.ResponseWriter, r *http.Request) {
		aggregate := r.URL.Query().Get("aggregate") == "1" || r.URL.Query().Get("aggregate") == "true"
		proxy(w, func() (json.RawMessage, error) {
			return client.GetInsightsOpts(r.URL.Query().Get("since"), r.URL.Query().Get("until"), aggregate)
		})
	})

	mux.HandleFunc("POST /api/insights/ai", func(w http.ResponseWriter, r *http.Request) {
		if !client.Connected() {
			writeErr(w, http.StatusUnauthorized, "hubungkan token dulu")
			return
		}
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		snapshot, err := client.SnapshotForAI(12)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		result, err := aiClient.AnalyzeThreads(snapshot)
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

	mux.HandleFunc("GET /api/ai/status", func(w http.ResponseWriter, r *http.Request) {
		out := map[string]any{
			"enabled":  aiClient.Enabled(),
			"provider": aiClient.Provider(),
			"model":    aiClient.Model(),
			"keys":     aiClient.KeysStatus(),
			"thumbnail": map[string]any{
				"enabled":  thumbClient.Enabled(),
				"provider": "openai",
				"model":    thumbClient.Model(),
			},
		}
		if aiClient.Enabled() {
			out["quota"] = aiClient.Quota()
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Gemini / AI API keys — simpan di .data/ai_keys.json (bukan edit .env).
	mux.HandleFunc("GET /api/ai/keys", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, aiClient.KeysStatus())
	})

	mux.HandleFunc("PUT /api/ai/keys", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Keys     []string `json:"keys"`
			KeysText string   `json:"keys_text"`
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
		writeJSON(w, http.StatusOK, aiMemory.Get())
	})

	mux.HandleFunc("PUT /api/ai/instructions", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Instructions string `json:"instructions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if err := aiMemory.SetInstructions(body.Instructions); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": aiMemory.Get()})
	})

	mux.HandleFunc("PUT /api/ai/niche", func(w http.ResponseWriter, r *http.Request) {
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
			err = aiMemory.SetNiches(body.Niches)
		} else {
			err = aiMemory.SetNiche(body.Niche)
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": aiMemory.Get()})
	})

	mux.HandleFunc("PUT /api/ai/brand", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Brand string `json:"brand"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if err := aiMemory.SetBrand(body.Brand); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": aiMemory.Get()})
	})

	mux.HandleFunc("POST /api/ai/memory/refresh", func(w http.ResponseWriter, r *http.Request) {
		if !client.Connected() {
			writeErr(w, http.StatusUnauthorized, "hubungkan token dulu")
			return
		}
		snapshot, err := client.SnapshotForAI(12)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		lessons := ai.BuildLessonsFromSnapshot(snapshot)
		if err := aiMemory.ApplyLessons(lessons); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": aiMemory.Get()})
	})

	mux.HandleFunc("POST /api/ai/memory/reset", func(w http.ResponseWriter, r *http.Request) {
		if err := aiMemory.ResetLearning(); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": aiMemory.Get()})
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
		if err := aiMemory.AddFeedback(body); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": aiMemory.Get()})
	})

	// Thumbnail ChatGPT (OpenAI Images) untuk utas Threads saja — bukan IG carousel.
	mux.HandleFunc("GET /api/ai/thumbnail/defaults", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, thumbClient.Defaults())
	})

	mux.HandleFunc("POST /api/ai/thumbnail", func(w http.ResponseWriter, r *http.Request) {
		if !client.Connected() {
			writeErr(w, http.StatusUnauthorized, "hubungkan token dulu")
			return
		}
		if !thumbClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "Thumbnail ChatGPT belum dikonfigurasi — set OPENAI_API_KEY di .env")
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
		name, err := ai.SaveThumbnailPNG(ai.DefaultThumbMediaDir(), result.PNG)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		rel := "/media/thumbs/" + name
		imageURL := rel
		if publicBase != "" {
			imageURL = publicBase + rel
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"image_url": imageURL,
			"path":      rel,
			"width":     result.Width,
			"height":    result.Height,
			"model":     result.Model,
			"size":      result.Size,
			"prompt":    result.Prompt,
			"note":      "Untuk utas Threads (bagian 1 IMAGE). Bukan untuk carousel IG.",
		})
	})

	mux.HandleFunc("POST /api/ai/generate", func(w http.ResponseWriter, r *http.Request) {
		if !client.Connected() {
			writeErr(w, http.StatusUnauthorized, "hubungkan token dulu")
			return
		}
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		var req ai.GenerateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		mem := aiMemory.Get()
		result, err := aiClient.GenerateContent(nil, mem, req)
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
		if result.DailyFocus != nil {
			// Niche ditentukan user — jangan biarkan AI overwrite fokus dari data.
			if niches := ai.NicheList(mem); len(niches) > 0 {
				result.DailyFocus.Focus = strings.Join(niches, " · ")
			} else {
				result.DailyFocus.Focus = ""
			}
			_ = aiMemory.SetDaily(*result.DailyFocus)
		}
		_ = aiMemory.RecordGeneration(ai.GenHistory{
			Topic:         req.Topic,
			Instructions:  firstNonEmpty(req.Instructions, mem.Instructions),
			Drafts:        result.Drafts,
			Consideration: result.Consideration,
		})
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/quota", func(w http.ResponseWriter, r *http.Request) {
		proxy(w, func() (json.RawMessage, error) { return client.GetQuota() })
	})

	mux.HandleFunc("GET /api/mentions", func(w http.ResponseWriter, r *http.Request) {
		proxy(w, func() (json.RawMessage, error) { return client.GetMentions() })
	})

	mux.HandleFunc("GET /api/permissions", func(w http.ResponseWriter, r *http.Request) {
		if !client.Connected() {
			writeJSON(w, http.StatusOK, map[string]any{"connected": false, "scopes": map[string]any{}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"connected": true,
			"scopes":    client.ProbePermissions(),
		})
	})

	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			writeErr(w, http.StatusBadRequest, "parameter q wajib")
			return
		}
		proxy(w, func() (json.RawMessage, error) {
			return client.KeywordSearch(q, r.URL.Query().Get("search_type"))
		})
	})

	mux.HandleFunc("GET /api/replies", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("media_id")
		if id == "" {
			writeErr(w, http.StatusBadRequest, "media_id wajib")
			return
		}
		proxy(w, func() (json.RawMessage, error) {
			if r.URL.Query().Get("raw") == "1" {
				return client.GetReplies(id)
			}
			deep := r.URL.Query().Get("deep") == "1"
			refresh := r.URL.Query().Get("refresh") == "1"
			return client.GetRepliesEnriched(id, deep, refresh)
		})
	})

	// Inbox: semua komentar masuk yang belum dibalas (dari post terbaru).
	mux.HandleFunc("GET /api/replies/pending", func(w http.ResponseWriter, r *http.Request) {
		maxPosts, _ := strconv.Atoi(r.URL.Query().Get("posts"))
		maxItems, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		proxy(w, func() (json.RawMessage, error) {
			return client.ListPendingInbox(maxPosts, maxItems)
		})
	})

	mux.HandleFunc("POST /api/replies/manage", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ReplyID string `json:"reply_id"`
			Hide    bool   `json:"hide"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ReplyID == "" {
			writeErr(w, http.StatusBadRequest, "reply_id wajib")
			return
		}
		proxy(w, func() (json.RawMessage, error) { return client.ManageReply(body.ReplyID, body.Hide) })
	})

	mux.HandleFunc("POST /api/publish", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MediaType    string `json:"media_type"`
			Text         string `json:"text"`
			ImageURL     string `json:"image_url"`
			VideoURL     string `json:"video_url"`
			ReplyControl string `json:"reply_control"`
			ReplyToID    string `json:"reply_to_id"`
			Publish      bool   `json:"publish"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		if body.MediaType == "" {
			body.MediaType = "TEXT"
		}

		form := url.Values{"media_type": {body.MediaType}}
		if body.Text != "" {
			form.Set("text", body.Text)
		}
		if body.ImageURL != "" {
			form.Set("image_url", body.ImageURL)
		}
		if body.VideoURL != "" {
			form.Set("video_url", body.VideoURL)
		}
		if body.ReplyControl != "" {
			form.Set("reply_control", body.ReplyControl)
		}
		if body.ReplyToID != "" {
			form.Set("reply_to_id", body.ReplyToID)
		}

		container, err := client.CreateContainer(form)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		var created struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(container, &created)

		out := map[string]any{"container": json.RawMessage(container)}
		if body.Publish && created.ID != "" {
			pub, err := client.Publish(created.ID)
			if err != nil {
				writeAPIErr(w, err)
				return
			}
			out["published"] = json.RawMessage(pub)
			if body.ReplyToID != "" {
				client.InvalidateReplyCaches()
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("GET /api/container", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			writeErr(w, http.StatusBadRequest, "id wajib")
			return
		}
		proxy(w, func() (json.RawMessage, error) { return client.GetContainerStatus(id) })
	})

	mux.HandleFunc("POST /api/repost", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MediaID string `json:"media_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MediaID == "" {
			writeErr(w, http.StatusBadRequest, "media_id wajib")
			return
		}
		proxy(w, func() (json.RawMessage, error) { return client.Repost(body.MediaID) })
	})

	mux.HandleFunc("DELETE /api/media/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		proxy(w, func() (json.RawMessage, error) { return client.DeleteMedia(id) })
	})

	mux.HandleFunc("GET /api/media/{id}/insights", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		proxy(w, func() (json.RawMessage, error) { return client.GetMediaInsights(id) })
	})

	// ——— Instagram (token terpisah) ———
	mux.HandleFunc("GET /api/ig/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"connected": ig.Connected(),
			"user_id":   ig.UserID(),
		})
	})

	mux.HandleFunc("POST /api/ig/token", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
			writeErr(w, http.StatusBadRequest, "token Instagram wajib diisi")
			return
		}
		ig.SetToken(body.Token)
		me, err := ig.GetMe()
		if err != nil {
			ig.ClearToken()
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "me": json.RawMessage(me)})
	})

	mux.HandleFunc("POST /api/ig/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		raw, err := ig.RefreshToken()
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, json.RawMessage(raw))
	})

	mux.HandleFunc("POST /api/ig/token/exchange", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
			writeErr(w, http.StatusBadRequest, "short-lived token wajib")
			return
		}
		secret := os.Getenv("INSTAGRAM_APP_SECRET")
		if secret == "" {
			writeErr(w, http.StatusBadRequest, "set INSTAGRAM_APP_SECRET di .env untuk exchange long-lived")
			return
		}
		raw, err := ig.ExchangeLongLived(body.Token, secret)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		me, meErr := ig.GetMe()
		if meErr != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "token": json.RawMessage(raw)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "token": json.RawMessage(raw), "me": json.RawMessage(me)})
	})

	mux.HandleFunc("DELETE /api/ig/token", func(w http.ResponseWriter, r *http.Request) {
		ig.ClearToken()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /api/ig/me", func(w http.ResponseWriter, r *http.Request) {
		proxy(w, func() (json.RawMessage, error) { return ig.GetMe() })
	})

	mux.HandleFunc("GET /api/ig/media", func(w http.ResponseWriter, r *http.Request) {
		proxy(w, func() (json.RawMessage, error) {
			return ig.GetMedia(r.URL.Query().Get("limit"))
		})
	})

	mux.HandleFunc("POST /api/ai/carousel", func(w http.ResponseWriter, r *http.Request) {
		if !aiClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi — set AI_API_KEY di .env")
			return
		}
		var req ai.CarouselRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		mem := aiMemory.Get()
		result, err := aiClient.GenerateCarousel(mem, req)
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

	// Auto-reply: generate draf balasan sesuai intent (preview di UI, kirim via /api/publish).
	mux.HandleFunc("POST /api/ai/replies", func(w http.ResponseWriter, r *http.Request) {
		if !client.Connected() {
			writeErr(w, http.StatusUnauthorized, "hubungkan token dulu")
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
		if intent == "" {
			writeErr(w, http.StatusBadRequest, "intent (arah balasan) wajib")
			return
		}
		mediaID := strings.TrimSpace(body.MediaID)
		postText := strings.TrimSpace(body.PostText)
		incoming := body.Incoming

		if mediaID != "" {
			if postText == "" {
				if raw, err := client.GetThreads("", ""); err == nil {
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
			if len(incoming) == 0 {
				enriched, err := client.GetRepliesEnriched(mediaID, false, true)
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

		mem := aiMemory.Get()
		result, err := aiClient.GenerateReplies(mem, ai.RepliesRequest{
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
		mem := aiMemory.Get()
		gen, err := aiClient.GenerateContent(nil, mem, ai.GenerateRequest{
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
			if niches := ai.NicheList(mem); len(niches) > 0 {
				gen.DailyFocus.Focus = strings.Join(niches, " · ")
			}
			_ = aiMemory.SetDaily(*gen.DailyFocus)
		}
		_ = aiMemory.RecordGeneration(ai.GenHistory{
			Topic:         req.Topic,
			Instructions:  mem.Instructions,
			Drafts:        gen.Drafts,
			Consideration: gen.Consideration,
		})

		var carousel *ai.CarouselResult
		if len(gen.Drafts) > 0 && len(gen.Drafts[0].Parts) >= 2 {
			carousel, _ = aiClient.GenerateCarousel(mem, ai.CarouselRequest{
				Parts: gen.Drafts[0].Parts,
				Brand: mem.Brand,
				Topic: req.Topic,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"date":      time.Now().Format("2006-01-02"),
			"brand":     mem.Brand,
			"threads":   gen,
			"carousel":  carousel,
			"pipeline":  []string{"generate_utas", "post_threads", "carousel_ig"},
		})
	})

	mux.HandleFunc("POST /api/ig/carousel/render", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text  string `json:"text"`
			Brand string `json:"brand"`
			Index int    `json:"index"` // 0-based; opsional
			Total int    `json:"total"` // opsional — tampilkan 01/06 + dots
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
			brand = aiMemory.Get().Brand
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
		dir := filepath.Join(lazyStore.MediaDir(), "_preview")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		path := filepath.Join(dir, fmt.Sprintf("%d.png", time.Now().UnixNano()))
		if err := lazy.RenderSlidePNG(path, brand, text, slideNum, slideTotal); err != nil {
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
		if !ig.Connected() {
			writeErr(w, http.StatusUnauthorized, "hubungkan token Instagram dulu")
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
				brand = aiMemory.Get().Brand
			}
			key := time.Now().Format("2006-01-02") + "/manual-" + fmt.Sprintf("%d", time.Now().Unix()%100000)
			rendered, err := lazy.RenderPartsPublic(lazyStore.MediaDir(), publicBase, brand, key, body.Parts)
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

		out, err := ig.PublishCarousel(urls, body.Caption)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"image_urls": urls,
			"result":    out,
		})
	})

	mux.HandleFunc("GET /api/lazy/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, lazyStore.GetConfig())
	})
	mux.HandleFunc("PUT /api/lazy/config", func(w http.ResponseWriter, r *http.Request) {
		var body lazy.Config
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "body tidak valid")
			return
		}
		cur := lazyStore.GetConfig()
		if strings.TrimSpace(body.Timezone) == "" {
			body.Timezone = cur.Timezone
		}
		cfg, err := lazyStore.SetConfig(body)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		// If just enabled, plan today immediately
		if cfg.Enabled {
			_ = lazySched.EnsureDayPlan(time.Now(), cfg.PostsPerDay, client)
		}
		writeJSON(w, http.StatusOK, cfg)
	})
	mux.HandleFunc("GET /api/lazy/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, lazySched.Status())
	})
	mux.HandleFunc("GET /api/lazy/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		job, ok := lazySched.GetJob(id)
		if !ok {
			writeErr(w, http.StatusNotFound, "job tidak ditemukan")
			return
		}
		writeJSON(w, http.StatusOK, job)
	})
	mux.HandleFunc("POST /api/lazy/run-now", func(w http.ResponseWriter, r *http.Request) {
		job, err := lazySched.RunNow()
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
	mux.HandleFunc("POST /api/lazy/replan", func(w http.ResponseWriter, r *http.Request) {
		if err := lazySched.ReplanToday(); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, lazySched.Status())
	})

	mux.HandleFunc("GET /api/buffer/status", func(w http.ResponseWriter, r *http.Request) {
		ok := buffClient != nil && buffClient.Enabled()
		out := map[string]any{
			"ok":      ok,
			"enabled": ok,
		}
		if ok {
			out["key_hint"] = buffClient.KeyHint()
			tokenOK := false
			if ch, err := buffClient.TikTokChannelID(); err == nil {
				out["tiktok_channel_id"] = ch
				tokenOK = true
			} else {
				out["tiktok_error"] = err.Error()
				if strings.Contains(strings.ToLower(err.Error()), "access token") {
					out["token_ok"] = false
				}
			}
			if ch, err := buffClient.TwitterChannelID(); err == nil {
				out["twitter_channel_id"] = ch
				tokenOK = true
			} else {
				out["twitter_error"] = err.Error()
				if strings.Contains(strings.ToLower(err.Error()), "access token") {
					tokenOK = false
				}
			}
			if _, denied := out["token_ok"]; !denied {
				out["token_ok"] = tokenOK
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Manual: utas teks → Buffer X/Twitter thread (Notify Me).
	mux.HandleFunc("POST /api/buffer/twitter", func(w http.ResponseWriter, r *http.Request) {
		if buffClient == nil || !buffClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "BUFFER_API_KEY belum di-set")
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
		res, err := buffClient.QueueTwitterThread(parts)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"parts":  len(parts),
			"buffer": res,
			"note":   "X Notify Me — selesai post dari notifikasi Buffer di HP",
		})
	})

	// Manual: kirim image URLs + caption ke Buffer TikTok (Notify Me).
	mux.HandleFunc("POST /api/buffer/tiktok", func(w http.ResponseWriter, r *http.Request) {
		if buffClient == nil || !buffClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "BUFFER_API_KEY belum di-set")
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
			mirrored, err := buffer.MirrorPublicURLs(lazyStore.MediaDir(), publicBase, key, urls)
			if err != nil {
				writeErr(w, http.StatusBadGateway, err.Error())
				return
			}
			urls = mirrored
		}
		res, err := buffClient.QueueTikTokPhotos(body.Caption, body.Title, urls)
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
		if buffClient == nil || !buffClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "BUFFER_API_KEY belum di-set")
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
				brand = aiMemory.Get().Brand
			}
			key := time.Now().Format("2006-01-02") + "/buf-car-" + fmt.Sprintf("%d", time.Now().Unix()%100000)
			rendered, err := lazy.RenderPartsPublic(lazyStore.MediaDir(), publicBase, brand, key, body.Parts)
			if err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			urls = rendered
		} else if len(urls) >= 1 {
			key := time.Now().Format("2006-01-02") + "/buf-car-" + fmt.Sprintf("%d", time.Now().Unix()%100000)
			mirrored, err := buffer.MirrorPublicURLs(lazyStore.MediaDir(), publicBase, key, urls)
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
		res, err := buffClient.QueueTikTokPhotos(caption, title, urls)
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
		if buffClient == nil || !buffClient.Enabled() {
			writeErr(w, http.StatusServiceUnavailable, "BUFFER_API_KEY belum di-set")
			return
		}
		if !ig.Connected() {
			writeErr(w, http.StatusUnauthorized, "hubungkan token Instagram dulu")
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
		media, err := ig.GetMediaByID(body.MediaID)
		if err != nil {
			writeAPIErr(w, err)
			return
		}
		srcURLs, err := instagram.ImageURLsFromMedia(media)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		caption := strings.TrimSpace(body.Caption)
		if caption == "" {
			caption = strings.TrimSpace(media.Caption)
		}
		title := strings.TrimSpace(body.Title)
		if title == "" {
			title = firstLine(caption, 80)
		}
		key := "ig-import/" + media.ID
		mirrored, err := buffer.MirrorPublicURLs(lazyStore.MediaDir(), publicBase, key, srcURLs)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "mirror: "+err.Error())
			return
		}
		res, err := buffClient.QueueTikTokPhotos(caption, title, mirrored)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"ig_media_id": media.ID,
			"media_type":  media.MediaType,
			"slides":      len(mirrored),
			"buffer":      res,
			"image_urls":  mirrored,
			"caption":     caption,
			"note":        "TikTok Notify Me — selesai post dari notifikasi Buffer di HP",
		})
	})

	mux.Handle("GET /media/lazy/", http.StripPrefix("/media/lazy/", http.FileServer(http.Dir(lazyStore.MediaDir()))))
	_ = os.MkdirAll(ai.DefaultThumbMediaDir(), 0o755)
	mux.Handle("GET /media/thumbs/", http.StripPrefix("/media/thumbs/", http.FileServer(http.Dir(ai.DefaultThumbMediaDir()))))

	log.Printf("Threads dashboard di http://localhost%s", addr)
	if publicBase != "" {
		log.Printf("PUBLIC_BASE_URL=%s (IG carousel media)", publicBase)
	} else {
		log.Println("PUBLIC_BASE_URL kosong — auto IG carousel akan di-skip")
	}
	if err := http.ListenAndServe(addr, withCORS(gate.Middleware(mux))); err != nil {
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
		w.WriteHeader(apiErr.Status)
		_, _ = w.Write(apiErr.Body)
		return
	}
	var igErr *instagram.APIError
	if errors.As(err, &igErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(igErr.Status)
		_, _ = w.Write(igErr.Body)
		return
	}
	writeErr(w, http.StatusBadGateway, err.Error())
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
