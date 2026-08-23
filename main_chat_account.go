package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"threads-dashboard/internal/ai"
)

type chatAcctBrief struct {
	Text   string
	Handle string
	Posts  int
	At     time.Time
}

var (
	chatAcctMu    sync.Mutex
	chatAcctBuild sync.Mutex
	chatAcctCache = map[string]chatAcctBrief{}
)

func attachAccountToChat(req *ai.ChatRequest) {
	if req == nil || !chatNeedsAccountContext(req.Messages) {
		return
	}
	brief := loadChatAccountBrief()
	if brief.Text != "" {
		req.AccountContext = brief.Text
	}
}

func chatNeedsAccountContext(messages []ai.ChatMessage) bool {
	text := ""
	for i := len(messages) - 1; i >= 0; i-- {
		role := strings.ToLower(strings.TrimSpace(messages[i].Role))
		if role == "user" || role == "" {
			text = strings.ToLower(strings.TrimSpace(messages[i].Text))
			break
		}
	}
	if text == "" {
		return false
	}
	terms := []string{
		"akun", "account", "threads aku", "instagram aku", "ig aku", "profil aku",
		"post aku", "postinganku", "kontenku", "performa", "engagement", "insight",
		"views", "likes", "replies", "followers", "audiens", "audience", "niche",
		"hook terbaik", "post terbaik", "post terburuk", "data real", "data akun",
		"yang nge-hit", "yang viral", "terbaru aku", "brand aku",
	}
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func cacheKey() string {
	if replizActive != nil {
		if id := strings.TrimSpace(replizActive.Get()); id != "" {
			return "rz:" + id
		}
	}
	return "rz:_"
}

func loadChatAccountBrief() chatAcctBrief {
	id := cacheKey()
	chatAcctMu.Lock()
	if cached, ok := chatAcctCache[id]; ok && time.Since(cached.At) < 10*time.Minute && cached.Text != "" {
		chatAcctMu.Unlock()
		return cached
	}
	chatAcctMu.Unlock()

	chatAcctBuild.Lock()
	defer chatAcctBuild.Unlock()

	chatAcctMu.Lock()
	if cached, ok := chatAcctCache[id]; ok && time.Since(cached.At) < 10*time.Minute && cached.Text != "" {
		chatAcctMu.Unlock()
		return cached
	}
	chatAcctMu.Unlock()

	brief := buildChatAccountBrief()
	chatAcctMu.Lock()
	chatAcctCache[id] = brief
	chatAcctMu.Unlock()
	return brief
}

func buildChatAccountBrief() chatAcctBrief {
	out := chatAcctBrief{At: time.Now()}
	in := ai.AccountBriefInput{}
	if m := mem(); m != nil {
		in.Memory = m.Get()
	}

	if replizCli == nil || !replizCli.Ready() {
		out.Text = ai.FormatAccountBrief(in)
		return out
	}

	// Account enrichment must never make chat feel hung. The UI receives an SSE
	// status immediately and this hard cap keeps first-token latency bounded.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	rzID, err := resolveReplizID(ctx, "")
	if err != nil {
		log.Printf("chat account resolve: %v", err)
		out.Text = ai.FormatAccountBrief(in)
		return out
	}
	if snap, err := replizCli.SnapshotForAI(ctx, rzID, 24); err == nil {
		in.Snapshot = snap
		if posts := snap["posts"]; posts != nil {
			switch t := posts.(type) {
			case []any:
				out.Posts = len(t)
			case []map[string]any:
				out.Posts = len(t)
			}
		}
		if prof, _ := snap["profile"].(map[string]any); prof != nil {
			if u, _ := prof["username"].(string); strings.TrimSpace(u) != "" {
				out.Handle = strings.TrimSpace(u)
				in.Handle = out.Handle
			}
			if n, _ := prof["name"].(string); strings.TrimSpace(n) != "" {
				in.Name = n
			}
		}
	} else {
		log.Printf("chat account snapshot: %v", err)
	}

	out.Text = ai.FormatAccountBrief(in)
	return out
}

func handleChatContext(w http.ResponseWriter, r *http.Request) {
	brief := loadChatAccountBrief()
	handle := strings.TrimSpace(brief.Handle)
	if handle != "" && !strings.HasPrefix(handle, "@") {
		handle = "@" + handle
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      brief.Text != "",
		"handle":  handle,
		"posts":   brief.Posts,
		"age_sec": int(time.Since(brief.At).Seconds()),
	})
}
