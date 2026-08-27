package llm_test

import (
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

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

func TestChunkSections(t *testing.T) {
	samples := make([]llm.SectionSample, 13)
	chunks := llm.ChunkSections(samples, 6)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 6 || len(chunks[1]) != 6 || len(chunks[2]) != 1 {
		t.Fatalf("unexpected chunk sizes: %d, %d, %d", len(chunks[0]), len(chunks[1]), len(chunks[2]))
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
