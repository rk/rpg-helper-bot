package search

import (
	"context"
	"log"
	"sort"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/textutil"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/vectors"
)

const (
	vectorCandidateLimit = 100
	finalResultLimit     = 10
)

type QueryRewriteFunc func(ctx context.Context, query, glossary string) (string, error)

type Service struct {
	Store        store.Store
	Vectors      *vectors.Store
	RewriteQuery QueryRewriteFunc
	ConceptsPath string
}

type Result struct {
	Hits  []models.SearchHit
	Debug models.ChatSearchDebug
}

type searchScope struct {
	pdfIDs    []string
	pdfOrder  map[string]int
	pdfTitle  map[string]string
	metaByPDF map[string]models.PDFIndexMeta
}

type scopedSearchOpts struct {
	rewriteQuery bool
}

func (s *Service) Search(ctx context.Context, gameID, query string) (*Result, error) {
	return s.SearchWithLimit(ctx, gameID, query, finalResultLimit)
}

// SearchWithLimit runs vector search and returns at most limit hits.
func (s *Service) SearchWithLimit(ctx context.Context, gameID, query string, limit int) (*Result, error) {
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

	catalog := s.loadConceptCatalog()
	var metaByPDF map[string]models.PDFIndexMeta
	if catalog != nil {
		metaByPDF, err = s.Store.ListPDFIndexMeta(pdfIDs)
		if err != nil {
			return nil, err
		}
	}

	citedSections := map[string]struct{}{}
	if catalog != nil && metaByPDF != nil {
		citedSections = CitationSectionIDs(query, metaByPDF, catalog)
	}

	return s.searchScoped(ctx, query, searchScope{
		pdfIDs:    pdfIDs,
		pdfOrder:  pdfOrder,
		pdfTitle:  pdfTitle,
		metaByPDF: metaByPDF,
	}, scopedSearchOpts{rewriteQuery: true}, citedSections, limit)
}

// SearchPDF runs vector search scoped to a single indexed PDF.
func (s *Service) SearchPDF(ctx context.Context, pdfID, query string, meta *models.PDFIndexMeta) (*Result, error) {
	query = strings.TrimSpace(query)
	debug := models.ChatSearchDebug{OriginalQuery: query, FTSQuery: query}
	if query == "" {
		return &Result{Debug: debug}, nil
	}

	pdf, err := s.Store.GetPDF(pdfID)
	if err != nil {
		return nil, err
	}

	metaByPDF := map[string]models.PDFIndexMeta{}
	if meta != nil {
		metaByPDF[pdfID] = *meta
	}

	catalog := s.loadConceptCatalog()
	citedSections := map[string]struct{}{}
	if catalog != nil && meta != nil {
		citedSections = CitationSectionIDs(query, metaByPDF, catalog)
	}

	return s.searchScoped(ctx, query, searchScope{
		pdfIDs:    []string{pdfID},
		pdfOrder:  map[string]int{pdfID: 0},
		pdfTitle:  map[string]string{pdfID: pdf.Title},
		metaByPDF: metaByPDF,
	}, scopedSearchOpts{rewriteQuery: false}, citedSections, finalResultLimit)
}

func (s *Service) searchScoped(ctx context.Context, query string, scope searchScope, opts scopedSearchOpts, citedSections map[string]struct{}, resultLimit int) (*Result, error) {
	debug := models.ChatSearchDebug{OriginalQuery: query, FTSQuery: query}
	if query == "" || len(scope.pdfIDs) == 0 {
		return &Result{Debug: debug}, nil
	}
	if s.Vectors == nil {
		return &Result{Debug: debug}, nil
	}

	searchQuery := query
	if opts.rewriteQuery && s.RewriteQuery != nil {
		catalog := s.loadConceptCatalog()
		glossaryBlock := ""
		if catalog != nil && len(scope.metaByPDF) > 0 {
			glossaryBlock = FormatGlossaryForSearchRewrite(scope.metaByPDF, scope.pdfTitle, catalog, query)
		}
		rewritten, rewriteErr := s.RewriteQuery(ctx, query, glossaryBlock)
		if rewriteErr != nil {
			debug.RewriteError = rewriteErr.Error()
		} else if rewritten = strings.TrimSpace(rewritten); rewritten != "" && rewritten != query {
			searchQuery = rewritten
			debug.QueryRewritten = true
			debug.FTSQuery = searchQuery
			log.Printf("search: query expanded %q -> %q", query, searchQuery)
		}
	}

	candidates, err := s.Vectors.QueryScoped(ctx, searchQuery, scope.pdfIDs, vectorCandidateLimit)
	if err != nil {
		return nil, err
	}
	debug.FTSCandidateCount = len(candidates)
	if len(candidates) == 0 {
		return &Result{Debug: debug}, nil
	}

	type scoredHit struct {
		hit      models.SearchHit
		adjScore float64
	}
	scored := make([]scoredHit, 0, len(candidates))
	for _, c := range candidates {
		snippet := snippet(c.PlainText, query, 240)
		h := models.SearchHit{
			SectionID:    c.SectionID,
			PDFID:        c.PDFID,
			PDFTitle:     scope.pdfTitle[c.PDFID],
			SectionTitle: c.Title,
			StartPage:    c.StartPage,
			EndPage:      c.EndPage,
			PDFSortOrder: scope.pdfOrder[c.PDFID],
			Score:        c.Score,
			Snippet:      snippet,
		}
		scored = append(scored, scoredHit{
			hit:      h,
			adjScore: adjustedScore(c.Score, c.SectionID, c.Title, c.PlainText, query, citedSections),
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].adjScore != scored[j].adjScore {
			return scored[i].adjScore > scored[j].adjScore
		}
		return scored[i].hit.PDFSortOrder > scored[j].hit.PDFSortOrder
	})

	hits := make([]models.SearchHit, 0, len(scored))
	for _, sh := range scored {
		hits = append(hits, sh.hit)
	}

	if resultLimit <= 0 {
		resultLimit = finalResultLimit
	}
	if len(hits) > resultLimit {
		hits = hits[:resultLimit]
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
			Snippet:      textutil.NormalizePDFText(h.Snippet),
		}
	}

	return &Result{Hits: hits, Debug: debug}, nil
}

// MergeSearchHits combines hit lists, keeping the first occurrence of each section and truncating to limit.
func MergeSearchHits(primary, secondary []models.SearchHit, limit int) []models.SearchHit {
	seen := map[string]struct{}{}
	var out []models.SearchHit
	appendHits := func(list []models.SearchHit) {
		for _, h := range list {
			if h.SectionID == "" {
				continue
			}
			if _, ok := seen[h.SectionID]; ok {
				continue
			}
			seen[h.SectionID] = struct{}{}
			out = append(out, h)
			if limit > 0 && len(out) >= limit {
				return
			}
		}
	}
	appendHits(primary)
	if limit == 0 || len(out) < limit {
		appendHits(secondary)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
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
