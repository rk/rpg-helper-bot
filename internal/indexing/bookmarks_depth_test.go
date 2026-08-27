package indexing

import (
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

func TestFlattenBookmarks_maxDepth(t *testing.T) {
	tree := []pdfcpu.Bookmark{
		{
			Title:    "Chapter 1",
			PageFrom: 1,
			Kids: []pdfcpu.Bookmark{
				{
					Title:    "Section A",
					PageFrom: 2,
					Kids: []pdfcpu.Bookmark{{
						Title:    "Subsection A1",
						PageFrom: 3,
					}},
				},
			},
		},
		{Title: "Chapter 2", PageFrom: 10},
	}

	cases := []struct {
		maxDepth int
		want     []string
	}{
		{1, []string{"Chapter 1", "Chapter 2"}},
		{2, []string{"Chapter 1", "Section A", "Chapter 2"}},
		{0, []string{"Chapter 1", "Section A", "Subsection A1", "Chapter 2"}},
	}

	for _, tc := range cases {
		var entries []tocEntry
		flattenBookmarks(tree, tc.maxDepth, 1, &entries)
		if len(entries) != len(tc.want) {
			t.Fatalf("depth %d: got %d entries, want %d: %+v", tc.maxDepth, len(entries), len(tc.want), entries)
		}
		for i, want := range tc.want {
			if entries[i].Title != want {
				t.Fatalf("depth %d: entry %d = %q, want %q", tc.maxDepth, i, entries[i].Title, want)
			}
		}
	}
}

func TestResolveBookmarkMaxDepth(t *testing.T) {
	depth2 := 2
	includeFalse := false
	includeTrue := true
	if got := ResolveBookmarkMaxDepth(&depth2, nil); got != 2 {
		t.Fatalf("got %d", got)
	}
	if got := ResolveBookmarkMaxDepth(nil, &includeFalse); got != 1 {
		t.Fatalf("got %d", got)
	}
	if got := ResolveBookmarkMaxDepth(nil, &includeTrue); got != BookmarkDepthUnlimited {
		t.Fatalf("got %d", got)
	}
	unlimited := 0
	if got := ResolveBookmarkMaxDepth(&unlimited, &includeFalse); got != BookmarkDepthUnlimited {
		t.Fatalf("explicit 0 should win over includeChildren=false, got %d", got)
	}
}
