package lazy

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

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
	Snippet  string             `json:"snippet,omitempty"`
	Channels TrackChannels      `json:"channels"`
	Metrics  map[string]float64 `json:"metrics,omitempty"`
	Deleted  bool               `json:"deleted,omitempty"` // views=0 → konten dihapus, tidak dihitung metrik
}

// TrackSummary agregat capaian tools.
type TrackSummary struct {
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	Failed     int     `json:"failed"`
	SkippedIG  int     `json:"skipped_ig"`
	Deleted    int     `json:"deleted"` // post 0 views (dihapus)
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
}

// TrackReport payload halaman tracking.
type TrackReport struct {
	From     string       `json:"from"`
	To       string       `json:"to"`
	Timezone string       `json:"timezone"`
	Summary  TrackSummary `json:"summary"`
	Jobs     []TrackItem  `json:"jobs"`
}

// BuildTrackReport merangkum job hasil Lazy (done/failed/skipped_ig).
// metrics=true → ambil insight Threads untuk root id (dibatasi 24 job terbaru).
func BuildTrackReport(store *Store, client *threads.Client, withMetrics bool) TrackReport {
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

	if withMetrics && client != nil && client.Connected() {
		enrichTrackMetrics(client, items, 24)
	}

	sum := TrackSummary{}
	for i := range items {
		it := &items[i]
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
		if it.Metrics == nil {
			continue
		}
		// 0 views = konten sudah dihapus / tidak valid → jangan masuk agregat
		if it.Metrics["views"] <= 0 {
			it.Deleted = true
			sum.Deleted++
			continue
		}
		sum.Measured++
		sum.Views += it.Metrics["views"]
		sum.Likes += it.Metrics["likes"]
		sum.Replies += it.Metrics["replies"]
		sum.Reposts += it.Metrics["reposts"]
		sum.Quotes += it.Metrics["quotes"]
	}
	sum.Engagement = sum.Likes + sum.Replies + sum.Reposts + sum.Quotes

	return TrackReport{
		From:     from,
		To:       to,
		Timezone: cfg.Timezone,
		Summary:  sum,
		Jobs:     items,
	}
}

func enrichTrackMetrics(client *threads.Client, items []TrackItem, limit int) {
	type job struct {
		idx int
		id  string
	}
	var queue []job
	for i, it := range items {
		if it.Channels.RootID == "" {
			continue
		}
		queue = append(queue, job{idx: i, id: it.Channels.RootID})
		if len(queue) >= limit {
			break
		}
	}
	if len(queue) == 0 {
		return
	}

	var mu sync.Mutex
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, q := range queue {
		wg.Add(1)
		go func(q job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			raw, err := client.GetMediaInsights(q.id)
			if err != nil {
				return
			}
			m := parseInsightMetrics(raw)
			if len(m) == 0 {
				return
			}
			mu.Lock()
			items[q.idx].Metrics = m
			mu.Unlock()
		}(q)
	}
	wg.Wait()
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
