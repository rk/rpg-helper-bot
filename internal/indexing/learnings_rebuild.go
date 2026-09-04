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
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

type learningsRebuildInputs struct {
	sections []models.TOCSection
	catalog  *rpgconcepts.ConceptCatalog
	samples  []llm.SectionSample
}

func loadLearningsRebuildInputs(st store.Store, conceptsPath, pdfID string) (learningsRebuildInputs, error) {
	pdf, err := st.GetPDF(pdfID)
	if err != nil {
		return learningsRebuildInputs{}, err
	}
	if pdf.IndexStatus != models.IndexStatusIndexed {
		return learningsRebuildInputs{}, fmt.Errorf("pdf must be indexed before rebuilding learnings")
	}
	sections, err := st.ListTOCSections(pdfID)
	if err != nil {
		return learningsRebuildInputs{}, err
	}
	if len(sections) == 0 {
		return learningsRebuildInputs{}, fmt.Errorf("add TOC sections before rebuilding learnings")
	}
	samples := llm.BuildSectionSamples(sections)
	if len(samples) == 0 {
		return learningsRebuildInputs{}, errNoSectionText
	}
	if conceptsPath == "" {
		conceptsPath = rpgconcepts.DefaultConceptsPath()
	}
	catalog, err := rpgconcepts.LoadConcepts(conceptsPath)
	if err != nil {
		return learningsRebuildInputs{}, fmt.Errorf("concepts: %w", err)
	}
	return learningsRebuildInputs{sections: sections, catalog: catalog, samples: samples}, nil
}

func finalizeLearningsMeta(meta *models.PDFIndexMeta, sections []models.TOCSection) {
	llm.AttachSectionIDs(meta.Cheatsheet, sections)
	models.NormalizeIndexMeta(meta)
	meta.LLMLearningsSkipped = cheatsheetIncomplete(meta)
}

// RebuildPDFGlossary runs only the glossary LLM pass, preserving features and cheatsheet.
func RebuildPDFGlossary(ctx context.Context, llmClient *llm.Client, st store.Store, inputs learningsRebuildInputs, pdfID string) (*models.PDFIndexMeta, error) {
	if llmClient == nil {
		return nil, errLLMUnavailable
	}
	meta, err := st.GetPDFIndexMeta(pdfID)
	if err != nil {
		meta = models.EmptyPDFIndexMeta()
	}

	progressGlossaryLLM(pdfID, 0, 1)
	glossary, err := llmClient.ExtractPDFGlossary(ctx, inputs.catalog, inputs.samples)
	if err != nil {
		progressError(pdfID, "Glossary rebuild failed")
		return meta, err
	}
	progressGlossaryLLM(pdfID, 1, 1)

	meta.Glossary = glossary
	finalizeLearningsMeta(meta, inputs.sections)
	if err := st.SavePDFIndexMeta(pdfID, meta); err != nil {
		return meta, err
	}
	progressLearningsPassDone(pdfID, "Glossary rebuild complete")
	return meta, nil
}

// RebuildPDFFeatures runs only the features LLM pass using saved glossary, preserving cheatsheet.
func RebuildPDFFeatures(ctx context.Context, llmClient *llm.Client, st store.Store, inputs learningsRebuildInputs, pdfID string) (*models.PDFIndexMeta, error) {
	if llmClient == nil {
		return nil, errLLMUnavailable
	}
	meta, err := st.GetPDFIndexMeta(pdfID)
	if err != nil {
		meta = models.EmptyPDFIndexMeta()
	}

	progressFeaturesLLM(pdfID, 0, 1)
	features, err := llmClient.DetectPDFFeatures(ctx, inputs.catalog, meta.Glossary, inputs.samples)
	if err != nil {
		progressError(pdfID, "Features rebuild failed")
		return meta, err
	}
	progressFeaturesLLM(pdfID, 1, 1)

	meta.Features = features
	finalizeLearningsMeta(meta, inputs.sections)
	if err := st.SavePDFIndexMeta(pdfID, meta); err != nil {
		return meta, err
	}
	progressLearningsPassDone(pdfID, "Features rebuild complete")
	return meta, nil
}

// RebuildPDFCheatsheet runs the cheatsheet pass for all detected features.
func RebuildPDFCheatsheet(ctx context.Context, searchSvc *search.Service, llmClient *llm.Client, st store.Store, catalog *rpgconcepts.ConceptCatalog, sections []models.TOCSection, pdfID string) (*models.PDFIndexMeta, error) {
	if llmClient == nil || catalog == nil || searchSvc == nil {
		return nil, errLLMUnavailable
	}
	meta, err := st.GetPDFIndexMeta(pdfID)
	if err != nil {
		meta = models.EmptyPDFIndexMeta()
	}
	if len(meta.Features) == 0 {
		return meta, errNoFeaturesForCheatsheet
	}
	if len(llm.BuildSectionSamples(sections)) == 0 {
		return meta, errNoSectionText
	}

	meta.Cheatsheet = nil
	return runCheatsheetRebuild(ctx, searchSvc, llmClient, st, catalog, sections, pdfID, meta, meta.Features, false)
}

