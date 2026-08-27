package rpgconcepts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConcepts(t *testing.T) {
	path := filepath.Join("..", "..", "data", "rpg-concepts.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("concepts file not present")
	}
	c, err := LoadConcepts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Features) < 3 {
		t.Fatalf("expected features, got %d", len(c.Features))
	}
	f, ok := c.FeatureByID("skill_check")
	if !ok || f.Name == "" {
		t.Fatal("skill_check feature missing")
	}
	skill, ok := c.FeatureByID("skill")
	if !ok || len(skill.Questions) == 0 {
		t.Fatal("skill feature questions missing")
	}
}

func TestMatchQueryFeatures(t *testing.T) {
	c := &ConceptCatalog{Features: []Feature{{
		ID: "skill_check", Synonyms: []string{"skill check", "trait test", "test"},
	}}}
	ids := c.MatchQueryFeatures("How does a skill check work?")
	if len(ids) != 1 || ids[0] != "skill_check" {
		t.Fatalf("got %v", ids)
	}
}

func TestScanText(t *testing.T) {
	c := &ConceptCatalog{Features: []Feature{{
		ID: "rate_of_fire", Synonyms: []string{"rate of fire", "rof"},
	}}}
	matches := c.ScanText("Rate of Fire: how many shots per action. RoF 2 means two dice.")
	if len(matches) < 2 {
		t.Fatalf("expected multiple matches, got %v", matches)
	}
}
