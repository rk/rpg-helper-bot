package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

func TestBuildCheatsheetFromSearchHits_catalogFallback(t *testing.T) {
	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{{
			ID:          "attribute",
			Name:        "Attribute",
			Description: "An Attribute represents inherent traits.",
		}},
	}
	c := &llm.Client{}
	entry, err := c.BuildCheatsheetFromSearchHits(context.Background(), catalog, nil, "attribute", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil || entry.Definition == "" {
		t.Fatalf("expected catalog fallback entry, got %+v", entry)
	}
	if entry.FeatureID != "attribute" {
		t.Fatalf("unexpected feature_id: %q", entry.FeatureID)
	}
}

func TestBuildCheatsheetFromSearchHits_includesDetectedFeaturesInUserMessage(t *testing.T) {
	var userMessage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, msg := range body.Messages {
			if msg.Role == "user" {
				userMessage = msg.Content
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"feature_id\":\"combat\",\"definition\":\"Use a ` + "`skill_check`" + `.\",\"citations\":[]}"}}]}`))
	}))
	defer srv.Close()

	catalog := &rpgconcepts.ConceptCatalog{
		Features: []rpgconcepts.Feature{
			{ID: "combat", Name: "Combat", Description: "Conflict resolution."},
			{ID: "skill_check", Name: "Skill Check", Description: "Rolling dice to resolve actions."},
		},
	}
	client := &llm.Client{BaseURL: srv.URL, Model: "test", HTTP: srv.Client()}
	hits := []models.SearchHit{{
		SectionID: "sec-1", SectionTitle: "Combat", StartPage: 1, EndPage: 2, Snippet: "Roll to hit.",
	}}
	entry, err := client.BuildCheatsheetFromSearchHits(
		context.Background(),
		catalog,
		nil,
		"combat",
		[]string{"combat", "skill_check"},
		hits,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil || entry.Definition == "" {
		t.Fatalf("expected entry, got %+v", entry)
	}
	if !strings.Contains(userMessage, "Detected features for this game") {
		t.Fatalf("expected detected features block in user message: %q", userMessage)
	}
	if !strings.Contains(userMessage, "skill_check (Skill Check)") {
		t.Fatalf("expected skill_check in detected list: %q", userMessage)
	}
	detectedSection := userMessage
	if idx := strings.Index(userMessage, "Detected features for this game"); idx >= 0 {
		detectedSection = userMessage[idx:]
		if end := strings.Index(detectedSection, "\nGlossary terms"); end >= 0 {
			detectedSection = detectedSection[:end]
		}
	}
	if strings.Contains(detectedSection, "- combat (Combat)") {
		t.Fatalf("target feature should be excluded from detected list: %q", detectedSection)
	}
}

func TestBuildSectionSamples_truncates(t *testing.T) {
	long := strings.Repeat("word ", 2000)
	sections := []models.TOCSection{{
		ID: "s1", Title: "Tests", StartPage: 1, EndPage: 2,
		PlainText: long,
	}}
	samples := llm.BuildSectionSamples(sections)
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	words := strings.Fields(strings.TrimSuffix(samples[0].Excerpt, "..."))
	if len(words) != 1000 {
		t.Fatalf("expected 1000 words in excerpt, got %d", len(words))
	}
}

func TestAttachSectionIDs(t *testing.T) {
	cheatsheet := []models.CheatsheetEntry{{
		FeatureID: "skill_check",
		Citations: []models.CheatsheetCitation{{SectionTitle: "Making Tests", StartPage: 42}},
	}}
	sections := []models.TOCSection{{ID: "sec-1", Title: "Making Tests"}}
	llm.AttachSectionIDs(cheatsheet, sections)
	if cheatsheet[0].Citations[0].SectionID != "sec-1" {
		t.Fatalf("section id not attached: %+v", cheatsheet[0].Citations[0])
	}
}

func TestMergeGlossary_dedupesTerms(t *testing.T) {
	existing := []models.PDFGlossaryEntry{{FeatureID: "skill_check", Terms: []string{"Tests"}}}
	updated := []models.PDFGlossaryEntry{{FeatureID: "skill_check", Terms: []string{"Tests", "Trait roll"}}}
	merged := llm.MergeGlossary(existing, updated)
	if len(merged) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(merged))
	}
	if len(merged[0].Terms) != 2 {
		t.Fatalf("expected 2 terms, got %v", merged[0].Terms)
	}
}
