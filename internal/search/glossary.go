package search

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

// ChatLearnings holds formatted glossary and cheatsheet blocks for chat prompts.
type ChatLearnings struct {
	Glossary   string
	Cheatsheet string
}

// GlossaryForGame loads formatted glossary and cheatsheet text for all PDFs in a game.
func GlossaryForGame(st store.Store, gameID string, userQuery string) (string, error) {
	learnings, err := LearningsForChat(st, gameID, userQuery)
	if err != nil {
		return "", err
	}
	return combineChatLearnings(learnings.Glossary, learnings.Cheatsheet), nil
}

// LearningsForChat loads glossary and cheatsheet blocks separately for chat prompts.
func LearningsForChat(st store.Store, gameID, userQuery string) (ChatLearnings, error) {
	pdfs, err := st.ListGamePDFs(gameID)
	if err != nil {
		return ChatLearnings{}, err
	}
	if len(pdfs) == 0 {
		return ChatLearnings{}, nil
	}
	ids := make([]string, len(pdfs))
	titles := map[string]string{}
	for i, p := range pdfs {
		ids[i] = p.ID
		titles[p.ID] = p.Title
	}
	meta, err := st.ListPDFIndexMeta(ids)
	if err != nil {
		return ChatLearnings{}, err
	}
	catalog := loadConceptCatalogFromEnv()
	return FormatChatLearnings(meta, titles, catalog, userQuery), nil
}

// FormatChatLearnings renders glossary and cheatsheet blocks from index meta.
func FormatChatLearnings(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string, catalog *rpgconcepts.ConceptCatalog, userQuery string) ChatLearnings {
	glossary, cheatsheet := formatChatLearnings(metaByPDF, pdfTitles, catalog, userQuery)
	return ChatLearnings{Glossary: glossary, Cheatsheet: cheatsheet}
}

func ExpandFTSWithGlossary(originalQuery, ftsQuery string, metaByPDF map[string]models.PDFIndexMeta, catalog *rpgconcepts.ConceptCatalog) string {
	if catalog == nil || len(metaByPDF) == 0 {
		return ftsQuery
	}
	featureIDs := matchedFeatureIDs(originalQuery+" "+ftsQuery, metaByPDF, catalog)
	if len(featureIDs) == 0 {
		return ftsQuery
	}
	featureSet := map[string]struct{}{}
	for _, id := range featureIDs {
		featureSet[id] = struct{}{}
	}

	seen := map[string]struct{}{}
	for _, w := range strings.Fields(ftsQuery) {
		seen[strings.ToLower(w)] = struct{}{}
	}
	parts := strings.Fields(ftsQuery)

	for _, meta := range metaByPDF {
		for _, g := range meta.Glossary {
			if _, ok := featureSet[g.FeatureID]; !ok {
				continue
			}
			for _, term := range g.Terms {
				term = strings.TrimSpace(term)
				if term == "" {
					continue
				}
				key := strings.ToLower(term)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				parts = append(parts, term)
			}
		}
	}
	return strings.Join(parts, " ")
}

func matchedFeatureIDs(query string, metaByPDF map[string]models.PDFIndexMeta, catalog *rpgconcepts.ConceptCatalog) []string {
	found := map[string]struct{}{}
	for _, id := range catalog.MatchQueryFeatures(query) {
		found[id] = struct{}{}
	}
	q := strings.ToLower(query)
	for _, meta := range metaByPDF {
		for _, g := range meta.Glossary {
			for _, term := range g.Terms {
				if term != "" && strings.Contains(q, strings.ToLower(term)) {
					found[g.FeatureID] = struct{}{}
				}
			}
		}
	}
	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	return ids
}

// FormatLearningsForPrompt renders glossary mappings and matching cheatsheet entries for chat context.
func FormatLearningsForPrompt(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string, catalog *rpgconcepts.ConceptCatalog, userQuery string) string {
	glossary, cheatsheet := formatChatLearnings(metaByPDF, pdfTitles, catalog, userQuery)
	return combineChatLearnings(glossary, cheatsheet)
}

