package llm

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

const (
	excerptMaxWords        = 1000
	learningsTimeout       = 180 * time.Second
	cheatsheetSearchHits   = 5
	cheatsheetExcerptWords = 800
	llmResponsePreview     = 500
)

// SectionSample is a section excerpt sent to LLM learnings passes.
type SectionSample struct {
	SectionID string
	Title     string
	StartPage int
	EndPage   int
	Excerpt   string
	PlainText string // full section text for word stats and matching
}

type glossaryLLMResponse struct {
	Glossary []models.PDFGlossaryEntry `json:"glossary"`
}

type featuresLLMResponse struct {
	Features []string `json:"features"`
}

type cheatsheetLLMResponse struct {
	FeatureID  string                      `json:"feature_id"`
	Definition string                      `json:"definition"`
	Citations  []models.CheatsheetCitation `json:"citations"`
}

type llmParseContext struct {
	Pass          string
	ChunkNum      int
	ChunkTotal    int
	FeatureID     string
	SectionTitles []string
}

// LearningsOptions configures progress and checkpoint callbacks for LLM learnings passes.
type LearningsOptions struct {
	OnProgress   func(phase string, current, total int)
	OnCheckpoint func(meta *models.PDFIndexMeta)
}

// BuildPDFLearnings runs glossary and features LLM passes (cheatsheet is built separately via search).
func (c *Client) BuildPDFLearnings(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, sections []SectionSample, opts LearningsOptions) (*models.PDFIndexMeta, error) {
	if c == nil || catalog == nil {
		return nil, fmt.Errorf("llm or catalog unavailable")
	}
	meta := models.EmptyPDFIndexMeta()

	if opts.OnProgress != nil {
		opts.OnProgress("glossary_llm", 0, 1)
	}
	glossary, err := c.extractPDFGlossaryFromWordStats(ctx, catalog, sections)
	if err != nil {
		return nil, fmt.Errorf("glossary pass: %w", err)
	}
	if opts.OnProgress != nil {
		opts.OnProgress("glossary_llm", 1, 1)
	}
	meta.Glossary = glossary
	emitLearningsCheckpoint(meta, opts)

	if opts.OnProgress != nil {
		opts.OnProgress("features_llm", 0, 1)
	}
	features, err := c.detectPDFFeaturesFromWordStats(ctx, catalog, glossary, sections)
	if err != nil {
		emitLearningsCheckpoint(meta, opts)
		return meta, fmt.Errorf("features pass: %w", err)
	}
	if opts.OnProgress != nil {
		opts.OnProgress("features_llm", 1, 1)
	}
	meta.Features = features
	emitLearningsCheckpoint(meta, opts)

	models.NormalizeIndexMeta(meta)
	return meta, nil
}

