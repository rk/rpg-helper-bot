package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

const (
	excerptMaxWords     = 1000
	learningsTimeout    = 120 * time.Second
	cheatsheetChunkSize = 4
	llmResponsePreview  = 500
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

// BuildPDFLearnings runs the 3-pass chunked LLM indexing pipeline.
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

	total := len(features)
	for i, featureID := range features {
		if opts.OnProgress != nil {
			opts.OnProgress("cheatsheet_llm", i, total)
		}
		relevant := filterSectionsForFeature(sections, glossary, featureID)
		entry, err := c.buildCheatsheetEntryChunked(ctx, catalog, glossary, featureID, relevant)
		if err != nil {
			emitLearningsCheckpoint(meta, opts)
			return meta, fmt.Errorf("cheatsheet pass %s: %w", featureID, err)
		}
		if entry != nil {
			meta.Cheatsheet = append(meta.Cheatsheet, *entry)
		}
		emitLearningsCheckpoint(meta, opts)
	}
	if opts.OnProgress != nil {
		opts.OnProgress("cheatsheet_llm", total, total)
	}

	models.NormalizeIndexMeta(meta)
	return meta, nil
}

// BuildPDFCheatsheet fills missing cheatsheet rows for features already detected in meta.
func (c *Client) BuildPDFCheatsheet(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, sections []SectionSample, meta *models.PDFIndexMeta, opts LearningsOptions) (*models.PDFIndexMeta, error) {
	if c == nil || catalog == nil {
		return nil, fmt.Errorf("llm or catalog unavailable")
	}
	if meta == nil {
		return nil, fmt.Errorf("index meta unavailable")
	}
	out := *meta
	if len(out.Features) == 0 {
		return &out, fmt.Errorf("no features to build cheatsheet for")
	}

	done := map[string]struct{}{}
	for _, entry := range out.Cheatsheet {
		if id := strings.TrimSpace(entry.FeatureID); id != "" {
			done[id] = struct{}{}
		}
	}

	total := len(out.Features)
	completed := len(done)
	var pending []string
	for _, featureID := range out.Features {
		if _, ok := done[featureID]; !ok {
			pending = append(pending, featureID)
		}
	}
	if len(pending) == 0 {
		models.NormalizeIndexMeta(&out)
		return &out, nil
	}

	for _, featureID := range pending {
		if opts.OnProgress != nil {
			opts.OnProgress("cheatsheet_llm", completed, total)
		}
		relevant := filterSectionsForFeature(sections, out.Glossary, featureID)
		entry, err := c.buildCheatsheetEntryChunked(ctx, catalog, out.Glossary, featureID, relevant)
		if err != nil {
			emitLearningsCheckpoint(&out, opts)
			return &out, fmt.Errorf("cheatsheet pass %s: %w", featureID, err)
		}
		if entry != nil {
			out.Cheatsheet = append(out.Cheatsheet, *entry)
		}
		completed++
		emitLearningsCheckpoint(&out, opts)
	}
	if opts.OnProgress != nil {
		opts.OnProgress("cheatsheet_llm", total, total)
	}

	models.NormalizeIndexMeta(&out)
	return &out, nil
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

	wordTSV := BuildWordFrequencyTSV(sections, wordStatsTopN)
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

func (c *Client) detectPDFFeaturesFromWordStats(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, glossary []models.PDFGlossaryEntry, sections []SectionSample) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, learningsTimeout)
	defer cancel()

	wordTSV := BuildWordFrequencyTSV(sections, wordStatsTopN)
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

func (c *Client) buildCheatsheetEntryChunked(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, glossary []models.PDFGlossaryEntry, featureID string, sections []SectionSample) (*models.CheatsheetEntry, error) {
	chunks := chunkSections(sections, cheatsheetChunkSize)
	if len(chunks) == 0 {
		chunks = [][]SectionSample{nil}
	}

	var draft *models.CheatsheetEntry
	for i, chunk := range chunks {
		updated, err := c.buildCheatsheetChunk(ctx, catalog, glossary, featureID, draft, chunk, i+1, len(chunks))
		if err != nil {
			return nil, err
		}
		if updated == nil {
			continue
		}
		draft = updated
	}
	return draft, nil
}

