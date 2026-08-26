package indexing_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/indexing"
)

func TestExtractBookmarkSections_swade(t *testing.T) {
	path := filepath.Join("..", "..", "fixtures", "Savage_Worlds_Adventure_Edition.pdf")
	if _, err := os.Stat(path); err != nil {
		t.Skip("fixture PDF not present")
	}

	sections, pageCount, err := indexing.ExtractBookmarkSections(path, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pageCount=%d sections=%d", pageCount, len(sections))
	for i, s := range sections {
		if i >= 8 {
			break
		}
		t.Logf("%d: %q %d-%d", i, s.Title, s.StartPage, s.EndPage)
	}
	if len(sections) < 5 {
		t.Fatalf("expected at least 5 bookmark sections, got %d", len(sections))
	}
}
