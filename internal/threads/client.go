package threads

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// earliestInsightsUnix — Meta menolak since/until sebelum 13 Apr 2024.
const earliestInsightsUnix int64 = 1712991600

// insightsFloorUnix: Meta cuma sediakan metrik 2 tahun terakhir (plus frontier Apr 2024).
func insightsFloorUnix(now int64) int64 {
	if now <= 0 {
		now = time.Now().Unix()
	}
	floor := time.Unix(now, 0).UTC().AddDate(-2, 0, 1).Unix()
	if floor < earliestInsightsUnix {
		return earliestInsightsUnix
	}
	return floor
}

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

	insightsCacheMu sync.Mutex
	insightsCache   map[string]cachedJSON
}

type cachedJSON struct {
	raw json.RawMessage
	at  time.Time
}

func New() *Client {
	return NewWithTokenPath(filepath.Join(".data", "access_token"))
}

func NewWithTokenPath(tokenPath string) *Client {
	c := &Client{
		http:          &http.Client{Timeout: 30 * time.Second},
		tokenPath:     tokenPath,
		repliesCache:  map[string]cachedJSON{},
		insightsCache: map[string]cachedJSON{},
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

// SetUserID menyimpan Threads user id dari OAuth.
func (c *Client) SetUserID(id string) {
	c.setUserID(strings.TrimSpace(id))
}

type APIError struct {
	Status int
	Body   json.RawMessage
}

func (e *APIError) Error() string {
	body := strings.TrimSpace(string(e.Body))
	if e.Status == 500 && (body == "" || body == "{}" || body == "null") {
		return "threads api: Meta mengembalikan 500 kosong — biasanya token kedaluwarsa/invalid, akun belum Threads Tester (+ accept invite), atau gangguan sementara di Meta. Coba Login dengan Threads ulang atau tempel long-lived token baru."
	}
	if body == "" {
		return fmt.Sprintf("threads api: status %d (body kosong dari Meta)", e.Status)
	}
	return fmt.Sprintf("threads api: status %d: %s", e.Status, body)
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

// DoGetURL fetches an absolute Graph URL (used for paging.next).
func (c *Client) DoGetURL(rawURL string) (json.RawMessage, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("url kosong")
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
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

// DoPaged follows Graph paging until exhausted (or maxPages).
// Returns a single JSON object {"data":[...]} merging all pages.
func (c *Client) DoPaged(path string, query url.Values, maxPages int) (json.RawMessage, error) {
	if maxPages <= 0 {
		maxPages = 20
	}
	if query == nil {
		query = url.Values{}
	}
	if query.Get("limit") == "" {
		query.Set("limit", "50")
	}

	var all []json.RawMessage
	seen := map[string]bool{}
	raw, err := c.Do(http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	for page := 0; page < maxPages; page++ {
		var env struct {
			Data   []json.RawMessage `json:"data"`
			Paging *struct {
				Cursors *struct {
					After string `json:"after"`
				} `json:"cursors"`
				Next string `json:"next"`
			} `json:"paging"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, err
		}
		for _, item := range env.Data {
			var idWrap struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(item, &idWrap)
			key := idWrap.ID
			if key == "" {
				key = string(item)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, item)
		}
		next := ""
		after := ""
		if env.Paging != nil {
			next = strings.TrimSpace(env.Paging.Next)
			if env.Paging.Cursors != nil {
				after = strings.TrimSpace(env.Paging.Cursors.After)
			}
		}
		if next == "" && after == "" {
			break
		}
		if len(env.Data) == 0 {
			break
		}
		if next != "" {
			raw, err = c.DoGetURL(next)
		} else {
			q := cloneValues(query)
			q.Set("after", after)
			raw, err = c.Do(http.MethodGet, path, q, nil)
		}
		if err != nil {
			return nil, err
		}
	}
	out, err := json.Marshal(map[string]any{"data": all})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func cloneValues(in url.Values) url.Values {
	out := make(url.Values, len(in))
	for k, vs := range in {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}

func (c *Client) applyMe(raw json.RawMessage) {
	var me struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if json.Unmarshal(raw, &me) != nil {
		return
	}
	c.mu.Lock()
	if me.ID != "" {
		c.userID = me.ID
	}
	if me.Username != "" {
		c.username = me.Username
	}
	c.mu.Unlock()
}

func (c *Client) GetUser(id string) (json.RawMessage, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("user id kosong")
	}
	raw, err := c.Do(http.MethodGet, "/"+id, url.Values{
		"fields": {"id,username,name,threads_profile_picture_url,threads_biography"},
	}, nil)
	if err != nil {
		return nil, err
	}
	c.applyMe(raw)
	return raw, nil
}

func (c *Client) GetMe() (json.RawMessage, error) {
	if id := strings.TrimSpace(c.UserID()); id != "" {
		if raw, err := c.GetUser(id); err == nil {
			return raw, nil
		}
	}
	fieldSets := []string{
		"id,username",
		"id,username,name,threads_profile_picture_url,threads_biography",
	}
	var last error
	for _, fields := range fieldSets {
		raw, err := c.Do(http.MethodGet, "/me", url.Values{"fields": {fields}}, nil)
		if err == nil {
			c.applyMe(raw)
			return raw, nil
		}
		last = err
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == 500 {
			continue
		}
		return nil, err
	}
	return nil, last
}

// SnapshotForAI mengemas profil + metrik post untuk dianalisis model AI.
func (c *Client) SnapshotForAI(limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 40
	}
	meRaw, err := c.GetMe()
	if err != nil {
		return nil, err
	}
	var me map[string]any
	_ = json.Unmarshal(meRaw, &me)

	detail, err := c.collectPostInsights(limit, "", "")
	if err != nil {
		return nil, err
	}

	formatMix := map[string]int{}
	weekdayMix := map[string]int{}
	hourMix := map[string]int{}
	var charLens []int
	var sumER, sumViews, sumEng float64
	erN := 0

	posts := make([]map[string]any, 0, len(detail.Posts))
	for i, p := range detail.Posts {
		text := strings.TrimSpace(p.Text)
		chars := utf8.RuneCountInString(text)
		charLens = append(charLens, chars)
		if chars > 700 {
			text = string([]rune(text)[:700]) + "…"
		}
		m := p.Metrics
		views := m["views"]
		eng := m["likes"] + m["replies"] + m["reposts"] + m["quotes"]
		var er float64
		if views > 0 {
			er = (eng / views) * 100
			sumER += er
			erN++
		}
		sumViews += views
		sumEng += eng

		mt := strings.TrimSpace(p.MediaType)
		if mt == "" {
			mt = "TEXT"
		}
		formatMix[mt]++

		weekday, hour, bucket := "", 0, ""
		if ts := postTimestampUnix(p.Timestamp); ts > 0 {
			t := time.Unix(ts, 0).In(time.Local)
			weekday = t.Weekday().String()
			hour = t.Hour()
			switch {
			case hour < 6:
				bucket = "dini_hari"
			case hour < 12:
				bucket = "pagi"
			case hour < 17:
				bucket = "siang"
			case hour < 21:
				bucket = "sore"
			default:
				bucket = "malam"
			}
			weekdayMix[weekday]++
			hourMix[bucket]++
		}

		posts = append(posts, map[string]any{
			"rank":            i + 1,
			"id":              p.ID,
			"text":            text,
			"chars":           chars,
			"media_type":      mt,
			"timestamp":       p.Timestamp,
			"weekday":         weekday,
			"hour":            hour,
			"daypart":         bucket,
			"permalink":       p.Permalink,
			"views":           m["views"],
			"likes":           m["likes"],
			"replies":         m["replies"],
			"reposts":         m["reposts"],
			"quotes":          m["quotes"],
			"score":           p.Score,
			"engagement":      eng,
			"engagement_rate": er,
		})
	}

	avgER := 0.0
	if erN > 0 {
		avgER = sumER / float64(erN)
	}
	medianChars, meanChars := 0, 0
	if n := len(charLens); n > 0 {
		s := 0
		for _, v := range charLens {
			s += v
		}
		meanChars = s / n
		cp := append([]int(nil), charLens...)
		sort.Ints(cp)
		medianChars = cp[n/2]
	}

	accountMetrics := map[string]any{}
	now := time.Now().Unix()
	since30 := now - 30*24*60*60
	if floor := insightsFloorUnix(now); since30 < floor {
		since30 = floor
	}
	if raw, err := c.GetInsightsOpts(strconv.FormatInt(since30, 10), strconv.FormatInt(now, 10), true, 0); err == nil {
		var wrap struct {
			Metrics map[string]float64 `json:"metrics"`
			Source  string             `json:"source"`
		}
		if json.Unmarshal(raw, &wrap) == nil {
			accountMetrics["source"] = wrap.Source
			accountMetrics["last_30d"] = wrap.Metrics
		}
	}

	// Ranking & distribusi biar AI tidak tebak buta.
	type scored struct {
		ID    string
		Score float64
		Views float64
		ER    float64
	}
	ranked := make([]scored, 0, len(posts))
	var viewsArr []float64
	var replySum float64
	for _, p := range posts {
		id, _ := p["id"].(string)
		sc, _ := p["score"].(float64)
		vw, _ := p["views"].(float64)
		er, _ := p["engagement_rate"].(float64)
		re, _ := p["replies"].(float64)
		ranked = append(ranked, scored{ID: id, Score: sc, Views: vw, ER: er})
		viewsArr = append(viewsArr, vw)
		replySum += re
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })
	topIDs, bottomIDs := []string{}, []string{}
	for i := 0; i < len(ranked) && i < 5; i++ {
		topIDs = append(topIDs, ranked[i].ID)
	}
	for i := len(ranked) - 1; i >= 0 && len(bottomIDs) < 5; i-- {
		bottomIDs = append(bottomIDs, ranked[i].ID)
	}
	sort.Float64s(viewsArr)
	percentile := func(p float64) float64 {
		if len(viewsArr) == 0 {
			return 0
		}
		idx := int(float64(len(viewsArr)-1) * p)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(viewsArr) {
			idx = len(viewsArr) - 1
		}
		return viewsArr[idx]
	}
	replyPerPost := 0.0
	if n := len(posts); n > 0 {
		replyPerPost = replySum / float64(n)
	}

	return map[string]any{
		"profile": map[string]any{
			"username":  me["username"],
			"name":      me["name"],
			"biography": me["threads_biography"],
		},
		"account_metrics": accountMetrics,
		"sample": map[string]any{
			"post_count":       len(posts),
			"totals":           detail.Totals,
			"avg_er_pct":       avgER,
			"sum_views":        sumViews,
			"sum_engagement":   sumEng,
			"format_mix":       formatMix,
			"weekday_mix":      weekdayMix,
			"daypart_mix":      hourMix,
			"median_chars":     medianChars,
			"mean_chars":       meanChars,
			"views_p25":        percentile(0.25),
			"views_p50":        percentile(0.50),
			"views_p75":        percentile(0.75),
			"avg_replies":      replyPerPost,
			"top_score_ids":    topIDs,
			"bottom_score_ids": bottomIDs,
		},
		"posts": posts,
	}, nil
}

func threadsListQuery(since, until string) url.Values {
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
	return q
}

func (c *Client) GetThreads(since, until string) (json.RawMessage, error) {
	return c.Do(http.MethodGet, "/me/threads", threadsListQuery(since, until), nil)
}

// GetThreadsPaged mengikuti paging Graph sampai maxPages (atau habis).
func (c *Client) GetThreadsPaged(since, until string, maxPages int) (json.RawMessage, error) {
	if maxPages <= 0 {
		maxPages = 8
	}
	return c.DoPaged("/me/threads", threadsListQuery(since, until), maxPages)
}

func (c *Client) GetInsights(since, until string) (json.RawMessage, error) {
	return c.GetInsightsOpts(since, until, false, -1)
}

// GetInsightsOpts: aggregate=false → cepat (followers saja).
// aggregate=true → engagement akun + breakdown post.
// postLimit: <0 default 40; 0 skip breakdown post; >0 batasi jumlah post.
func (c *Client) GetInsightsOpts(since, until string, aggregate bool, postLimit int) (json.RawMessage, error) {
	cacheKey := fmt.Sprintf("%t|%d|%s|%s", aggregate, postLimit, since, until)
	c.insightsCacheMu.Lock()
	if ent, ok := c.insightsCache[cacheKey]; ok && time.Since(ent.at) < 90*time.Second {
		raw := ent.raw
		c.insightsCacheMu.Unlock()
		return raw, nil
	}
	c.insightsCacheMu.Unlock()

	raw, err := c.fetchInsightsOpts(since, until, aggregate, postLimit)
	if err != nil {
		return nil, err
	}
	c.insightsCacheMu.Lock()
	c.insightsCache[cacheKey] = cachedJSON{raw: raw, at: time.Now()}
	c.insightsCacheMu.Unlock()
	return raw, nil
}

func (c *Client) fetchInsightsOpts(since, until string, aggregate bool, postLimit int) (json.RawMessage, error) {
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

	valueOf := func(it metricItem) float64 {
		if it.TotalValue != nil {
			return toFloat(it.TotalValue.Value)
		}
		var sum float64
		for _, v := range it.Values {
			sum += toFloat(v.Value)
		}
		return sum
	}
	hasEngagementItems := func() bool {
		for _, it := range items {
			if it.Name != "" && it.Name != "followers_count" {
				return true
			}
		}
		return false
	}

	if aggregate {
		// Clamp ke frontier Meta + isi default "semua" kalau kosong (bukan 2 hari Meta).
		since, until = normalizeInsightRange(since, until)
		fetch := func(metric, s, u string) ([]metricItem, error) {
			q := url.Values{"metric": {metric}}
			if s != "" {
				q.Set("since", s)
			}
			if u != "" {
				q.Set("until", u)
			}
			raw, err := c.Do(http.MethodGet, "/me/threads_insights", q, nil)
			if err != nil {
				q.Set("period", "day")
				raw, err = c.Do(http.MethodGet, "/me/threads_insights", q, nil)
				if err != nil {
					return nil, err
				}
			}
			var e envelope
			if json.Unmarshal(raw, &e) != nil {
				return nil, fmt.Errorf("gagal parse threads_insights")
			}
			return e.Data, nil
		}
		fetchRange := func(metric string) ([]metricItem, error) {
			got, err := fetch(metric, since, until)
			if err == nil {
				return got, nil
			}
			chunks := insightRangeChunks(since, until)
			if len(chunks) <= 1 {
				return nil, err
			}
			var all []metricItem
			for _, ch := range chunks {
				part, perr := fetch(metric, ch[0], ch[1])
				if perr != nil {
					return nil, err
				}
				all = append(all, part...)
			}
			return all, nil
		}
		appendMetrics := func(metric string) {
			got, err := fetchRange(metric)
			if err != nil {
				if accountErr == "" {
					accountErr = threadsInsightUserMsg(err)
				}
				return
			}
			items = append(items, got...)
		}

		// Meta menolak campur time-series (views), total value, dan clicks dalam 1 request.
		appendMetrics("likes,replies,reposts,quotes")
		if !hasEngagementItems() {
			appendMetrics("likes,replies,reposts")
		}
		appendMetrics("views")
		appendMetrics("clicks")
		if hasEngagementItems() {
			source = "account"
			accountErr = ""
		}

		// Breakdown post mengikuti rentang since/until.
		if postLimit < 0 {
			postLimit = 100
		}
		if postLimit > 0 {
			detail, err := c.collectPostInsights(postLimit, since, until)
			if err == nil {
				posts = detail.Posts
				totals = detail.Totals
				hasEngagement := hasEngagementItems()
				if !hasEngagement {
					source = "posts_aggregate"
					if accountErr == "" {
						accountErr = "Insight engagement level-akun dari Meta tidak tersedia; menampilkan agregasi dari post terbaru."
					} else {
						accountErr = accountErr + " — menampilkan agregasi dari post."
					}
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
	}

	// Satukan metrik sejenis (chunk 30 hari / time-series views).
	{
		order := make([]string, 0, len(items))
		sum := map[string]float64{}
		title := map[string]string{}
		period := map[string]string{}
		seen := map[string]bool{}
		for _, it := range items {
			name := strings.TrimSpace(it.Name)
			if name == "" {
				continue
			}
			if !seen[name] {
				seen[name] = true
				order = append(order, name)
			}
			sum[name] += valueOf(it)
			if it.Title != "" {
				title[name] = it.Title
			}
			if it.Period != "" {
				period[name] = it.Period
			}
		}
		merged := make([]metricItem, 0, len(order))
		for _, name := range order {
			it := metricItem{Name: name, Period: period[name], Title: title[name]}
			v := sum[name]
			it.Values = []struct {
				Value any `json:"value"`
			}{{Value: v}}
			merged = append(merged, it)
		}
		items = merged
	}

	metrics := map[string]float64{}
	for _, it := range items {
		metrics[it.Name] = valueOf(it)
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
	if aggregate {
		out["since"] = since
		out["until"] = until
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

// normalizeInsightRange: kosong = sejak frontier Meta → sekarang (bukan default 2 hari API).
// until tidak boleh > now — Meta menolak timestamp masa depan (mis. akhir hari lokal).
func normalizeInsightRange(since, until string) (string, string) {
	now := time.Now().Unix()
	floor := insightsFloorUnix(now)
	s := parseUnixBound(since)
	u := parseUnixBound(until)
	if s <= 0 && u <= 0 {
		return strconv.FormatInt(floor, 10), strconv.FormatInt(now, 10)
	}
	if s <= 0 || s < floor {
		s = floor
	}
	if u <= 0 || u > now {
		u = now
	}
	if u < s {
		u = s
	}
	return strconv.FormatInt(s, 10), strconv.FormatInt(u, 10)
}

func insightRangeChunks(since, until string) [][2]string {
	s := parseUnixBound(since)
	u := parseUnixBound(until)
	if s <= 0 || u <= 0 || u <= s {
		return nil
	}
	const maxSpan = int64(30 * 24 * 60 * 60)
	if u-s <= maxSpan {
		return nil
	}
	var chunks [][2]string
	for cur := s; cur < u; {
		end := cur + maxSpan
		if end > u {
			end = u
		}
		chunks = append(chunks, [2]string{
			strconv.FormatInt(cur, 10),
			strconv.FormatInt(end, 10),
		})
		cur = end
	}
	return chunks
}

func threadsInsightUserMsg(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		var parsed struct {
			Error struct {
				UserMsg   string `json:"error_user_msg"`
				UserTitle string `json:"error_user_title"`
				Message   string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(apiErr.Body, &parsed) == nil {
			if parsed.Error.UserMsg != "" {
				return parsed.Error.UserMsg
			}
			if parsed.Error.UserTitle != "" {
				return parsed.Error.UserTitle
			}
			if parsed.Error.Message != "" {
				return parsed.Error.Message
			}
		}
	}
	return err.Error()
}

func parseUnixBound(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func postTimestampUnix(ts string) int64 {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Unix()
	}
	if t, err := time.Parse("2006-01-02T15:04:05+0000", ts); err == nil {
		return t.Unix()
	}
	return 0
}

func (c *Client) collectPostInsights(limit int, since, until string) (postInsightDetail, error) {
	out := postInsightDetail{
		Totals: map[string]float64{
			"views": 0, "likes": 0, "replies": 0, "reposts": 0, "quotes": 0,
		},
	}
	if limit <= 0 {
		limit = 100
	}
	since, until = normalizeInsightRange(since, until)
	maxPages := limit/25 + 4
	if maxPages < 6 {
		maxPages = 6
	}
	if maxPages > 20 {
		maxPages = 20
	}
	threadsRaw, err := c.GetThreadsPaged(since, until, maxPages)
	if err != nil {
		// Fallback: ambil tanpa filter API, filter tanggal di sisi kita
		threadsRaw, err = c.GetThreadsPaged("", "", maxPages)
		if err != nil {
			return out, err
		}
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

	sinceU := parseUnixBound(since)
	untilU := parseUnixBound(until)

	type job struct {
		post postInsightRow
	}
	jobs := make([]job, 0, limit)
	for _, p := range list.Data {
		if strings.EqualFold(p.MediaType, "REPOST_FACADE") || p.ID == "" {
			continue
		}
		if ts := postTimestampUnix(p.Timestamp); ts > 0 {
			if sinceU > 0 && ts < sinceU {
				continue
			}
			if untilU > 0 && ts > untilU {
				continue
			}
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
	sem := make(chan struct{}, 8)
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
	detail, err := c.collectPostInsights(limit, "", "")
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
	return c.DoPaged("/"+mediaID+"/replies", url.Values{
		"fields":  {"id,text,username,timestamp,permalink,hide_status,has_replies,is_reply,is_reply_owned_by_me,replied_to,root_post,media_url,thumbnail_url,media_type"},
		"reverse": {"false"},
		"limit":   {"50"},
	}, 20)
}

// GetConversation returns the full reply tree (all depths) under a post.
func (c *Client) GetConversation(mediaID string) (json.RawMessage, error) {
	return c.DoPaged("/"+mediaID+"/conversation", url.Values{
		"fields":  {"id,text,username,timestamp,permalink,hide_status,has_replies,is_reply,is_reply_owned_by_me,replied_to,root_post,media_url,thumbnail_url,media_type"},
		"reverse": {"false"},
		"limit":   {"50"},
	}, 20)
}

// GetMedia fetches a single Threads media object.
func (c *Client) GetMedia(mediaID string) (json.RawMessage, error) {
	return c.Do(http.MethodGet, "/"+mediaID, url.Values{
		"fields": {"id,text,username,timestamp,permalink,is_reply,replied_to,root_post,has_replies,media_type"},
	}, nil)
}

// ThreadParts is root + owned reply chain (utas) as ordered texts.
type ThreadParts struct {
	RootID string   `json:"root_id"`
	Parts  []string `json:"parts"`
	IDs    []string `json:"ids,omitempty"`
	Count  int      `json:"count"`
}

// GetOwnedThreadParts returns root post text + semua balasan milik sendiri
// (bagian utas), urut waktu — cocok untuk carousel / compose.
func (c *Client) GetOwnedThreadParts(mediaID string) (ThreadParts, error) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return ThreadParts{}, fmt.Errorf("media_id wajib")
	}

	raw, err := c.GetMedia(mediaID)
	if err != nil {
		return ThreadParts{}, err
	}
	var media struct {
		ID       string `json:"id"`
		Text     string `json:"text"`
		IsReply  bool   `json:"is_reply"`
		RootPost *struct {
			ID string `json:"id"`
		} `json:"root_post"`
	}
	if err := json.Unmarshal(raw, &media); err != nil {
		return ThreadParts{}, err
	}

	rootID := media.ID
	if media.IsReply && media.RootPost != nil && media.RootPost.ID != "" {
		rootID = media.RootPost.ID
		rootRaw, err := c.GetMedia(rootID)
		if err != nil {
			return ThreadParts{}, err
		}
		if err := json.Unmarshal(rootRaw, &media); err != nil {
			return ThreadParts{}, err
		}
	}

	out := ThreadParts{RootID: rootID}
	rootText := strings.TrimSpace(media.Text)
	if rootText != "" {
		out.Parts = append(out.Parts, rootText)
		out.IDs = append(out.IDs, rootID)
	}

	convRaw, convErr := c.GetConversation(rootID)
	if convErr != nil {
		// Tanpa conversation, kembalikan root saja.
		out.Count = len(out.Parts)
		return out, nil
	}
	var env struct {
		Data []struct {
			ID               string `json:"id"`
			Text             string `json:"text"`
			Username         string `json:"username"`
			Timestamp        string `json:"timestamp"`
			IsReplyOwnedByMe bool   `json:"is_reply_owned_by_me"`
			RepliedTo        *struct {
				ID string `json:"id"`
			} `json:"replied_to"`
		} `json:"data"`
	}
	if json.Unmarshal(convRaw, &env) != nil || len(env.Data) == 0 {
		out.Count = len(out.Parts)
		return out, nil
	}

	c.mu.RLock()
	me := strings.ToLower(c.username)
	c.mu.RUnlock()

	type node struct {
		id, text, parent, ts string
	}
	var owned []node
	seen := map[string]bool{rootID: true}
	for _, it := range env.Data {
		id := strings.TrimSpace(it.ID)
		if id == "" || seen[id] {
			continue
		}
		mine := it.IsReplyOwnedByMe
		if !mine && me != "" && strings.ToLower(it.Username) == me {
			mine = true
		}
		if !mine {
			continue
		}
		text := strings.TrimSpace(it.Text)
		if text == "" {
			continue
		}
		parent := ""
		if it.RepliedTo != nil {
			parent = it.RepliedTo.ID
		}
		owned = append(owned, node{id: id, text: text, parent: parent, ts: it.Timestamp})
		seen[id] = true
	}

	// Utas berantai: ikuti reply_to milik sendiri dari root.
	byParent := map[string][]node{}
	for _, n := range owned {
		byParent[n.parent] = append(byParent[n.parent], n)
	}
	for p := range byParent {
		sort.Slice(byParent[p], func(i, j int) bool {
			return byParent[p][i].ts < byParent[p][j].ts
		})
	}

	current := rootID
	chained := map[string]bool{}
	for len(out.Parts) < 20 {
		kids := byParent[current]
		if len(kids) == 0 {
			break
		}
		next := kids[0]
		if chained[next.id] {
			break
		}
		out.Parts = append(out.Parts, next.text)
		out.IDs = append(out.IDs, next.id)
		chained[next.id] = true
		current = next.id
	}

	// Fallback: kalau belum kebawa semua milik sendiri (reply flat ke root),
	// tambahkan sisanya urut waktu.
	if len(chained) < len(owned) {
		sort.Slice(owned, func(i, j int) bool { return owned[i].ts < owned[j].ts })
		for _, n := range owned {
			if chained[n.id] {
				continue
			}
			out.Parts = append(out.Parts, n.text)
			out.IDs = append(out.IDs, n.id)
			if len(out.Parts) >= 20 {
				break
			}
		}
	}

	out.Count = len(out.Parts)
	return out, nil
}

// GetMyReplies returns replies authored by the connected account.
func (c *Client) GetMyReplies(limit string) (json.RawMessage, error) {
	if limit == "" {
		limit = "50"
	}
	return c.DoPaged("/me/replies", url.Values{
		"fields": {"id,text,username,timestamp,permalink,replied_to,root_post,is_reply_owned_by_me"},
		"limit":  {limit},
	}, 3)
}

func (c *Client) InvalidateReplyCaches() {
	c.answeredMu.Lock()
	c.answeredSet = nil
	c.answeredAt = time.Time{}
	c.answeredMu.Unlock()

	c.repliesCacheMu.Lock()
	c.repliesCache = map[string]cachedJSON{}
	c.repliesCacheMu.Unlock()

	c.insightsCacheMu.Lock()
	c.insightsCache = map[string]cachedJSON{}
	c.insightsCacheMu.Unlock()
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

	var mark func(n node)
	mark = func(n node) {
		id := idOf(n)
		kids, _ := n["children"].([]node)
		for i := range kids {
			mark(kids[i])
		}
		n["children"] = kids
		if owned(n) {
			n["answered"] = true
			n["reply_status"] = "mine"
			n["is_mine"] = true
			return
		}
		// Hanya "answered" bila kita membalas LANGSUNG ke node ini —
		// jangan wariskan dari anak, supaya nested pending tetap terlihat.
		isAns := answered[id]
		n["answered"] = isAns
		if isAns {
			n["reply_status"] = "answered"
		} else {
			n["reply_status"] = "pending"
		}
	}

	answeredN, pendingN := 0, 0
	var walkCount func(n node)
	walkCount = func(n node) {
		if !owned(n) {
			if a, _ := n["answered"].(bool); a {
				answeredN++
			} else {
				pendingN++
			}
		}
		kids, _ := n["children"].([]node)
		for i := range kids {
			walkCount(kids[i])
		}
	}

	incoming := make([]node, 0, len(roots))
	for _, r := range roots {
		mark(r)
		incoming = append(incoming, r)
		walkCount(r)
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

// ListPendingInbox mengumpulkan komentar masuk yang belum dibalas dari post terbaru.
func (c *Client) ListPendingInbox(maxPosts, maxItems int) (json.RawMessage, error) {
	if maxPosts <= 0 {
		maxPosts = 20
	}
	if maxPosts > 40 {
		maxPosts = 40
	}
	if maxItems <= 0 {
		maxItems = 40
	}
	if maxItems > 80 {
		maxItems = 80
	}

	threadsRaw, err := c.GetThreads("", "")
	if err != nil {
		return nil, err
	}
	var list struct {
		Data []struct {
			ID         string `json:"id"`
			Text       string `json:"text"`
			Timestamp  string `json:"timestamp"`
			MediaType  string `json:"media_type"`
			HasReplies bool   `json:"has_replies"`
			IsReply    bool   `json:"is_reply"`
			RepliedTo  *struct {
				ID string `json:"id"`
			} `json:"replied_to"`
			Permalink string `json:"permalink"`
		} `json:"data"`
	}
	if err := json.Unmarshal(threadsRaw, &list); err != nil {
		return nil, err
	}

	type postRef struct {
		ID, Text, Timestamp, Permalink string
	}
	var candidates []postRef
	for _, p := range list.Data {
		if strings.EqualFold(p.MediaType, "REPOST_FACADE") || p.ID == "" {
			continue
		}
		if p.IsReply || (p.RepliedTo != nil && p.RepliedTo.ID != "") {
			continue
		}
		if !p.HasReplies {
			continue
		}
		candidates = append(candidates, postRef{
			ID: p.ID, Text: p.Text, Timestamp: p.Timestamp, Permalink: p.Permalink,
		})
		if len(candidates) >= maxPosts {
			break
		}
	}

	type pendingItem struct {
		ID            string `json:"id"`
		Username      string `json:"username"`
		Text          string `json:"text"`
		Timestamp     string `json:"timestamp"`
		Permalink     string `json:"permalink,omitempty"`
		MediaID       string `json:"media_id"`
		PostText      string `json:"post_text"`
		PostTimestamp string `json:"post_timestamp,omitempty"`
		PostPermalink string `json:"post_permalink,omitempty"`
	}

	items := make([]pendingItem, 0, maxItems)
	scanned := 0
	for _, post := range candidates {
		if len(items) >= maxItems {
			break
		}
		raw, err := c.GetRepliesEnriched(post.ID, false, false)
		scanned++
		if err != nil {
			continue
		}
		type treeNode struct {
			ID        string     `json:"id"`
			Username  string     `json:"username"`
			Text      string     `json:"text"`
			Timestamp string     `json:"timestamp"`
			Permalink string     `json:"permalink"`
			IsMine    bool       `json:"is_mine"`
			Answered  bool       `json:"answered"`
			Children  []treeNode `json:"children"`
		}
		var roots []treeNode
		if err := json.Unmarshal(raw, &struct {
			Data *[]treeNode `json:"data"`
		}{Data: &roots}); err != nil {
			continue
		}
		var walk func([]treeNode)
		walk = func(nodes []treeNode) {
			for _, n := range nodes {
				if len(items) >= maxItems {
					return
				}
				if !n.IsMine && !n.Answered && strings.TrimSpace(n.ID) != "" {
					items = append(items, pendingItem{
						ID: n.ID, Username: n.Username, Text: n.Text, Timestamp: n.Timestamp,
						Permalink: n.Permalink, MediaID: post.ID, PostText: post.Text,
						PostTimestamp: post.Timestamp, PostPermalink: post.Permalink,
					})
				}
				if len(n.Children) > 0 {
					walk(n.Children)
				}
			}
		}
		walk(roots)
	}

	return json.Marshal(map[string]any{
		"items":         items,
		"count":         len(items),
		"posts_scanned": scanned,
		"posts_capped":  len(candidates),
	})
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
