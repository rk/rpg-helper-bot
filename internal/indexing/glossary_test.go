package indexing

import (
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

func TestCheatsheetIncomplete(t *testing.T) {
	meta := &models.PDFIndexMeta{
		Features:   []string{"skill_check", "wild_die"},
		Cheatsheet: []models.CheatsheetEntry{{FeatureID: "skill_check", Definition: "x"}},
	}
	if !cheatsheetIncomplete(meta) {
		t.Fatal("expected incomplete")
	}
	meta.Cheatsheet = append(meta.Cheatsheet, models.CheatsheetEntry{FeatureID: "wild_die", Definition: "y"})
	if cheatsheetIncomplete(meta) {
		t.Fatal("expected complete")
	}
}

func TestCheatsheetIncomplete_noFeatures(t *testing.T) {
	meta := &models.PDFIndexMeta{
		Glossary: []models.PDFGlossaryEntry{{FeatureID: "skill_check", Terms: []string{"Tests"}}},
	}
	if cheatsheetIncomplete(meta) {
		t.Fatal("glossary-only meta should not count as incomplete cheatsheet")
	}
}
