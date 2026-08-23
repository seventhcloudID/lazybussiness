package repliz

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func compactPostStats(raw []byte) map[string]float64 {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return map[string]float64{}
	}
	if inner, ok := m["data"].(map[string]any); ok {
		m = inner
	}
	if inner, ok := m["statistic"].(map[string]any); ok {
		m = inner
	}
	keys := []string{
		"like", "likes", "likeCount", "totalLikes", "comment", "comments", "commentCount",
		"share", "shares", "shareCount", "reach",
		"saved", "saves", "views", "videoViews", "uniqueVideoViews", "plays", "playCount",
		"impression", "impressions", "watched", "favourite", "interaction", "interactions",
		"replies", "reply", "repost", "reposts", "quotes", "quote", "retweet", "bookmark",
	}
	out := map[string]float64{}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		n := asFloat(v)
		if n == 0 {
			if inner, ok := v.(map[string]any); ok {
				n = firstNonZero(asFloat(inner["total"]), asFloat(inner["value"]), asFloat(inner["count"]))
			}
		}
		out[k] = n
		if alias := aliasStatKey(k); alias != k && n != 0 {
			if cur, exists := out[alias]; !exists || cur == 0 {
				out[alias] = n
			}
		}
	}
	return out
}

func aliasStatKey(k string) string {
	switch k {
	case "like", "likeCount", "totalLikes":
		return "likes"
	case "comment", "commentCount":
		return "comments"
	case "saves":
		return "saved"
	case "share", "shareCount":
		return "shares"
	case "retweet", "repost":
		return "reposts"
	case "reply":
		return "replies"
	case "quote":
		return "quotes"
	case "videoViews", "uniqueVideoViews", "plays", "playCount", "impressions", "impression", "watched":
		return "views"
	default:
		return k
	}
}

func unwrapStatObject(m map[string]any) map[string]any {
	for i := 0; i < 4 && m != nil; i++ {
		if inner, ok := m["data"].(map[string]any); ok && inner != nil {
			m = inner
			continue
		}
		if inner, ok := m["statistic"].(map[string]any); ok && inner != nil {
			m = inner
			continue
		}
		break
	}
	return m
}

func nestedFloat(v any) float64 {
	n := asFloat(v)
	if n != 0 {
		return n
	}
	m, ok := v.(map[string]any)
	if !ok {
		return 0
	}
	return firstNonZero(
		asFloat(m["total"]),
		asFloat(m["value"]),
		asFloat(m["count"]),
		asFloat(m["followersCount"]),
		asFloat(m["followerCount"]),
	)
}

func flattenStatNum(v any) any {
	if v == nil {
		return nil
	}
	if _, ok := v.(map[string]any); ok {
		if n := nestedFloat(v); n != 0 {
			return n
		}
	}
	return v
}

func compactStatistic(raw []byte) map[string]any {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return map[string]any{}
	}
	m = unwrapStatObject(m)
	out := map[string]any{}
	copyKeys := []string{
		"followersCount", "followerCount", "subscribersCount", "subscriberCount", "follower",
		"followingCount", "following", "likes", "comments", "reach", "saves", "shares",
		"accountsEngaged", "totalInteractions", "replies", "reposts", "quotes",
		"profileLinksTaps", "videosCount", "mediaCount", "totalLikes", "videoViews", "impressions",
	}
	for _, k := range copyKeys {
		if v, ok := m[k]; ok {
			out[k] = flattenStatNum(v)
		}
	}
	if nestedFloat(out["followersCount"]) == 0 {
		for _, k := range []string{"followerCount", "subscribersCount", "subscriberCount", "follower"} {
			n := nestedFloat(out[k])
			if n == 0 {
				n = nestedFloat(m[k])
			}
			if n > 0 {
				out["followersCount"] = n
				break
			}
		}
	}
	if nestedFloat(out["followingCount"]) == 0 {
		if n := nestedFloat(m["following"]); n > 0 {
			out["followingCount"] = n
		}
	}
	switch v := m["views"].(type) {
	case map[string]any:
		if t, ok := v["total"]; ok {
			out["views"] = t
		}
	default:
		if v != nil {
			out["views"] = v
		}
	}
	if ages, ok := m["audienceAges"].([]any); ok {
		out["audienceAges"] = ages
	}
	if genders, ok := m["audienceGenders"].([]any); ok {
		out["audienceGenders"] = genders
	}
	if cities, ok := m["audienceCities"].([]any); ok {
		out["audienceCities"] = topPct(cities, "cityName", 8)
	}
	if countries, ok := m["audienceCountries"].([]any); ok {
		out["audienceCountries"] = topPct(countries, "country", 6)
	}
	if metrics, ok := m["metrics"].([]any); ok && len(metrics) > 0 {
		last := pickLatestMetric(metrics)
		if last != nil {
			for _, k := range []string{
				"videoViews", "uniqueVideoViews", "profileViews", "engagedAudience",
				"likes", "comments", "shares",
			} {
				if v, ok := last[k]; ok {
					out[k] = flattenStatNum(v)
				}
			}
			if nestedFloat(out["followersCount"]) == 0 {
				if v, ok := last["followersCount"]; ok {
					out["followersCount"] = flattenStatNum(v)
				} else if v, ok := last["followerCount"]; ok {
					out["followersCount"] = flattenStatNum(v)
				}
			}
		}
	}
	return out
}

func pickLatestMetric(metrics []any) map[string]any {
	var best map[string]any
	var bestFollowers float64
	for _, it := range metrics {
		row, ok := it.(map[string]any)
		if !ok {
			continue
		}
		fc := nestedFloat(row["followersCount"])
		if fc == 0 {
			fc = nestedFloat(row["followerCount"])
		}
		vv := firstNonZero(nestedFloat(row["videoViews"]), nestedFloat(row["views"]), nestedFloat(row["reach"]))
		if fc == 0 && vv == 0 && nestedFloat(row["likes"]) == 0 {
			continue
		}
		if fc >= bestFollowers {
			best = row
			bestFollowers = fc
		}
	}
	if best != nil {
		return best
	}
	if row, ok := metrics[len(metrics)-1].(map[string]any); ok {
		return row
	}
	return nil
}

func topPct(items []any, labelKey string, n int) []map[string]any {
	type row struct {
		label string
		pct   float64
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		label := fmt.Sprint(m[labelKey])
		pct := asFloat(m["percentage"])
		if pct == 0 {
			pct = asFloat(m["value"])
		}
		if strings.TrimSpace(label) == "" || strings.EqualFold(label, "<nil>") {
			continue
		}
		rows = append(rows, row{label: label, pct: pct})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pct > rows[j].pct })
	if len(rows) > n {
		rows = rows[:n]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{"label": r.label, "value": r.pct})
	}
	return out
}

func AsFloat(v any) float64 {
	return asFloat(v)
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
