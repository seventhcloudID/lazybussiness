package repliz

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (c *Client) SnapshotForAI(ctx context.Context, accountID string, limit int) (map[string]any, error) {
	if limit <= 0 || limit > 24 {
		limit = 24
	}
	until := time.Now().Unix()
	since := until - 90*24*60*60
	raw, err := c.Insights(ctx, strconv.FormatInt(since, 10), strconv.FormatInt(until, 10), limit, accountID)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Metrics       map[string]float64 `json:"metrics"`
		Totals        map[string]float64 `json:"totals"`
		Posts         []postRow          `json:"posts"`
		Account       map[string]any     `json:"repliz_account"`
		Engagement    float64            `json:"engagement"`
		EngagementRate float64           `json:"engagement_rate"`
		PostCount     int                `json:"post_count"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}

	formatMix := map[string]int{}
	weekdayMix := map[string]int{}
	hourMix := map[string]int{}
	var sumER, sumViews, sumEng float64
	erN := 0
	posts := make([]map[string]any, 0, len(wrap.Posts))
	for i, p := range wrap.Posts {
		text := strings.TrimSpace(p.Text)
		chars := utf8.RuneCountInString(text)
		if chars > 280 {
			text = string([]rune(text)[:280]) + "…"
		}
		m := p.Metrics
		if m == nil {
			m = map[string]float64{}
		}
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
		if ts := parseCreated(p.Timestamp); ts > 0 {
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
			"views":           views,
			"likes":           m["likes"],
			"replies":         m["replies"],
			"reposts":         m["reposts"],
			"quotes":          m["quotes"],
			"score":           p.Score,
			"engagement":      eng,
			"engagement_rate": er,
		})
	}
	avgER := wrap.EngagementRate
	if erN > 0 && avgER == 0 {
		avgER = sumER / float64(erN)
	}
	uname, _ := wrap.Account["username"].(string)
	name, _ := wrap.Account["name"].(string)
	typ, _ := wrap.Account["type"].(string)
	id, _ := wrap.Account["id"].(string)
	return map[string]any{
		"source": "repliz",
		"profile": map[string]any{
			"id":       id,
			"username": uname,
			"name":     name,
			"type":     typ,
		},
		"account_metrics": map[string]any{
			"source":   "repliz",
			"last_30d": wrap.Metrics,
		},
		"sample": map[string]any{
			"post_count":     len(posts),
			"totals":         wrap.Totals,
			"avg_er_pct":     avgER,
			"sum_views":      sumViews,
			"sum_engagement": sumEng,
			"format_mix":     formatMix,
			"weekday_mix":    weekdayMix,
			"daypart_mix":    hourMix,
		},
		"posts": posts,
	}, nil
}
