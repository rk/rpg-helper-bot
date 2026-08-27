package search_test

import (
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
)

func TestFormatGlossaryForSearchRewrite_includesMatchedTerms(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{{
			ID:       "skill_check",
			Name:     "Skill Check",
			Synonyms: []string{"skill check", "test"},
		}},
	}
	metaByPDF := map[string]models.PDFIndexMeta{
		"pdf-1": {
			Glossary: []models.PDFGlossaryEntry{{
				FeatureID: "skill_check",
				Terms:     []string{"Tests", "Trait roll"},
			}},
		},
	}
	titles := map[string]string{"pdf-1": "Core Rules"}

	out := search.FormatGlossaryForSearchRewrite(metaByPDF, titles, catalog, "how do skill checks work")
	if !strings.Contains(out, "Tests") || !strings.Contains(out, "Trait roll") {
		t.Fatalf("expected glossary terms in rewrite block: %q", out)
	}
	if !strings.Contains(out, "Skill Check") {
		t.Fatalf("expected feature label in rewrite block: %q", out)
	}
}

func TestFormatGlossaryForSearchRewrite_emptyWithoutGlossary(t *testing.T) {
	out := search.FormatGlossaryForSearchRewrite(map[string]models.PDFIndexMeta{
		"pdf-1": {Glossary: nil},
	}, map[string]string{"pdf-1": "Core"}, nil, "combat")
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
}
