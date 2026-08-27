package search_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func TestLookupCheatsheetByFeature(t *testing.T) {
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

	meta := &models.PDFIndexMeta{
		Features: []string{"skill_check"},
		Cheatsheet: []models.CheatsheetEntry{{
			FeatureID:  "skill_check",
			Definition: "Roll a trait die against TN.",
			Citations: []models.CheatsheetCitation{{
				SectionID: "sec-1", SectionTitle: "Making Tests", StartPage: 42,
			}},
		}},
	}
	if err := s.SavePDFIndexMeta(pdf.ID, meta); err != nil {
		t.Fatal(err)
	}

	out, err := search.LookupCheatsheetByFeature(s, game.ID, "skill_check")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Roll a trait die") || !strings.Contains(out, "Making Tests") {
		t.Fatalf("unexpected cheatsheet output: %q", out)
	}

	empty, err := search.LookupCheatsheetByFeature(s, game.ID, "feat")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(empty, "No cheatsheet entry") {
		t.Fatalf("expected empty message, got %q", empty)
	}
}

func TestLookupGlossaryByFeature(t *testing.T) {
	t.Setenv("RPG_HELPER_CONCEPTS_FILE", filepath.Join("..", "..", "data", "rpg-concepts.yaml"))
	dir := t.TempDir()
	st, err := store.OpenSQLite(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	pdf := &models.PDF{Title: "Core", FilePath: "/tmp/core.pdf"}
	if err := st.SavePDF(pdf); err != nil {
		t.Fatal(err)
	}
	game := &models.Game{Name: "Table"}
	if err := st.SaveGame(game); err != nil {
		t.Fatal(err)
	}
	if err := st.SetGamePDFs(game.ID, []string{pdf.ID}); err != nil {
		t.Fatal(err)
	}

	meta := &models.PDFIndexMeta{
		Glossary: []models.PDFGlossaryEntry{{
			FeatureID: "skill_check",
			Terms:     []string{"Tests", "Trait roll"},
		}},
	}
	if err := st.SavePDFIndexMeta(pdf.ID, meta); err != nil {
		t.Fatal(err)
	}

	out, err := search.LookupGlossaryByFeature(st, game.ID, "skill_check")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Tests") || !strings.Contains(out, "Catalog synonyms") {
		t.Fatalf("unexpected glossary output: %q", out)
	}
}

func TestLookupGlossaryByTerm(t *testing.T) {
	t.Setenv("RPG_HELPER_CONCEPTS_FILE", filepath.Join("..", "..", "data", "rpg-concepts.yaml"))
	dir := t.TempDir()
	st, err := store.OpenSQLite(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	pdf := &models.PDF{Title: "Core", FilePath: "/tmp/core.pdf"}
	if err := st.SavePDF(pdf); err != nil {
		t.Fatal(err)
	}
	game := &models.Game{Name: "Table"}
	if err := st.SaveGame(game); err != nil {
		t.Fatal(err)
	}
	if err := st.SetGamePDFs(game.ID, []string{pdf.ID}); err != nil {
		t.Fatal(err)
	}

	meta := &models.PDFIndexMeta{
		Glossary: []models.PDFGlossaryEntry{{
			FeatureID: "skill_check", Terms: []string{"Tests"},
		}},
	}
	if err := st.SavePDFIndexMeta(pdf.ID, meta); err != nil {
		t.Fatal(err)
	}

	out, err := search.LookupGlossaryByTerm(st, game.ID, "Tests")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Tests") {
		t.Fatalf("expected term mapping, got %q", out)
	}
}
