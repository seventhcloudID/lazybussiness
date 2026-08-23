package repliz

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type postRow struct {
	ID        string             `json:"id"`
	Text      string             `json:"text"`
	Permalink string             `json:"permalink,omitempty"`
	Timestamp string             `json:"timestamp,omitempty"`
	MediaType string             `json:"media_type,omitempty"`
	MediaURL  string             `json:"media_url,omitempty"`
	Metrics   map[string]float64 `json:"metrics"`
	Score     float64            `json:"score"`
}

func (c *Client) Insights(ctx context.Context, since, until string, postLimit int, accountID string) (json.RawMessage, error) {
	if !c.Ready() {
		return nil, fmt.Errorf("Repliz belum disambungkan — set REPLIZ_ACCESS_KEY dan REPLIZ_SECRET_KEY")
	}
	acc, err := c.ResolveAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	id := acc.AccountID()

	stat, err := c.GetAccountStatistic(ctx, id)
	if err != nil {
		stat = map[string]any{}
	}

	requested := postLimit
	limit := postLimit
	if limit < 0 {
		limit = 40
	}
	if limit > 40 {
		limit = 40
	}
	var rawPosts []Post
	if limit != 0 {
		rawPosts, err = c.ListContentUpTo(ctx, id, limit)
		if err != nil {
			rawPosts = nil
		}
	}

	sinceU, untilU := parseUnix(since), parseUnix(until)
	if untilU <= 0 {
		untilU = time.Now().Unix()
	}

	stats := make([]map[string]float64, len(rawPosts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for i, p := range rawPosts {
		pid := p.PostID()
		if pid == "" {
			continue
		}
		wg.Add(1)
		go func(i int, contentID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			s, e := c.GetContentStatistic(ctx, contentID, id)
			if e != nil {
				s = map[string]float64{}
			}
			stats[i] = s
		}(i, pid)
	}
	wg.Wait()

	posts := make([]postRow, 0, len(rawPosts))
	totals := map[string]float64{}
	for i, p := range rawPosts {
		stamp := p.CreatedStamp()
		ts := parseCreated(stamp)
		if sinceU > 0 && ts > 0 && ts < sinceU {
			continue
		}
		if untilU > 0 && ts > 0 && ts > untilU {
			continue
		}
		m := normalizePostMetrics(stats[i])
		for k, v := range m {
			totals[k] += v
		}
		text := strings.TrimSpace(p.Title)
		if text == "" {
			text = strings.TrimSpace(p.Description)
		}
		eng := m["likes"] + m["replies"] + m["reposts"] + m["quotes"]
		mediaURL := ""
		if len(p.Medias) > 0 {
			mediaURL = strings.TrimSpace(p.Medias[0].URL)
			if mediaURL == "" {
				mediaURL = strings.TrimSpace(p.Medias[0].Thumbnail)
			}
		}
		row := postRow{
			ID:        p.PostID(),
			Text:      text,
			Permalink: strings.TrimSpace(p.URL),
			Timestamp: formatTimestamp(stamp),
			MediaType: mediaTypeOf(p),
			MediaURL:  mediaURL,
			Metrics:   m,
			Score:     m["views"] + m["likes"]*3 + m["replies"]*4 + m["reposts"]*5 + m["quotes"]*5,
		}
		_ = eng
		posts = append(posts, row)
	}

	followers := firstNonZero(
		nestedFloat(stat["followersCount"]),
		nestedFloat(stat["followerCount"]),
		nestedFloat(stat["subscribersCount"]),
		nestedFloat(stat["subscriberCount"]),
		nestedFloat(stat["follower"]),
	)
	following := firstNonZero(nestedFloat(stat["followingCount"]), nestedFloat(stat["following"]))
	mediaCount := firstNonZero(nestedFloat(stat["videosCount"]), nestedFloat(stat["mediaCount"]))
	reach := nestedFloat(stat["reach"])
	saves := nestedFloat(stat["saves"])
	engaged := nestedFloat(stat["accountsEngaged"])
	interactions := nestedFloat(stat["totalInteractions"])
	postViews := totals["views"]
	postLikes := totals["likes"]
	postReplies := totals["replies"]
	postReposts := totals["reposts"]
	postQuotes := totals["quotes"]
	accViews := firstNonZero(asFloat(stat["views"]), asFloat(stat["videoViews"]), asFloat(stat["impressions"]))
	accLikes := firstNonZero(asFloat(stat["likes"]), asFloat(stat["totalLikes"]))
	accReplies := firstNonZero(asFloat(stat["replies"]), asFloat(stat["comments"]))
	accReposts := firstNonZero(asFloat(stat["reposts"]), asFloat(stat["shares"]))
	accQuotes := asFloat(stat["quotes"])

	views, likes, replies, reposts, quotes := postViews, postLikes, postReplies, postReposts, postQuotes
	if postViews+postLikes+postReplies+postReposts+postQuotes == 0 {
		views, likes, replies, reposts, quotes = accViews, accLikes, accReplies, accReposts, accQuotes
	}

	metrics := map[string]float64{
		"followers_count":    followers,
		"follows_count":      following,
		"media_count":        mediaCount,
		"views":              views,
		"likes":              likes,
		"replies":            replies,
		"reposts":            reposts,
		"quotes":             quotes,
		"reach":              reach,
		"saves":              saves,
		"accounts_engaged":   engaged,
		"total_interactions": interactions,
	}
	eng := likes + replies + reposts + quotes
	var engRate float64
	if views > 0 {
		engRate = (eng / views) * 100
	}

	items := []map[string]any{}
	for _, name := range []string{"views", "likes", "replies", "reposts", "quotes", "followers_count"} {
		items = append(items, map[string]any{
			"name": name,
			"values": []map[string]any{
				{"value": metrics[name]},
			},
		})
	}

	out := map[string]any{
		"data":            items,
		"source":          "repliz",
		"metrics":         metrics,
		"totals":          totals,
		"engagement":      eng,
		"engagement_rate": engRate,
		"followers_count": followers,
		"follows_count":   following,
		"media_count":     mediaCount,
		"posts":           posts,
		"post_count":      len(posts),
		"post_limit":      40,
		"repliz_account": map[string]any{
			"id":           id,
			"username":     acc.Username,
			"name":         acc.Name,
			"type":         acc.Type,
			"is_connected": acc.IsConnected,
		},
	}
	if requested > 40 {
		out["sample_capped"] = true
	}
	if sinceU > 0 {
		out["since"] = sinceU
	} else if strings.TrimSpace(since) != "" {
		out["since"] = since
	}
	if untilU > 0 {
		out["until"] = untilU
	} else if strings.TrimSpace(until) != "" {
		out["until"] = until
	}
	return json.Marshal(out)
}

func (c *Client) Feed(ctx context.Context, since, until, accountID string, limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 40
	}
	raw, err := c.Insights(ctx, since, until, limit, accountID)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Posts   []postRow      `json:"posts"`
		Account map[string]any `json:"repliz_account"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	uname, _ := wrap.Account["username"].(string)
	items := make([]map[string]any, 0, len(wrap.Posts))
	for _, p := range wrap.Posts {
		items = append(items, map[string]any{
			"id":         p.ID,
			"text":       p.Text,
			"timestamp":  p.Timestamp,
			"permalink":  p.Permalink,
			"media_type": p.MediaType,
			"media_url":  p.MediaURL,
			"username":   uname,
			"metrics":    p.Metrics,
		})
	}
	return json.Marshal(map[string]any{
		"data":           items,
		"source":         "repliz",
		"repliz_account": wrap.Account,
	})
}

func (c *Client) ResolveAccount(ctx context.Context, accountID string) (Account, error) {
	if id := strings.TrimSpace(accountID); id != "" {
		return c.GetAccount(ctx, id)
	}
	list, err := c.ListAccounts(ctx)
	if err != nil {
		return Account{}, err
	}
	return PickConnected(list)
}

func PickConnected(list []Account) (Account, error) {
	for _, a := range list {
		if a.IsConnected && a.AccountID() != "" {
			return a, nil
		}
	}
	for _, a := range list {
		if a.AccountID() != "" {
			return a, nil
		}
	}
	return Account{}, fmt.Errorf("tidak ada akun Repliz yang terhubung")
}

func FindAccount(list []Account, id string) (Account, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Account{}, false
	}
	for _, a := range list {
		if a.AccountID() == id {
			return a, true
		}
	}
	return Account{}, false
}

func normalizePostMetrics(s map[string]float64) map[string]float64 {
	if s == nil {
		s = map[string]float64{}
	}
	get := func(keys ...string) float64 {
		for _, k := range keys {
			if v := s[k]; v != 0 {
				return v
			}
		}
		return 0
	}
	return map[string]float64{
		"views":   get("views", "videoViews", "watched", "impression", "impressions", "plays", "playCount", "reach"),
		"likes":   get("likes", "like", "likeCount", "totalLikes", "favourite"),
		"replies": get("replies", "reply", "comments", "comment", "commentCount"),
		"reposts": get("reposts", "repost", "shares", "share", "shareCount", "retweet"),
		"quotes":  get("quotes", "quote"),
	}
}

func mediaTypeOf(p Post) string {
	t := strings.ToUpper(strings.TrimSpace(p.Type))
	switch t {
	case "CAROUSEL", "CAROUSEL_ALBUM":
		return "CAROUSEL_ALBUM"
	case "IMAGE", "PHOTO", "PICTURE":
		return "IMAGE"
	case "VIDEO", "REEL", "REELS":
		return "VIDEO"
	case "TEXT", "THREAD", "THREADS":
		return "TEXT"
	}
	if len(p.Medias) > 1 {
		return "CAROUSEL_ALBUM"
	}
	if len(p.Medias) == 1 {
		mt := strings.ToLower(p.Medias[0].Type)
		if strings.Contains(mt, "video") {
			return "VIDEO"
		}
		return "IMAGE"
	}
	return "TEXT"
}

func parseUnix(s string) int64 {
	return parseCreated(s)
}

func formatTimestamp(s string) string {
	ts := parseCreated(s)
	if ts <= 0 {
		return strings.TrimSpace(s)
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

func parseCreated(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 1e12 {
			return n / 1000
		}
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 1e9 {
		n := int64(f)
		if n > 1e12 {
			return n / 1000
		}
		return n
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}

func firstNonZero(vals ...float64) float64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}