// BuildCheatsheetFromSearchHits synthesizes one cheatsheet entry from search-ranked section excerpts.
func (c *Client) BuildCheatsheetFromSearchHits(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, glossary []models.PDFGlossaryEntry, featureID string, hits []models.SearchHit, sectionText map[string]string) (*models.CheatsheetEntry, error) {
	if c == nil || catalog == nil {
		return nil, fmt.Errorf("llm or catalog unavailable")
	}
	feat, ok := catalog.FeatureByID(featureID)
	if !ok {
		return nil, fmt.Errorf("unknown feature_id %q", featureID)
	}

	if len(hits) == 0 {
		log.Printf("llm learnings: no search hits for feature %q; using catalog description fallback", featureID)
		def := strings.TrimSpace(feat.Description)
		if def == "" {
			return nil, nil
		}
		return &models.CheatsheetEntry{
			FeatureID:  featureID,
			Definition: def,
			Citations:  nil,
		}, nil
	}

	limit := cheatsheetSearchHits
	if len(hits) < limit {
		limit = len(hits)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Build a cheatsheet entry for feature_id=%q.\n", featureID))
	b.WriteString("Catalog entry:\n")
	fmt.Fprintf(&b, "- %s (%s): %s; synonyms=%v\n", feat.ID, feat.Name, feat.Description, feat.Synonyms)
	if len(feat.Questions) > 0 {
		b.WriteString("\nCatalog questions to address in the definition (when excerpts support an answer):\n")
		for _, q := range feat.Questions {
			fmt.Fprintf(&b, "- %s\n", q)
		}
	}
	b.WriteString("\nGlossary terms for this feature:\n")
	for _, g := range glossary {
		if g.FeatureID == featureID {
			fmt.Fprintf(&b, "- %v\n", g.Terms)
		}
	}
	b.WriteString("\nSearch-ranked section excerpts (best matches first):\n")
	for i, hit := range hits[:limit] {
		text := sectionText[hit.SectionID]
		if text == "" {
			text = hit.Snippet
		}
		excerpt := truncateWords(text, cheatsheetExcerptWords)
		fmt.Fprintf(&b, "\n## [%d] %s [section_id=%s] (pages %d-%d)\n%s\n",
			i+1, hit.SectionTitle, hit.SectionID, hit.StartPage, hit.EndPage, excerpt)
	}

	ctx, cancel := context.WithTimeout(ctx, learningsTimeout)
	defer cancel()

	systemPrompt := renderPrompt(PromptCheatsheetExtract, nil)
	raw, err := c.CompleteJSON(ctx, systemPrompt, b.String())
	if err != nil {
		return nil, err
	}
	resp, err := parseCheatsheetJSON(raw, llmParseContext{
		Pass:       "cheatsheet",
		ChunkNum:   1,
		ChunkTotal: 1,
		FeatureID:  featureID,
	})
	if err != nil {
		return nil, err
	}
	if resp.FeatureID == "" {
		resp.FeatureID = featureID
	}
	if _, ok := catalog.FeatureByID(resp.FeatureID); !ok {
		return nil, fmt.Errorf("unknown feature_id %q", resp.FeatureID)
	}
	resp.Definition = strings.TrimSpace(resp.Definition)
	if resp.Definition == "" {
		return nil, nil
	}
	return &models.CheatsheetEntry{
		FeatureID:  resp.FeatureID,
		Definition: resp.Definition,
		Citations:  resp.Citations,
	}, nil
}

func emitLearningsCheckpoint(meta *models.PDFIndexMeta, opts LearningsOptions) {
	if opts.OnCheckpoint == nil {
		return
	}
	snap := *meta
	models.NormalizeIndexMeta(&snap)
	opts.OnCheckpoint(&snap)
}

func (c *Client) extractPDFGlossaryFromWordStats(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, sections []SectionSample) ([]models.PDFGlossaryEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, learningsTimeout)
	defer cancel()

	wordTSV := BuildWordFrequencyTSV(sections, wordStatsTopN, catalog)
	tokenCount := DocumentTokenCount(sections)

	var b strings.Builder
	b.WriteString("Map book-specific terminology to canonical feature IDs using the document word-frequency dictionary.\n")
	b.WriteString(fmt.Sprintf("Document unique tokens: %d. Showing top %d by count.\n\n", tokenCount, wordStatsTopN))
	b.WriteString("Canonical feature catalog:\n")
	for _, f := range catalog.Features {
		fmt.Fprintf(&b, "- %s (%s): %s; synonyms=%v\n", f.ID, f.Name, f.Description, f.Synonyms)
	}
	b.WriteString("\nWord frequency TSV:\n")
	b.WriteString(wordTSV)
	b.WriteString("\nSelect terms from the dictionary that map to catalog features. ")
	b.WriteString("Return glossary terms in this book's natural casing (e.g. Tests, not TESTS).\n")

	systemPrompt := renderPrompt(PromptGlossaryExtract, nil)
	raw, err := c.CompleteJSON(ctx, systemPrompt, b.String())
	if err != nil {
		return nil, err
	}
	var resp glossaryLLMResponse
	if err := parseLLMJSON(raw, llmParseContext{
		Pass:       "glossary",
		ChunkNum:   1,
		ChunkTotal: 1,
	}, &resp); err != nil {
		return nil, err
	}
	return validateGlossary(resp.Glossary, catalog), nil
}

// ExtractPDFGlossary runs the glossary LLM pass over section word stats.
func (c *Client) ExtractPDFGlossary(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, sections []SectionSample) ([]models.PDFGlossaryEntry, error) {
	return c.extractPDFGlossaryFromWordStats(ctx, catalog, sections)
}

