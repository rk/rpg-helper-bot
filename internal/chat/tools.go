package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

const SearchToolLimit = 5

// Tools executes chat tool calls against game index data and search.
type Tools struct {
	Store           store.Store
	Search          *search.Service
	Catalog         *rpgconcepts.ConceptCatalog
	GameID          string
	accumulatedHits []models.SearchHit
	lastSearchDebug models.ChatSearchDebug
}

func NewTools(st store.Store, searchSvc *search.Service, gameID string) *Tools {
	catalog, _ := rpgconcepts.LoadConcepts(rpgconcepts.DefaultConceptsPath())
	return &Tools{
		Store:   st,
		Search:  searchSvc,
		Catalog: catalog,
		GameID:  gameID,
	}
}

// SearchHits returns all search hits accumulated during tool calls.
func (t *Tools) SearchHits() []models.SearchHit {
	return t.accumulatedHits
}

// LastDebug returns debug metadata from the most recent search tool call.
func (t *Tools) LastDebug() models.ChatSearchDebug {
	return t.lastSearchDebug
}

func (t *Tools) Execute(ctx context.Context, name string, argsJSON string) (string, error) {
	switch name {
	case "lookup_cheatsheet":
		var args struct {
			FeatureID string `json:"feature_id"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		return search.LookupCheatsheetByFeature(t.Store, t.GameID, args.FeatureID)
	case "lookup_glossary":
		var args struct {
			FeatureID string `json:"feature_id"`
			Term      string `json:"term"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		if strings.TrimSpace(args.FeatureID) != "" {
			return search.LookupGlossaryByFeature(t.Store, t.GameID, args.FeatureID)
		}
		if strings.TrimSpace(args.Term) != "" {
			return search.LookupGlossaryByTerm(t.Store, t.GameID, args.Term)
		}
		return "", fmt.Errorf("feature_id or term is required")
	case "search":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		query := strings.TrimSpace(args.Query)
		if query == "" {
			return "", fmt.Errorf("query is required")
		}
		if t.Search == nil {
			return "Search is unavailable.", nil
		}
		result, err := t.Search.SearchWithLimit(ctx, t.GameID, query, SearchToolLimit)
		if err != nil {
			return "", err
		}
		if result != nil {
			t.lastSearchDebug = result.Debug
			t.accumulatedHits = search.MergeSearchHits(t.accumulatedHits, result.Hits, 0)
		}
		if result == nil || len(result.Hits) == 0 {
			return "No matching rule excerpts found.", nil
		}
		return search.FormatSearchHits(result.Hits), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
