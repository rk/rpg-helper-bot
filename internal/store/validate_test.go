package store_test

import (
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func TestValidateGame(t *testing.T) {
	if err := store.ValidateGame(models.Game{}); err == nil {
		t.Fatal("expected error for empty name")
	}
	if err := store.ValidateGame(models.Game{Name: "My Campaign"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTOCSections(t *testing.T) {
	err := store.ValidateTOCSections([]models.TOCSection{
		{Title: "A", StartPage: 5, EndPage: 3},
	}, 10)
	if err == nil {
		t.Fatal("expected end page validation error")
	}
}
