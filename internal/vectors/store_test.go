package vectors_test

import (
	"context"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/embed"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/vectors"
)

func TestQueryScoped_filtersByPDF(t *testing.T) {
	ctx := context.Background()
	vs, err := vectors.New(embed.HashEmbed)
	if err != nil {
		t.Fatal(err)
	}

	if err := vs.UpsertSection(ctx, "a1", "pdf-a", "Alpha", "combat wild attack rules", 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := vs.UpsertSection(ctx, "b1", "pdf-b", "Beta", "combat wild attack other book", 1, 2); err != nil {
		t.Fatal(err)
	}

	hits, err := vs.QueryScoped(ctx, "combat wild attack", []string{"pdf-a"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 scoped hit, got %d", len(hits))
	}
	if hits[0].SectionID != "a1" {
		t.Fatalf("expected section a1, got %q", hits[0].SectionID)
	}
	if hits[0].StartPage != 1 || hits[0].EndPage != 2 {
		t.Fatalf("unexpected page range: %+v", hits[0])
	}
}
