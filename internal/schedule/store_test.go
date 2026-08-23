package schedule

import (
	"path/filepath"
	"testing"
	"time"
)

func TestClaimDueRespectsRunAt(t *testing.T) {
	dir := t.TempDir()
	s := NewStoreAt(filepath.Join(dir, "scheduled_posts.json"))
	future := time.Now().UTC().Add(2 * time.Hour)
	past := time.Now().UTC().Add(-5 * time.Minute)

	s.list = []Post{
		{ID: "future", Status: StatusPending, RunAt: future, Text: "later"},
		{ID: "due", Status: StatusPending, RunAt: past, Text: "now"},
	}
	_ = s.saveLocked()

	got, ok := s.ClaimDue(time.Now())
	if !ok {
		t.Fatal("expected due post")
	}
	if got.ID != "due" {
		t.Fatalf("got %s want due", got.ID)
	}
	if got.Status != StatusRunning {
		t.Fatalf("status %s", got.Status)
	}

	_, ok = s.ClaimDue(time.Now())
	if ok {
		t.Fatal("should not claim future post")
	}
}

func TestClaimDueSkipsZeroAndAlreadyPublished(t *testing.T) {
	dir := t.TempDir()
	s := NewStoreAt(filepath.Join(dir, "scheduled_posts.json"))
	past := time.Now().UTC().Add(-time.Minute)
	s.list = []Post{
		{ID: "zero", Status: StatusPending, Text: "bad"},
		{ID: "doneish", Status: StatusPending, RunAt: past, Text: "x", ThreadsIDs: []string{"123"}},
	}
	_ = s.saveLocked()

	_, ok := s.ClaimDue(time.Now())
	if ok {
		t.Fatal("should not claim zero or already-published")
	}
	p, _ := s.Get("doneish")
	if p.Status != StatusDone {
		t.Fatalf("already-published should become done, got %s", p.Status)
	}
}

func TestRecoverStuckIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := NewStoreAt(filepath.Join(dir, "scheduled_posts.json"))
	s.list = []Post{
		{ID: "pub", Status: StatusRunning, RunAt: time.Now().UTC().Add(-time.Hour), ThreadsIDs: []string{"99"}},
		{ID: "mid", Status: StatusRunning, RunAt: time.Now().UTC().Add(time.Hour)},
	}
	_ = s.saveLocked()

	n := s.RecoverStuck()
	if n != 2 {
		t.Fatalf("recovered %d", n)
	}
	pub, _ := s.Get("pub")
	if pub.Status != StatusDone {
		t.Fatalf("published running → done, got %s", pub.Status)
	}
	mid, _ := s.Get("mid")
	if mid.Status != StatusPending {
		t.Fatalf("unpublished running → pending, got %s", mid.Status)
	}
	// Future pending must not be claimable yet.
	_, ok := s.ClaimDue(time.Now())
	if ok {
		t.Fatal("recovered future post must not publish early")
	}
}

func TestCreateRejectsPast(t *testing.T) {
	dir := t.TempDir()
	s := NewStoreAt(filepath.Join(dir, "scheduled_posts.json"))
	_, err := s.Create(CreateInput{
		RunAt: time.Now().Add(-2 * time.Minute),
		Text:  "hello",
	})
	if err == nil {
		t.Fatal("expected error for past run_at")
	}
}
