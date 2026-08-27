package search

import (
	"context"
	"log"
	"sort"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/embed"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
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
	ConceptsPath string
}

type Result struct {
	Hits  []models.SearchHit
	Debug models.ChatSearchDebug
}

func (s *Service) Search(ctx context.Context, gameID, query string) (*Result, error) {
	query = strings.TrimSpace(query)
	debug := models.ChatSearchDebug{OriginalQuery: query, FTSQuery: query}
	if query == "" {
		return &Result{Debug: debug}, nil
	}

	gamePDFs, err := s.Store.ListGamePDFs(gameID)
	if err != nil {
		return nil, err
	}
	if len(gamePDFs) == 0 {
		return &Result{Debug: debug}, nil
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
		rewritten, rewriteErr := s.RewriteQuery(ctx, query)
		if rewriteErr != nil {
			debug.RewriteError = rewriteErr.Error()
		} else if rewritten = strings.TrimSpace(rewritten); rewritten != "" {
			ftsQuery = rewritten
			debug.QueryRewritten = ftsQuery != query
			if debug.QueryRewritten {
				log.Printf("search: FTS query rewritten %q -> %q", query, ftsQuery)
			}
		}
	}
	debug.FTSQuery = ftsQuery

	catalog := s.loadConceptCatalog()
	if catalog != nil {
		metaByPDF, err := s.Store.ListPDFIndexMeta(pdfIDs)
		if err != nil {
			return nil, err
		}
		expanded := ExpandFTSWithGlossary(query, ftsQuery, metaByPDF, catalog)
		if expanded != ftsQuery {
			debug.QueryRewritten = true
			ftsQuery = expanded
			debug.FTSQuery = ftsQuery
		}
	}

	candidates, err := s.Store.FTSSearch(pdfIDs, ftsQuery, ftsCandidateLimit)
	if err != nil {
		return nil, err
	}
	debug.FTSCandidateCount = len(candidates)
	if len(candidates) == 0 {
		return &Result{Debug: debug}, nil
	}

	queryVec, err := s.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	hits := make([]models.SearchHit, 0, len(candidates))
	type scoredHit struct {
		hit      models.SearchHit
		adjScore float64
	}
	scored := make([]scoredHit, 0, len(candidates))
	for _, c := range candidates {
		docVec, err := s.Embed(ctx, c.Title+"\n"+c.PlainText)
		if err != nil {
			continue
		}
		score := embed.Cosine(queryVec, docVec)
		snippet := snippet(c.PlainText, query, 240)
		h := models.SearchHit{
			SectionID:    c.SectionID,
			PDFID:        c.PDFID,
			PDFTitle:     pdfTitle[c.PDFID],
			SectionTitle: c.Title,
			StartPage:    c.StartPage,
			EndPage:      c.EndPage,
			PDFSortOrder: pdfOrder[c.PDFID],
			Score:        score,
			FTSRank:      c.Rank,
			Snippet:      snippet,
		}
		scored = append(scored, scoredHit{
			hit:      h,
			adjScore: adjustedScore(score, c.Title, c.PlainText, query),
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].adjScore != scored[j].adjScore {
			return scored[i].adjScore > scored[j].adjScore
		}
		// Later PDFs (higher sort_order) override on tie/conflict.
		return scored[i].hit.PDFSortOrder > scored[j].hit.PDFSortOrder
	})

	for _, sh := range scored {
		hits = append(hits, sh.hit)
	}

	if len(hits) > finalResultLimit {
		hits = hits[:finalResultLimit]
	}

	debug.Hits = make([]models.ChatSearchHit, len(hits))
	for i, h := range hits {
		debug.Hits[i] = models.ChatSearchHit{
			Rank:         i + 1,
			SectionID:    h.SectionID,
			PDFTitle:     h.PDFTitle,
			SectionTitle: h.SectionTitle,
			StartPage:    h.StartPage,
			EndPage:      h.EndPage,
			PDFSortOrder: h.PDFSortOrder,
			EmbedScore:   h.Score,
			FTSRank:      h.FTSRank,
			Snippet:      textutil.NormalizePDFText(h.Snippet),
		}
	}

	return &Result{Hits: hits, Debug: debug}, nil
}

func (s *Service) loadConceptCatalog() *rpgconcepts.ConceptCatalog {
	path := s.ConceptsPath
	if path == "" {
		path = rpgconcepts.DefaultConceptsPath()
	}
	catalog, err := rpgconcepts.LoadConcepts(path)
	if err != nil {
		return nil
	}
	return catalog
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