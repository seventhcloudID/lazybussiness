package lazy

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/threads"
)

// TrackChannels ringkasan channel per job.
type TrackChannels struct {
	Threads bool   `json:"threads"`
	IG      bool   `json:"ig"`
	X       bool   `json:"x"`
	TikTok  bool   `json:"tiktok"`
	RootID  string `json:"threads_root,omitempty"`
	IGMedia string `json:"ig_media_id,omitempty"`
}

// TrackItem satu hasil Lazy (bukan antrian pending).
type TrackItem struct {
	Job
	Snippet         string                        `json:"snippet,omitempty"`
	Channels        TrackChannels                 `json:"channels"`
	Metrics         map[string]float64            `json:"metrics,omitempty"`          // total gabungan platform
	PlatformMetrics map[string]map[string]float64 `json:"platform_metrics,omitempty"` // threads|ig|…
	Deleted         bool                          `json:"deleted,omitempty"`          // total views=0 → konten hilang
}

// TrackSummary agregat capaian tools.
type TrackSummary struct {
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	Failed     int     `json:"failed"`
	SkippedIG  int     `json:"skipped_ig"`
	Deleted    int     `json:"deleted"`  // post 0 views (dihapus)
	Measured   int     `json:"measured"` // post dengan metrik valid (>0 views)
	Threads    int     `json:"threads"`
	IG         int     `json:"ig"`
	X          int     `json:"x"`
	TikTok     int     `json:"tiktok"`
	Views      float64 `json:"views"`
	Likes      float64 `json:"likes"`
	Replies    float64 `json:"replies"`
	Reposts    float64 `json:"reposts"`
	Quotes     float64 `json:"quotes"`
	Engagement float64 `json:"engagement"`
	// Per-platform totals (dari platform_metrics).
	ViewsThreads  float64 `json:"views_threads"`
	ViewsIG       float64 `json:"views_ig"`
	LikesThreads  float64 `json:"likes_threads"`
	LikesIG       float64 `json:"likes_ig"`
	RepliesThreads float64 `json:"replies_threads"`
	RepliesIG     float64 `json:"replies_ig"`
}

// TrackReport payload halaman tracking.
type TrackReport struct {
	From     string       `json:"from"`
	To       string       `json:"to"`
	Timezone string       `json:"timezone"`
	Summary  TrackSummary `json:"summary"`
	Jobs     []TrackItem  `json:"jobs"`
	Note     string       `json:"note,omitempty"`
}

// BuildTrackReport merangkum job hasil Lazy (done/failed/skipped_ig).
// metrics=true → ambil insight dari semua platform yang connect (Threads + IG).
func BuildTrackReport(store *Store, thClient *threads.Client, igClient *instagram.Client, withMetrics bool) TrackReport {
	cfg := store.GetConfig()
	loc := store.Location()
	now := time.Now().In(loc)
	to := now.Format("2006-01-02")
	from := now.AddDate(0, 0, -30).Format("2006-01-02")

	all := store.ListJobs()
	items := make([]TrackItem, 0, len(all))
	for _, j := range all {
		if j.Date < from {
			continue
		}
		switch j.Status {
		case StatusDone, StatusFailed, StatusSkippedIG:
		default:
			continue
		}
		snip := ""
		if len(j.Parts) > 0 {
			snip = j.Parts[0]
			if len([]rune(snip)) > 140 {
				snip = string([]rune(snip)[:140]) + "…"
			}
		}
		ch := TrackChannels{
			Threads: len(j.ThreadsIDs) > 0,
			IG:      j.IGMediaID != "" || j.IGContainer != "",
			X:       j.BufferXPostID != "",
			TikTok:  j.BufferPostID != "",
			IGMedia: j.IGMediaID,
		}
		if len(j.ThreadsIDs) > 0 {
			ch.RootID = j.ThreadsIDs[0]
		}
		items = append(items, TrackItem{
			Job:      j,
			Snippet:  snip,
			Channels: ch,
		})
	}

	sort.Slice(items, func(i, k int) bool {
		if items[i].FinishedAt.Equal(items[k].FinishedAt) {
			return items[i].ScheduledAt.After(items[k].ScheduledAt)
		}
		return items[i].FinishedAt.After(items[k].FinishedAt)
	})

	if withMetrics {
		enrichTrackMetrics(thClient, igClient, items, 24)
	}

	// Tandai yang dihapus (total 0 views setelah diukur), lalu buang dari daftar tampilan.
	visible := make([]TrackItem, 0, len(items))
	sum := TrackSummary{}
	for i := range items {
		it := items[i]
		if it.Metrics != nil {
			alive := it.Metrics["views"] > 0 || it.Metrics["likes"] > 0 ||
				it.Metrics["replies"] > 0 || it.Metrics["reposts"] > 0 || it.Metrics["quotes"] > 0
			if !alive {
				it.Deleted = true
				sum.Deleted++
				continue // jangan tampilkan
			}
		}
		sum.Total++
		switch it.Status {
		case StatusDone:
			sum.Done++
		case StatusFailed:
			sum.Failed++
		case StatusSkippedIG:
			sum.SkippedIG++
		}
		if it.Channels.Threads {
			sum.Threads++
		}
		if it.Channels.IG {
			sum.IG++
		}
		if it.Channels.X {
			sum.X++
		}
		if it.Channels.TikTok {
			sum.TikTok++
		}
		if it.Metrics != nil {
			sum.Measured++
			sum.Views += it.Metrics["views"]
			sum.Likes += it.Metrics["likes"]
			sum.Replies += it.Metrics["replies"]
			sum.Reposts += it.Metrics["reposts"]
			sum.Quotes += it.Metrics["quotes"]
		}
		if pm := it.PlatformMetrics; pm != nil {
			if t := pm["threads"]; t != nil {
				sum.ViewsThreads += t["views"]
				sum.LikesThreads += t["likes"]
				sum.RepliesThreads += t["replies"]
			}
			if g := pm["ig"]; g != nil {
				sum.ViewsIG += g["views"]
				sum.LikesIG += g["likes"]
				sum.RepliesIG += g["replies"]
			}
		}
		visible = append(visible, it)
	}
	sum.Engagement = sum.Likes + sum.Replies + sum.Reposts + sum.Quotes

	note := "Metrik digabung dari Threads + Instagram (kalau terkoneksi). X/TikTok via Buffer belum expose analytics API."
	return TrackReport{
		From:     from,
		To:       to,
		Timezone: cfg.Timezone,
		Summary:  sum,
		Jobs:     visible,
		Note:     note,
	}
}

