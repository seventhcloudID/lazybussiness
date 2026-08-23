package ai

import (
	"reflect"
	"testing"
)

func TestCleanStrList(t *testing.T) {
	got := cleanStrList([]string{"  Satu ", "", "satu", "Dua", "Tiga"}, 2)
	want := []string{"Satu", "Dua"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanStrList() = %#v, want %#v", got, want)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("  ", " pilihan ", "cadangan"); got != "pilihan" {
		t.Fatalf("firstNonEmpty() = %q, want %q", got, "pilihan")
	}
}
