package indexing

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/vectors"
)

const thumbnailMaxPx = 200

type Service struct {
	Store   store.Store
	Vectors *vectors.Store
	DataDir string
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

	pageCount, err := PageCount(pdf.FilePath)
	if err == nil && pageCount > 0 {
		pdf.PageCount = pageCount
		_ = s.Store.SavePDF(pdf)
	}

	thumbPath := filepath.Join(s.DataDir, "thumbnails", pdfID+".png")
	if err := RenderThumbnail(pdf.FilePath, thumbPath, thumbnailMaxPx); err != nil {
		_ = s.Store.SetPDFIndexStatus(pdfID, models.IndexStatusError, nil)
		return nil, fmt.Errorf("thumbnail: %w", err)
	}

	indexed := 0
	for _, sec := range sections {
		text, err := ExtractPages(pdf.FilePath, sec.StartPage, sec.EndPage)
		if err != nil {
			_ = s.Store.SetPDFIndexStatus(pdfID, models.IndexStatusError, nil)
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

	now := time.Now().UTC()
	if err := s.Store.SetPDFThumbnail(pdfID, thumbPath); err != nil {
		return nil, err
	}
	if err := s.Store.SetPDFIndexStatus(pdfID, models.IndexStatusIndexed, &now); err != nil {
		return nil, err
	}

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
