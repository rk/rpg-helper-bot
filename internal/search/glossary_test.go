package search

import (
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

func TestExpandFTSWithGlossary(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{Features: []rpgconcepts.Feature{{
		ID: "skill_check", Synonyms: []string{"skill check", "test"},
	}}}
	meta := map[string]models.PDFIndexMeta{
		"pdf1": {Glossary: []models.PDFGlossary{{FeatureID: "skill_check", PDFTerm: "Tests"}}},
	}
	out := ExpandFTSWithGlossary("how does a skill check work", "skill check work", meta, catalog)
	found := false
	for _, w := range strings.Fields(out) {
		if w == "Tests" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Tests in %q", out)
	}
}
