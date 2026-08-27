package llm

import (
	"strings"
	"testing"
)

func TestTokenizeForWordStats(t *testing.T) {
	tokens := tokenizeForWordStats("Rate of Fire: 2D6+1, Tests!")
	if !hasToken(tokens, "RATE") || !hasToken(tokens, "FIRE") || !hasToken(tokens, "2D6") || !hasToken(tokens, "TESTS") {
		t.Fatalf("unexpected tokens: %v", tokens)
	}
	if !hasToken(tokens, "OF") {
		t.Fatalf("expected two-letter tokens kept: %v", tokens)
	}
}

func TestBuildWordFrequencyTSV_ordersByCount(t *testing.T) {
	samples := []SectionSample{
		{Title: "Combat", PlainText: "Tests Tests trait"},
		{Title: "Skills", PlainText: "Tests trait trait"},
	}
	tsv := BuildWordFrequencyTSV(samples, 10)
	if !strings.Contains(tsv, "TRAIT\t3") {
		t.Fatalf("missing TRAIT count: %q", tsv)
	}
	if !strings.Contains(tsv, "TESTS\t3") {
		t.Fatalf("missing TESTS count: %q", tsv)
	}
	// Header + rows
	lines := strings.Split(strings.TrimSpace(tsv), "\n")
	if lines[0] != "word\tcount" {
		t.Fatalf("bad header: %q", lines[0])
	}
}

func TestBuildWordFrequencyTSV_topN(t *testing.T) {
	samples := []SectionSample{{PlainText: strings.Repeat("alpha beta ", 50)}}
	tsv := BuildWordFrequencyTSV(samples, 1)
	lines := strings.Split(strings.TrimSpace(tsv), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header + 1 row, got %d lines: %q", len(lines), tsv)
	}
}

func hasToken(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
