package llm

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

const llmJSONDumpMax = 16384

var llmJSONKeyTypo = regexp.MustCompile(`"([a-z_]+):_:`)

func parseLLMJSON(raw string, ctx llmParseContext, dest any) error {
	normalized := normalizeJSONResponse(raw)
	candidates := []string{normalized, repairLLMJSON(normalized)}
	if ctx.Pass == "features" {
		for _, c := range append([]string(nil), candidates...) {
			if strings.HasPrefix(strings.TrimSpace(c), "[") {
				candidates = append(candidates, `{"features":`+c+`}`)
			}
		}
	}

	var lastErr error
	for _, candidate := range uniqueStrings(candidates) {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(candidate), dest); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	logLLMJSONParseFailure(ctx, raw, normalized, lastErr)
	return fmt.Errorf("parse %s json (chunk %d/%d%s): %v (see log for full raw and extracted JSON)",
		ctx.Pass, ctx.ChunkNum, ctx.ChunkTotal, featureLabel(ctx.FeatureID), lastErr)
}

func logLLMJSONParseFailure(ctx llmParseContext, raw, extracted string, parseErr error) {
	log.Printf("llm learnings: parse %s json failed (chunk %d/%d%s): %v; sections=%q",
		ctx.Pass, ctx.ChunkNum, ctx.ChunkTotal, featureLabel(ctx.FeatureID), parseErr,
		strings.Join(ctx.SectionTitles, ", "))
	log.Printf("llm learnings: raw LLM response (%d bytes):\n%s", len(raw), dumpLLMText(raw))
	log.Printf("llm learnings: extracted JSON (%d bytes):\n%s", len(extracted), dumpLLMText(extracted))
}

func dumpLLMText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	if len(s) > llmJSONDumpMax {
		return s[:llmJSONDumpMax] + fmt.Sprintf("\n… truncated %d bytes", len(s)-llmJSONDumpMax)
	}
	return s
}

func repairLLMJSON(s string) string {
	s = llmJSONKeyTypo.ReplaceAllString(s, `"$1":`)
	s = stripJSONComments(s)
	s = repairJSONStringLiterals(s)
	s = regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(s, `$1`)
	return s
}

// stripJSONComments removes // and /* */ comments outside JSON string values.
func stripJSONComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inString := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				b.WriteByte(c)
				escape = false
				continue
			}
			switch c {
			case '\\':
				b.WriteByte(c)
				escape = true
			case '"':
				inString = false
				b.WriteByte(c)
			default:
				b.WriteByte(c)
			}
			continue
		}
		if c == '"' {
			inString = true
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) {
			switch s[i+1] {
			case '/':
				i += 2
				for i < len(s) && s[i] != '\n' && s[i] != '\r' {
					i++
				}
				if i < len(s) {
					b.WriteByte(s[i])
				}
				continue
			case '*':
				i += 2
				for i+1 < len(s) {
					if s[i] == '*' && s[i+1] == '/' {
						i++
						break
					}
					i++
				}
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// repairJSONStringLiterals escapes raw control characters inside JSON string values.
func repairJSONStringLiterals(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 32)
	inString := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				b.WriteByte(c)
				escape = false
				continue
			}
			switch c {
			case '\\':
				b.WriteByte(c)
				escape = true
			case '"':
				inString = false
				b.WriteByte(c)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				b.WriteByte(c)
			}
			continue
		}
		if c == '"' {
			inString = true
		}
		b.WriteByte(c)
	}
	return b.String()
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// normalizeJSONResponse trims the model reply and extracts the first balanced JSON value.
// With JSON mode enabled, responses should already be raw JSON; fence stripping remains a fallback.
func normalizeJSONResponse(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimSpace(stripMarkdownCodeFence(raw))
	}
	if obj := sliceJSONValue(raw, '{', '}'); obj != "" {
		return obj
	}
	if arr := sliceJSONValue(raw, '[', ']'); arr != "" {
		return arr
	}
	return raw
}

func sliceJSONValue(s string, open, close byte) string {
	start := strings.IndexByte(s, open)
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start : i+1])
			}
		}
	}
	return ""
}

func stripMarkdownCodeFence(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "```") {
		return raw
	}
	raw = raw[3:]
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "json"))
	if idx := strings.LastIndex(raw, "```"); idx >= 0 {
		raw = raw[:idx]
	}
	return strings.TrimSpace(raw)
}

type flexCheatsheetCitation struct {
	SectionID    json.RawMessage `json:"section_id,omitempty"`
	SectionTitle string          `json:"section_title"`
	StartPage    int             `json:"start_page"`
	EndPage      int             `json:"end_page,omitempty"`
}

type flexCheatsheetResponse struct {
	FeatureID  string                   `json:"feature_id"`
	Definition string                   `json:"definition"`
	Citations  []flexCheatsheetCitation `json:"citations"`
}

func parseCheatsheetJSON(raw string, ctx llmParseContext) (*cheatsheetLLMResponse, error) {
	normalized := normalizeJSONResponse(raw)
	candidates := []string{normalized, repairLLMJSON(normalized)}

	var lastErr error
	for _, candidate := range uniqueStrings(candidates) {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		var flex flexCheatsheetResponse
		if err := json.Unmarshal([]byte(candidate), &flex); err != nil {
			lastErr = err
			continue
		}
		return &cheatsheetLLMResponse{
			FeatureID:  flex.FeatureID,
			Definition: flex.Definition,
			Citations:  normalizeCheatsheetCitations(flex.Citations),
		}, nil
	}

	logLLMJSONParseFailure(ctx, raw, normalized, lastErr)
	return nil, fmt.Errorf("parse %s json (chunk %d/%d%s): %v (see log for full raw and extracted JSON)",
		ctx.Pass, ctx.ChunkNum, ctx.ChunkTotal, featureLabel(ctx.FeatureID), lastErr)
}

func normalizeCheatsheetCitations(in []flexCheatsheetCitation) []models.CheatsheetCitation {
	var out []models.CheatsheetCitation
	for _, c := range in {
		ids := parseFlexibleSectionIDs(c.SectionID)
		if len(ids) == 0 {
			if strings.TrimSpace(c.SectionTitle) == "" && c.StartPage <= 0 {
				continue
			}
			out = append(out, models.CheatsheetCitation{
				SectionTitle: strings.TrimSpace(c.SectionTitle),
				StartPage:    c.StartPage,
				EndPage:      c.EndPage,
			})
			continue
		}
		for _, id := range ids {
			out = append(out, models.CheatsheetCitation{
				SectionID:    id,
				SectionTitle: strings.TrimSpace(c.SectionTitle),
				StartPage:    c.StartPage,
				EndPage:      c.EndPage,
			})
		}
	}
	return out
}

func parseFlexibleSectionIDs(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return []string{s}
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		var out []string
		for _, id := range arr {
			if id = strings.TrimSpace(id); id != "" {
				out = append(out, id)
			}
		}
		return out
	}
	return nil
}
