package llm

import "testing"

func TestParseSearchKeywords(t *testing.T) {
	got := parseSearchKeywords(`necromancer necromancy undead`)
	if got != "necromancer necromancy undead" {
		t.Fatalf("got %q", got)
	}
}

func TestParseSearchKeywords_firstLineOnly(t *testing.T) {
	got := parseSearchKeywords("soak wounds vigor\nExplanation: soaking damage")
	if got != "soak wounds vigor" {
		t.Fatalf("got %q", got)
	}
}

func TestMergeSearchQueries(t *testing.T) {
	got := mergeSearchQueries("necromancer build", "necromancy undead")
	want := "necromancer build necromancy undead"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMergeSearchQueries_dedupes(t *testing.T) {
	got := mergeSearchQueries("wild attack", "Wild Attack combat")
	if got != "wild attack combat" {
		t.Fatalf("got %q", got)
	}
}
