package indexing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitPageTextByHeadings_swadePage95(t *testing.T) {
	path := filepath.Join("..", "..", "fixtures", "Savage_Worlds_Adventure_Edition.pdf")
	if _, err := os.Stat(path); err != nil {
		t.Skip("fixture PDF not present")
	}

	fullText, err := ExtractPages(path, 95, 95)
	if err != nil {
		t.Fatal(err)
	}

	titles := []string{
		"Other Movement Issues",
		"Attacks",
		"Melee Attacks",
		"Ranged Attacks",
	}
	splits := SplitPageTextByHeadings(fullText, titles)

	movement := splits["Other Movement Issues"]
	if !strings.Contains(movement, "JUMPING") {
		t.Fatalf("movement section missing JUMPING: %q", movement[:min(80, len(movement))])
	}
	if strings.Contains(movement, "Rate of Fire") {
		t.Fatal("movement section should not include Rate of Fire")
	}

	ranged := splits["Ranged Attacks"]
	if !strings.Contains(ranged, "Rate of Fire") {
		t.Fatalf("ranged section missing Rate of Fire: %q", ranged[:min(120, len(ranged))])
	}
	if !strings.Contains(ranged, "Shooting skill") {
		t.Fatal("ranged section missing Shooting skill intro")
	}

	melee := splits["Melee Attacks"]
	if !strings.Contains(melee, "Parry score") {
		t.Fatalf("melee section missing Parry: %q", melee)
	}
	if strings.Contains(melee, "Rate of Fire") {
		t.Fatal("melee section should not include Rate of Fire")
	}
}

func TestDedupeTOCEntries_keepsSamePageSiblings(t *testing.T) {
	entries := []tocEntry{
		{Title: "Other Movement Issues", StartPage: 95},
		{Title: "Attacks", StartPage: 95},
		{Title: "Melee Attacks", StartPage: 95},
		{Title: "Ranged Attacks", StartPage: 95},
		{Title: "Applying Damage", StartPage: 96},
	}
	out := dedupeTOCEntries(entries)
	if len(out) != 5 {
		t.Fatalf("got %d entries, want 5: %+v", len(out), out)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
