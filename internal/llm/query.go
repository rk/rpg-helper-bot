package llm

import (
	"context"
	"strings"
	"time"
)

const searchRewriteSystemPrompt = `You rewrite tabletop RPG rules questions into concise keyword lists for full-text search.
Include the original important terms plus closely related words, synonyms, and word stems.
Example: "necromancer" should also include necromancy; "shooting" may include ranged gunfire.
Reply with ONLY space-separated keywords. No punctuation, labels, or explanation. At most 12 words.`

func (c *Client) RewriteSearchQuery(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil
	}
	if c == nil {
		return query, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	raw, err := c.Complete(ctx, searchRewriteSystemPrompt, query)
	if err != nil {
		return "", err
	}
	rewritten := parseSearchKeywords(raw)
	if rewritten == "" {
		return query, nil
	}
	return mergeSearchQueries(query, rewritten), nil
}

func parseSearchKeywords(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if idx := strings.IndexAny(raw, "\n\r"); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.Trim(raw, `"'`)
	raw = strings.ReplaceAll(raw, ",", " ")
	return strings.Join(strings.Fields(raw), " ")
}

func mergeSearchQueries(original, rewritten string) string {
	seen := map[string]struct{}{}
	parts := make([]string, 0, 16)
	addWords := func(s string) {
		for _, w := range strings.Fields(s) {
			key := strings.ToLower(strings.Trim(w, `"'`))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			parts = append(parts, strings.Trim(w, `"'`))
		}
	}
	addWords(original)
	addWords(rewritten)
	return strings.Join(parts, " ")
}
