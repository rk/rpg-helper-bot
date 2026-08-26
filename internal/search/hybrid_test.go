package search_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/embed"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func TestHybridSearchRespectsGameScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := store.OpenSQLite(path)
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

	sections := []models.TOCSection{
		{Title: "Combat Rules", StartPage: 1, EndPage: 5},
		{Title: "Magic Rules", StartPage: 6, EndPage: 10},
	}
	if err := s.SaveTOCSections(pdf.ID, sections); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.ListTOCSections(pdf.ID)
	if err := s.SaveSectionText(saved[0].ID, "Combat uses action cards and wild attack rules."); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSectionText(saved[1].ID, "Magic spells cost power points."); err != nil {
		t.Fatal(err)
	}

	svc := &search.Service{Store: s, Embed: embed.HashEmbed}
	hits, err := svc.Search(context.Background(), game.ID, "wild attack combat")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected search hits")
	}
	if hits[0].SectionTitle != "Combat Rules" {
		t.Fatalf("expected combat section first, got %q", hits[0].SectionTitle)
	}
}
