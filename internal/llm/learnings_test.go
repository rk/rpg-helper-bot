package llm_test

import (
	"context"
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

func TestBuildCheatsheetFromSearchHits_catalogFallback(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{{
			ID:          "attribute",
			Name:        "Attribute",
			Description: "An Attribute represents inherent traits.",
		}},
	}
	c := &llm.Client{}
	entry, err := c.BuildCheatsheetFromSearchHits(context.Background(), catalog, nil, "attribute", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil || entry.Definition == "" {
		t.Fatalf("expected catalog fallback entry, got %+v", entry)
	}
	if entry.FeatureID != "attribute" {
		t.Fatalf("unexpected feature_id: %q", entry.FeatureID)
	}
}

func TestBuildSectionSamples_truncates(t *testing.T) {
	long := strings.Repeat("word ", 2000)
	sections := []models.TOCSection{{
		ID: "s1", Title: "Tests", StartPage: 1, EndPage: 2,
		PlainText: long,
	}}
	samples := llm.BuildSectionSamples(sections)
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	words := strings.Fields(strings.TrimSuffix(samples[0].Excerpt, "..."))
	if len(words) != 1000 {
		t.Fatalf("expected 1000 words in excerpt, got %d", len(words))
	}
}

func TestAttachSectionIDs(t *testing.T) {
	cheatsheet := []models.CheatsheetEntry{{
		FeatureID: "skill_check",
		Citations: []models.CheatsheetCitation{{SectionTitle: "Making Tests", StartPage: 42}},
	}}
	sections := []models.TOCSection{{ID: "sec-1", Title: "Making Tests"}}
	llm.AttachSectionIDs(cheatsheet, sections)
	if cheatsheet[0].Citations[0].SectionID != "sec-1" {
		t.Fatalf("section id not attached: %+v", cheatsheet[0].Citations[0])
	}
}

func TestMergeGlossary_dedupesTerms(t *testing.T) {
	existing := []models.PDFGlossaryEntry{{FeatureID: "skill_check", Terms: []string{"Tests"}}}
	updated := []models.PDFGlossaryEntry{{FeatureID: "skill_check", Terms: []string{"Tests", "Trait roll"}}}
	merged := llm.MergeGlossary(existing, updated)
	if len(merged) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(merged))
	}
	if len(merged[0].Terms) != 2 {
		t.Fatalf("expected 2 terms, got %v", merged[0].Terms)
	}
}
