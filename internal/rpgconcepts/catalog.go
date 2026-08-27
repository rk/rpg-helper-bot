package rpgconcepts

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Feature struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Questions   []string `yaml:"questions"`
	Synonyms    []string `yaml:"synonyms"`
}

type catalogFile struct {
	Features []Feature `yaml:"features"`
}

type ConceptCatalog struct {
	Features []Feature
	byID     map[string]Feature
}

func DefaultConceptsPath() string {
	if p := strings.TrimSpace(os.Getenv("RPG_HELPER_CONCEPTS_FILE")); p != "" {
		return p
	}
	return "data/rpg-concepts.yaml"
}

func LoadConcepts(path string) (*ConceptCatalog, error) {
	if path == "" {
		path = DefaultConceptsPath()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read concepts %s: %w", path, err)
	}
	var raw catalogFile
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse concepts %s: %w", path, err)
	}
	c := &ConceptCatalog{
		Features: raw.Features,
		byID:     map[string]Feature{},
	}
	for _, f := range raw.Features {
		if f.ID == "" {
			continue
		}
		c.byID[f.ID] = f
	}
	return c, nil
}

func (c *ConceptCatalog) FeatureByID(id string) (Feature, bool) {
	if c == nil {
		return Feature{}, false
	}
	if f, ok := c.byID[id]; ok {
		return f, true
	}
	for _, f := range c.Features {
		if f.ID == id {
			return f, true
		}
	}
	return Feature{}, false
}

func (c *ConceptCatalog) AllSynonyms() []string {
	var out []string
	for _, f := range c.Features {
		out = append(out, f.Synonyms...)
	}
	return out
}

type Match struct {
	FeatureID string
	Synonym   string
	Term      string
}

// MatchQueryFeatures returns feature IDs whose synonyms appear in query (word-boundary).
func (c *ConceptCatalog) MatchQueryFeatures(query string) []string {
	if c == nil {
		return nil
	}
	q := strings.ToLower(query)
	found := map[string]struct{}{}
	for _, f := range c.Features {
		for _, syn := range f.Synonyms {
			if syn == "" {
				continue
			}
			if wordMatch(q, syn) {
				found[f.ID] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ScanText finds synonym matches in body text.
func (c *ConceptCatalog) ScanText(text string) []Match {
	if c == nil {
		return nil
	}
	lower := strings.ToLower(text)
	var out []Match
	seen := map[string]struct{}{}
	for _, f := range c.Features {
		for _, syn := range f.Synonyms {
			if syn == "" || len(syn) < 2 {
				continue
			}
			if !wordMatch(lower, syn) {
				continue
			}
			key := f.ID + "\x00" + strings.ToLower(syn)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, Match{FeatureID: f.ID, Synonym: syn, Term: syn})
		}
	}
	return out
}

func wordMatch(haystack, phrase string) bool {
	phrase = strings.ToLower(strings.TrimSpace(phrase))
	if phrase == "" {
		return false
	}
	pattern := `(?i)(?:^|[^\p{L}\p{N}])` + regexp.QuoteMeta(phrase) + `(?:$|[^\p{L}\p{N}])`
	if regexp.MustCompile(pattern).MatchString(" " + haystack + " ") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(haystack), phrase)
}