// RebuildPDFCheatsheetMissing runs the cheatsheet pass only for features without entries.
func RebuildPDFCheatsheetMissing(ctx context.Context, searchSvc *search.Service, llmClient *llm.Client, st store.Store, catalog *rpgconcepts.ConceptCatalog, sections []models.TOCSection, pdfID string) (*models.PDFIndexMeta, error) {
	if llmClient == nil || catalog == nil || searchSvc == nil {
		return nil, errLLMUnavailable
	}
	meta, err := st.GetPDFIndexMeta(pdfID)
	if err != nil {
		meta = models.EmptyPDFIndexMeta()
	}
	if len(meta.Features) == 0 {
		return meta, errNoFeaturesForCheatsheet
	}
	if len(llm.BuildSectionSamples(sections)) == 0 {
		return meta, errNoSectionText
	}

	pending := pendingCheatsheetFeatures(meta)
	if len(pending) == 0 {
		finalizeLearningsMeta(meta, sections)
		if err := st.SavePDFIndexMeta(pdfID, meta); err != nil {
			return meta, err
		}
		progressLearningsPassDone(pdfID, "Cheatsheet already complete")
		return meta, nil
	}
	return runCheatsheetRebuild(ctx, searchSvc, llmClient, st, catalog, sections, pdfID, meta, pending, false)
}

// RebuildPDFCheatsheetFeature re-runs the cheatsheet pass for one feature, replacing any existing entry.
func RebuildPDFCheatsheetFeature(ctx context.Context, searchSvc *search.Service, llmClient *llm.Client, st store.Store, catalog *rpgconcepts.ConceptCatalog, sections []models.TOCSection, pdfID, featureID string) (*models.PDFIndexMeta, error) {
	if llmClient == nil || catalog == nil || searchSvc == nil {
		return nil, errLLMUnavailable
	}
	featureID = strings.TrimSpace(featureID)
	if featureID == "" {
		return nil, fmt.Errorf("feature_id is required")
	}
	meta, err := st.GetPDFIndexMeta(pdfID)
	if err != nil {
		meta = models.EmptyPDFIndexMeta()
	}
	if !featureAllowedForCheatsheetRebuild(meta, featureID) {
		return nil, fmt.Errorf("feature %q is not in detected features or cheatsheet", featureID)
	}
	if len(llm.BuildSectionSamples(sections)) == 0 {
		return meta, errNoSectionText
	}
	return runCheatsheetRebuild(ctx, searchSvc, llmClient, st, catalog, sections, pdfID, meta, []string{featureID}, true)
}

func runCheatsheetRebuild(
	ctx context.Context,
	searchSvc *search.Service,
	llmClient *llm.Client,
	st store.Store,
	catalog *rpgconcepts.ConceptCatalog,
	sections []models.TOCSection,
	pdfID string,
	meta *models.PDFIndexMeta,
	featureIDs []string,
	force bool,
) (*models.PDFIndexMeta, error) {
	beginCheatsheetProgress(pdfID, len(featureIDs), catalog)

	saveMeta := func(m *models.PDFIndexMeta) error {
		return st.SavePDFIndexMeta(pdfID, m)
	}
	checkpoint := func(m *models.PDFIndexMeta) {
		finalizeLearningsMeta(m, sections)
		if err := saveMeta(m); err != nil {
			log.Printf("index: save cheatsheet checkpoint for pdf %s: %v", pdfID, err)
		}
	}

	progress := cheatsheetProgress{
		onProgress: func(phase string, current, total int, featureID string) {
			if phase == "cheatsheet_llm" {
				progressCheatsheetLLM(pdfID, current, total, featureID, catalog)
			}
		},
		onCheckpoint: checkpoint,
	}

	built, err := rebuildCheatsheetFeatures(ctx, searchSvc, llmClient, catalog, pdfID, sections, meta, featureIDs, progress, force)
	if built == nil {
		built = meta
	}
	finalizeLearningsMeta(built, sections)

	if saveErr := saveMeta(built); saveErr != nil {
		return built, saveErr
	}
	if err != nil {
		progressError(pdfID, "Cheatsheet rebuild failed")
		return built, err
	}
	progressCheatsheetDone(pdfID, len(built.Features))
	return built, nil
}
