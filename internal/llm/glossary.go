package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

type glossaryLLMResponse struct {
	Glossary []models.PDFGlossary `json:"glossary"`
}

// RefinePDFGlossary uses the chat model to refine book-specific glossary mappings.
func (c *Client) RefinePDFGlossary(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, draft *models.PDFIndexMeta, samples map[string]string) (*models.PDFIndexMeta, error) {
	if c == nil || catalog == nil || draft == nil {
		return draft, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	userMsg := buildGlossaryUserMessage(catalog, draft, samples)
	systemPrompt := renderPrompt(PromptGlossaryExtract, nil)
	raw, err := c.Complete(ctx, systemPrompt, userMsg)
	if err != nil {
		return nil, err
	}
	var resp glossaryLLMResponse
	if err := json.Unmarshal([]byte(extractJSON(raw)), &resp); err != nil {
		return nil, fmt.Errorf("parse glossary json: %w", err)
	}

	out := *draft
	out.LLMGlossarySkipped = false
	out.Glossary = mergeGlossary(draft.Glossary, resp.Glossary, catalog)
	return &out, nil
}

func buildGlossaryUserMessage(catalog *rpgconcepts.ConceptCatalog, draft *models.PDFIndexMeta, samples map[string]string) string {
	var b strings.Builder
	b.WriteString("Canonical feature catalog:\n")
	for _, f := range catalog.Features {
		fmt.Fprintf(&b, "- %s (%s): synonyms=%v\n", f.ID, f.Name, f.Synonyms)
	}
	b.WriteString("\nDraft offline glossary:\n")
	for _, g := range draft.Glossary {
		fmt.Fprintf(&b, "- %s => %q (%s)\n", g.FeatureID, g.PDFTerm, g.Evidence)
	}
	b.WriteString("\nSection excerpts:\n")
	for featureID, sample := range samples {
		fmt.Fprintf(&b, "\n## %s\n%s\n", featureID, sample)
	}
	return b.String()
}

func mergeGlossary(offline, llm []models.PDFGlossary, catalog *rpgconcepts.ConceptCatalog) []models.PDFGlossary {
	seen := map[string]models.PDFGlossary{}
	for _, g := range offline {
		if _, ok := catalog.FeatureByID(g.FeatureID); !ok {
			continue
		}
		key := g.FeatureID + "\x00" + strings.ToLower(g.PDFTerm)
		seen[key] = g
	}
	for _, g := range llm {
		if g.FeatureID == "" || g.PDFTerm == "" {
			continue
		}
		if _, ok := catalog.FeatureByID(g.FeatureID); !ok {
			continue
		}
		key := g.FeatureID + "\x00" + strings.ToLower(g.PDFTerm)
		seen[key] = g
	}
	out := make([]models.PDFGlossary, 0, len(seen))
	for _, g := range seen {
		out = append(out, g)
	}
	sortGlossary(out)
	return out
}

func sortGlossary(g []models.PDFGlossary) {
	for i := 0; i < len(g); i++ {
		for j := i + 1; j < len(g); j++ {
			if g[j].FeatureID < g[i].FeatureID || (g[j].FeatureID == g[i].FeatureID && g[j].PDFTerm < g[i].PDFTerm) {
				g[i], g[j] = g[j], g[i]
			}
		}
	}
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "{"); idx >= 0 {
		if end := strings.LastIndex(raw, "}"); end > idx {
			return raw[idx : end+1]
		}
	}
	return raw
}
