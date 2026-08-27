package llm

import (
	"context"
	"strings"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpg"
)

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

	systemPrompt := renderPrompt(PromptSearchRewrite, nil)
	raw, err := c.Complete(ctx, systemPrompt, query)
	if err != nil {
		return "", err
	}
	rewritten := parseSearchKeywords(raw)
	if rewritten == "" {
		return query, nil
	}
	merged := mergeSearchQueries(query, rewritten)
	return rpg.PreserveDiceInFTSQuery(query, merged), nil
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
