package rpg

import (
	"regexp"
	"strings"
)

// diceTokenRe matches common tabletop dice notation: d8, 2D6, 2D6-2, 2D6+1, 2D (pool).
var diceTokenRe = regexp.MustCompile(`(?i)\b(\d*[dD]\d+(?:[+-]\d+)?|\d+[dD](?:[+-]\d+)?)\b`)

// ExtractDiceTokens returns dice notation tokens from text, preserving original casing.
func ExtractDiceTokens(text string) []string {
	matches := diceTokenRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		key := strings.ToLower(m)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, m)
	}
	return out
}

// PreserveDiceInFTSQuery re-injects dice tokens from the original query into a merged FTS query.
func PreserveDiceInFTSQuery(original, merged string) string {
	tokens := ExtractDiceTokens(original)
	if len(tokens) == 0 {
		return merged
	}
	seen := map[string]struct{}{}
	for _, w := range strings.Fields(merged) {
		seen[strings.ToLower(w)] = struct{}{}
	}
	parts := strings.Fields(merged)
	for _, tok := range tokens {
		if _, ok := seen[strings.ToLower(tok)]; ok {
			continue
		}
		parts = append(parts, tok)
		seen[strings.ToLower(tok)] = struct{}{}
	}
	return strings.Join(parts, " ")
}