func combineChatLearnings(glossary, cheatsheet string) string {
	glossary = strings.TrimSpace(glossary)
	cheatsheet = strings.TrimSpace(cheatsheet)
	if glossary == "" && cheatsheet == "" {
		return ""
	}
	var out strings.Builder
	if glossary != "" {
		out.WriteString(glossary)
	}
	if cheatsheet != "" {
		if glossary != "" {
			out.WriteByte('\n')
		}
		out.WriteString(cheatsheet)
	}
	return out.String()
}

func formatChatLearnings(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string, catalog *rpgconcepts.ConceptCatalog, userQuery string) (glossaryBlock, cheatsheetBlock string) {
	if len(metaByPDF) == 0 {
		return "", ""
	}
	matched := matchedFeaturesForQuery(userQuery, metaByPDF, catalog)

	var b strings.Builder
	glossaryLines := 0
	for pdfID, meta := range metaByPDF {
		title := pdfTitles[pdfID]
		if title == "" {
			title = pdfID
		}
		for _, g := range meta.Glossary {
			if len(g.Terms) == 0 {
				continue
			}
			fmt.Fprintf(&b, "- %s: %s → %s\n", title, g.FeatureID, strings.Join(g.Terms, ", "))
			glossaryLines++
		}
	}
	if glossaryLines > 0 {
		glossaryBlock = "Book terminology (this game's indexed PDFs):\n" + b.String()
	}

	cheatsheetLines := 0
	var cs strings.Builder
	for pdfID, meta := range metaByPDF {
		title := pdfTitles[pdfID]
		if title == "" {
			title = pdfID
		}
		for _, c := range meta.Cheatsheet {
			if !cheatsheetEntryMatches(c, matched) {
				continue
			}
			if c.Definition == "" {
				continue
			}
			name := featureDisplayName(catalog, c.FeatureID)
			fmt.Fprintf(&cs, "- %s / %s (%s): %s\n", title, name, c.FeatureID, c.Definition)
			for i, cit := range c.Citations {
				writeCheatsheetCitationLine(&cs, i+1, cit)
			}
			cheatsheetLines++
		}
	}
	if cheatsheetLines > 0 {
		header := "Cheatsheet (matching user question)"
		if len(matched) == 0 {
			header = "Cheatsheet (indexed features)"
		}
		cheatsheetBlock = header + ":\n" + cs.String()
	}
	return glossaryBlock, cheatsheetBlock
}

func matchedFeaturesForQuery(userQuery string, metaByPDF map[string]models.PDFIndexMeta, catalog *rpgconcepts.ConceptCatalog) map[string]struct{} {
	matched := map[string]struct{}{}
	if catalog != nil && strings.TrimSpace(userQuery) != "" {
		for _, id := range matchedFeatureIDs(userQuery, metaByPDF, catalog) {
			matched[id] = struct{}{}
		}
	}
	return matched
}

func cheatsheetEntryMatches(entry models.CheatsheetEntry, matched map[string]struct{}) bool {
	if len(matched) == 0 {
		return true
	}
	_, ok := matched[entry.FeatureID]
	return ok
}

func featureDisplayName(catalog *rpgconcepts.ConceptCatalog, featureID string) string {
	if catalog != nil {
		if feat, ok := catalog.FeatureByID(featureID); ok {
			if name := strings.TrimSpace(feat.Name); name != "" {
				return name
			}
		}
	}
	return featureID
}

func writeCheatsheetCitationLine(b *strings.Builder, index int, cit models.CheatsheetCitation) {
	pageSuffix := fmt.Sprintf("(page %d)", cit.StartPage)
	if cit.EndPage > 0 && cit.EndPage != cit.StartPage {
		pageSuffix = fmt.Sprintf("(pages %d-%d)", cit.StartPage, cit.EndPage)
	}
	if cit.SectionID != "" {
		fmt.Fprintf(b, "  [%d] %s [section_id=%s] %s\n", index, cit.SectionTitle, cit.SectionID, pageSuffix)
		return
	}
	fmt.Fprintf(b, "  [%d] %s %s\n", index, cit.SectionTitle, pageSuffix)
}

// FormatGlossaryForPrompt renders per-PDF glossary mappings for chat context.
func FormatGlossaryForPrompt(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string) string {
	return FormatLearningsForPrompt(metaByPDF, pdfTitles, nil, "")
}

