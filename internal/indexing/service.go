package indexing

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/vectors"
)

const thumbnailMaxPx = 200

type Service struct {
	Store   store.Store
	Vectors *vectors.Store
	DataDir string
	LLM     *llm.Client
	ConceptsPath string
}

type Result struct {
	PDFID         string            `json:"pdf_id"`
	IndexStatus   models.IndexStatus `json:"index_status"`
	PageCount     int               `json:"page_count"`
	ThumbnailPath string            `json:"thumbnail_path,omitempty"`
	SectionsIndexed int             `json:"sections_indexed"`
}

func (s *Service) IndexPDF(ctx context.Context, pdfID string) (*Result, error) {
	if err := ToolsAvailable(); err != nil {
		return nil, err
	}

	pdf, err := s.Store.GetPDF(pdfID)
	if err != nil {
		return nil, err
	}
	if store.ProbePath(pdf.FilePath) != models.PathStatusOK {
		return nil, fmt.Errorf("pdf path is missing: %s", pdf.FilePath)
	}

	_ = s.Store.SetPDFIndexStatus(pdfID, models.IndexStatusIndexing, nil)

	sections, err := s.Store.ListTOCSections(pdfID)
	if err != nil {
		return nil, err
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("add TOC sections before indexing")
	}

	beginProgress(pdfID, len(sections))

	pageCount, err := PageCount(pdf.FilePath)
	if err == nil && pageCount > 0 {
		pdf.PageCount = pageCount
		_ = s.Store.SavePDF(pdf)
	}

	thumbPath := filepath.Join(s.DataDir, "thumbnails", pdfID+".png")
	progressThumbnail(pdfID, len(sections))
	if err := RenderThumbnail(pdf.FilePath, thumbPath, thumbnailMaxPx); err != nil {
		_ = s.Store.SetPDFIndexStatus(pdfID, models.IndexStatusError, nil)
		progressError(pdfID, "thumbnail failed")
		return nil, fmt.Errorf("thumbnail: %w", err)
	}

	splitTexts, err := s.buildSplitPageTexts(pdf.FilePath, sections)
	if err != nil {
		_ = s.Store.SetPDFIndexStatus(pdfID, models.IndexStatusError, nil)
		return nil, err
	}

	indexed := 0
	for _, sec := range sections {
		progressSection(pdfID, indexed+1, len(sections), sec.Title)
		text, err := sectionPlainText(pdf.FilePath, sec, splitTexts)
		if err != nil {
			_ = s.Store.SetPDFIndexStatus(pdfID, models.IndexStatusError, nil)
			progressError(pdfID, sec.Title)
			return nil, fmt.Errorf("section %q: %w", sec.Title, err)
		}
		if err := s.Store.SaveSectionText(sec.ID, text); err != nil {
			return nil, err
		}
		if s.Vectors != nil {
			if err := s.Vectors.UpsertSection(ctx, sec.ID, pdfID, sec.Title, text); err != nil {
				return nil, fmt.Errorf("vector index section %q: %w", sec.Title, err)
			}
		}
		indexed++
	}

	sections, err = s.Store.ListTOCSections(pdfID)
	if err != nil {
		return nil, err
	}
	conceptPath := s.ConceptsPath
	if conceptPath == "" {
		conceptPath = rpgconcepts.DefaultConceptsPath()
	}
	if catalog, err := rpgconcepts.LoadConcepts(conceptPath); err != nil {
		log.Printf("index: concepts load skipped: %v", err)
	} else {
		saveMeta := func(m *models.PDFIndexMeta) error {
			return s.Store.SavePDFIndexMeta(pdfID, m)
		}
		meta := BuildPDFLearningsLLM(ctx, s.LLM, catalog, sections, pdfID, saveMeta)
		if err := s.Store.SavePDFIndexMeta(pdfID, meta); err != nil {
			return nil, fmt.Errorf("save index meta: %w", err)
		}
	}

	now := time.Now().UTC()
	progressFinalize(pdfID)
	if err := s.Store.SetPDFThumbnail(pdfID, thumbPath); err != nil {
		return nil, err
	}
	if err := s.Store.SetPDFIndexStatus(pdfID, models.IndexStatusIndexed, &now); err != nil {
		return nil, err
	}
	progressDone(pdfID, indexed)

	return &Result{
		PDFID:           pdfID,
		IndexStatus:     models.IndexStatusIndexed,
		PageCount:       pdf.PageCount,
		ThumbnailPath:   thumbPath,
		SectionsIndexed: indexed,
	}, nil
}

func (s *Service) RebuildVectors(ctx context.Context) error {
	if s.Vectors == nil {
		return nil
	}
	sections, err := s.Store.ListIndexedSections()
	if err != nil {
		return err
	}
	for _, sec := range sections {
		if err := s.Vectors.UpsertSection(ctx, sec.ID, sec.PDFID, sec.Title, sec.PlainText); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) buildSplitPageTexts(pdfPath string, sections []models.TOCSection) (map[string]map[string]string, error) {
	groups := SamePageSectionGroups(sections)
	if len(groups) == 0 {
		return nil, nil
	}

	out := make(map[string]map[string]string, len(groups))
	for key, group := range groups {
		fullText, err := ExtractPages(pdfPath, key.start, key.end)
		if err != nil {
			return nil, err
		}
		titles := make([]string, len(group))
		for i, sec := range group {
			titles[i] = sec.Title
		}
		out[pageRangeKey(key)] = SplitPageTextByHeadings(fullText, titles)
	}
	return out, nil
}

func sectionPlainText(pdfPath string, sec models.TOCSection, splitTexts map[string]map[string]string) (string, error) {
	if sec.StartPage == sec.EndPage && splitTexts != nil {
		if byTitle, ok := splitTexts[pageRangeKey(pageRange{sec.StartPage, sec.EndPage})]; ok {
			if text, ok := byTitle[sec.Title]; ok && text != "" {
				return text, nil
			}
		}
	}
	return ExtractPages(pdfPath, sec.StartPage, sec.EndPage)
}

func (s *Service) RebuildCheatsheet(ctx context.Context, pdfID string) (*models.PDFIndexMeta, error) {
	pdf, err := s.Store.GetPDF(pdfID)
	if err != nil {
		return nil, err
	}
	if pdf.IndexStatus != models.IndexStatusIndexed {
		return nil, fmt.Errorf("pdf must be indexed before rebuilding cheatsheet")
	}

	sections, err := s.Store.ListTOCSections(pdfID)
	if err != nil {
		return nil, err
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("add TOC sections before rebuilding cheatsheet")
	}

	conceptPath := s.ConceptsPath
	if conceptPath == "" {
		conceptPath = rpgconcepts.DefaultConceptsPath()
	}
	catalog, err := rpgconcepts.LoadConcepts(conceptPath)
	if err != nil {
		return nil, fmt.Errorf("concepts: %w", err)
	}

	return RebuildPDFCheatsheet(ctx, s.LLM, s.Store, catalog, sections, pdfID)
}

func pageRangeKey(key pageRange) string {
	return fmt.Sprintf("%d-%d", key.start, key.end)
}
