package search_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func TestFormatChatLearnings_cheatsheetWithCitations(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{Features: []rpgconcepts.Feature{{
		ID: "skill_check", Name: "Skill Check", Synonyms: []string{"skill check", "test"},
	}}}
	meta := map[string]models.PDFIndexMeta{
		"pdf1": {Cheatsheet: []models.CheatsheetEntry{{
			FeatureID:  "skill_check",
			Definition: "Roll a trait die against a target number.",
			Citations: []models.CheatsheetCitation{{
				SectionID: "sec-tests", SectionTitle: "Making Tests", StartPage: 42, EndPage: 43,
			}},
		}}},
	}
	learnings := search.FormatChatLearnings(meta, map[string]string{"pdf1": "Core"}, catalog, "how do skill checks work")
	if learnings.Glossary != "" {
		t.Fatalf("expected empty glossary block, got %q", learnings.Glossary)
	}
	cheatsheet := learnings.Cheatsheet
	if !strings.Contains(cheatsheet, "Skill Check") || !strings.Contains(cheatsheet, "Roll a trait die") {
		t.Fatalf("expected cheatsheet definition: %q", cheatsheet)
	}
	if !strings.Contains(cheatsheet, "Making Tests") || !strings.Contains(cheatsheet, "section_id=sec-tests") {
		t.Fatalf("expected citation with section id: %q", cheatsheet)
	}
}

func TestCheatsheetCitationHits_loadsCitedSections(t *testing.T) {
	dir := t.TempDir()
	s, err := store.OpenSQLite(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	pdf := &models.PDF{Title: "Core", FilePath: "/tmp/core.pdf"}
	if err := s.SavePDF(pdf); err != nil {
		t.Fatal(err)
	}
	game := &models.Game{Name: "Table"}
	if err := s.SaveGame(game); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGamePDFs(game.ID, []string{pdf.ID}); err != nil {
		t.Fatal(err)
	}

	sections := []models.TOCSection{{Title: "Making Tests", StartPage: 42, EndPage: 43}}
	if err := s.SaveTOCSections(pdf.ID, sections); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.ListTOCSections(pdf.ID)
	if err := s.SaveSectionText(saved[0].ID, strings.Repeat("trait roll target number ", 80)); err != nil {
		t.Fatal(err)
	}

	meta := &models.PDFIndexMeta{
		Features: []string{"skill_check"},
		Cheatsheet: []models.CheatsheetEntry{{
			FeatureID:  "skill_check",
			Definition: "Roll a trait die.",
			Citations: []models.CheatsheetCitation{{
				SectionID: saved[0].ID, SectionTitle: "Making Tests", StartPage: 42, EndPage: 43,
			}},
		}},
	}
	if err := s.SavePDFIndexMeta(pdf.ID, meta); err != nil {
		t.Fatal(err)
	}

	hits, err := search.CheatsheetCitationHits(s, game.ID, "skill check")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 citation hit, got %d", len(hits))
	}
	if hits[0].SectionID != saved[0].ID {
		t.Fatalf("unexpected section id: %q", hits[0].SectionID)
	}
	if !strings.Contains(hits[0].Snippet, "trait roll") {
		t.Fatalf("expected section text in snippet: %q", hits[0].Snippet)
	}
}

func TestMergeSearchHits_prefersPrimary(t *testing.T) {
	primary := []models.SearchHit{{SectionID: "a", SectionTitle: "Primary"}}
	secondary := []models.SearchHit{
		{SectionID: "a", SectionTitle: "Secondary"},
		{SectionID: "b", SectionTitle: "Other"},
	}
	merged := search.MergeSearchHits(primary, secondary, 10)
	if len(merged) != 2 || merged[0].SectionTitle != "Primary" || merged[1].SectionID != "b" {
		t.Fatalf("unexpected merge order: %+v", merged)
	}
}