// FormatGlossaryForSearchRewrite renders indexed glossary mappings for the search rewrite LLM.
// When the query matches catalog or glossary features, only those mappings are included; otherwise all are shown.
func FormatGlossaryForSearchRewrite(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string, catalog *rpgconcepts.ConceptCatalog, query string) string {
	if len(metaByPDF) == 0 {
		return ""
	}
	matched := map[string]struct{}{}
	if catalog != nil && strings.TrimSpace(query) != "" {
		for _, id := range matchedFeatureIDs(query, metaByPDF, catalog) {
			matched[id] = struct{}{}
		}
	}
	lines := formatGlossaryLines(metaByPDF, pdfTitles, catalog, matched)
	if len(lines) == 0 && len(matched) > 0 {
		lines = formatGlossaryLines(metaByPDF, pdfTitles, catalog, nil)
	}
	if len(lines) == 0 {
		return ""
	}
	return "Book terminology for indexed PDFs (include matching PDF-specific terms in your keywords):\n" + strings.Join(lines, "\n")
}

func formatGlossaryLines(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string, catalog *rpgconcepts.ConceptCatalog, featureFilter map[string]struct{}) []string {
	var lines []string
	for pdfID, meta := range metaByPDF {
		title := pdfTitles[pdfID]
		if title == "" {
			title = pdfID
		}
		for _, g := range meta.Glossary {
			if len(g.Terms) == 0 {
				continue
			}
			if len(featureFilter) > 0 {
				if _, ok := featureFilter[g.FeatureID]; !ok {
					continue
				}
			}
			featureLabel := g.FeatureID
			if catalog != nil {
				if feat, ok := catalog.FeatureByID(g.FeatureID); ok {
					if name := strings.TrimSpace(feat.Name); name != "" {
						featureLabel = name + " (" + g.FeatureID + ")"
					}
				}
			}
			lines = append(lines, fmt.Sprintf("- %s / %s: %s", title, featureLabel, strings.Join(g.Terms, ", ")))
		}
	}
	sort.Strings(lines)
	return lines
}

func loadConceptCatalogFromEnv() *rpgconcepts.ConceptCatalog {
	catalog, err := rpgconcepts.LoadConcepts(rpgconcepts.DefaultConceptsPath())
	if err != nil {
		return nil
	}
	return catalog
}

// CitationSectionIDs returns section IDs cited for features matching the query.
func CitationSectionIDs(query string, metaByPDF map[string]models.PDFIndexMeta, catalog *rpgconcepts.ConceptCatalog) map[string]struct{} {
	out := map[string]struct{}{}
	if catalog == nil {
		return out
	}
	matched := matchedFeatureIDs(query, metaByPDF, catalog)
	if len(matched) == 0 {
		return out
	}
	featureSet := map[string]struct{}{}
	for _, id := range matched {
		featureSet[id] = struct{}{}
	}
	for _, meta := range metaByPDF {
		for _, c := range meta.Cheatsheet {
			if _, ok := featureSet[c.FeatureID]; !ok {
				continue
			}
			for _, cit := range c.Citations {
				if cit.SectionID != "" {
					out[cit.SectionID] = struct{}{}
				}
			}
		}
	}
	return out
}

// FeatureDefinitionQuery builds a keyword query for hybrid search when building cheatsheet entries.
func FeatureDefinitionQuery(catalog *rpgconcepts.ConceptCatalog, glossary []models.PDFGlossaryEntry, featureID string) string {
	if catalog == nil {
		return featureID + " rules definition"
	}
	seen := map[string]struct{}{}
	var parts []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		parts = append(parts, s)
	}
	if feat, ok := catalog.FeatureByID(featureID); ok {
		add(feat.Name)
		for _, syn := range feat.Synonyms {
			add(syn)
		}
		for _, w := range strings.Fields(feat.Description) {
			w = strings.Trim(w, ".,;:\"'()[]")
			if len(w) >= 4 {
				add(w)
			}
		}
		for _, q := range feat.Questions {
			for _, w := range strings.Fields(q) {
				w = strings.Trim(w, ".,;:?\"'()[]")
				if len(w) >= 4 {
					add(w)
				}
			}
		}
	}
	for _, g := range glossary {
		if g.FeatureID != featureID {
			continue
		}
		for _, term := range g.Terms {
			add(term)
		}
	}
	add("rules")
	add("definition")
	if len(parts) == 0 {
		return featureID + " rules definition"
	}
	return strings.Join(parts, " ")
}

