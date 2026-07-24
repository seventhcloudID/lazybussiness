package threads

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const BaseURL = "https://graph.threads.net/v1.0"

type Client struct {
	mu        sync.RWMutex
	token     string
	http      *http.Client
	userID    string
	username  string
	tokenPath string

	answeredMu  sync.Mutex
	answeredSet map[string]bool
	answeredAt  time.Time

	repliesCacheMu sync.Mutex
	repliesCache   map[string]cachedJSON
}

type cachedJSON struct {
	raw json.RawMessage
	at  time.Time
}

func New() *Client {
	c := &Client{
		http:         &http.Client{Timeout: 30 * time.Second},
		tokenPath:    filepath.Join(".data", "access_token"),
		repliesCache: map[string]cachedJSON{},
	}
	c.loadTokenFile()
	return c
}

func (c *Client) SetToken(token string) {
	c.mu.Lock()
	c.token = strings.TrimSpace(token)
	c.userID = ""
	c.username = ""
	c.persistTokenLocked()
	c.mu.Unlock()
	c.InvalidateReplyCaches()
}

func (c *Client) ClearToken() {
	c.mu.Lock()
	c.token = ""
	c.userID = ""
	c.username = ""
	if c.tokenPath != "" {
		_ = os.Remove(c.tokenPath)
	}
	c.mu.Unlock()
	c.InvalidateReplyCaches()
}

func (c *Client) persistTokenLocked() {
	if c.tokenPath == "" {
		return
	}
	if c.token == "" {
		_ = os.Remove(c.tokenPath)
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.tokenPath), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(c.tokenPath, []byte(c.token), 0o600)
}

func (c *Client) loadTokenFile() {
	if c.tokenPath == "" {
		return
	}
	b, err := os.ReadFile(c.tokenPath)
	if err != nil {
		return
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return
	}
	c.token = tok
}

func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

func (c *Client) Connected() bool {
	return c.Token() != ""
}

func (c *Client) UserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

func (c *Client) setUserID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userID = id
}

type APIError struct {
	Status int
	Body   json.RawMessage
}

func (e *APIError) Error() string {
	return fmt.Sprintf("threads api: status %d: %s", e.Status, string(e.Body))
}

