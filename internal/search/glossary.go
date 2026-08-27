package search

import (
	"fmt"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

// GlossaryForGame loads formatted glossary and cheatsheet text for all PDFs in a game.
func GlossaryForGame(st store.Store, gameID string, userQuery string) (string, error) {
	pdfs, err := st.ListGamePDFs(gameID)
	if err != nil {
		return "", err
	}
	if len(pdfs) == 0 {
		return "", nil
	}
	ids := make([]string, len(pdfs))
	titles := map[string]string{}
	for i, p := range pdfs {
		ids[i] = p.ID
		titles[p.ID] = p.Title
	}
	meta, err := st.ListPDFIndexMeta(ids)
	if err != nil {
		return "", err
	}
	catalog := loadConceptCatalogFromEnv()
	return FormatLearningsForPrompt(meta, titles, catalog, userQuery), nil
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
	if len(metaByPDF) == 0 {
		return ""
	}
	matched := map[string]struct{}{}
	if catalog != nil && userQuery != "" {
		for _, id := range matchedFeatureIDs(userQuery, metaByPDF, catalog) {
			matched[id] = struct{}{}
		}
	}

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

	cheatsheetLines := 0
	var cs strings.Builder
	for pdfID, meta := range metaByPDF {
		title := pdfTitles[pdfID]
		if title == "" {
			title = pdfID
		}
		for _, c := range meta.Cheatsheet {
			if len(matched) > 0 {
				if _, ok := matched[c.FeatureID]; !ok {
					continue
				}
			}
			if c.Definition == "" {
				continue
			}
			name := c.FeatureID
			if catalog != nil {
				if feat, ok := catalog.FeatureByID(c.FeatureID); ok {
					name = feat.Name
				}
			}
			fmt.Fprintf(&cs, "- %s / %s: %s\n", title, name, c.Definition)
			for i, cit := range c.Citations {
				if cit.EndPage > 0 && cit.EndPage != cit.StartPage {
					fmt.Fprintf(&cs, "  [%d] %s (pages %d-%d)\n", i+1, cit.SectionTitle, cit.StartPage, cit.EndPage)
				} else {
					fmt.Fprintf(&cs, "  [%d] %s (page %d)\n", i+1, cit.SectionTitle, cit.StartPage)
				}
			}
			cheatsheetLines++
		}
	}

	if glossaryLines == 0 && cheatsheetLines == 0 {
		return ""
	}

	var out strings.Builder
	if glossaryLines > 0 {
		out.WriteString("Book terminology (this game's indexed PDFs):\n")
		out.WriteString(b.String())
	}
	if cheatsheetLines > 0 {
		if glossaryLines > 0 {
			out.WriteByte('\n')
		}
		out.WriteString("Cheatsheet (matching user question):\n")
		out.WriteString(cs.String())
	}
	return out.String()
}

// FormatGlossaryForPrompt renders per-PDF glossary mappings for chat context.
func FormatGlossaryForPrompt(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string) string {
	return FormatLearningsForPrompt(metaByPDF, pdfTitles, nil, "")
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
