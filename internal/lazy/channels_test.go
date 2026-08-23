package lazy

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeChannels(t *testing.T) {
	got := normalizeChannels([]string{" Threads ", "IG", "tiktok", "threads", "x"})
	want := []string{"threads", "instagram", "tiktok"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeChannels() = %#v, want %#v", got, want)
	}
}

func TestThreadsCoverCannotBeDisabled(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreAt(filepath.Join(dir, "config.json"), filepath.Join(dir, "jobs.json"), filepath.Join(dir, "media"))
	cfg, err := store.SetConfig(Config{Channels: []string{"threads"}, ThumbnailEnabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ThumbnailEnabled {
		t.Fatal("Threads cover must always stay enabled")
	}
}

func TestConfigHasChannel(t *testing.T) {
	cfg := Config{Channels: []string{"threads", "instagram"}}
	if !cfg.HasChannel("threads") || !cfg.HasChannel("ig") || cfg.HasChannel("tiktok") {
		t.Fatalf("unexpected channel selection: %#v", cfg.Channels)
	}
}
