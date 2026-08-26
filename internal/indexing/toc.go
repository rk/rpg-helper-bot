package indexing

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

var (
	tocDotLeaderRe    = regexp.MustCompile(`^(.+?)\s*[.·…]{2,}\s*(\d{1,4})\s*$`)
	tocTabLeaderRe    = regexp.MustCompile(`^(.+?)\t+\s*(\d{1,4})\s*$`)
	tocSpaceLeaderRe  = regexp.MustCompile(`^(.+?)\s{2,}(\d{1,4})\s*$`)
	tocLeadingPageRe  = regexp.MustCompile(`^(\d{1,4})\s+(.+)$`)
	tocTrailingPageRe = regexp.MustCompile(`^(.+?)(\d{1,4})$`)
)

type tocEntry struct {
	Title     string
	StartPage int
}

var tocSkipTitles = map[string]struct{}{
	"contents":           {},
	"table of contents":  {},
	"tableofcontents":    {},
	"index":              {},
}

// ExtractTOCSections reads ToC pages from a PDF and returns section page ranges.
func ExtractTOCSections(pdfPath string, tocStart, tocEnd int) ([]models.TOCSection, int, error) {
	if tocStart < 1 || tocEnd < tocStart {
		return nil, 0, fmt.Errorf("invalid ToC page range %d-%d", tocStart, tocEnd)
	}
	pageCount, err := PageCount(pdfPath)
	if err != nil {
		return nil, 0, err
	}
	text, err := ExtractPages(pdfPath, tocStart, tocEnd)
	if err != nil {
		return nil, 0, err
	}
	entries := ParseTOCLines(text)
	if len(entries) == 0 {
		return nil, pageCount, fmt.Errorf("no ToC entries found on pages %d-%d", tocStart, tocEnd)
	}
	return BuildTOCSections(entries, pageCount), pageCount, nil
}

func ParseTOCLines(text string) []tocEntry {
	var out []tocEntry
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		title, page, ok := parseTOCLine(line)
		if !ok {
			continue
		}
		title = normalizeTOCTitle(title)
		if len(title) < 2 {
			continue
		}
		if _, skip := tocSkipTitles[strings.ToLower(title)]; skip {
			continue
		}
		out = append(out, tocEntry{Title: title, StartPage: page})
	}
	return dedupeTOCEntries(out)
}

func parseTOCLine(line string) (title string, page int, ok bool) {
	normalized := normalizeTOCLine(line)
	if normalized == "" {
		return "", 0, false
	}

	if m := tocLeadingPageRe.FindStringSubmatch(normalized); len(m) == 3 {
		if title, page, ok = parseTitlePage(m[2], m[1]); ok {
			return title, page, true
		}
	}

	for _, re := range []*regexp.Regexp{tocDotLeaderRe, tocTabLeaderRe, tocSpaceLeaderRe} {
		if m := re.FindStringSubmatch(normalized); len(m) == 3 {
			return parseTitlePage(m[1], m[2])
		}
	}

	if m := tocTrailingPageRe.FindStringSubmatch(normalized); len(m) == 3 {
		return parseTitlePage(strings.TrimSpace(m[1]), m[2])
	}

	fields := strings.Fields(normalized)
	if len(fields) < 2 {
		return "", 0, false
	}
	last := fields[len(fields)-1]
	if !isDigits(last) {
		return "", 0, false
	}
	return parseTitlePage(strings.Join(fields[:len(fields)-1], " "), last)
}

func normalizeTOCLine(raw string) string {
	raw = cleanPDFBackspaces(raw)
	var b strings.Builder
	lastSpace := false
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace && b.Len() > 0 {
				b.WriteRune(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func cleanPDFBackspaces(s string) string {
	var out []rune
	for _, r := range s {
		if r == '\b' {
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			continue
		}
		if r == '\f' {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func parseTitlePage(titleRaw, pageRaw string) (string, int, bool) {
	page, err := parsePageNumber(pageRaw)
	if err != nil || page < 1 {
		return "", 0, false
	}
	title := normalizeTOCTitle(titleRaw)
	if title == "" || isMostlyDigits(title) {
		return "", 0, false
	}
	return title, page, true
}

func parsePageNumber(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !isDigits(raw) {
		return 0, fmt.Errorf("invalid page")
	}
	n := 0
	for _, r := range raw {
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func normalizeTOCTitle(s string) string {
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimRight(s, ".·…-–—")
	return strings.TrimSpace(s)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isMostlyDigits(s string) bool {
	digits := 0
	letters := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits++
		} else if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters == 0 && digits > 0
}

func dedupeTOCEntries(entries []tocEntry) []tocEntry {
	if len(entries) == 0 {
		return nil
	}
	byPage := map[int]tocEntry{}
	order := make([]int, 0, len(entries))
	for _, e := range entries {
		if prev, ok := byPage[e.StartPage]; ok {
			if len(e.Title) <= len(prev.Title) {
				continue
			}
		} else {
			order = append(order, e.StartPage)
		}
		byPage[e.StartPage] = e
	}
	sort.Ints(order)
	out := make([]tocEntry, 0, len(order))
	for _, page := range order {
		out = append(out, byPage[page])
	}
	return out
}

// BuildTOCSections assigns end pages: next start - 1, last section through pageCount.
func BuildTOCSections(entries []tocEntry, pageCount int) []models.TOCSection {
	if len(entries) == 0 {
		return nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].StartPage < entries[j].StartPage
	})

	sections := make([]models.TOCSection, len(entries))
	for i, e := range entries {
		end := e.StartPage
		if i+1 < len(entries) {
			end = entries[i+1].StartPage - 1
		} else if pageCount > 0 {
			end = pageCount
		}
		if end < e.StartPage {
			end = e.StartPage
		}
		if pageCount > 0 && end > pageCount {
			end = pageCount
		}
		sections[i] = models.TOCSection{
			Title:     e.Title,
			StartPage: e.StartPage,
			EndPage:   end,
			SortOrder: i,
		}
	}
	return sections
}
