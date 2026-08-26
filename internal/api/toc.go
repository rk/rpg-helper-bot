package api

import (
	"net/http"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/indexing"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

type extractTOCRequest struct {
	Source          string `json:"source"` // "pages" (default) or "bookmarks"
	StartPage       int    `json:"start_page,omitempty"`
	EndPage         int    `json:"end_page,omitempty"`
	IncludeChildren *bool  `json:"include_children,omitempty"`
}

type extractTOCResponse struct {
	Sections  []models.TOCSection `json:"sections"`
	PageCount int                 `json:"page_count"`
	Source    string              `json:"source"`
}

func (s *Server) registerTOCRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/pdfs/{id}/toc/extract", s.handleExtractTOC)
}

func (s *Server) handleExtractTOC(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pdf, err := s.Store.GetPDF(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if store.ProbePath(pdf.FilePath) != models.PathStatusOK {
		writeError(w, http.StatusBadRequest, errPDFPathMissing())
		return
	}

	var req extractTOCRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	source := strings.TrimSpace(strings.ToLower(req.Source))
	if source == "" {
		source = "pages"
	}

	var sections []models.TOCSection
	var pageCount int

	switch source {
	case "bookmarks":
		includeChildren := true
		if req.IncludeChildren != nil {
			includeChildren = *req.IncludeChildren
		}
		sections, pageCount, err = indexing.ExtractBookmarkSections(pdf.FilePath, includeChildren)
	case "pages":
		if err := indexing.ToolsAvailable(); err != nil {
			writeError(w, http.StatusServiceUnavailable, err)
			return
		}
		sections, pageCount, err = indexing.ExtractTOCSections(pdf.FilePath, req.StartPage, req.EndPage)
	default:
		writeError(w, http.StatusBadRequest, errInvalidTOCSource())
		return
	}

	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for i := range sections {
		sections[i].PDFID = id
	}

	if pageCount > 0 && pdf.PageCount != pageCount {
		pdf.PageCount = pageCount
		_ = s.Store.SavePDF(pdf)
	}

	writeJSON(w, http.StatusOK, extractTOCResponse{
		Sections:  sections,
		PageCount: pageCount,
		Source:    source,
	})
}

func (s *Server) applyImportedTOC(pdfID, filePath, source string, startPage, endPage int, includeChildren bool) (int, error) {
	var (
		sections  []models.TOCSection
		pageCount int
		err       error
	)

	switch source {
	case "bookmarks":
		sections, pageCount, err = indexing.ExtractBookmarkSections(filePath, includeChildren)
	case "pages":
		if err := indexing.ToolsAvailable(); err != nil {
			return 0, err
		}
		sections, pageCount, err = indexing.ExtractTOCSections(filePath, startPage, endPage)
	default:
		return 0, errInvalidTOCSource()
	}
	if err != nil {
		return 0, err
	}
	for i := range sections {
		sections[i].PDFID = pdfID
	}
	if err := s.Store.SaveTOCSections(pdfID, sections); err != nil {
		return 0, err
	}
	if pageCount > 0 {
		pdf, err := s.Store.GetPDF(pdfID)
		if err != nil {
			return pageCount, nil
		}
		pdf.PageCount = pageCount
		_ = s.Store.SavePDF(pdf)
	}
	return pageCount, nil
}

func errInvalidTOCSource() error {
	return &apiError{msg: `toc source must be "pages" or "bookmarks"`}
}

func errPDFPathMissing() error {
	return &apiError{msg: "pdf file path is missing or unreachable"}
}
