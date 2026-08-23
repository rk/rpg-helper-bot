package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

func TestPDFScopedToGame(t *testing.T) {
	s, _ := openTestStore(t)

	gameA := models.Game{Name: "Game A"}
	gameB := models.Game{Name: "Game B"}
	if err := s.SaveGame(&gameA); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveGame(&gameB); err != nil {
		t.Fatal(err)
	}

	pdfPath := filepath.Join(t.TempDir(), "shared.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}

	pdfA := models.PDF{GameID: gameA.ID, Title: "Rules A", FilePath: pdfPath}
	if err := s.AddPDF(&pdfA); err != nil {
		t.Fatal(err)
	}
	pdfB := models.PDF{GameID: gameB.ID, Title: "Rules B", FilePath: pdfPath}
	if err := s.AddPDF(&pdfB); err != nil {
		t.Fatal(err)
	}

	if err := s.SaveTOCSections(pdfA.ID, gameA.ID, []models.TOCSection{
		{Title: "Combat", StartPage: 1, EndPage: 2},
	}); err != nil {
		t.Fatal(err)
	}

	sectionsB, err := s.ListTOCSections(pdfB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sectionsB) != 0 {
		t.Fatalf("expected independent TOC per game PDF, got %+v", sectionsB)
	}

	if err := s.AddPDF(&models.PDF{GameID: gameA.ID, Title: "Dup", FilePath: pdfPath}); err == nil {
		t.Fatal("expected duplicate path on same game to fail")
	}

	if err := s.RemovePDF(pdfA.ID, gameB.ID); err == nil {
		t.Fatal("expected remove with wrong game to fail")
	}
}