const cheatsheetCitationSnippetWords = 400

// CheatsheetCitationHits returns search hits for sections cited by matching cheatsheet entries.
func CheatsheetCitationHits(st store.Store, gameID, userQuery string) ([]models.SearchHit, error) {
	pdfs, err := st.ListGamePDFs(gameID)
	if err != nil {
		return nil, err
	}
	if len(pdfs) == 0 {
		return nil, nil
	}
	ids := make([]string, len(pdfs))
	titles := map[string]string{}
	pdfOrder := map[string]int{}
	for i, p := range pdfs {
		ids[i] = p.ID
		titles[p.ID] = p.Title
		pdfOrder[p.ID] = p.SortOrder
	}
	metaByPDF, err := st.ListPDFIndexMeta(ids)
	if err != nil {
		return nil, err
	}
	catalog := loadConceptCatalogFromEnv()
	matched := matchedFeaturesForQuery(userQuery, metaByPDF, catalog)

	var hits []models.SearchHit
	seen := map[string]struct{}{}
	for _, pdf := range pdfs {
		sections, err := st.ListTOCSections(pdf.ID)
		if err != nil {
			return nil, err
		}
		sectionByID := map[string]models.TOCSection{}
		for _, sec := range sections {
			sectionByID[sec.ID] = sec
		}
		meta := metaByPDF[pdf.ID]
		for _, entry := range meta.Cheatsheet {
			if !cheatsheetEntryMatches(entry, matched) || entry.Definition == "" {
				continue
			}
			for _, cit := range entry.Citations {
				secID := strings.TrimSpace(cit.SectionID)
				if secID == "" {
					continue
				}
				if _, ok := seen[secID]; ok {
					continue
				}
				sec, ok := sectionByID[secID]
				if !ok || strings.TrimSpace(sec.PlainText) == "" {
					continue
				}
				seen[secID] = struct{}{}
				startPage := cit.StartPage
				endPage := cit.EndPage
				if startPage == 0 {
					startPage = sec.StartPage
				}
				if endPage == 0 {
					endPage = sec.EndPage
				}
				title := cit.SectionTitle
				if title == "" {
					title = sec.Title
				}
				hits = append(hits, models.SearchHit{
					SectionID:    secID,
					PDFID:        pdf.ID,
					PDFTitle:     titles[pdf.ID],
					SectionTitle: title,
					StartPage:    startPage,
					EndPage:      endPage,
					PDFSortOrder: pdfOrder[pdf.ID],
					Score:        1,
					Snippet:      cheatsheetCitationSnippet(sec.PlainText),
				})
			}
		}
	}
	return hits, nil
}

func cheatsheetCitationSnippet(text string) string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) <= cheatsheetCitationSnippetWords {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:cheatsheetCitationSnippetWords], " ") + "..."
}

// FormatSearchHits renders search hits as numbered excerpts for tool results.
func FormatSearchHits(hits []models.SearchHit) string {
	if len(hits) == 0 {
		return "No matching rule excerpts found."
	}
	var b strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&b, "\n[%d] %s — %s (pages %d-%d)\n%s\n",
			i+1, h.PDFTitle, h.SectionTitle, h.StartPage, h.EndPage, h.Snippet)
	}
	return strings.TrimSpace(b.String())
}

