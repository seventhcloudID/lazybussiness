package lazy

import (
	"encoding/json"
	"sort"
	"time"

	"threads-dashboard/internal/threads"
)

var fallbackMinutes = []int{
	7*60 + 30,  // 07:30
	12 * 60,    // 12:00
	15*60 + 30, // 15:30
	19 * 60,    // 19:00
	21 * 60,    // 21:00
	9 * 60,     // 09:00
	17 * 60,    // 17:00
	22*60 + 30, // 22:30
	10*60 + 30, // 10:30
	14 * 60,    // 14:00
	18*60 + 30, // 18:30
	20*60 + 30, // 20:30
}

// BestSlotTimes picks N local-day times from post insights (or fallback),
// spaced at least 90 minutes apart.
func BestSlotTimes(client *threads.Client, loc *time.Location, day time.Time, n int) []time.Time {
	if n < 1 {
		n = 5
	}
	if n > 12 {
		n = 12
	}
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)

	minutes := rankedHourMinutes(client, loc)
	if len(minutes) == 0 {
		minutes = append([]int(nil), fallbackMinutes...)
	}

	selected := pickSpacedMinutes(minutes, n, 90)
	out := make([]time.Time, 0, len(selected))
	for _, m := range selected {
		out = append(out, day.Add(time.Duration(m)*time.Minute))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func rankedHourMinutes(client *threads.Client, loc *time.Location) []int {
	if client == nil || !client.Connected() {
		return nil
	}
	raw, err := client.GetInsightsOpts("", "", true, 20)
	if err != nil {
		return nil
	}
	var parsed struct {
		Posts []struct {
			Timestamp string             `json:"timestamp"`
			Score     float64            `json:"score"`
			Metrics   map[string]float64 `json:"metrics"`
		} `json:"posts"`
	}
	if json.Unmarshal(raw, &parsed) != nil || len(parsed.Posts) == 0 {
		return nil
	}

	type bucket struct {
		hour  int
		score float64
		n     int
	}
	byHour := map[int]*bucket{}
	for _, p := range parsed.Posts {
		ts, err := time.Parse(time.RFC3339, p.Timestamp)
		if err != nil {
			ts, err = time.Parse(time.RFC3339Nano, p.Timestamp)
			if err != nil {
				continue
			}
		}
		local := ts.In(loc)
		h := local.Hour()
		sc := p.Score
		if sc == 0 && p.Metrics != nil {
			sc = p.Metrics["views"] + p.Metrics["likes"]*3 + p.Metrics["replies"]*4
		}
		b := byHour[h]
		if b == nil {
			b = &bucket{hour: h}
			byHour[h] = b
		}
		b.score += sc
		b.n++
	}
	if len(byHour) == 0 {
		return nil
	}
	list := make([]*bucket, 0, len(byHour))
	for _, b := range byHour {
		list = append(list, b)
	}
	sort.Slice(list, func(i, j int) bool {
		ai := list[i].score / float64(max(list[i].n, 1))
		aj := list[j].score / float64(max(list[j].n, 1))
		if ai == aj {
			return list[i].hour < list[j].hour
		}
		return ai > aj
	})

	out := make([]int, 0, len(list)+len(fallbackMinutes))
	for _, b := range list {
		// Prefer :30 for odd hours to stagger
		m := b.hour * 60
		if b.hour%2 == 1 {
			m += 30
		}
		out = append(out, m)
	}
	// Append fallbacks for extra slots
	out = append(out, fallbackMinutes...)
	return out
}

func pickSpacedMinutes(ranked []int, n, minGapMin int) []int {
	var selected []int
	for _, m := range ranked {
		m = normalizeMinute(m)
		ok := true
		for _, s := range selected {
			if abs(m-s) < minGapMin {
				ok = false
				break
			}
		}
		if ok {
			selected = append(selected, m)
		}
		if len(selected) >= n {
			break
		}
	}
	// fill from evenly spaced day if still short
	for len(selected) < n {
		step := (24 * 60) / (n + 1)
		candidate := (len(selected) + 1) * step
		candidate = normalizeMinute(candidate)
		ok := true
		for _, s := range selected {
			if abs(candidate-s) < minGapMin {
				ok = false
				break
			}
		}
		if ok {
			selected = append(selected, candidate)
		} else {
			selected = append(selected, normalizeMinute(candidate+minGapMin))
		}
	}
	sort.Ints(selected)
	return selected[:n]
}

func ensureMinGap(times []time.Time, gap time.Duration) []time.Time {
	if len(times) == 0 {
		return times
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	out := make([]time.Time, len(times))
	out[0] = times[0]
	for i := 1; i < len(times); i++ {
		t := times[i]
		if t.Before(out[i-1].Add(gap)) {
			t = out[i-1].Add(gap)
		}
		out[i] = t
	}
	return out
}

func normalizeMinute(m int) int {
	for m < 0 {
		m += 24 * 60
	}
	m = m % (24 * 60)
	// avoid 00:00–05:00 night dump unless intentional
	if m < 5*60 {
		m += 6 * 60
	}
	if m >= 23*60+30 {
		m = 22*60 + 30
	}
	return m
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
