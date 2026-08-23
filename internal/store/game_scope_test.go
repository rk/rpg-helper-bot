package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

func TestPDFLibrarySharedAcrossGames(t *testing.T) {
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

	pdf := models.PDF{Title: "Shared Rules", FilePath: pdfPath}
	if err := s.SavePDF(&pdf); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTOCSections(pdf.ID, []models.TOCSection{
		{Title: "Combat", StartPage: 1, EndPage: 2},
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.AttachPDFToGame(gameA.ID, pdf.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AttachPDFToGame(gameB.ID, pdf.ID); err != nil {
		t.Fatal(err)
	}

	attachedA, err := s.ListGamePDFs(gameA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attachedA) != 1 || attachedA[0].ID != pdf.ID {
		t.Fatalf("game A attachments: %+v", attachedA)
	}

	if err := s.DetachPDFFromGame(gameA.ID, pdf.ID); err != nil {
		t.Fatal(err)
	}
	attachedB, err := s.ListGamePDFs(gameB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attachedB) != 1 {
		t.Fatalf("game B should still have pdf attached, got %+v", attachedB)
	}

	if err := s.DeletePDF(pdf.ID); err == nil {
		t.Fatal("expected delete to fail while attached to game B")
	}
}