func (c *Client) buildCheatsheetChunk(ctx context.Context, catalog *rpgconcepts.ConceptCatalog, glossary []models.PDFGlossaryEntry, featureID string, draft *models.CheatsheetEntry, chunk []SectionSample, chunkNum, chunkTotal int) (*models.CheatsheetEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, learningsTimeout)
	defer cancel()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Build or refine a cheatsheet entry for feature_id=%q.\n", featureID))
	b.WriteString(fmt.Sprintf("Processing section chunk %d of %d.\n", chunkNum, chunkTotal))
	b.WriteString("Catalog entry:\n")
	if feat, ok := catalog.FeatureByID(featureID); ok {
		fmt.Fprintf(&b, "- %s (%s): %s; synonyms=%v\n", feat.ID, feat.Name, feat.Description, feat.Synonyms)
	}
	b.WriteString("\nGlossary terms for this feature:\n")
	for _, g := range glossary {
		if g.FeatureID == featureID {
			fmt.Fprintf(&b, "- %v\n", g.Terms)
		}
	}
	writeDraftCheatsheet(&b, draft)
	b.WriteString("\nNew section excerpts in this chunk:\n")
	for _, sec := range chunk {
		fmt.Fprintf(&b, "\n## %s [section_id=%s] (pages %d-%d)\n%s\n", sec.Title, sec.SectionID, sec.StartPage, sec.EndPage, sec.Excerpt)
	}

	systemPrompt := renderPrompt(PromptCheatsheetExtract, nil)
	raw, err := c.CompleteJSON(ctx, systemPrompt, b.String())
	if err != nil {
		return nil, err
	}
	var resp *cheatsheetLLMResponse
	resp, err = parseCheatsheetJSON(raw, llmParseContext{
		Pass:          "cheatsheet",
		ChunkNum:      chunkNum,
		ChunkTotal:    chunkTotal,
		FeatureID:     featureID,
		SectionTitles: sectionTitles(chunk),
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
	if resp.Definition == "" && draft == nil {
		return nil, nil
	}
	entry := models.CheatsheetEntry{
		FeatureID:  resp.FeatureID,
		Definition: resp.Definition,
		Citations:  resp.Citations,
	}
	if entry.Definition == "" && draft != nil {
		entry.Definition = draft.Definition
	}
	entry.Citations = mergeCitations(draftCitations(draft), entry.Citations)
	if entry.Definition == "" {
		return draft, nil
	}
	return &entry, nil
}

func draftCitations(draft *models.CheatsheetEntry) []models.CheatsheetCitation {
	if draft == nil {
		return nil
	}
	return draft.Citations
}

func chunkSections(sections []SectionSample, size int) [][]SectionSample {
	if size <= 0 {
		size = cheatsheetChunkSize
	}
	if len(sections) == 0 {
		return nil
	}
	var chunks [][]SectionSample
	for i := 0; i < len(sections); i += size {
		end := i + size
		if end > len(sections) {
			end = len(sections)
		}
		chunks = append(chunks, sections[i:end])
	}
	return chunks
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

func mergeCitations(existing, updated []models.CheatsheetCitation) []models.CheatsheetCitation {
	seen := map[string]struct{}{}
	var out []models.CheatsheetCitation
	for _, list := range [][]models.CheatsheetCitation{existing, updated} {
		for _, c := range list {
			key := strings.ToLower(strings.TrimSpace(c.SectionTitle)) + "\x00" + fmt.Sprintf("%d", c.StartPage)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}

func writeDraftCheatsheet(b *strings.Builder, draft *models.CheatsheetEntry) {
	if draft == nil {
		b.WriteString("\nCurrent draft cheatsheet: (empty — first chunk)\n")
		return
	}
	raw, _ := json.Marshal(draft)
	b.WriteString("\nCurrent draft cheatsheet (refine definition and extend citations):\n")
	b.Write(raw)
	b.WriteByte('\n')
}

func buildSectionChunkMessage(catalog *rpgconcepts.ConceptCatalog, sections []SectionSample, intro string) string {
	var b strings.Builder
	b.WriteString("\n\nCanonical feature catalog:\n")
	for _, f := range catalog.Features {
		fmt.Fprintf(&b, "- %s (%s): %s; synonyms=%v\n", f.ID, f.Name, f.Description, f.Synonyms)
	}
	b.WriteString("\n")
	b.WriteString(intro)
	b.WriteString("\n")
	for _, sec := range sections {
		fmt.Fprintf(&b, "\n## %s [section_id=%s] (pages %d-%d)\n%s\n", sec.Title, sec.SectionID, sec.StartPage, sec.EndPage, sec.Excerpt)
	}
	return b.String()
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

func filterSectionsForFeature(sections []SectionSample, glossary []models.PDFGlossaryEntry, featureID string) []SectionSample {
	var terms []string
	for _, g := range glossary {
		if g.FeatureID == featureID {
			terms = append(terms, g.Terms...)
		}
	}
	if len(terms) == 0 {
		return sections
	}
	var matched []SectionSample
	for _, sec := range sections {
		body := sec.PlainText
		if body == "" {
			body = sec.Excerpt
		}
		hay := strings.ToLower(sec.Title + "\n" + body)
		for _, term := range terms {
			if strings.Contains(hay, strings.ToLower(term)) {
				matched = append(matched, sec)
				break
			}
		}
	}
	if len(matched) == 0 {
		return sections
	}
	return matched
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

func sectionTitles(chunk []SectionSample) []string {
	titles := make([]string, 0, len(chunk))
	for _, sec := range chunk {
		if t := strings.TrimSpace(sec.Title); t != "" {
			titles = append(titles, t)
		}
	}
	return titles
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

// ChunkSections splits section samples for tests and callers.
func ChunkSections(sections []SectionSample, size int) [][]SectionSample {
	return chunkSections(sections, size)
}

// MergeGlossary combines glossary entries by feature_id.
func MergeGlossary(existing, updated []models.PDFGlossaryEntry) []models.PDFGlossaryEntry {
	return mergeGlossary(existing, updated)
}
