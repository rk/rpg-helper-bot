package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func openTestStore(t *testing.T) (*store.SQLiteStore, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

func TestGamePDFTOCRoundTrip(t *testing.T) {
	s, _ := openTestStore(t)

	game := models.Game{
		Name:               "Stormwreck",
		Notes:              "Coastal campaign",
		OptionalRulesOptIn: "We use optional downtime rules",
	}
	if err := s.SaveGame(&game); err != nil {
		t.Fatalf("SaveGame: %v", err)
	}
	saved, err := s.GetGame(game.ID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if saved.OptionalRulesOptIn != game.OptionalRulesOptIn {
		t.Fatalf("optional opt-in mismatch: %q", saved.OptionalRulesOptIn)
	}

	pdfPath := filepath.Join(t.TempDir(), "core.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}
	pdf := models.PDF{
		GameID:   game.ID,
		Title:    "Core Rulebook",
		FilePath: pdfPath,
	}
	if err := s.AddPDF(&pdf); err != nil {
		t.Fatalf("AddPDF: %v", err)
	}

	sections := []models.TOCSection{
		{PDFID: pdf.ID, Title: "Character Creation", StartPage: 12, EndPage: 28, SortOrder: 0},
		{PDFID: pdf.ID, Title: "Combat", StartPage: 29, EndPage: 54, SortOrder: 1},
		{PDFID: pdf.ID, Title: "Downtime", StartPage: 90, EndPage: 95, SortOrder: 2, Optional: true},
	}
	if err := s.SaveTOCSections(pdf.ID, sections); err != nil {
		t.Fatalf("SaveTOCSections: %v", err)
	}

	loaded, err := s.ListTOCSections(pdf.ID)
	if err != nil {
		t.Fatalf("ListTOCSections: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(loaded))
	}
	if !loaded[2].Optional {
		t.Fatal("expected third section to be optional")
	}

	refs, err := s.ListOptionalSections(game.ID)
	if err != nil {
		t.Fatalf("ListOptionalSections: %v", err)
	}
	if len(refs) != 1 || refs[0].SectionTitle != "Downtime" {
		t.Fatalf("unexpected optional refs: %+v", refs)
	}

	total, optional, err := s.CountTOCSections(pdf.ID)
	if err != nil {
		t.Fatalf("CountTOCSections: %v", err)
	}
	if total != 3 || optional != 1 {
		t.Fatalf("counts = %d/%d", total, optional)
	}
}

func TestArchiveRestoreGame(t *testing.T) {
	s, _ := openTestStore(t)
	game := models.Game{Name: "Archive Me"}
	if err := s.SaveGame(&game); err != nil {
		t.Fatal(err)
	}
	if err := s.ArchiveGame(game.ID); err != nil {
		t.Fatal(err)
	}
	active, err := s.ListGames(store.ListActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("expected 0 active games, got %d", len(active))
	}
	archived, err := s.ListGames(store.ListArchived)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived game, got %d", len(archived))
	}
	if err := s.RestoreGame(game.ID); err != nil {
		t.Fatal(err)
	}
	active, err = s.ListGames(store.ListActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active game after restore, got %d", len(active))
	}
}

func TestSaveTOCSectionsReplaceAll(t *testing.T) {
	s, _ := openTestStore(t)
	game := models.Game{Name: "Replace Test", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.SaveGame(&game); err != nil {
		t.Fatal(err)
	}
	pdfPath := filepath.Join(t.TempDir(), "book.pdf")
	if err := os.WriteFile(pdfPath, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	pdf := models.PDF{GameID: game.ID, Title: "Book", FilePath: pdfPath}
	if err := s.AddPDF(&pdf); err != nil {
		t.Fatal(err)
	}

	first := []models.TOCSection{
		{Title: "A", StartPage: 1, EndPage: 2},
		{Title: "B", StartPage: 3, EndPage: 4},
	}
	if err := s.SaveTOCSections(pdf.ID, first); err != nil {
		t.Fatal(err)
	}
	second := []models.TOCSection{
		{Title: "Only", StartPage: 10, EndPage: 11, Optional: true},
	}
	if err := s.SaveTOCSections(pdf.ID, second); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.ListTOCSections(pdf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Title != "Only" {
		t.Fatalf("replace-all failed: %+v", loaded)
	}
}

func TestValidateTOCSections(t *testing.T) {
	err := store.ValidateTOCSections([]models.TOCSection{
		{Title: "Bad", StartPage: 5, EndPage: 2},
	}, 0)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
