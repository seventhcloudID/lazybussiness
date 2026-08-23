package ai

import "testing"

func TestMemoryFitsAccount(t *testing.T) {
	m := Memory{Brand: "Bimo Septiawan", Niche: "AI Personal"}
	if m.FitsAccount("faktakalbar_", "Fakta Kalbar") {
		t.Fatal("workspace brand must not match unrelated IG account")
	}
	m2 := Memory{Brand: "@bimosept"}
	if !m2.FitsAccount("bimosept", "Bimo") {
		t.Fatal("handle should match threads username")
	}
	empty := Memory{}
	if empty.FitsAccount("faktakalbar_", "Fakta Kalbar") {
		t.Fatal("empty brand must not match")
	}
}
