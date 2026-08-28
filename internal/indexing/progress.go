package indexing

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

// IndexPhase identifies the current indexing stage.
type IndexPhase string

const (
	IndexPhaseIdle          IndexPhase = "idle"
	IndexPhaseThumbnail     IndexPhase = "thumbnail"
	IndexPhaseSections      IndexPhase = "sections"
	IndexPhaseGlossaryLLM   IndexPhase = "glossary_llm"
	IndexPhaseFeaturesLLM   IndexPhase = "features_llm"
	IndexPhaseCheatsheetLLM IndexPhase = "cheatsheet_llm"
	IndexPhaseFinalize      IndexPhase = "finalize"
	IndexPhaseDone          IndexPhase = "done"
	IndexPhaseError         IndexPhase = "error"
)

// IndexProgress is a snapshot of indexing work for one PDF.
type IndexProgress struct {
	PDFID   string     `json:"pdf_id"`
	Phase   IndexPhase `json:"phase"`
	Current int        `json:"current"`
	Total   int        `json:"total"`
	Percent float64    `json:"percent"`
	Message string     `json:"message"`
	Active  bool       `json:"active"`
}

var progressStore sync.Map // pdfID -> IndexProgress

func setProgress(p IndexProgress) {
	p.Active = p.Phase != IndexPhaseIdle && p.Phase != IndexPhaseDone && p.Phase != IndexPhaseError
	progressStore.Store(p.PDFID, p)
}

func clearProgress(pdfID string) {
	progressStore.Delete(pdfID)
}

// GetIndexProgress returns the latest progress for a PDF, or idle if none.
func GetIndexProgress(pdfID string) IndexProgress {
	if v, ok := progressStore.Load(pdfID); ok {
		return v.(IndexProgress)
	}
	return IndexProgress{PDFID: pdfID, Phase: IndexPhaseIdle, Active: false}
}

func beginProgress(pdfID string, totalSections int) {
	setProgress(IndexProgress{
		PDFID:   pdfID,
		Phase:   IndexPhaseThumbnail,
		Current: 0,
		Total:   totalSections,
		Percent: 2,
		Message: "Rendering thumbnail",
	})
}

func progressThumbnail(pdfID string, total int) {
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseThumbnail, Total: total,
		Percent: 8, Message: "Rendering thumbnail",
	})
}

func progressSection(pdfID string, current, total int, title string) {
	if total <= 0 {
		total = 1
	}
	// Sections span ~8–60% of the bar.
	pct := 8 + (float64(current)/float64(total))*52
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseSections,
		Current: current, Total: total, Percent: pct,
		Message: "Indexing section: " + title,
	})
}

func progressGlossaryLLM(pdfID string, current, total int) {
	if total <= 0 {
		total = 1
	}
	pct := 60 + (float64(current)/float64(total))*10
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseGlossaryLLM,
		Current: current, Total: total, Percent: pct,
		Message: fmt.Sprintf("Extracting glossary (LLM %d/%d)", current, total),
	})
}

func progressFeaturesLLM(pdfID string, current, total int) {
	if total <= 0 {
		total = 1
	}
	pct := 70 + (float64(current)/float64(total))*8
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseFeaturesLLM,
		Current: current, Total: total, Percent: pct,
		Message: fmt.Sprintf("Detecting features (LLM %d/%d)", current, total),
	})
}

func progressCheatsheetLLM(pdfID string, current, total int, featureID string, catalog *rpgconcepts.ConceptCatalog) {
	if total <= 0 {
		total = 1
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	pct := 78 + (float64(current)/float64(total))*17
	msg := cheatsheetProgressMessage(current, total, featureID, catalog)
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseCheatsheetLLM,
		Current: current, Total: total, Percent: pct,
		Message: msg,
	})
}

func cheatsheetProgressMessage(current, total int, featureID string, catalog *rpgconcepts.ConceptCatalog) string {
	if featureID == "" {
		if current >= total && total > 0 {
			return fmt.Sprintf("Cheatsheet complete (%d/%d)", total, total)
		}
		return fmt.Sprintf("Building cheatsheet entries (%d/%d)", current, total)
	}
	label := featureProgressLabel(catalog, featureID)
	return fmt.Sprintf("Building cheatsheet: %s (%d/%d)", label, current, total)
}

func featureProgressLabel(catalog *rpgconcepts.ConceptCatalog, featureID string) string {
	featureID = strings.TrimSpace(featureID)
	if featureID == "" {
		return "feature"
	}
	if catalog != nil {
		if feat, ok := catalog.FeatureByID(featureID); ok {
			if name := strings.TrimSpace(feat.Name); name != "" {
				return name
			}
		}
	}
	return featureID
}

func beginCheatsheetProgress(pdfID string, total int, catalog *rpgconcepts.ConceptCatalog) {
	if total <= 0 {
		total = 1
	}
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseCheatsheetLLM,
		Current: 0, Total: total, Percent: 78,
		Message: cheatsheetProgressMessage(0, total, "", catalog),
	})
}

func progressCheatsheetDone(pdfID string, total int) {
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseDone,
		Current: total, Total: total, Percent: 100,
		Message: "Cheatsheet rebuild complete", Active: false,
	})
}

func progressLearningsPassDone(pdfID string, message string) {
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseDone,
		Percent: 100, Message: message, Active: false,
	})
}

func progressFinalize(pdfID string) {
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseFinalize,
		Percent: 96, Message: "Finishing up",
	})
}

func progressDone(pdfID string, total int) {
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseDone,
		Current: total, Total: total, Percent: 100,
		Message: "Complete", Active: false,
	})
}

func progressError(pdfID string, msg string) {
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseError,
		Percent: 0, Message: msg, Active: false,
	})
}