// LookupCheatsheetByFeature returns cheatsheet entries for one feature across a game's PDFs.
func LookupCheatsheetByFeature(st store.Store, gameID, featureID string) (string, error) {
	featureID = strings.TrimSpace(featureID)
	if featureID == "" {
		return "", fmt.Errorf("feature_id is required")
	}
	metaByPDF, titles, catalog, err := gameIndexMeta(st, gameID)
	if err != nil {
		return "", err
	}
	if len(metaByPDF) == 0 {
		return fmt.Sprintf("No cheatsheet entry for feature_id=%s in this game's indexed PDFs.", featureID), nil
	}
	featureFilter := map[string]struct{}{featureID: {}}
	var cs strings.Builder
	lines := 0
	for pdfID, meta := range metaByPDF {
		title := titles[pdfID]
		for _, c := range meta.Cheatsheet {
			if !cheatsheetEntryMatches(c, featureFilter) {
				continue
			}
			if c.Definition == "" {
				continue
			}
			name := featureDisplayName(catalog, c.FeatureID)
			fmt.Fprintf(&cs, "- %s / %s (%s): %s\n", title, name, c.FeatureID, c.Definition)
			for i, cit := range c.Citations {
				writeCheatsheetCitationLine(&cs, i+1, cit)
			}
			lines++
		}
	}
	if lines == 0 {
		return fmt.Sprintf("No cheatsheet entry for feature_id=%s in this game's indexed PDFs.", featureID), nil
	}
	return strings.TrimSpace(cs.String()), nil
}

// LookupGlossaryByFeature returns book terminology and catalog synonyms for a feature.
func LookupGlossaryByFeature(st store.Store, gameID, featureID string) (string, error) {
	featureID = strings.TrimSpace(featureID)
	if featureID == "" {
		return "", fmt.Errorf("feature_id is required")
	}
	metaByPDF, titles, catalog, err := gameIndexMeta(st, gameID)
	if err != nil {
		return "", err
	}
	featureFilter := map[string]struct{}{featureID: {}}
	return formatGlossaryLookup(metaByPDF, titles, catalog, featureFilter), nil
}

// LookupGlossaryByTerm resolves a term to feature(s) and returns glossary mappings.
func LookupGlossaryByTerm(st store.Store, gameID, term string) (string, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return "", fmt.Errorf("term is required")
	}
	metaByPDF, titles, catalog, err := gameIndexMeta(st, gameID)
	if err != nil {
		return "", err
	}
	featureFilter := map[string]struct{}{}
	if catalog != nil {
		for _, id := range matchedFeatureIDs(term, metaByPDF, catalog) {
			featureFilter[id] = struct{}{}
		}
	}
	if len(featureFilter) == 0 {
		return fmt.Sprintf("No glossary mapping found for term %q.", term), nil
	}
	return formatGlossaryLookup(metaByPDF, titles, catalog, featureFilter), nil
}

func formatGlossaryLookup(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string, catalog *rpgconcepts.ConceptCatalog, featureFilter map[string]struct{}) string {
	lines := formatGlossaryLines(metaByPDF, pdfTitles, catalog, featureFilter)
	var b strings.Builder
	if len(lines) > 0 {
		b.WriteString("Book terminology:\n")
		b.WriteString(strings.Join(lines, "\n"))
	}
	var synLines []string
	for id := range featureFilter {
		if catalog == nil {
			continue
		}
		if feat, ok := catalog.FeatureByID(id); ok && len(feat.Synonyms) > 0 {
			name := feat.Name
			if name == "" {
				name = id
			}
			synLines = append(synLines, fmt.Sprintf("- %s (%s): %s", name, id, strings.Join(feat.Synonyms, ", ")))
		}
	}
	sort.Strings(synLines)
	if len(synLines) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("Catalog synonyms:\n")
		b.WriteString(strings.Join(synLines, "\n"))
	}
	if b.Len() == 0 {
		return "No glossary entries found for the requested feature(s)."
	}
	return strings.TrimSpace(b.String())
}

func gameIndexMeta(st store.Store, gameID string) (map[string]models.PDFIndexMeta, map[string]string, *rpgconcepts.ConceptCatalog, error) {
	pdfs, err := st.ListGamePDFs(gameID)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(pdfs) == 0 {
		return map[string]models.PDFIndexMeta{}, map[string]string{}, loadConceptCatalogFromEnv(), nil
	}
	ids := make([]string, len(pdfs))
	titles := map[string]string{}
	for i, p := range pdfs {
		ids[i] = p.ID
		titles[p.ID] = p.Title
	}
	meta, err := st.ListPDFIndexMeta(ids)
	if err != nil {
		return nil, nil, nil, err
	}
	return meta, titles, loadConceptCatalogFromEnv(), nil
}
