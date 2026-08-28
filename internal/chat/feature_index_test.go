package chat_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/chat"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func TestBuildFeatureIndex_onlyDetectedFeatures(t *testing.T) {
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

	meta := &models.PDFIndexMeta{Features: []string{"skill_check"}}
	if err := st.SavePDFIndexMeta(pdf.ID, meta); err != nil {
		t.Fatal(err)
	}

	out, err := chat.BuildFeatureIndex(st, game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "skill_check") || !strings.Contains(out, "Skill Check") {
		t.Fatalf("expected detected feature in index: %q", out)
	}
	if strings.Contains(out, "saving_throw") {
		t.Fatalf("expected undetected features omitted: %q", out)
	}
	if strings.Contains(out, "Questions") || strings.Contains(out, "How are skill checks") {
		t.Fatalf("expected questions omitted: %q", out)
	}
	if strings.Contains(out, "Synonyms:") {
		t.Fatalf("expected catalog synonyms omitted from feature index: %q", out)
	}
}

func TestBuildFeatureIndex_emptyWhenNoneDetected(t *testing.T) {
	t.Setenv("RPG_HELPER_CONCEPTS_FILE", filepath.Join("..", "..", "data", "rpg-concepts.yaml"))

	dir := t.TempDir()
	st, err := store.OpenSQLite(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	game := &models.Game{Name: "Table"}
	if err := st.SaveGame(game); err != nil {
		t.Fatal(err)
	}

	out, err := chat.BuildFeatureIndex(st, game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No features detected") {
		t.Fatalf("expected empty index message, got %q", out)
	}
}
