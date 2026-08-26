package textutil

import "testing"

func TestNormalizePDFText_smartPunctuation(t *testing.T) {
	in := "make a \u201cSoak\u201d roll\u2026"
	want := `make a "Soak" roll...`
	if got := NormalizePDFText(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
