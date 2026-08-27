package indexing

import (
	"sync"
)

// IndexPhase identifies the current indexing stage.
type IndexPhase string

const (
	IndexPhaseIdle      IndexPhase = "idle"
	IndexPhaseThumbnail IndexPhase = "thumbnail"
	IndexPhaseSections  IndexPhase = "sections"
	IndexPhaseGlossary  IndexPhase = "glossary"
	IndexPhaseFinalize  IndexPhase = "finalize"
	IndexPhaseDone      IndexPhase = "done"
	IndexPhaseError     IndexPhase = "error"
)

// IndexProgress is a snapshot of indexing work for one PDF.
type IndexProgress struct {
	PDFID    string     `json:"pdf_id"`
	Phase    IndexPhase `json:"phase"`
	Current  int        `json:"current"`
	Total    int        `json:"total"`
	Percent  float64    `json:"percent"`
	Message  string     `json:"message"`
	Active   bool       `json:"active"`
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
	// Sections span ~8–82% of the bar.
	pct := 8 + (float64(current)/float64(total))*74
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseSections,
		Current: current, Total: total, Percent: pct,
		Message: "Indexing section: " + title,
	})
}

func progressGlossary(pdfID string) {
	setProgress(IndexProgress{
		PDFID: pdfID, Phase: IndexPhaseGlossary,
		Percent: 88, Message: "Building glossary",
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
