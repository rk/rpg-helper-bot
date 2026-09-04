package api

import (
	"encoding/json"
	"net/http"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

func (s *Server) registerIndexMetaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/pdfs/{id}/index-meta", s.handleGetIndexMeta)
	mux.HandleFunc("PUT /api/pdfs/{id}/index-meta", s.handlePutIndexMeta)
	mux.HandleFunc("POST /api/pdfs/{id}/index-meta/rebuild-glossary", s.handleRebuildGlossary)
	mux.HandleFunc("POST /api/pdfs/{id}/index-meta/rebuild-features", s.handleRebuildFeatures)
	mux.HandleFunc("POST /api/pdfs/{id}/index-meta/rebuild-cheatsheet", s.handleRebuildCheatsheet)
	mux.HandleFunc("POST /api/pdfs/{id}/index-meta/rebuild-cheatsheet-missing", s.handleRebuildCheatsheetMissing)
	mux.HandleFunc("POST /api/pdfs/{id}/index-meta/rebuild-cheatsheet/{featureId}", s.handleRebuildCheatsheetFeature)
}

func (s *Server) handleGetIndexMeta(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	meta, err := s.Store.GetPDFIndexMeta(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handlePutIndexMeta(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Store.GetPDF(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	var meta models.PDFIndexMeta
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	models.NormalizeIndexMeta(&meta)
	if err := s.Store.SavePDFIndexMeta(id, &meta); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleRebuildCheatsheet(w http.ResponseWriter, r *http.Request) {
	if s.Indexer == nil {
		writeError(w, http.StatusServiceUnavailable, errServiceUnavailable("indexing"))
		return
	}
	id := r.PathValue("id")
	if _, err := s.Store.GetPDF(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	meta, err := s.Indexer.RebuildCheatsheet(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleRebuildCheatsheetMissing(w http.ResponseWriter, r *http.Request) {
	if s.Indexer == nil {
		writeError(w, http.StatusServiceUnavailable, errServiceUnavailable("indexing"))
		return
	}
	id := r.PathValue("id")
	if _, err := s.Store.GetPDF(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	meta, err := s.Indexer.RebuildCheatsheetMissing(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleRebuildCheatsheetFeature(w http.ResponseWriter, r *http.Request) {
	if s.Indexer == nil {
		writeError(w, http.StatusServiceUnavailable, errServiceUnavailable("indexing"))
		return
	}
	id := r.PathValue("id")
	featureID := r.PathValue("featureId")
	if _, err := s.Store.GetPDF(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	meta, err := s.Indexer.RebuildCheatsheetFeature(r.Context(), id, featureID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleRebuildGlossary(w http.ResponseWriter, r *http.Request) {
	if s.Indexer == nil {
		writeError(w, http.StatusServiceUnavailable, errServiceUnavailable("indexing"))
		return
	}
	id := r.PathValue("id")
	if _, err := s.Store.GetPDF(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	meta, err := s.Indexer.RebuildGlossary(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleRebuildFeatures(w http.ResponseWriter, r *http.Request) {
	if s.Indexer == nil {
		writeError(w, http.StatusServiceUnavailable, errServiceUnavailable("indexing"))
		return
	}
	id := r.PathValue("id")
	if _, err := s.Store.GetPDF(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	meta, err := s.Indexer.RebuildFeatures(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

