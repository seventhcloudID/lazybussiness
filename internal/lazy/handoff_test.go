package lazy

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEnqueueHandoffAttachesToPendingJob(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreAt(
		filepath.Join(dir, "lazy_config.json"),
		filepath.Join(dir, "lazy_jobs.json"),
		filepath.Join(dir, "lazy-media"),
	)
	when := time.Date(2026, 8, 23, 14, 0, 0, 0, time.UTC)
	if err := store.AddJobs([]Job{{
		ID:          "2026-08-23-01",
		Date:        "2026-08-23",
		ScheduledAt: when,
		Status:      StatusPending,
	}}); err != nil {
		t.Fatal(err)
	}

	attached, pending, err := store.EnqueueHandoff(ContentHandoff{
		Parts:      []string{"Hook bagian 1", "Bagian 2"},
		Title:      "Judul utas",
		CoverTitle: "Headline cover",
		ThumbURL:   "/media/thumbs/cover.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatal("expected attached job, got pending queue")
	}
	if attached != "2026-08-23-01" {
		t.Fatalf("attached=%q want 2026-08-23-01", attached)
	}

	job, ok := store.GetJob("2026-08-23-01")
	if !ok {
		t.Fatal("job missing")
	}
	if len(job.PrefilledParts) != 2 || job.PrefilledThumbURL != "/media/thumbs/cover.png" {
		t.Fatalf("prefilled not stored: %+v", job)
	}
	if store.PendingHandoff() != nil {
		t.Fatal("pending handoff should be cleared after attach")
	}
}

func TestEnqueueHandoffQueuesWhenNoPendingJob(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreAt(
		filepath.Join(dir, "lazy_config.json"),
		filepath.Join(dir, "lazy_jobs.json"),
		filepath.Join(dir, "lazy-media"),
	)

	attached, pending, err := store.EnqueueHandoff(ContentHandoff{
		Parts:    []string{"Hook", "Isi"},
		ThumbURL: "/media/thumbs/x.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if attached != "" || !pending {
		t.Fatalf("attached=%q pending=%v", attached, pending)
	}
	if store.PendingHandoff() == nil {
		t.Fatal("expected pending handoff")
	}

	when := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	if err := store.AddJobs([]Job{{
		ID:          "2026-08-24-01",
		Date:        "2026-08-24",
		ScheduledAt: when,
		Status:      StatusPending,
	}}); err != nil {
		t.Fatal(err)
	}
	job, ok := store.GetJob("2026-08-24-01")
	if !ok {
		t.Fatal("job missing")
	}
	if job.PrefilledThumbURL != "/media/thumbs/x.png" {
		t.Fatalf("handoff not applied on new job: %+v", job)
	}
}

func TestResolveThumbPublicURL(t *testing.T) {
	d := &Deps{Public: "https://flowa.example.com"}
	got, err := d.resolveThumbPublicURL("/media/thumbs/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://flowa.example.com/media/thumbs/a.png" {
		t.Fatalf("got %q", got)
	}
}
