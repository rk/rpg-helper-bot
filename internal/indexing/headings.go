package indexing

import (
	"sort"
	"strings"
	"unicode"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

type headingAnchor struct {
	title     string
	lineIndex int
}

// SplitPageTextByHeadings splits single-page text into sections keyed by title.
// Titles are matched as whole lines (case-insensitive, whitespace-normalized).
// Only headings present in titles are used as split points; document order wins.
func SplitPageTextByHeadings(fullText string, titles []string) map[string]string {
	out := make(map[string]string, len(titles))
	if fullText == "" || len(titles) == 0 {
		return out
	}

	lines := strings.Split(fullText, "\n")
	normTitle := make(map[string]string, len(titles))
	for _, title := range titles {
		normTitle[normalizeHeading(title)] = title
		out[title] = ""
	}

	var anchors []headingAnchor
	for i, line := range lines {
		if title, ok := normTitle[normalizeHeading(line)]; ok {
			anchors = append(anchors, headingAnchor{title: title, lineIndex: i})
		}
	}
	if len(anchors) == 0 {
		return out
	}

	sort.SliceStable(anchors, func(i, j int) bool {
		return anchors[i].lineIndex < anchors[j].lineIndex
	})

	// Drop duplicate anchors for the same title (keep first occurrence).
	seen := map[string]bool{}
	deduped := anchors[:0]
	for _, a := range anchors {
		if seen[a.title] {
			continue
		}
		seen[a.title] = true
		deduped = append(deduped, a)
	}
	anchors = deduped

	for i, anchor := range anchors {
		end := len(lines)
		if i+1 < len(anchors) {
			end = anchors[i+1].lineIndex
		}
		out[anchor.title] = strings.TrimSpace(strings.Join(lines[anchor.lineIndex:end], "\n"))
	}
	return out
}

func normalizeHeading(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace && b.Len() > 0 {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(unicode.ToLower(r))
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

// SamePageSectionGroups returns section IDs grouped by single-page range when
// multiple sections share the same page.
func SamePageSectionGroups(sections []models.TOCSection) map[pageRange][]models.TOCSection {
	groups := map[pageRange][]models.TOCSection{}
	for _, sec := range sections {
		if sec.StartPage != sec.EndPage {
			continue
		}
		key := pageRange{sec.StartPage, sec.EndPage}
		groups[key] = append(groups[key], sec)
	}
	for key, group := range groups {
		if len(group) <= 1 {
			delete(groups, key)
		}
	}
	return groups
}

type pageRange struct {
	start int
	end   int
}
