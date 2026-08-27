package indexing

import (
	"context"
	"fmt"
	"log"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
)

type cheatsheetProgress struct {
	onProgress   func(phase string, current, total int, featureID string)
	onCheckpoint func(*models.PDFIndexMeta)
}

func buildPDFCheatsheet(
	ctx context.Context,
	searchSvc *search.Service,
	llmClient *llm.Client,
	catalog *rpgconcepts.ConceptCatalog,
	pdfID string,
	sections []models.TOCSection,
	meta *models.PDFIndexMeta,
	progress cheatsheetProgress,
) (*models.PDFIndexMeta, error) {
	if searchSvc == nil || llmClient == nil || catalog == nil {
		return meta, errLLMUnavailable
	}
	if meta == nil {
		meta = models.EmptyPDFIndexMeta()
	}
	if len(meta.Features) == 0 {
		return meta, nil
	}

	sectionText := sectionTextByID(sections)
	done := map[string]struct{}{}
	for _, entry := range meta.Cheatsheet {
		if id := entry.FeatureID; id != "" {
			done[id] = struct{}{}
		}
	}

	total := len(meta.Features)
	completed := len(done)
	for _, featureID := range meta.Features {
		if _, ok := done[featureID]; ok {
			continue
		}
		if progress.onProgress != nil {
			progress.onProgress("cheatsheet_llm", completed+1, total, featureID)
		}
		entry, err := buildCheatsheetForFeature(ctx, searchSvc, llmClient, catalog, pdfID, featureID, meta, sectionText)
		if err != nil {
			if progress.onCheckpoint != nil {
				progress.onCheckpoint(meta)
			}
			return meta, fmt.Errorf("cheatsheet pass %s: %w", featureID, err)
		}
		if entry != nil {
			meta.Cheatsheet = append(meta.Cheatsheet, *entry)
		}
		completed++
		if progress.onCheckpoint != nil {
			progress.onCheckpoint(meta)
		}
	}
	if progress.onProgress != nil {
		progress.onProgress("cheatsheet_llm", total, total, "")
	}
	models.NormalizeIndexMeta(meta)
	return meta, nil
}

func buildCheatsheetForFeature(
	ctx context.Context,
	searchSvc *search.Service,
	llmClient *llm.Client,
	catalog *rpgconcepts.ConceptCatalog,
	pdfID, featureID string,
	meta *models.PDFIndexMeta,
	sectionText map[string]string,
) (*models.CheatsheetEntry, error) {
	query := search.FeatureDefinitionQuery(catalog, meta.Glossary, featureID)
	result, err := searchSvc.SearchPDF(ctx, pdfID, query, meta)
	if err != nil {
		return nil, err
	}
	if len(result.Hits) == 0 {
		log.Printf("index: no search hits for feature %q in pdf %s (query=%q)", featureID, pdfID, query)
	}
	return llmClient.BuildCheatsheetFromSearchHits(ctx, catalog, meta.Glossary, featureID, result.Hits, sectionText)
}

func sectionTextByID(sections []models.TOCSection) map[string]string {
	out := make(map[string]string, len(sections))
	for _, sec := range sections {
		if text := sec.PlainText; text != "" {
			out[sec.ID] = text
		}
	}
	return out
}

func pendingCheatsheetFeatures(meta *models.PDFIndexMeta) []string {
	if meta == nil {
		return nil
	}
	done := map[string]struct{}{}
	for _, entry := range meta.Cheatsheet {
		if entry.FeatureID != "" {
			done[entry.FeatureID] = struct{}{}
		}
	}
	var pending []string
	for _, id := range meta.Features {
		if _, ok := done[id]; !ok {
			pending = append(pending, id)
		}
	}
	return pending
}