func enrichTrackMetrics(thClient *threads.Client, igClient *instagram.Client, items []TrackItem, limit int) {
	type job struct {
		idx int
	}
	var queue []job
	for i, it := range items {
		canTh := it.Channels.RootID != "" && thClient != nil && thClient.Connected()
		canIG := it.Channels.IGMedia != "" && igClient != nil && igClient.Connected()
		if !canTh && !canIG {
			continue
		}
		queue = append(queue, job{idx: i})
		if limit > 0 && len(queue) >= limit {
			break
		}
	}
	if len(queue) == 0 {
		return
	}

	var mu sync.Mutex
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, q := range queue {
		wg.Add(1)
		go func(q job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			it := items[q.idx]
			by := map[string]map[string]float64{}
			attempted := false

			if it.Channels.RootID != "" && thClient != nil && thClient.Connected() {
				attempted = true
				m := map[string]float64{}
				if raw, err := thClient.GetMediaInsights(it.Channels.RootID); err == nil {
					m = parseInsightMetrics(raw)
				}
				if len(m) == 0 {
					m = map[string]float64{
						"views": 0, "likes": 0, "replies": 0, "reposts": 0, "quotes": 0,
					}
				}
				by["threads"] = normalizePlatformMetrics(m)
			}

			if it.Channels.IGMedia != "" && igClient != nil && igClient.Connected() {
				attempted = true
				m := map[string]float64{}
				if mm, err := igClient.GetMediaMetrics(it.Channels.IGMedia); err == nil && mm != nil {
					m["views"] = mm.Views
					m["likes"] = mm.Likes
					m["replies"] = mm.Replies
					m["reach"] = mm.Reach
					// Kalau insight views kosong tapi ada engagement, jangan paksa 0-delete —
					// pakai reach atau minimal proxy likes+comments sebagai sinyal hidup.
					if m["views"] <= 0 && mm.Reach > 0 {
						m["views"] = mm.Reach
					}
				}
				by["ig"] = normalizePlatformMetrics(m)
			}

			if !attempted || len(by) == 0 {
				return
			}

			total := sumPlatformMetrics(by)
			mu.Lock()
			items[q.idx].PlatformMetrics = by
			items[q.idx].Metrics = total
			mu.Unlock()
		}(q)
	}
	wg.Wait()
}

func normalizePlatformMetrics(m map[string]float64) map[string]float64 {
	if m == nil {
		m = map[string]float64{}
	}
	out := map[string]float64{
		"views":   m["views"],
		"likes":   m["likes"],
		"replies": m["replies"],
		"reposts": m["reposts"],
		"quotes":  m["quotes"],
	}
	if r, ok := m["reach"]; ok && r > 0 {
		out["reach"] = r
	}
	return out
}

func sumPlatformMetrics(by map[string]map[string]float64) map[string]float64 {
	total := map[string]float64{
		"views": 0, "likes": 0, "replies": 0, "reposts": 0, "quotes": 0,
	}
	for _, m := range by {
		for _, k := range []string{"views", "likes", "replies", "reposts", "quotes"} {
			total[k] += m[k]
		}
	}
	return total
}

func parseInsightMetrics(raw json.RawMessage) map[string]float64 {
	var wrap struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value any `json:"value"`
			} `json:"values"`
			TotalValue *struct {
				Value any `json:"value"`
			} `json:"total_value"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return nil
	}
	out := map[string]float64{}
	for _, d := range wrap.Data {
		var val float64
		if d.TotalValue != nil {
			val = anyFloat(d.TotalValue.Value)
		} else if len(d.Values) > 0 {
			val = anyFloat(d.Values[0].Value)
		}
		out[d.Name] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func anyFloat(v any) float64 {
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
	default:
		return 0
	}
}
