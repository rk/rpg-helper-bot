package vectors

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/philippgille/chromem-go"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/embed"
)

const collectionName = "toc_sections"

type Candidate struct {
	SectionID string
	PDFID     string
	Title     string
	StartPage int
	EndPage   int
	PlainText string
	Score     float64
}

type Store struct {
	mu         sync.RWMutex
	db         *chromem.DB
	collection *chromem.Collection
	embed      embed.Func
}

func New(embedding embed.Func) (*Store, error) {
	db := chromem.NewDB()
	collection, err := db.GetOrCreateCollection(collectionName, nil, chromem.EmbeddingFunc(embedding))
	if err != nil {
		return nil, err
	}
	return &Store{db: db, collection: collection, embed: embedding}, nil
}

func (s *Store) UpsertSection(ctx context.Context, sectionID, pdfID, title, plainText string, startPage, endPage int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	content := title + "\n\n" + plainText
	meta := map[string]string{
		"pdf_id":     pdfID,
		"title":      title,
		"start_page": strconv.Itoa(startPage),
		"end_page":   strconv.Itoa(endPage),
	}
	_ = s.collection.Delete(ctx, nil, nil, sectionID)
	return s.collection.AddDocuments(ctx, []chromem.Document{{
		ID:       sectionID,
		Content:  content,
		Metadata: meta,
	}}, 1)
}

func (s *Store) DeleteSection(ctx context.Context, sectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.collection.Delete(ctx, nil, nil, sectionID)
}

func (s *Store) DeletePDFSections(ctx context.Context, sectionIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range sectionIDs {
		if err := s.collection.Delete(ctx, nil, nil, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Query(ctx context.Context, query string, n int) ([]chromem.Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n <= 0 {
		return nil, fmt.Errorf("n must be positive")
	}
	return s.collection.Query(ctx, query, n, nil, nil)
}

// QueryScoped returns the top similarity matches limited to the given PDF IDs.
func (s *Store) QueryScoped(ctx context.Context, query string, pdfIDs []string, limit int) ([]Candidate, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	if len(pdfIDs) == 0 {
		return nil, nil
	}

	allowed := make(map[string]struct{}, len(pdfIDs))
	for _, id := range pdfIDs {
		allowed[id] = struct{}{}
	}

	s.mu.RLock()
	total := s.collection.Count()
	s.mu.RUnlock()

	if total == 0 {
		return nil, nil
	}

	fetchN := limit * 5
	if fetchN > total {
		fetchN = total
	}
	if fetchN < limit {
		fetchN = limit
	}
	if fetchN > total {
		fetchN = total
	}

	results, err := s.Query(ctx, query, fetchN)
	if err != nil {
		return nil, err
	}

	out := make([]Candidate, 0, limit)
	for _, r := range results {
		pdfID := r.Metadata["pdf_id"]
		if pdfID == "" {
			continue
		}
		if _, ok := allowed[pdfID]; !ok {
			continue
		}

		title := r.Metadata["title"]
		startPage, _ := strconv.Atoi(r.Metadata["start_page"])
		endPage, _ := strconv.Atoi(r.Metadata["end_page"])
		plainText := plainTextFromContent(r.Content, title)

		out = append(out, Candidate{
			SectionID: r.ID,
			PDFID:     pdfID,
			Title:     title,
			StartPage: startPage,
			EndPage:   endPage,
			PlainText: plainText,
			Score:     float64(r.Similarity),
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func plainTextFromContent(content, title string) string {
	prefix := title + "\n\n"
	if strings.HasPrefix(content, prefix) {
		return content[len(prefix):]
	}
	if idx := strings.Index(content, "\n\n"); idx >= 0 {
		return content[idx+2:]
	}
	return content
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.collection.Count()
}
