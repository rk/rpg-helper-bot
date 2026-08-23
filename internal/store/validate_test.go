package store_test

import (
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func TestValidateGame(t *testing.T) {
	if err := store.ValidateGame(models.Game{Name: "  "}); err == nil {
		t.Fatal("expected empty name error")
	}
	if err := store.ValidateGame(models.Game{Name: "Valid"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateTOCSectionsPageRange(t *testing.T) {
	err := store.ValidateTOCSections([]models.TOCSection{
		{Title: "Chapter", StartPage: 1, EndPage: 5},
	}, 3)
	if err == nil {
		t.Fatal("expected page count validation error")
	}
}
