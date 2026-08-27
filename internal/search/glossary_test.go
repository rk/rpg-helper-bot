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
		"pdf1": {Glossary: []models.PDFGlossaryEntry{{FeatureID: "skill_check", Terms: []string{"Tests", "Trait roll"}}}},
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

func TestExpandFTSWithGlossary_termReverseLookup(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{Features: []rpgconcepts.Feature{{
		ID: "skill_check", Synonyms: []string{"skill check"},
	}}}
	meta := map[string]models.PDFIndexMeta{
		"pdf1": {Glossary: []models.PDFGlossaryEntry{{FeatureID: "skill_check", Terms: []string{"Tests", "Trait roll"}}}},
	}
	out := ExpandFTSWithGlossary("how do Tests work", "tests work", meta, catalog)
	if !strings.Contains(out, "Trait roll") {
		t.Fatalf("expected Trait roll from reverse lookup in %q", out)
	}
}

func TestCitationSectionIDs(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{Features: []rpgconcepts.Feature{{
		ID: "rate_of_fire", Synonyms: []string{"rate of fire", "rof"},
	}}}
	meta := map[string]models.PDFIndexMeta{
		"pdf1": {Cheatsheet: []models.CheatsheetEntry{{
			FeatureID: "rate_of_fire",
			Citations: []models.CheatsheetCitation{{SectionID: "sec-rof", SectionTitle: "Ranged", StartPage: 95}},
		}}},
	}
	ids := CitationSectionIDs("how does rate of fire work", meta, catalog)
	if _, ok := ids["sec-rof"]; !ok {
		t.Fatalf("expected sec-rof in citations: %+v", ids)
	}
}
