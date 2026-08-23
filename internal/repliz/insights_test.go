package repliz

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseCreatedUnixAndISO(t *testing.T) {
	sec := int64(1724112000)
	if got := parseCreated("1724112000"); got != sec {
		t.Fatalf("unix sec: got %d want %d", got, sec)
	}
	if got := parseCreated("1724112000000"); got != sec {
		t.Fatalf("unix ms: got %d want %d", got, sec)
	}
	iso := "2024-08-20T00:00:00Z"
	want, _ := time.Parse(time.RFC3339, iso)
	if got := parseCreated(iso); got != want.Unix() {
		t.Fatalf("iso: got %d want %d", got, want.Unix())
	}
	if got := parseUnix("2024-08-20"); got == 0 {
		t.Fatal("ISO date should parse as unix")
	}
}

func TestFormatTimestampRFC3339(t *testing.T) {
	got := formatTimestamp("1724112000")
	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Fatalf("expected RFC3339, got %q err %v", got, err)
	}
}

func TestFlexStringNumberOrString(t *testing.T) {
	var p Post
	if err := json.Unmarshal([]byte(`{"createdAt":1724112000,"title":"x"}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.CreatedStamp() != "1724112000" {
		t.Fatalf("got %q", p.CreatedStamp())
	}
	if err := json.Unmarshal([]byte(`{"createdAt":"2024-08-20T00:00:00Z"}`), &p); err != nil {
		t.Fatal(err)
	}
	if parseCreated(p.CreatedStamp()) == 0 {
		t.Fatal("iso createdAt should parse")
	}
}

func TestNormalizePostMetricsAliases(t *testing.T) {
	m := normalizePostMetrics(map[string]float64{
		"like":       10,
		"comments":   3,
		"shares":     2,
		"videoViews": 100,
	})
	if m["likes"] != 10 || m["replies"] != 3 || m["reposts"] != 2 || m["views"] != 100 {
		t.Fatalf("got %+v", m)
	}
}

func TestCompactPostStatsAliases(t *testing.T) {
	raw := []byte(`{"like":8,"videoViews":50,"comment":1}`)
	got := compactPostStats(raw)
	if got["likes"] != 8 {
		t.Fatalf("likes alias: %+v", got)
	}
	if got["views"] != 50 {
		t.Fatalf("views alias: %+v", got)
	}
	if got["comments"] != 1 {
		t.Fatalf("comments alias: %+v", got)
	}
}

func TestCompactStatisticUnwrapsDataAndNestedFollowers(t *testing.T) {
	raw := []byte(`{
		"data": {
			"followersCount": {"total": 18420},
			"followingCount": 312,
			"videosCount": 88,
			"metrics": [
				{"date": "2026-08-01", "likes": 10},
				{"date": "2026-08-19", "followersCount": 18450, "views": 900}
			]
		}
	}`)
	got := compactStatistic(raw)
	if nestedFloat(got["followersCount"]) != 18420 {
		t.Fatalf("followers from nested total: %+v", got["followersCount"])
	}
	if nestedFloat(got["followingCount"]) != 312 {
		t.Fatalf("following: %+v", got["followingCount"])
	}
}

func TestCompactStatisticFollowersFromMetricsOnly(t *testing.T) {
	raw := []byte(`{"metrics":[{"likes":1},{"followersCount":"2200","videoViews":50}]}`)
	got := compactStatistic(raw)
	if nestedFloat(got["followersCount"]) != 2200 {
		t.Fatalf("followers from metrics: %+v", got)
	}
}
