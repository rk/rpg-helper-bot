package llm

import (
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

func TestFormatDetectedFeaturesBlock_excludesTarget(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{
			{ID: "combat", Name: "Combat", Description: "Conflict resolution."},
			{ID: "skill_check", Name: "Skill Check", Description: "Rolling dice to resolve actions."},
			{ID: "attribute", Name: "Attribute", Description: "Inherent traits."},
		},
	}
	block := formatDetectedFeaturesBlock(catalog, "combat", []string{"combat", "skill_check", "attribute", "unknown"})
	if block == "" {
		t.Fatal("expected non-empty block")
	}
	if !strings.Contains(block, "skill_check") || !strings.Contains(block, "attribute") {
		t.Fatalf("expected related features in block: %q", block)
	}
	if strings.Contains(block, "- combat ") {
		t.Fatalf("target feature should be excluded: %q", block)
	}
	if strings.Contains(block, "unknown") {
		t.Fatalf("unknown feature should be skipped: %q", block)
	}
}

func TestFormatDetectedFeaturesBlock_emptyWhenOnlyTarget(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{
			{ID: "combat", Name: "Combat", Description: "Conflict resolution."},
		},
	}
	if block := formatDetectedFeaturesBlock(catalog, "combat", []string{"combat"}); block != "" {
		t.Fatalf("expected empty block, got %q", block)
	}
}
