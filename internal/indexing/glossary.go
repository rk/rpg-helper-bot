package indexing

import (
	"context"
	"log"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
)

// BuildPDFLearningsLLM runs the 3-pass learnings pipeline during indexing.
// Glossary and features use LLM word-stats; cheatsheet uses hybrid search + one LLM call per feature.
// Checkpoints meta via saveMeta as each step completes.
func BuildPDFLearningsLLM(ctx context.Context, searchSvc *search.Service, llmClient *llm.Client, catalog *rpgconcepts.ConceptCatalog, sections []models.TOCSection, pdfID string, saveMeta func(*models.PDFIndexMeta) error) *models.PDFIndexMeta {
	meta := models.EmptyPDFIndexMeta()
	if llmClient == nil || catalog == nil || searchSvc == nil {
		meta.LLMLearningsSkipped = true
		return meta
	}

	samples := llm.BuildSectionSamples(sections)
	if len(samples) == 0 {
		meta.LLMLearningsSkipped = true
		return meta
	}

	checkpoint := func(m *models.PDFIndexMeta) {
		llm.AttachSectionIDs(m.Cheatsheet, sections)
		models.NormalizeIndexMeta(m)
		m.LLMLearningsSkipped = cheatsheetIncomplete(m)
		if saveMeta != nil {
			if err := saveMeta(m); err != nil {
				log.Printf("index: save index meta checkpoint for pdf %s: %v", pdfID, err)
			}
		}
	}

	progress := cheatsheetProgress{
		onProgress: func(phase string, current, total int, featureID string) {
			switch phase {
			case "glossary_llm":
				progressGlossaryLLM(pdfID, current, total)
			case "features_llm":
				progressFeaturesLLM(pdfID, current, total)
			case "cheatsheet_llm":
				progressCheatsheetLLM(pdfID, current, total, featureID, catalog)
			}
		},
		onCheckpoint: checkpoint,
	}

	opts := llm.LearningsOptions{
		OnProgress: func(phase string, current, total int) {
			progress.onProgress(phase, current, total, "")
		},
		OnCheckpoint: checkpoint,
	}

	built, err := llmClient.BuildPDFLearnings(ctx, catalog, samples, opts)
	if built == nil {
		built = models.EmptyPDFIndexMeta()
	}
	if err != nil {
		log.Printf("index: LLM glossary/features failed for pdf %s (%d sections): %v", pdfID, len(samples), err)
		built.LLMLearningsSkipped = true
		if saveMeta != nil {
			_ = saveMeta(built)
		}
		return built
	}

	built, err = buildPDFCheatsheet(ctx, searchSvc, llmClient, catalog, pdfID, sections, built, progress)
	llm.AttachSectionIDs(built.Cheatsheet, sections)
	models.NormalizeIndexMeta(built)
	built.LLMLearningsSkipped = err != nil || cheatsheetIncomplete(built)

	if err != nil {
		log.Printf("index: cheatsheet build incomplete for pdf %s: %v", pdfID, err)
		if saveMeta != nil {
			if saveErr := saveMeta(built); saveErr != nil {
				log.Printf("index: save partial index meta for pdf %s: %v", pdfID, saveErr)
			}
		}
		return built
	}
	return built
}

func cheatsheetIncomplete(meta *models.PDFIndexMeta) bool {
	if meta == nil || len(meta.Features) == 0 {
		return false
	}
	done := map[string]struct{}{}
	for _, entry := range meta.Cheatsheet {
		if id := strings.TrimSpace(entry.FeatureID); id != "" {
			done[id] = struct{}{}
		}
	}
	for _, featureID := range meta.Features {
		if _, ok := done[featureID]; !ok {
			return true
		}
	}
	return false
}
