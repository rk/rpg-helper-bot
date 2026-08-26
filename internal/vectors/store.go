package vectors

import (
	"context"
	"fmt"
	"sync"

	"github.com/philippgille/chromem-go"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/embed"
)

const collectionName = "toc_sections"

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

func (s *Store) UpsertSection(ctx context.Context, sectionID, pdfID, title, plainText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	content := title + "\n\n" + plainText
	meta := map[string]string{
		"pdf_id": pdfID,
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

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.collection.Count()
}
