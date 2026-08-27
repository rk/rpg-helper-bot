package indexing

import (
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

func TestCheatsheetProgressMessage_withFeature(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{{
			ID:   "attribute_check",
			Name: "Attribute Check",
		}},
	}
	msg := cheatsheetProgressMessage(3, 8, "attribute_check", catalog)
	if !strings.Contains(msg, "Attribute Check") {
		t.Fatalf("expected feature name in message: %q", msg)
	}
	if !strings.Contains(msg, "3/8") {
		t.Fatalf("expected progress fraction in message: %q", msg)
	}
}

func TestCheatsheetProgressMessage_withoutFeature(t *testing.T) {
	msg := cheatsheetProgressMessage(0, 5, "", nil)
	if !strings.Contains(msg, "0/5") {
		t.Fatalf("expected starting fraction: %q", msg)
	}
}
