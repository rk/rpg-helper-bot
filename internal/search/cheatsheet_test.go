package search_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func testConceptCatalog() *rpgconcepts.ConceptCatalog {
	return &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{{
			ID:          "attribute",
			Name:        "Attribute",
			Description: "An Attribute represents a character's inherent traits, such as strength or vigor.",
			Questions:   []string{"How are attributes gained?", "How many attributes does the character start with?"},
			Synonyms:    []string{"ability", "trait"},
		}},
	}
}

func TestSearchPDF_scopedToSinglePDF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	pdf := &models.PDF{Title: "Core", FilePath: "/tmp/core.pdf"}
	if err := s.SavePDF(pdf); err != nil {
		t.Fatal(err)
	}
	other := &models.PDF{Title: "Other", FilePath: "/tmp/other.pdf"}
	if err := s.SavePDF(other); err != nil {
		t.Fatal(err)
	}

	for _, spec := range []struct {
		id    string
		title string
		text  string
	}{
		{pdf.ID, "Attributes", "Strength and Vigor are core attributes for trait rolls."},
		{other.ID, "Attributes", "Unrelated attribute text in another book."},
	} {
		sections := []models.TOCSection{{Title: spec.title, StartPage: 1, EndPage: 2}}
		if err := s.SaveTOCSections(spec.id, sections); err != nil {
			t.Fatal(err)
		}
		saved, _ := s.ListTOCSections(spec.id)
		if err := s.SaveSectionText(saved[0].ID, spec.text); err != nil {
			t.Fatal(err)
		}
	}

	vs := newVectorStore(t)
	syncVectorsFromStore(t, vs, s)
	svc := newSearchService(t, s, vs, nil)
	result, err := svc.SearchPDF(context.Background(), pdf.ID, "attribute strength vigor rules definition", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) == 0 {
		t.Fatal("expected search hits")
	}
	for _, hit := range result.Hits {
		if hit.PDFID != pdf.ID {
			t.Fatalf("expected hits from target pdf only, got pdf_id=%q", hit.PDFID)
		}
	}
	if !strings.Contains(result.Hits[0].SectionTitle, "Attributes") {
		t.Fatalf("unexpected top hit: %+v", result.Hits[0])
	}
}

func TestFeatureDefinitionQuery_includesCatalogAndGlossary(t *testing.T) {
	catalog := testConceptCatalog()
	q := search.FeatureDefinitionQuery(catalog, []models.PDFGlossaryEntry{{
		FeatureID: "attribute",
		Terms:     []string{"STR", "Vigor"},
	}}, "attribute")
	if !strings.Contains(q, "Attribute") {
		t.Fatalf("expected catalog name in query: %q", q)
	}
	if !strings.Contains(q, "STR") || !strings.Contains(strings.ToLower(q), "vigor") {
		t.Fatalf("expected glossary terms in query: %q", q)
	}
	if !strings.Contains(q, "rules") || !strings.Contains(q, "definition") {
		t.Fatalf("expected rules/definition in query: %q", q)
	}
	if !strings.Contains(strings.ToLower(q), "attributes") || !strings.Contains(strings.ToLower(q), "character") {
		t.Fatalf("expected catalog question keywords in query: %q", q)
	}
}
