package indexing

import (
	"context"
	"fmt"
	"log"
	"strings"

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
	return rebuildCheatsheetFeatures(ctx, searchSvc, llmClient, catalog, pdfID, sections, meta, meta.Features, progress, false)
}

func rebuildCheatsheetFeatures(
	ctx context.Context,
	searchSvc *search.Service,
	llmClient *llm.Client,
	catalog *rpgconcepts.ConceptCatalog,
	pdfID string,
	sections []models.TOCSection,
	meta *models.PDFIndexMeta,
	featureIDs []string,
	progress cheatsheetProgress,
	force bool,
) (*models.PDFIndexMeta, error) {
	if meta == nil {
		meta = models.EmptyPDFIndexMeta()
	}
	if len(featureIDs) == 0 {
		models.NormalizeIndexMeta(meta)
		return meta, nil
	}

	sectionText := sectionTextByID(sections)
	done := map[string]struct{}{}
	if !force {
		for _, entry := range meta.Cheatsheet {
			if id := entry.FeatureID; id != "" {
				done[id] = struct{}{}
			}
		}
	}

	total := len(featureIDs)
	completed := 0
	var skipped []string
	for _, featureID := range featureIDs {
		if !force {
			if _, ok := done[featureID]; ok {
				continue
			}
		}
		completed++
		if progress.onProgress != nil {
			progress.onProgress("cheatsheet_llm", completed, total, featureID)
		}
		entry, err := buildCheatsheetForFeature(ctx, searchSvc, llmClient, catalog, pdfID, featureID, meta, sectionText)
		if err != nil {
			if progress.onCheckpoint != nil {
				progress.onCheckpoint(meta)
			}
			return meta, fmt.Errorf("cheatsheet pass %s: %w", featureID, err)
		}
		if entry == nil {
			entry = catalogCheatsheetFallback(catalog, featureID)
		}
		if entry != nil {
			upsertCheatsheetEntry(meta, *entry)
		} else {
			skipped = append(skipped, featureID)
		}
		if progress.onCheckpoint != nil {
			progress.onCheckpoint(meta)
		}
	}
	if progress.onProgress != nil {
		progress.onProgress("cheatsheet_llm", total, total, "")
	}
	models.NormalizeIndexMeta(meta)
	if len(skipped) > 0 {
		return meta, fmt.Errorf("cheatsheet entries could not be generated for: %s", strings.Join(skipped, ", "))
	}
	return meta, nil
}

func catalogCheatsheetFallback(catalog *rpgconcepts.ConceptCatalog, featureID string) *models.CheatsheetEntry {
	if catalog == nil {
		return nil
	}
	feat, ok := catalog.FeatureByID(featureID)
	if !ok {
		return nil
	}
	def := strings.TrimSpace(feat.Description)
	if def == "" {
		return nil
	}
	log.Printf("index: using catalog description fallback for cheatsheet feature %q", featureID)
	return &models.CheatsheetEntry{
		FeatureID:  featureID,
		Definition: def,
		Citations:  nil,
	}
}

func upsertCheatsheetEntry(meta *models.PDFIndexMeta, entry models.CheatsheetEntry) {
	for i, existing := range meta.Cheatsheet {
		if existing.FeatureID == entry.FeatureID {
			meta.Cheatsheet[i] = entry
			return
		}
	}
	meta.Cheatsheet = append(meta.Cheatsheet, entry)
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

func featureAllowedForCheatsheetRebuild(meta *models.PDFIndexMeta, featureID string) bool {
	if meta == nil || featureID == "" {
		return false
	}
	for _, id := range meta.Features {
		if id == featureID {
			return true
		}
	}
	for _, entry := range meta.Cheatsheet {
		if entry.FeatureID == featureID {
			return true
		}
	}
	return false
}
