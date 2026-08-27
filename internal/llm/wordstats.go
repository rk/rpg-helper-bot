package llm

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

const wordStatsTopN = 1000

type wordCount struct {
	word  string
	count int
}

// BuildWordFrequencyTSV tokenizes the full document corpus and returns the top N
// word counts as TSV (word<TAB>count). Tokens are uppercased alnum-only strings.
func BuildWordFrequencyTSV(samples []SectionSample, topN int) string {
	if topN <= 0 {
		topN = wordStatsTopN
	}
	counts := countDocumentTokens(samples)
	ranked := rankWordCounts(counts, topN)

	var b strings.Builder
	b.WriteString("word\tcount\n")
	for _, wc := range ranked {
		fmt.Fprintf(&b, "%s\t%d\n", wc.word, wc.count)
	}
	return b.String()
}

func countDocumentTokens(samples []SectionSample) map[string]int {
	counts := map[string]int{}
	for _, sec := range samples {
		for _, token := range tokenizeForWordStats(sec.Title) {
			counts[token]++
		}
		text := sec.PlainText
		if text == "" {
			text = sec.Excerpt
		}
		for _, token := range tokenizeForWordStats(text) {
			counts[token]++
		}
	}
	return counts
}

func tokenizeForWordStats(text string) []string {
	text = strings.ToUpper(text)
	var normalized strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			normalized.WriteRune(r)
		} else {
			normalized.WriteByte(' ')
		}
	}
	tokens := strings.Fields(normalized.String())
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if len(tok) < 2 {
			continue
		}
		out = append(out, tok)
	}
	return out
}

func rankWordCounts(counts map[string]int, topN int) []wordCount {
	ranked := make([]wordCount, 0, len(counts))
	for word, count := range counts {
		ranked = append(ranked, wordCount{word: word, count: count})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].word < ranked[j].word
	})
	if len(ranked) > topN {
		ranked = ranked[:topN]
	}
	return ranked
}

// DocumentTokenCount returns unique token count for logging/diagnostics.
func DocumentTokenCount(samples []SectionSample) int {
	return len(countDocumentTokens(samples))
}

// FeatureCandidatesFromWordStats returns feature IDs with catalog/glossary tokens present in the document.
func FeatureCandidatesFromWordStats(catalog *rpgconcepts.ConceptCatalog, glossary []models.PDFGlossaryEntry, samples []SectionSample) []string {
	if catalog == nil {
		return nil
	}
	counts := countDocumentTokens(samples)
	found := map[string]struct{}{}

	for _, f := range catalog.Features {
		for _, syn := range f.Synonyms {
			for _, tok := range tokenizeForWordStats(syn) {
				if counts[tok] > 0 {
					found[f.ID] = struct{}{}
					break
				}
			}
		}
	}
	for _, g := range glossary {
		for _, term := range g.Terms {
			for _, tok := range tokenizeForWordStats(term) {
				if counts[tok] > 0 {
					found[g.FeatureID] = struct{}{}
					break
				}
			}
		}
	}

	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
