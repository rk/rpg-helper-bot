package search

import (
	"context"
	"log"
	"sort"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/embed"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/textutil"
)

const (
	ftsCandidateLimit = 100
	finalResultLimit  = 10
)

type QueryRewriteFunc func(ctx context.Context, query string) (string, error)

type Service struct {
	Store        store.Store
	Embed        embed.Func
	RewriteQuery QueryRewriteFunc
}

func (s *Service) Search(ctx context.Context, gameID, query string) ([]models.SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	gamePDFs, err := s.Store.ListGamePDFs(gameID)
	if err != nil {
		return nil, err
	}
	if len(gamePDFs) == 0 {
		return nil, nil
	}

	pdfOrder := map[string]int{}
	pdfTitle := map[string]string{}
	pdfIDs := make([]string, 0, len(gamePDFs))
	for _, p := range gamePDFs {
		pdfOrder[p.ID] = p.SortOrder
		pdfTitle[p.ID] = p.Title
		pdfIDs = append(pdfIDs, p.ID)
	}

	ftsQuery := query
	if s.RewriteQuery != nil {
		if rewritten, err := s.RewriteQuery(ctx, query); err == nil {
			if rewritten = strings.TrimSpace(rewritten); rewritten != "" {
				ftsQuery = rewritten
				if ftsQuery != query {
					log.Printf("search: FTS query rewritten %q -> %q", query, ftsQuery)
				}
			}
		}
	}

	candidates, err := s.Store.FTSSearch(pdfIDs, ftsQuery, ftsCandidateLimit)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	queryVec, err := s.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	hits := make([]models.SearchHit, 0, len(candidates))
	for _, c := range candidates {
		docVec, err := s.Embed(ctx, c.Title+"\n"+c.PlainText)
		if err != nil {
			continue
		}
		score := embed.Cosine(queryVec, docVec)
		snippet := snippet(c.PlainText, query, 240)
		hits = append(hits, models.SearchHit{
			SectionID:    c.SectionID,
			PDFID:        c.PDFID,
			PDFTitle:     pdfTitle[c.PDFID],
			SectionTitle: c.Title,
			StartPage:    c.StartPage,
			EndPage:      c.EndPage,
			PDFSortOrder: pdfOrder[c.PDFID],
			Score:        score,
			Snippet:      snippet,
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		// Later PDFs (higher sort_order) override on tie/conflict.
		return hits[i].PDFSortOrder > hits[j].PDFSortOrder
	})

	if len(hits) > finalResultLimit {
		hits = hits[:finalResultLimit]
	}
	return hits, nil
}

func snippet(text, query string, maxLen int) string {
	text = textutil.NormalizePDFText(text)
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= maxLen {
		return text
	}
	lower := strings.ToLower(text)
	q := strings.ToLower(strings.TrimSpace(query))
	if idx := strings.Index(lower, q); idx >= 0 {
		start := idx - 40
		if start < 0 {
			start = 0
		}
		end := start + maxLen
		if end > len(text) {
			end = len(text)
		}
		return strings.TrimSpace(text[start:end]) + "…"
	}
	return text[:maxLen] + "…"
}
