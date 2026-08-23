package repliz

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestPickConnectedPrefersConnected(t *testing.T) {
	list := []Account{
		{ID: "a", Username: "off", Type: "threads", IsConnected: false},
		{ID: "b", Username: "ig", Type: "instagram", IsConnected: true},
		{ID: "c", Username: "th", Type: "threads", IsConnected: true},
	}
	got, err := PickConnected(list)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountID() != "b" {
		t.Fatalf("got %s want first connected id b", got.AccountID())
	}
}

func TestFindAccount(t *testing.T) {
	list := []Account{{OID: "x1", Username: "n"}}
	acc, ok := FindAccount(list, "x1")
	if !ok || acc.Username != "n" {
		t.Fatalf("expected oid match, ok=%v acc=%+v", ok, acc)
	}
	if _, ok := FindAccount(list, "missing"); ok {
		t.Fatal("missing should not match")
	}
}

func TestLiveListAccounts(t *testing.T) {
	loadDotEnv("../../.env")
	c := NewFromEnv()
	if !c.Ready() {
		t.Skip("no Repliz keys")
	}
	list, err := c.ListAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("Repliz tidak mengembalikan akun")
	}
	t.Logf("base=%s accounts=%d first=@%s type=%s connected=%v", c.base(), len(list), list[0].Username, list[0].Type, list[0].IsConnected)
}

func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
			v = strings.Trim(v, `"`)
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

func TestNormalizeBaseStripsPublic(t *testing.T) {
	if got := NormalizeBase("https://api.repliz.com/public"); got != "https://api.repliz.com" {
		t.Fatalf("got %s", got)
	}
	if got := NormalizeBase("https://api.repliz.com/public/"); got != "https://api.repliz.com" {
		t.Fatalf("got %s", got)
	}
	if got := NormalizeBase(""); got != "https://api.repliz.com" {
		t.Fatalf("got %s", got)
	}
}
