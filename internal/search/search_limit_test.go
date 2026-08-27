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

func TestSearchWithLimit(t *testing.T) {
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

	sections := []models.TOCSection{
		{Title: "Combat One", StartPage: 1, EndPage: 2},
		{Title: "Combat Two", StartPage: 3, EndPage: 4},
		{Title: "Combat Three", StartPage: 5, EndPage: 6},
		{Title: "Combat Four", StartPage: 7, EndPage: 8},
		{Title: "Combat Five", StartPage: 9, EndPage: 10},
		{Title: "Combat Six", StartPage: 11, EndPage: 12},
	}
	if err := s.SaveTOCSections(pdf.ID, sections); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.ListTOCSections(pdf.ID)
	for i, sec := range saved {
		text := "combat wild attack rules section " + sec.Title
		if err := s.SaveSectionText(sec.ID, text); err != nil {
			t.Fatal(err)
		}
		_ = i
	}

	svc := &search.Service{Store: s, Embed: embed.HashEmbed}
	result, err := svc.SearchWithLimit(context.Background(), game.ID, "combat wild attack", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) > 3 {
		t.Fatalf("expected at most 3 hits, got %d", len(result.Hits))
	}
	if len(result.Hits) == 0 {
		t.Fatal("expected at least one hit")
	}
}
