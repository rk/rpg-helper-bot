package indexing

import (
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

func TestCompilePDFIndexMeta_rateOfFire(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{Features: []rpgconcepts.Feature{{
		ID: "rate_of_fire", Name: "Rate of Fire", Synonyms: []string{"rate of fire", "rof"},
	}}}
	sections := []models.TOCSection{{
		Title: "Ranged Attacks", StartPage: 95, PlainText: "Rate of Fire: how many shots per action. RoF 2 means two dice.",
	}}
	meta := CompilePDFIndexMeta(catalog, sections)
	if len(meta.Features) != 1 || meta.Features[0].FeatureID != "rate_of_fire" {
		t.Fatalf("features: %+v", meta.Features)
	}
	if len(meta.Glossary) == 0 {
		t.Fatal("expected glossary entries")
	}
}
