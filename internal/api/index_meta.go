package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

func (s *Server) registerIndexMetaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/pdfs/{id}/index-meta", s.handleGetIndexMeta)
	mux.HandleFunc("PUT /api/pdfs/{id}/index-meta", s.handlePutIndexMeta)
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
	normalizeIndexMeta(&meta)
	if err := s.Store.SavePDFIndexMeta(id, &meta); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func normalizeIndexMeta(meta *models.PDFIndexMeta) {
	if meta.Features == nil {
		meta.Features = []models.PDFFeature{}
	}
	if meta.Glossary == nil {
		meta.Glossary = []models.PDFGlossary{}
	}
	if meta.Cheatsheet == nil {
		meta.Cheatsheet = []models.CheatsheetEntry{}
	}

	glossary := meta.Glossary[:0]
	for _, g := range meta.Glossary {
		g.FeatureID = strings.TrimSpace(g.FeatureID)
		g.PDFTerm = strings.TrimSpace(g.PDFTerm)
		g.Evidence = strings.TrimSpace(g.Evidence)
		if g.FeatureID == "" || g.PDFTerm == "" {
			continue
		}
		glossary = append(glossary, g)
	}
	meta.Glossary = glossary

	features := meta.Features[:0]
	for _, f := range meta.Features {
		f.FeatureID = strings.TrimSpace(f.FeatureID)
		if f.FeatureID == "" {
			continue
		}
		if f.Sections == nil {
			f.Sections = []string{}
		}
		if f.Terms == nil {
			f.Terms = []string{}
		}
		features = append(features, f)
	}
	meta.Features = features

	cheatsheet := meta.Cheatsheet[:0]
	for _, c := range meta.Cheatsheet {
		c.FeatureID = strings.TrimSpace(c.FeatureID)
		c.FeatureName = strings.TrimSpace(c.FeatureName)
		c.Section = strings.TrimSpace(c.Section)
		if c.FeatureID == "" {
			continue
		}
		if c.PDFTerms == nil {
			c.PDFTerms = []string{}
		}
		cheatsheet = append(cheatsheet, c)
	}
	meta.Cheatsheet = cheatsheet
}