func (c *Client) detectPDFFeaturesFromWordStats(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, glossary []models.PDFGlossaryEntry, sections []SectionSample) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, learningsTimeout)
	defer cancel()

	wordTSV := BuildWordFrequencyTSV(sections, wordStatsTopN, catalog)
	candidates := FeatureCandidatesFromWordStats(catalog, glossary, sections)

	var b strings.Builder
	b.WriteString("Detect which canonical features this game actually uses from the word-frequency dictionary.\n")
	b.WriteString(fmt.Sprintf("Document unique tokens: %d. Showing top %d by count.\n\n", DocumentTokenCount(sections), wordStatsTopN))
	b.WriteString("Canonical feature catalog:\n")
	for _, f := range catalog.Features {
		fmt.Fprintf(&b, "- %s (%s): %s; synonyms=%v\n", f.ID, f.Name, f.Description, f.Synonyms)
	}
	b.WriteString("\nGlossary from pass 1:\n")
	for _, g := range glossary {
		fmt.Fprintf(&b, "- %s: %v\n", g.FeatureID, g.Terms)
	}
	if len(candidates) > 0 {
		b.WriteString("\nOffline token-match candidates (hint only):\n")
		for _, id := range candidates {
			b.WriteString("- ")
			b.WriteString(id)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\nWord frequency TSV:\n")
	b.WriteString(wordTSV)
	b.WriteString("\nReturn only feature_ids with clear evidence in the dictionary and glossary.\n")

	systemPrompt := renderPrompt(PromptFeatureDetect, nil)
	raw, err := c.CompleteJSON(ctx, systemPrompt, b.String())
	if err != nil {
		return nil, err
	}
	var resp featuresLLMResponse
	if err := parseLLMJSON(raw, llmParseContext{
		Pass:       "features",
		ChunkNum:   1,
		ChunkTotal: 1,
	}, &resp); err != nil {
		return nil, err
	}
	return validateFeatures(resp.Features, catalog), nil
}

// DetectPDFFeatures runs the features LLM pass using glossary and section word stats.
func (c *Client) DetectPDFFeatures(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, glossary []models.PDFGlossaryEntry, sections []SectionSample) ([]string, error) {
	return c.detectPDFFeaturesFromWordStats(ctx, catalog, glossary, sections)
}

func mergeGlossary(existing, updated []models.PDFGlossaryEntry) []models.PDFGlossaryEntry {
	byFeature := map[string]map[string]struct{}{}
	var order []string
	add := func(entries []models.PDFGlossaryEntry) {
		for _, g := range entries {
			id := strings.TrimSpace(g.FeatureID)
			if id == "" {
				continue
			}
			if byFeature[id] == nil {
				byFeature[id] = map[string]struct{}{}
				order = append(order, id)
			}
			for _, t := range g.Terms {
				t = strings.TrimSpace(t)
				if t != "" {
					byFeature[id][t] = struct{}{}
				}
			}
		}
	}
	add(existing)
	add(updated)
	out := make([]models.PDFGlossaryEntry, 0, len(order))
	for _, id := range order {
		entry := models.PDFGlossaryEntry{FeatureID: id}
		for t := range byFeature[id] {
			entry.Terms = append(entry.Terms, t)
		}
		sort.Strings(entry.Terms)
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FeatureID < out[j].FeatureID })
	return out
}

func BuildSectionSamples(sections []models.TOCSection) []SectionSample {
	out := make([]SectionSample, 0, len(sections))
	for _, sec := range sections {
		text := strings.TrimSpace(sec.PlainText)
		if text == "" {
			continue
		}
		excerpt := truncateWords(text, excerptMaxWords)
		out = append(out, SectionSample{
			SectionID: sec.ID,
			Title:     sec.Title,
			StartPage: sec.StartPage,
			EndPage:   sec.EndPage,
			Excerpt:   excerpt,
			PlainText: text,
		})
	}
	return out
}

func AttachSectionIDs(cheatsheet []models.CheatsheetEntry, sections []models.TOCSection) {
	byTitle := map[string]string{}
	for _, sec := range sections {
		byTitle[strings.ToLower(strings.TrimSpace(sec.Title))] = sec.ID
	}
	for i := range cheatsheet {
		for j := range cheatsheet[i].Citations {
			cit := &cheatsheet[i].Citations[j]
			if cit.SectionID != "" {
				continue
			}
			if id, ok := byTitle[strings.ToLower(strings.TrimSpace(cit.SectionTitle))]; ok {
				cit.SectionID = id
			}
		}
	}
}

func validateGlossary(entries []models.PDFGlossaryEntry, catalog *rpgconcepts.ConceptCatalog) []models.PDFGlossaryEntry {
	var out []models.PDFGlossaryEntry
	for _, g := range entries {
		g.FeatureID = strings.TrimSpace(g.FeatureID)
		if g.FeatureID == "" {
			continue
		}
		if _, ok := catalog.FeatureByID(g.FeatureID); !ok {
			continue
		}
		var terms []string
		for _, t := range g.Terms {
			t = strings.TrimSpace(t)
			if t != "" {
				terms = append(terms, t)
			}
		}
		if len(terms) == 0 {
			continue
		}
		g.Terms = terms
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FeatureID < out[j].FeatureID })
	return out
}

func validateFeatures(ids []string, catalog *rpgconcepts.ConceptCatalog) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := catalog.FeatureByID(id); !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func featureLabel(featureID string) string {
	if featureID == "" {
		return ""
	}
	return fmt.Sprintf(" feature=%q", featureID)
}

func previewLLMText(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func truncateWords(text string, maxWords int) string {
	words := strings.Fields(strings.TrimSpace(text))
	if maxWords <= 0 || len(words) <= maxWords {
		return strings.TrimSpace(text)
	}
	return strings.Join(words[:maxWords], " ") + "..."
}

// MergeGlossary combines glossary entries by feature_id.
func MergeGlossary(existing, updated []models.PDFGlossaryEntry) []models.PDFGlossaryEntry {
	return mergeGlossary(existing, updated)
}
