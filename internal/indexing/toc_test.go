package indexing

import (
	"testing"
)

func TestParseTOCLines_dotLeaders(t *testing.T) {
	text := `Table of Contents

Introduction ........................ 5
Character Creation .................. 12
Combat ............................ 45
`
	entries := ParseTOCLines(text)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	if entries[0].Title != "Introduction" || entries[0].StartPage != 5 {
		t.Fatalf("first entry: %+v", entries[0])
	}
	if entries[2].Title != "Combat" || entries[2].StartPage != 45 {
		t.Fatalf("third entry: %+v", entries[2])
	}
}

func TestParseTOCLines_tabsAndSpaces(t *testing.T) {
	text := "Skills\t\t18\nEdges and Hindrances  22\n"
	entries := ParseTOCLines(text)
	if len(entries) != 2 {
		t.Fatalf("got %d entries", len(entries))
	}
	if entries[1].StartPage != 22 {
		t.Fatalf("second page: %d", entries[1].StartPage)
	}
}

func TestBuildTOCSections_endPages(t *testing.T) {
	entries := []tocEntry{
		{Title: "Intro", StartPage: 5},
		{Title: "Combat", StartPage: 12},
		{Title: "Index", StartPage: 90},
	}
	sections := BuildTOCSections(entries, 100)
	if len(sections) != 3 {
		t.Fatalf("got %d sections", len(sections))
	}
	if sections[0].EndPage != 11 {
		t.Fatalf("intro end: %d", sections[0].EndPage)
	}
	if sections[1].EndPage != 89 {
		t.Fatalf("combat end: %d", sections[1].EndPage)
	}
	if sections[2].EndPage != 100 {
		t.Fatalf("index end: %d", sections[2].EndPage)
	}
}

func TestParseTOCLines_pdfBackspace(t *testing.T) {
	text := "Characters\b9\nRaces \b\xff\xff12\n5 The Adventure Toolkit\b\n"
	entries := ParseTOCLines(text)
	if len(entries) < 2 {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	found := map[int]string{}
	for _, e := range entries {
		found[e.StartPage] = e.Title
	}
	if found[9] == "" || found[12] == "" {
		t.Fatalf("entries: %+v", entries)
	}
}

func TestParseTOCLines_skipsBareNumbers(t *testing.T) {
	text := "123\nReal Section ..... 10\n"
	entries := ParseTOCLines(text)
	if len(entries) != 1 || entries[0].StartPage != 10 {
		t.Fatalf("entries: %+v", entries)
	}
}
