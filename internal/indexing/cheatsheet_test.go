package indexing

import (
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

func TestPendingCheatsheetFeatures(t *testing.T) {
	meta := &models.PDFIndexMeta{
		Features:   []string{"skill_check", "wild_die", "benny"},
		Cheatsheet: []models.CheatsheetEntry{{FeatureID: "skill_check", Definition: "x"}},
	}
	pending := pendingCheatsheetFeatures(meta)
	if len(pending) != 2 || pending[0] != "wild_die" || pending[1] != "benny" {
		t.Fatalf("unexpected pending: %v", pending)
	}
}

func TestUpsertCheatsheetEntry_replacesExisting(t *testing.T) {
	meta := &models.PDFIndexMeta{
		Cheatsheet: []models.CheatsheetEntry{
			{FeatureID: "skill_check", Definition: "old"},
			{FeatureID: "benny", Definition: "keep"},
		},
	}
	upsertCheatsheetEntry(meta, models.CheatsheetEntry{FeatureID: "skill_check", Definition: "new"})
	if len(meta.Cheatsheet) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(meta.Cheatsheet))
	}
	if meta.Cheatsheet[0].Definition != "new" {
		t.Fatalf("expected replacement, got %+v", meta.Cheatsheet[0])
	}
}

func TestUpsertCheatsheetEntry_appendsNew(t *testing.T) {
	meta := &models.PDFIndexMeta{}
	upsertCheatsheetEntry(meta, models.CheatsheetEntry{FeatureID: "feat", Definition: "new"})
	if len(meta.Cheatsheet) != 1 || meta.Cheatsheet[0].FeatureID != "feat" {
		t.Fatalf("unexpected cheatsheet: %+v", meta.Cheatsheet)
	}
}

func TestFeatureAllowedForCheatsheetRebuild(t *testing.T) {
	meta := &models.PDFIndexMeta{
		Features:   []string{"skill_check"},
		Cheatsheet: []models.CheatsheetEntry{{FeatureID: "manual_row", Definition: "x"}},
	}
	if !featureAllowedForCheatsheetRebuild(meta, "skill_check") {
		t.Fatal("expected detected feature allowed")
	}
	if !featureAllowedForCheatsheetRebuild(meta, "manual_row") {
		t.Fatal("expected cheatsheet row allowed")
	}
	if featureAllowedForCheatsheetRebuild(meta, "unknown") {
		t.Fatal("expected unknown feature rejected")
	}
}

func TestCatalogCheatsheetFallback(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{{
			ID:          "opposed_roll",
			Description: "Two characters roll against each other.",
		}},
	}
	entry := catalogCheatsheetFallback(catalog, "opposed_roll")
	if entry == nil || entry.Definition == "" {
		t.Fatalf("expected fallback entry, got %+v", entry)
	}
	if catalogCheatsheetFallback(catalog, "missing") != nil {
		t.Fatal("expected nil for unknown feature")
	}
}
