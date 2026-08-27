package models_test

import (
	"encoding/json"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

func TestParsePDFIndexMeta_legacyV1(t *testing.T) {
	raw := `{
		"features":[{"feature_id":"skill_check","sections":["Tests"],"terms":["test"]}],
		"glossary":[{"feature_id":"skill_check","pdf_term":"Tests","evidence":"Support Vs. Test"}],
		"cheatsheet":[{"feature_id":"skill_check","feature_name":"Skill Check","pdf_terms":["Tests"],"section":"Tests","start_page":42}],
		"llm_glossary_skipped":true
	}`
	meta, err := models.ParsePDFIndexMeta(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Features) != 1 || meta.Features[0] != "skill_check" {
		t.Fatalf("features: %+v", meta.Features)
	}
	if len(meta.Glossary) != 1 || len(meta.Glossary[0].Terms) != 1 || meta.Glossary[0].Terms[0] != "Tests" {
		t.Fatalf("glossary: %+v", meta.Glossary)
	}
	if !meta.LLMLearningsSkipped {
		t.Fatal("expected llm_learnings_skipped from legacy flag")
	}
}

func TestParsePDFIndexMeta_v2(t *testing.T) {
	raw := `{
		"glossary":[{"feature_id":"skill_check","terms":["Tests","Trait roll"]}],
		"features":["skill_check"],
		"cheatsheet":[{"feature_id":"skill_check","definition":"Roll a trait die.","citations":[{"section_title":"Tests","start_page":42}]}]
	}`
	meta, err := models.ParsePDFIndexMeta(raw)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Cheatsheet[0].Definition != "Roll a trait die." {
		t.Fatalf("cheatsheet: %+v", meta.Cheatsheet)
	}
}

func TestNormalizeIndexMeta_dedupesTerms(t *testing.T) {
	meta := &models.PDFIndexMeta{
		Glossary: []models.PDFGlossaryEntry{{FeatureID: "skill_check", Terms: []string{"Tests", "tests", " Tests "}}},
	}
	models.NormalizeIndexMeta(meta)
	if len(meta.Glossary[0].Terms) != 1 {
		t.Fatalf("expected deduped terms, got %v", meta.Glossary[0].Terms)
	}
}

func TestNormalizeIndexMeta_roundTripJSON(t *testing.T) {
	meta := models.EmptyPDFIndexMeta()
	meta.Features = []string{"wild_die"}
	meta.Glossary = []models.PDFGlossaryEntry{{FeatureID: "wild_die", Terms: []string{"Wild Die"}}}
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := models.ParsePDFIndexMeta(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Features) != 1 {
		t.Fatalf("round trip features: %+v", parsed.Features)
	}
}
