package rpg

import (
	"strings"
	"testing"
)

func TestExtractDiceTokens(t *testing.T) {
	got := ExtractDiceTokens("He has D8 Agility and rolls 2D6-2 damage with 2D pool")
	want := []string{"D8", "2D6-2", "2D"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if !strings.EqualFold(got[i], want[i]) {
			t.Fatalf("token[%d]=%q want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestPreserveDiceInFTSQuery(t *testing.T) {
	original := "D8 Agility 2D6-2 crossbow RoF 2"
	merged := "agility crossbow rate fire shooting"
	out := PreserveDiceInFTSQuery(original, merged)
	for _, tok := range []string{"D8", "2D6-2"} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(tok)) {
			t.Fatalf("expected %q in %q", tok, out)
		}
	}
}