func (c *Client) Do(method, path string, query url.Values, form url.Values) (json.RawMessage, error) {
	token := c.Token()
	if token == "" {
		return nil, fmt.Errorf("token belum di-set")
	}

	u, err := url.Parse(BaseURL + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, vs := range query {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("access_token", token)
	u.RawQuery = q.Encode()

	var body io.Reader
	contentType := ""
	if form != nil {
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, &APIError{Status: res.StatusCode, Body: raw}
	}
	return raw, nil
}

func (c *Client) GetMe() (json.RawMessage, error) {
	raw, err := c.Do(http.MethodGet, "/me", url.Values{
		"fields": {"id,username,name,threads_profile_picture_url,threads_biography"},
	}, nil)
	if err != nil {
		return nil, err
	}
	var me struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if json.Unmarshal(raw, &me) == nil {
		c.mu.Lock()
		if me.ID != "" {
			c.userID = me.ID
		}
		if me.Username != "" {
			c.username = me.Username
		}
		c.mu.Unlock()
	}
	return raw, nil
}

// SnapshotForAI mengemas profil + metrik post untuk dianalisis model AI.
func (c *Client) SnapshotForAI(limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 12
	}
	meRaw, err := c.GetMe()
	if err != nil {
		return nil, err
	}
	var me map[string]any
	_ = json.Unmarshal(meRaw, &me)

	detail, err := c.collectPostInsights(limit)
	if err != nil {
		return nil, err
	}

	posts := make([]map[string]any, 0, len(detail.Posts))
	for i, p := range detail.Posts {
		text := strings.TrimSpace(p.Text)
		if len([]rune(text)) > 280 {
			text = string([]rune(text)[:280]) + "…"
		}
		m := p.Metrics
		views := m["views"]
		eng := m["likes"] + m["replies"] + m["reposts"] + m["quotes"]
		var er float64
		if views > 0 {
			er = (eng / views) * 100
		}
		posts = append(posts, map[string]any{
			"rank":           i + 1,
			"id":             p.ID,
			"text":           text,
			"media_type":     p.MediaType,
			"timestamp":      p.Timestamp,
			"views":          m["views"],
			"likes":          m["likes"],
			"replies":        m["replies"],
			"reposts":        m["reposts"],
			"quotes":         m["quotes"],
			"score":          p.Score,
			"engagement":     eng,
			"engagement_rate": er,
		})
	}

	return map[string]any{
		"profile": map[string]any{
			"username":   me["username"],
			"name":       me["name"],
			"biography":  me["threads_biography"],
		},
		"totals": detail.Totals,
		"posts":  posts,
	}, nil
}

func (c *Client) GetThreads(since, until string) (json.RawMessage, error) {
	q := url.Values{
		"fields": {"id,media_type,media_url,thumbnail_url,text,permalink,timestamp,shortcode,has_replies,username,is_reply,replied_to,root_post"},
		"limit":  {"50"},
	}
	if since != "" {
		q.Set("since", since)
	}
	if until != "" {
		q.Set("until", until)
	}
	return c.Do(http.MethodGet, "/me/threads", q, nil)
}

func (c *Client) GetInsights(since, until string) (json.RawMessage, error) {
	return c.GetInsightsOpts(since, until, false)
}

// GetInsightsOpts: aggregate=false → cepat (followers saja untuk ringkasan).
// aggregate=true → engagement akun + breakdown post terbaru untuk halaman Insight.
func (c *Client) GetInsightsOpts(since, until string, aggregate bool) (json.RawMessage, error) {
	type metricItem struct {
		Name       string `json:"name"`
		Period     string `json:"period,omitempty"`
		Title      string `json:"title,omitempty"`
		TotalValue *struct {
			Value any `json:"value"`
		} `json:"total_value,omitempty"`
		Values []struct {
			Value any `json:"value"`
		} `json:"values,omitempty"`
	}
	type envelope struct {
		Data []metricItem `json:"data"`
	}

	var items []metricItem
	var accountErr string
	source := "none"

	// 1) followers_count — cepat
	if folRaw, folErr := c.Do(http.MethodGet, "/me/threads_insights", url.Values{
		"metric": {"followers_count"},
	}, nil); folErr == nil {
		var e envelope
		if json.Unmarshal(folRaw, &e) == nil {
			items = append(items, e.Data...)
		}
	}

	var posts []postInsightRow
	totals := map[string]float64{}

	if aggregate {
		q := url.Values{"metric": {"views,likes,replies,reposts,quotes,clicks"}}
		if since != "" {
			q.Set("since", since)
		}
		if until != "" {
			q.Set("until", until)
		}
		if engRaw, engErr := c.Do(http.MethodGet, "/me/threads_insights", q, nil); engErr == nil {
			var e envelope
			if json.Unmarshal(engRaw, &e) == nil {
				items = append(items, e.Data...)
				source = "account"
			}
		} else {
			accountErr = engErr.Error()
			if apiErr, ok := engErr.(*APIError); ok {
				var parsed struct {
					Error struct {
						UserMsg   string `json:"error_user_msg"`
						UserTitle string `json:"error_user_title"`
						Message   string `json:"message"`
					} `json:"error"`
				}
				if json.Unmarshal(apiErr.Body, &parsed) == nil {
					if parsed.Error.UserMsg != "" {
						accountErr = parsed.Error.UserMsg
					} else if parsed.Error.UserTitle != "" {
						accountErr = parsed.Error.UserTitle
					} else if parsed.Error.Message != "" {
						accountErr = parsed.Error.Message
					}
				}
			}
		}

		// Selalu ambil breakdown post (lebih actionable daripada angka akun saja)
		detail, err := c.collectPostInsights(12)
		if err == nil {
			posts = detail.Posts
			totals = detail.Totals
			hasEngagement := false
			for _, it := range items {
				if it.Name != "followers_count" {
					hasEngagement = true
					break
				}
			}
			if !hasEngagement {
				source = "posts_aggregate"
				accountErr = "Insight engagement level-akun dari Meta tidak tersedia; menampilkan agregasi dari post terbaru."
				for _, name := range []string{"views", "likes", "replies", "reposts", "quotes"} {
					items = append(items, metricItem{
						Name:   name,
						Period: "lifetime",
						Title:  metricTitle(name),
						Values: []struct {
							Value any `json:"value"`
						}{{Value: totals[name]}},
					})
				}
			} else if source == "none" {
				source = "account"
			}
		} else if source == "account" {
			// tetap OK tanpa posts
		} else if accountErr == "" {
			accountErr = err.Error()
		}
	}

	metrics := map[string]float64{}
	for _, it := range items {
		var val float64
		if it.TotalValue != nil {
			val = toFloat(it.TotalValue.Value)
		} else if len(it.Values) > 0 {
			val = toFloat(it.Values[0].Value)
		}
		metrics[it.Name] = val
	}
	if len(totals) == 0 {
		totals = metrics
	}

	views := metrics["views"]
	if views == 0 {
		views = totals["views"]
	}
	eng := metrics["likes"] + metrics["replies"] + metrics["reposts"] + metrics["quotes"]
	if eng == 0 {
		eng = totals["likes"] + totals["replies"] + totals["reposts"] + totals["quotes"]
	}
	var engRate float64
	if views > 0 {
		engRate = (eng / views) * 100
	}

	out := map[string]any{
		"data":            items,
		"source":          source,
		"metrics":         metrics,
		"totals":          totals,
		"engagement":      eng,
		"engagement_rate": engRate,
		"posts":           posts,
		"post_count":      len(posts),
	}
	if accountErr != "" && aggregate {
		out["warning"] = accountErr
	}
	return json.Marshal(out)
}

func metricTitle(name string) string {
	switch name {
	case "views":
		return "Views"
	case "likes":
		return "Likes"
	case "replies":
		return "Balasan"
	case "reposts":
		return "Repost"
	case "quotes":
		return "Kutipan"
	case "clicks":
		return "Klik tautan"
	case "followers_count":
		return "Pengikut"
	default:
		return name
	}
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}

type postInsightRow struct {
	ID        string             `json:"id"`
	Text      string             `json:"text"`
	Permalink string             `json:"permalink,omitempty"`
	Timestamp string             `json:"timestamp,omitempty"`
	MediaType string             `json:"media_type,omitempty"`
	Metrics   map[string]float64 `json:"metrics"`
	Score     float64            `json:"score"`
}

type postInsightDetail struct {
	Posts  []postInsightRow
	Totals map[string]float64
}

func (c *Client) collectPostInsights(limit int) (postInsightDetail, error) {
	out := postInsightDetail{
		Totals: map[string]float64{
			"views": 0, "likes": 0, "replies": 0, "reposts": 0, "quotes": 0,
		},
	}
	if limit <= 0 {
		limit = 12
	}
	threadsRaw, err := c.GetThreads("", "")
	if err != nil {
		return out, err
	}
	var list struct {
		Data []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			Permalink string `json:"permalink"`
			Timestamp string `json:"timestamp"`
			MediaType string `json:"media_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(threadsRaw, &list); err != nil {
		return out, err
	}

	type job struct {
		post postInsightRow
	}
	jobs := make([]job, 0, limit)
	for _, p := range list.Data {
		if strings.EqualFold(p.MediaType, "REPOST_FACADE") || p.ID == "" {
			continue
		}
		jobs = append(jobs, job{post: postInsightRow{
			ID: p.ID, Text: p.Text, Permalink: p.Permalink,
			Timestamp: p.Timestamp, MediaType: p.MediaType,
			Metrics: map[string]float64{},
		}})
		if len(jobs) >= limit {
			break
		}
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	rows := make([]postInsightRow, 0, len(jobs))

	for _, j := range jobs {
		wg.Add(1)
		go func(row postInsightRow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			raw, err := c.GetMediaInsights(row.ID)
			if err != nil {
				return
			}
			var env struct {
				Data []struct {
					Name   string `json:"name"`
					Values []struct {
						Value any `json:"value"`
					} `json:"values"`
				} `json:"data"`
			}
			if json.Unmarshal(raw, &env) != nil {
				return
			}
			for _, m := range env.Data {
				if len(m.Values) == 0 {
					continue
				}
				row.Metrics[m.Name] = toFloat(m.Values[0].Value)
			}
			row.Score = row.Metrics["views"] + row.Metrics["likes"]*3 + row.Metrics["replies"]*4 + row.Metrics["reposts"]*5 + row.Metrics["quotes"]*5

			mu.Lock()
			defer mu.Unlock()
			for k, v := range row.Metrics {
				out.Totals[k] += v
			}
			rows = append(rows, row)
		}(j.post)
	}
	wg.Wait()

	// urutkan skor tertinggi
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].Score > rows[i].Score {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	out.Posts = rows
	return out, nil
}

func (c *Client) aggregatePostInsights(limit int) ([]aggMetric, error) {
	detail, err := c.collectPostInsights(limit)
	if err != nil {
		return nil, err
	}
	var out []aggMetric
	for _, name := range []string{"views", "likes", "replies", "reposts", "quotes"} {
		out = append(out, aggMetric{
			Name: name, Period: "lifetime", Title: metricTitle(name), Value: detail.Totals[name],
		})
	}
	return out, nil
}

type aggMetric struct {
	Name   string
	Period string
	Title  string
	Value  float64
}

func (c *Client) GetQuota() (json.RawMessage, error) {
	return c.Do(http.MethodGet, "/me/threads_publishing_limit", url.Values{
		"fields": {"quota_usage,config,reply_quota_usage,reply_config"},
	}, nil)
}

func (c *Client) GetMentions() (json.RawMessage, error) {
	return c.Do(http.MethodGet, "/me/mentions", url.Values{
		"fields": {"id,text,username,permalink,timestamp"},
		"limit":  {"25"},
	}, nil)
}

// ProbePermissions mengetes endpoint terkait tiap scope (bukan hanya cek token terhubung).
func (c *Client) ProbePermissions() map[string]any {
	type check struct {
		ok  bool
		err string
	}
	run := func(fn func() error) check {
		if err := fn(); err != nil {
			msg := err.Error()
			if apiErr, ok := err.(*APIError); ok {
				var parsed struct {
					Error struct {
						Message string `json:"message"`
						Code    int    `json:"code"`
					} `json:"error"`
				}
				if json.Unmarshal(apiErr.Body, &parsed) == nil && parsed.Error.Message != "" {
					msg = parsed.Error.Message
				}
			}
			return check{ok: false, err: msg}
		}
		return check{ok: true}
	}

	results := map[string]check{
		"threads_basic": run(func() error {
			_, err := c.GetMe()
			return err
		}),
		"threads_content_publish": run(func() error {
			_, err := c.GetQuota()
			return err
		}),
		"threads_manage_insights": run(func() error {
			_, err := c.Do(http.MethodGet, "/me/threads_insights", url.Values{
				"metric": {"followers_count"},
			}, nil)
			return err
		}),
		"threads_read_replies": run(func() error {
			raw, err := c.GetThreads("", "")
			if err != nil {
				return err
			}
			var list struct {
				Data []struct {
					ID        string `json:"id"`
					MediaType string `json:"media_type"`
				} `json:"data"`
			}
			if json.Unmarshal(raw, &list) != nil || len(list.Data) == 0 {
				return nil // tidak ada post untuk diuji → anggap OK jika basic OK
			}
			for _, p := range list.Data {
				if strings.EqualFold(p.MediaType, "REPOST_FACADE") || p.ID == "" {
					continue
				}
				_, err = c.GetReplies(p.ID)
				return err
			}
			return nil
		}),
	}

	out := map[string]any{}
	for k, v := range results {
		item := map[string]any{"ok": v.ok}
		if v.err != "" {
			item["error"] = v.err
		}
		out[k] = item
	}
	// Scope yang tidak bisa diuji tanpa aksi berbahaya
	out["threads_manage_replies"] = map[string]any{"ok": nil, "note": "Tidak diuji otomatis (aksi ubah balasan)"}
	out["threads_delete"] = map[string]any{"ok": nil, "note": "Tidak diuji otomatis (aksi hapus)"}
	return out
}

func (c *Client) KeywordSearch(q, searchType string) (json.RawMessage, error) {
	if searchType == "" {
		searchType = "RECENT"
	}
	return c.Do(http.MethodGet, "/keyword_search", url.Values{
		"q":           {q},
		"search_type": {searchType},
		"fields":      {"id,text,username,permalink,timestamp,media_type"},
	}, nil)
}

func (c *Client) GetReplies(mediaID string) (json.RawMessage, error) {
	return c.Do(http.MethodGet, "/"+mediaID+"/replies", url.Values{
		"fields":  {"id,text,username,timestamp,permalink,hide_status,has_replies,is_reply,is_reply_owned_by_me,replied_to,root_post,media_url,thumbnail_url,media_type"},
		"reverse": {"false"},
	}, nil)
}

// GetConversation returns the full reply tree (all depths) under a post.
func (c *Client) GetConversation(mediaID string) (json.RawMessage, error) {
	return c.Do(http.MethodGet, "/"+mediaID+"/conversation", url.Values{
		"fields":  {"id,text,username,timestamp,permalink,hide_status,has_replies,is_reply,is_reply_owned_by_me,replied_to,root_post,media_url,thumbnail_url,media_type"},
		"reverse": {"false"},
	}, nil)
}

// GetMyReplies returns replies authored by the connected account.
func (c *Client) GetMyReplies(limit string) (json.RawMessage, error) {
	if limit == "" {
		limit = "50"
	}
	return c.Do(http.MethodGet, "/me/replies", url.Values{
		"fields": {"id,text,username,timestamp,permalink,replied_to,root_post,is_reply_owned_by_me"},
		"limit":  {limit},
	}, nil)
}

func (c *Client) InvalidateReplyCaches() {
	c.answeredMu.Lock()
	c.answeredSet = nil
	c.answeredAt = time.Time{}
	c.answeredMu.Unlock()

	c.repliesCacheMu.Lock()
	c.repliesCache = map[string]cachedJSON{}
	c.repliesCacheMu.Unlock()
}

func (c *Client) InvalidateMediaReplies(mediaID string) {
	c.repliesCacheMu.Lock()
	delete(c.repliesCache, mediaID)
	c.repliesCacheMu.Unlock()
	// answered set may be stale after we just replied
	c.answeredMu.Lock()
	c.answeredSet = nil
	c.answeredAt = time.Time{}
	c.answeredMu.Unlock()
}

func (c *Client) loadAnsweredTargets(force bool) map[string]bool {
	c.answeredMu.Lock()
	if !force && c.answeredSet != nil && time.Since(c.answeredAt) < 2*time.Minute {
		out := c.answeredSet
		c.answeredMu.Unlock()
		return out
	}
	c.answeredMu.Unlock()

	answered := map[string]bool{}
	if myRaw, myErr := c.GetMyReplies("100"); myErr == nil {
		var mine struct {
			Data []struct {
				RepliedTo *struct {
					ID string `json:"id"`
				} `json:"replied_to"`
			} `json:"data"`
		}
		if json.Unmarshal(myRaw, &mine) == nil {
			for _, r := range mine.Data {
				if r.RepliedTo != nil && r.RepliedTo.ID != "" {
					answered[r.RepliedTo.ID] = true
				}
			}
		}
	}

	c.answeredMu.Lock()
	c.answeredSet = answered
	c.answeredAt = time.Now()
	c.answeredMu.Unlock()
	return answered
}

// GetRepliesEnriched returns a nested reply tree for inbox UI.
// Prefer a single /conversation call; mark answered from owned nested replies
// and/or cached /me/replies targets. deep is kept for API compat (always nested).
func (c *Client) GetRepliesEnriched(mediaID string, deep, refresh bool) (json.RawMessage, error) {
	_ = deep
	cacheKey := mediaID + ":tree"
	if !refresh {
		c.repliesCacheMu.Lock()
		if ent, ok := c.repliesCache[cacheKey]; ok && time.Since(ent.at) < 45*time.Second {
			raw := ent.raw
			c.repliesCacheMu.Unlock()
			return raw, nil
		}
		c.repliesCacheMu.Unlock()
	}

	var (
		convRaw    json.RawMessage
		convErr    error
		repliesRaw json.RawMessage
		answered   map[string]bool
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		convRaw, convErr = c.GetConversation(mediaID)
		if convErr != nil {
			repliesRaw, convErr = c.GetReplies(mediaID)
		}
	}()
	go func() {
		defer wg.Done()
		answered = c.loadAnsweredTargets(refresh)
	}()
	wg.Wait()
	if convErr != nil {
		return nil, convErr
	}
	if answered == nil {
		answered = map[string]bool{}
	}

	src := convRaw
	if len(src) == 0 {
		src = repliesRaw
	}
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(src, &env); err != nil {
		return nil, err
	}

	type node = map[string]any
	byID := map[string]node{}
	parentOf := map[string]string{}
	order := make([]string, 0, len(env.Data))

	idOf := func(m map[string]any) string {
		id, _ := m["id"].(string)
		return id
	}
	parentID := func(m map[string]any) string {
		if rt, ok := m["replied_to"].(map[string]any); ok {
			id, _ := rt["id"].(string)
			return id
		}
		return ""
	}
	owned := func(m map[string]any) bool {
		if v, ok := m["is_reply_owned_by_me"].(bool); ok && v {
			return true
		}
		c.mu.RLock()
		me := strings.ToLower(c.username)
		c.mu.RUnlock()
		u, _ := m["username"].(string)
		return me != "" && strings.ToLower(u) == me
	}

	for _, item := range env.Data {
		id := idOf(item)
		if id == "" {
			continue
		}
		// shallow copy so we can attach children
		cp := make(node, len(item)+4)
		for k, v := range item {
			cp[k] = v
		}
		cp["children"] = []node{}
		byID[id] = cp
		parentOf[id] = parentID(item)
		order = append(order, id)
		if owned(item) {
			if pid := parentOf[id]; pid != "" {
				answered[pid] = true
			}
		}
	}

	var roots []node
	for _, id := range order {
		n := byID[id]
		pid := parentOf[id]
		if pid == mediaID || pid == "" {
			roots = append(roots, n)
			continue
		}
		if p := byID[pid]; p != nil {
			kids, _ := p["children"].([]node)
			p["children"] = append(kids, n)
			continue
		}
		// parent missing from payload — show as top-level
		roots = append(roots, n)
	}

	var mark func(n node) bool
	mark = func(n node) bool {
		id := idOf(n)
		kids, _ := n["children"].([]node)
		childAnswered := false
		for i := range kids {
			if mark(kids[i]) {
				childAnswered = true
			}
		}
		n["children"] = kids
		isAns := answered[id] || childAnswered || owned(n)
		// own top-level reply to post doesn't count as "answered incoming"
		if owned(n) && parentOf[id] == mediaID {
			isAns = true // our own comment on our post
		}
		if !owned(n) {
			n["answered"] = isAns
			if isAns {
				n["reply_status"] = "answered"
			} else {
				n["reply_status"] = "pending"
			}
		} else {
			n["answered"] = true
			n["reply_status"] = "mine"
			n["is_mine"] = true
		}
		return isAns || owned(n)
	}

	answeredN, pendingN := 0, 0
	incoming := make([]node, 0, len(roots))
	for _, r := range roots {
		mark(r)
		if owned(r) {
			// skip own top-level in incoming list? keep them in tree under filter
			incoming = append(incoming, r)
			continue
		}
		incoming = append(incoming, r)
		if a, _ := r["answered"].(bool); a {
			answeredN++
		} else {
			pendingN++
		}
	}

	out, err := json.Marshal(map[string]any{
		"data":           incoming,
		"media_id":       mediaID,
		"count":          answeredN + pendingN,
		"answered_count": answeredN,
		"pending_count":  pendingN,
		"total_nodes":    len(byID),
		"source":         "conversation",
		"cached":         false,
	})
	if err != nil {
		return nil, err
	}

	c.repliesCacheMu.Lock()
	c.repliesCache[cacheKey] = cachedJSON{raw: out, at: time.Now()}
	c.repliesCacheMu.Unlock()
	return out, nil
}

func (c *Client) ManageReply(replyID string, hide bool) (json.RawMessage, error) {
	hideVal := "false"
	if hide {
		hideVal = "true"
	}
	return c.Do(http.MethodPost, "/"+replyID+"/manage_reply", nil, url.Values{
		"hide": {hideVal},
	})
}

func (c *Client) CreateContainer(params url.Values) (json.RawMessage, error) {
	return c.Do(http.MethodPost, "/me/threads", nil, params)
}

func (c *Client) Publish(creationID string) (json.RawMessage, error) {
	return c.Do(http.MethodPost, "/me/threads_publish", nil, url.Values{
		"creation_id": {creationID},
	})
}

// WaitContainer menunggu status FINISHED/PUBLISHED sebelum threads_publish (hindari 4279009).
func (c *Client) WaitContainer(creationID string, maxWait time.Duration) error {
	creationID = strings.TrimSpace(creationID)
	if creationID == "" {
		return fmt.Errorf("creation_id wajib")
	}
	if maxWait <= 0 {
		maxWait = 90 * time.Second
	}
	deadline := time.Now().Add(maxWait)
	var last string
	for {
		raw, err := c.GetContainerStatus(creationID)
		if err != nil {
			// Resource belum terlihat — tunggu, jangan gagal langsung
			if time.Now().After(deadline) {
				return err
			}
			time.Sleep(2 * time.Second)
			continue
		}
		var out struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(raw, &out)
		last = strings.ToUpper(strings.TrimSpace(out.Status))
		switch last {
		case "FINISHED", "PUBLISHED":
			return nil
		case "ERROR", "EXPIRED":
			return fmt.Errorf("container gagal (%s): %s", last, string(raw))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout menunggu container (status=%s)", last)
		}
		time.Sleep(2 * time.Second)
	}
}

// PublishContainer waits then publishes; retries on media-not-found (4279009).
func (c *Client) PublishContainer(creationID string) (json.RawMessage, error) {
	if err := c.WaitContainer(creationID, 60*time.Second); err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 1; attempt <= 4; attempt++ {
		raw, err := c.Publish(creationID)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		if !isThreadsMediaNotFound(err) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt*2) * time.Second)
	}
	return nil, lastErr
}

func isThreadsMediaNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "4279009") ||
		strings.Contains(s, "Media Tidak Ditemukan") ||
		strings.Contains(s, "Media Not Found") ||
		strings.Contains(s, "does not exist")
}

func (c *Client) GetContainerStatus(id string) (json.RawMessage, error) {
	return c.Do(http.MethodGet, "/"+id, url.Values{
		"fields": {"id,status,error_message"},
	}, nil)
}

func (c *Client) DeleteMedia(id string) (json.RawMessage, error) {
	return c.Do(http.MethodDelete, "/"+id, nil, nil)
}

func (c *Client) Repost(mediaID string) (json.RawMessage, error) {
	return c.Do(http.MethodPost, "/"+mediaID+"/repost", nil, nil)
}

func (c *Client) GetMediaInsights(mediaID string) (json.RawMessage, error) {
	return c.Do(http.MethodGet, "/"+mediaID+"/insights", url.Values{
		"metric": {"views,likes,replies,reposts,quotes"},
	}, nil)
}

func (c *Client) RefreshToken() (json.RawMessage, error) {
	token := c.Token()
	if token == "" {
		return nil, fmt.Errorf("token belum di-set")
	}
	u := "https://graph.threads.net/refresh_access_token?" + url.Values{
		"grant_type":   {"th_refresh_token"},
		"access_token": {token},
	}.Encode()

	res, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, &APIError{Status: res.StatusCode, Body: raw}
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &out); err == nil && out.AccessToken != "" {
		c.SetToken(out.AccessToken)
	}
	return raw, nil
}
