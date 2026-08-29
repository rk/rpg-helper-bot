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

// englishStopwords excludes high-frequency English words from word-frequency stats.
// Catalog synonym tokens are never treated as stopwords (see protectedCatalogTokens).
var englishStopwords = map[string]struct{}{
	"A": {}, "AN": {}, "AND": {}, "ARE": {}, "AS": {}, "AT": {}, "BE": {}, "BEEN": {}, "BEING": {},
	"BUT": {}, "BY": {}, "CAN": {}, "COULD": {}, "DID": {}, "DO": {}, "DOES": {}, "DOING": {}, "DONE": {},
	"FOR": {}, "FROM": {}, "HAD": {}, "HAS": {}, "HAVE": {}, "HAVING": {}, "HE": {}, "HER": {}, "HERE": {},
	"HERS": {}, "HIM": {}, "HIS": {}, "HOW": {}, "I": {}, "IF": {}, "IN": {}, "INTO": {}, "IS": {}, "IT": {},
	"ITS": {}, "JUST": {}, "MAY": {}, "MIGHT": {}, "MORE": {}, "MOST": {}, "MUST": {}, "MY": {}, "NO": {},
	"NOR": {}, "NOT": {}, "OF": {}, "OFF": {}, "ON": {}, "ONCE": {}, "ONE": {}, "ONLY": {}, "OR": {}, "OTHER": {},
	"OUR": {}, "OUT": {}, "OVER": {}, "OWN": {}, "SAME": {}, "SHALL": {}, "SHE": {}, "SHOULD": {}, "SO": {},
	"SOME": {}, "SUCH": {}, "THAN": {}, "THAT": {}, "THE": {}, "THEIR": {}, "THEM": {}, "THEN": {}, "THERE": {},
	"THESE": {}, "THEY": {}, "THIS": {}, "THOSE": {}, "THROUGH": {}, "TO": {}, "TOO": {}, "UNDER": {}, "UP": {},
	"US": {}, "VERY": {}, "WAS": {}, "WE": {}, "WERE": {}, "WHAT": {}, "WHEN": {}, "WHERE": {}, "WHICH": {},
	"WHO": {}, "WHOM": {}, "WHY": {}, "WILL": {}, "WITH": {}, "WOULD": {}, "YOU": {}, "YOUR": {}, "YOURS": {},
	"ALSO": {}, "ABOUT": {}, "AFTER": {}, "AGAIN": {}, "ALL": {}, "ALMOST": {}, "ALONG": {}, "ALREADY": {},
	"ALTHOUGH": {}, "ALWAYS": {}, "AM": {}, "AMONG": {}, "ANY": {}, "ANYONE": {}, "ANYTHING": {}, "ANYWHERE": {},
	"AREAS": {}, "AROUND": {}, "AWAY": {}, 	"BECAUSE": {}, "BEFORE": {}, "BEHIND": {}, "BELOW": {},
	"BETWEEN": {}, "BEYOND": {}, "BOTH": {}, "DOWN": {}, "DURING": {}, "EACH": {}, "EITHER": {}, "ELSE": {},
	"EVEN": {}, "EVERY": {}, "EVERYONE": {}, "EVERYTHING": {}, "FEW": {}, "FURTHER": {}, "GET": {}, "GETS": {},
	"GOT": {}, "HOWEVER": {}, "INSIDE": {}, "INSTEAD": {}, "ITSELF": {}, "LESS": {}, "LIKE": {},
	"MANY": {}, "MAYBE": {}, "MUCH": {}, "NEED": {}, "NEEDS": {}, "NEITHER": {}, "NEVER": {}, "NEXT": {}, "NOW": {},
	"OFTEN": {}, "ONTO": {}, "PERHAPS": {}, "RATHER": {}, "REALLY": {}, "SINCE": {}, "STILL": {},
	"THOUGH": {}, "TILL": {}, "TOWARD": {}, "TOWARDS": {}, "UNLESS": {}, "UNTIL": {}, "UPON": {},
	"USED": {}, "USING": {}, "USUALLY": {}, "VIA": {}, "WELL": {}, "WHILE": {}, "WITHIN": {}, "WITHOUT": {},
	"YET": {},
}

// BuildWordFrequencyTSV tokenizes the full document corpus and returns the top N
// word counts as TSV (word<TAB>count). Tokens are uppercased alnum-only strings.
// Common English stopwords are excluded unless they appear in the concept catalog synonyms.
// Catalog synonym tokens with document matches are always included, even below top N.
func BuildWordFrequencyTSV(samples []SectionSample, topN int, catalog *rpgconcepts.ConceptCatalog) string {
	if topN <= 0 {
		topN = wordStatsTopN
	}
	counts := countDocumentTokens(samples)
	protected := protectedCatalogTokens(catalog)
	ranked := rankWordCounts(counts, topN, protected)

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

func protectedCatalogTokens(catalog *rpgconcepts.ConceptCatalog) map[string]struct{} {
	protected := map[string]struct{}{}
	if catalog == nil {
		return protected
	}
	for _, f := range catalog.Features {
		for _, syn := range f.Synonyms {
			for _, tok := range tokenizeForWordStats(syn) {
				protected[tok] = struct{}{}
			}
		}
	}
	return protected
}

func isEnglishStopword(word string, protected map[string]struct{}) bool {
	if _, ok := protected[word]; ok {
		return false
	}
	_, ok := englishStopwords[word]
	return ok
}

func rankWordCounts(counts map[string]int, topN int, protected map[string]struct{}) []wordCount {
	ranked := make([]wordCount, 0, len(counts))
	for word, count := range counts {
		if isEnglishStopword(word, protected) {
			continue
		}
		ranked = append(ranked, wordCount{word: word, count: count})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].word < ranked[j].word
	})

	included := map[string]struct{}{}
	if len(ranked) > topN {
		for _, wc := range ranked[:topN] {
			included[wc.word] = struct{}{}
		}
		ranked = ranked[:topN]
	} else {
		for _, wc := range ranked {
			included[wc.word] = struct{}{}
		}
	}

	var forced []wordCount
	for word := range protected {
		count, ok := counts[word]
		if !ok || count <= 0 {
			continue
		}
		if _, ok := included[word]; ok {
			continue
		}
		forced = append(forced, wordCount{word: word, count: count})
	}
	sort.Slice(forced, func(i, j int) bool {
		if forced[i].count != forced[j].count {
			return forced[i].count > forced[j].count
		}
		return forced[i].word < forced[j].word
	})
	return append(ranked, forced...)
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
