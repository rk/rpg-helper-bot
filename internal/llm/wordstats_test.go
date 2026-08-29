package llm

import (
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
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
	tsv := BuildWordFrequencyTSV(samples, 10, nil)
	if !strings.Contains(tsv, "TRAIT\t3") {
		t.Fatalf("missing TRAIT count: %q", tsv)
	}
	if !strings.Contains(tsv, "TESTS\t3") {
		t.Fatalf("missing TESTS count: %q", tsv)
	}
	lines := strings.Split(strings.TrimSpace(tsv), "\n")
	if lines[0] != "word\tcount" {
		t.Fatalf("bad header: %q", lines[0])
	}
}

func TestBuildWordFrequencyTSV_topN(t *testing.T) {
	samples := []SectionSample{{PlainText: strings.Repeat("alpha beta ", 50)}}
	tsv := BuildWordFrequencyTSV(samples, 1, nil)
	lines := strings.Split(strings.TrimSpace(tsv), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header + 1 row, got %d lines: %q", len(lines), tsv)
	}
}

func TestBuildWordFrequencyTSV_excludesStopwords(t *testing.T) {
	samples := []SectionSample{{PlainText: strings.Repeat("the the the combat combat ", 20)}}
	tsv := BuildWordFrequencyTSV(samples, 10, nil)
	if strings.Contains(tsv, "THE\t") {
		t.Fatalf("expected stopword THE excluded: %q", tsv)
	}
	if !strings.Contains(tsv, "COMBAT\t") {
		t.Fatalf("expected COMBAT kept: %q", tsv)
	}
}

func TestBuildWordFrequencyTSV_forceIncludesCatalogSynonym(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{{
			ID:       "benny",
			Synonyms: []string{"benny", "hero point"},
		}},
	}
	samples := []SectionSample{{PlainText: strings.Repeat("the the the alpha ", 50) + " benny"}}
	tsv := BuildWordFrequencyTSV(samples, 1, catalog)
	if !strings.Contains(tsv, "BENNY\t") {
		t.Fatalf("expected protected synonym BENNY included: %q", tsv)
	}
	if strings.Contains(tsv, "THE\t") {
		t.Fatalf("expected stopword THE excluded: %q", tsv)
	}
}

func TestBuildWordFrequencyTSV_keepsProtectedStopwordShape(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{{
			ID:       "target_number",
			Synonyms: []string{"target"},
		}},
	}
	samples := []SectionSample{{PlainText: "the target value and the other target"}}
	tsv := BuildWordFrequencyTSV(samples, 2, catalog)
	if !strings.Contains(tsv, "TARGET\t") {
		t.Fatalf("expected protected synonym TARGET included: %q", tsv)
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
