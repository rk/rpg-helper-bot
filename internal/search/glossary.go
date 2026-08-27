package search

import (
	"fmt"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

// GlossaryForGame loads formatted glossary text for all PDFs in a game.
func GlossaryForGame(st store.Store, gameID string) (string, error) {
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
	return FormatGlossaryForPrompt(meta, titles), nil
}
func ExpandFTSWithGlossary(originalQuery, ftsQuery string, metaByPDF map[string]models.PDFIndexMeta, catalog *rpgconcepts.ConceptCatalog) string {
	if catalog == nil || len(metaByPDF) == 0 {
		return ftsQuery
	}
	featureIDs := catalog.MatchQueryFeatures(originalQuery + " " + ftsQuery)
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
			term := strings.TrimSpace(g.PDFTerm)
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
	return strings.Join(parts, " ")
}

// FormatGlossaryForPrompt renders per-PDF glossary mappings for chat context.
func FormatGlossaryForPrompt(metaByPDF map[string]models.PDFIndexMeta, pdfTitles map[string]string) string {
	if len(metaByPDF) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Book terminology (this game's indexed PDFs):\n")
	lines := 0
	for pdfID, meta := range metaByPDF {
		title := pdfTitles[pdfID]
		if title == "" {
			title = pdfID
		}
		for _, g := range meta.Glossary {
			if g.PDFTerm == "" {
				continue
			}
			fmt.Fprintf(&b, "- %s: %q maps to %s", title, g.PDFTerm, g.FeatureID)
			if g.Evidence != "" {
				fmt.Fprintf(&b, " (%s)", g.Evidence)
			}
			b.WriteByte('\n')
			lines++
		}
	}
	if lines == 0 {
		return ""
	}
	return b.String()
}
