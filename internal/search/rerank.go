package search

import (
	"strings"
	"unicode"
)

const (
	titleMatchBoostPerTerm = 0.06
	maxTitleMatchBoost     = 0.24
	maxTablePenalty        = 0.18
)

// adjustedScore applies lightweight rerank nudges on top of embedding similarity.
func adjustedScore(embedScore float64, sectionTitle, plainText, originalQuery string) float64 {
	return embedScore + titleMatchBoost(sectionTitle, originalQuery) - tableDensityPenalty(plainText)
}

func titleMatchBoost(title, originalQuery string) float64 {
	titleLower := strings.ToLower(title)
	var boost float64
	for _, term := range strings.Fields(strings.ToLower(originalQuery)) {
		term = strings.Trim(term, ".,?!;:\"'()[]")
		if len(term) < 3 || isStopword(term) {
			continue
		}
		if strings.Contains(titleLower, term) {
			boost += titleMatchBoostPerTerm
		}
	}
	if boost > maxTitleMatchBoost {
		boost = maxTitleMatchBoost
	}
	return boost
}

func tableDensityPenalty(text string) float64 {
	if text == "" {
		return 0
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return 0
	}
	tabular := 0
	nonEmpty := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		if looksTabularLine(line) {
			tabular++
		}
	}
	if nonEmpty == 0 {
		return 0
	}
	penalty := (float64(tabular) / float64(nonEmpty)) * maxTablePenalty
	if penalty > maxTablePenalty {
		penalty = maxTablePenalty
	}
	return penalty
}

func looksTabularLine(line string) bool {
	digits := 0
	letters := 0
	for _, r := range line {
		if unicode.IsDigit(r) {
			digits++
		} else if unicode.IsLetter(r) {
			letters++
		}
	}
	total := digits + letters
	if total == 0 {
		return false
	}
	fields := strings.Fields(line)
	numFields := 0
	for _, f := range fields {
		if mostlyDigits(f) {
			numFields++
		}
	}
	if numFields >= 3 && len(fields) <= 10 {
		return true
	}
	return float64(digits)/float64(total) > 0.4 && len(line) < 100
}

func mostlyDigits(s string) bool {
	if s == "" {
		return false
	}
	digits := 0
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
		digits++
	}
	return digits > 0
}

func isStopword(term string) bool {
	switch term {
	case "the", "and", "for", "with", "from", "that", "this", "what", "need", "has", "have", "are", "was", "were", "how", "does", "can", "you", "your":
		return true
	default:
		return false
	}
}
