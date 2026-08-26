package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func openTestStore(t *testing.T) (*store.SQLiteStore, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return s, path
}

func TestSharedPDFAcrossGames(t *testing.T) {
	s, _ := openTestStore(t)
	defer s.Close()

	pdf := &models.PDF{Title: "Core Rules", FilePath: "/tmp/core.pdf"}
	if err := s.SavePDF(pdf); err != nil {
		t.Fatal(err)
	}

	g1 := &models.Game{Name: "Campaign A"}
	g2 := &models.Game{Name: "Campaign B"}
	if err := s.SaveGame(g1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveGame(g2); err != nil {
		t.Fatal(err)
	}

	if err := s.SetGamePDFs(g1.ID, []string{pdf.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGamePDFs(g2.ID, []string{pdf.ID}); err != nil {
		t.Fatal(err)
	}

	refs, err := s.CountPDFGameRefs(pdf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refs != 2 {
		t.Fatalf("expected 2 game refs, got %d", refs)
	}
}

func TestGamePDFOrder(t *testing.T) {
	s, _ := openTestStore(t)
	defer s.Close()

	g := &models.Game{Name: "Ordered Game"}
	p1 := &models.PDF{Title: "Base", FilePath: "/tmp/base.pdf"}
	p2 := &models.PDF{Title: "Expansion", FilePath: "/tmp/exp.pdf"}
	for _, item := range []*models.PDF{p1, p2} {
		if err := s.SavePDF(item); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveGame(g); err != nil {
		t.Fatal(err)
	}

	if err := s.SetGamePDFs(g.ID, []string{p2.ID, p1.ID}); err != nil {
		t.Fatal(err)
	}
	entries, err := s.ListGamePDFs(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Title != "Expansion" || entries[1].Title != "Base" {
		t.Fatalf("unexpected order: %+v", entries)
	}
}

func TestArchiveAndRestoreGame(t *testing.T) {
	s, _ := openTestStore(t)
	defer s.Close()

	g := &models.Game{Name: "Archive Me"}
	if err := s.SaveGame(g); err != nil {
		t.Fatal(err)
	}
	if err := s.ArchiveGame(g.ID); err != nil {
		t.Fatal(err)
	}
	active, err := s.ListGames(store.ListActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active games, got %d", len(active))
	}
	if err := s.RestoreGame(g.ID); err != nil {
		t.Fatal(err)
	}
	active, err = s.ListGames(store.ListActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active game, got %d", len(active))
	}
}

func TestSingleRunningGame(t *testing.T) {
	s, _ := openTestStore(t)
	defer s.Close()

	g1 := &models.Game{Name: "Table 1"}
	g2 := &models.Game{Name: "Table 2"}
	if err := s.SaveGame(g1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveGame(g2); err != nil {
		t.Fatal(err)
	}

	if _, err := s.SetGameRunning(g1.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetGameRunning(g2.ID, true); err != nil {
		t.Fatal(err)
	}

	running, err := s.GetRunningGame()
	if err != nil {
		t.Fatal(err)
	}
	if running == nil || running.ID != g2.ID {
		t.Fatalf("expected g2 running, got %+v", running)
	}

	g1Updated, err := s.GetGame(g1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if g1Updated.Running {
		t.Fatal("g1 should no longer be running")
	}
}

func TestArchiveStopsRunning(t *testing.T) {
	s, _ := openTestStore(t)
	defer s.Close()

	g := &models.Game{Name: "Running Archive"}
	if err := s.SaveGame(g); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetGameRunning(g.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.ArchiveGame(g.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := s.GetGame(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Running {
		t.Fatal("archiving should stop running")
	}
}

func TestPathProbe(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.pdf")
	if err := os.WriteFile(file, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}

	if store.ProbePath(file) != models.PathStatusOK {
		t.Fatal("expected ok for existing file")
	}
	if store.ProbePath(filepath.Join(dir, "missing.pdf")) != models.PathStatusMissing {
		t.Fatal("expected missing for absent file")
	}
}

func TestDeletePDFBlockedWhenAttached(t *testing.T) {
	s, _ := openTestStore(t)
	defer s.Close()

	pdf := &models.PDF{Title: "Attached", FilePath: "/tmp/a.pdf"}
	g := &models.Game{Name: "Uses PDF"}
	if err := s.SavePDF(pdf); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveGame(g); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGamePDFs(g.ID, []string{pdf.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeletePDF(pdf.ID); err == nil {
		t.Fatal("expected delete to fail when attached")
	}
}

func TestSaveTOCSections(t *testing.T) {
	s, _ := openTestStore(t)
	defer s.Close()

	pdf := &models.PDF{Title: "Book", FilePath: "/tmp/book.pdf", PageCount: 100}
	if err := s.SavePDF(pdf); err != nil {
		t.Fatal(err)
	}
	sections := []models.TOCSection{
		{Title: "Combat", StartPage: 10, EndPage: 40},
		{Title: "Magic", StartPage: 41, EndPage: 80},
	}
	if err := s.SaveTOCSections(pdf.ID, sections); err != nil {
		t.Fatal(err)
	}
	saved, err := s.ListTOCSections(pdf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 || saved[0].Title != "Combat" || saved[1].SortOrder != 1 {
		t.Fatalf("unexpected sections: %+v", saved)
	}
}
