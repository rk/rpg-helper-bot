package search_test

import (
	"context"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/embed"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/vectors"
)

func newVectorStore(t *testing.T) *vectors.Store {
	t.Helper()
	vs, err := vectors.New(embed.HashEmbed)
	if err != nil {
		t.Fatal(err)
	}
	return vs
}

func syncVectorsFromStore(t *testing.T, vs *vectors.Store, s store.Store) {
	t.Helper()
	sections, err := s.ListIndexedSections()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, sec := range sections {
		if err := vs.UpsertSection(ctx, sec.ID, sec.PDFID, sec.Title, sec.PlainText, sec.StartPage, sec.EndPage); err != nil {
			t.Fatal(err)
		}
	}
}

func newSearchService(t *testing.T, s store.Store, vs *vectors.Store, rewrite search.QueryRewriteFunc) *search.Service {
	t.Helper()
	svc := &search.Service{Store: s, Vectors: vs, RewriteQuery: rewrite}
	return svc
}
