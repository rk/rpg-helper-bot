package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

const maxPDFUploadBytes = 200 << 20 // 200 MiB

type pdfImportOptions struct {
	TOCSource          string
	TOCStartPage       int
	TOCEndPage         int
	TOCIncludeChildren bool
}

func (s *Server) handleUploadPDF(w http.ResponseWriter, r *http.Request) {
	if s.DataDir == "" {
		writeError(w, http.StatusServiceUnavailable, errUploadUnavailable())
		return
	}
	if err := r.ParseMultipartForm(maxPDFUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid upload: %w", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("pdf file is required"))
		return
	}
	defer file.Close()

	if !isPDFFilename(header.Filename) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("only PDF files are supported"))
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = titleFromFilename(header.Filename)
	}
	if title == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("pdf title is required"))
		return
	}

	pdfsDir := filepath.Join(s.DataDir, "pdfs")
	if err := os.MkdirAll(pdfsDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	pdfID := uuid.NewString()
	destPath := filepath.Join(pdfsDir, pdfID+".pdf")
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	written, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(destPath)
		writeError(w, http.StatusBadRequest, copyErr)
		return
	}
	if closeErr != nil {
		_ = os.Remove(destPath)
		writeError(w, http.StatusInternalServerError, closeErr)
		return
	}
	if written == 0 {
		_ = os.Remove(destPath)
		writeError(w, http.StatusBadRequest, fmt.Errorf("uploaded file is empty"))
		return
	}

	p := &models.PDF{ID: pdfID, Title: title, FilePath: destPath}
	if err := s.Store.SavePDF(p); err != nil {
		_ = os.Remove(destPath)
		writeError(w, http.StatusBadRequest, err)
		return
	}

	opts := pdfImportOptions{
		TOCSource:          strings.TrimSpace(strings.ToLower(r.FormValue("toc_source"))),
		TOCStartPage:       formInt(r, "toc_start_page"),
		TOCEndPage:         formInt(r, "toc_end_page"),
		TOCIncludeChildren: formBoolDefault(r, "toc_include_children", true),
	}
	s.respondCreatePDF(w, p, opts)
}

func (s *Server) respondCreatePDF(w http.ResponseWriter, p *models.PDF, opts pdfImportOptions) {
	resp := createPDFResponse{}
	var tocErr error

	tocSource := opts.TOCSource
	if tocSource == "" && opts.TOCStartPage > 0 && opts.TOCEndPage > 0 {
		tocSource = "pages"
	}
	if tocSource != "" {
		if store.ProbePath(p.FilePath) != models.PathStatusOK {
			tocErr = errPDFPathMissing()
		} else {
			_, tocErr = s.applyImportedTOC(p.ID, p.FilePath, tocSource, opts.TOCStartPage, opts.TOCEndPage, opts.TOCIncludeChildren)
		}
	}

	summary, err := s.enrichPDF(*p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if tocErr == nil && tocSource != "" {
		if refreshed, err := s.Store.GetPDF(p.ID); err == nil {
			summary, _ = s.enrichPDF(*refreshed)
		}
	}
	resp.PDFSummary = summary
	if tocErr != nil {
		resp.TOCExtractError = tocErr.Error()
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) removeManagedPDF(path string) {
	if s.DataDir == "" || strings.TrimSpace(path) == "" {
		return
	}
	pdfsDir, err := filepath.Abs(filepath.Join(s.DataDir, "pdfs"))
	if err != nil {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	if !strings.HasPrefix(abs, pdfsDir+string(os.PathSeparator)) {
		return
	}
	_ = os.Remove(abs)
}

func isPDFFilename(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".pdf")
}

func titleFromFilename(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	if base == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
}

func formInt(r *http.Request, key string) int {
	v, err := strconv.Atoi(strings.TrimSpace(r.FormValue(key)))
	if err != nil {
		return 0
	}
	return v
}

func formBoolDefault(r *http.Request, key string, fallback bool) bool {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func errUploadUnavailable() error {
	return &apiError{msg: "pdf upload is unavailable"}
}
