package models

import (
	"encoding/json"
	"sort"
	"strings"
)

type legacyPDFIndexMeta struct {
	Features            json.RawMessage `json:"features"`
	Glossary            json.RawMessage `json:"glossary"`
	Cheatsheet          json.RawMessage `json:"cheatsheet"`
	LLMGlossarySkipped  bool            `json:"llm_glossary_skipped,omitempty"`
	LLMLearningsSkipped bool            `json:"llm_learnings_skipped,omitempty"`
}

type legacyPDFFeature struct {
	FeatureID string   `json:"feature_id"`
	Sections  []string `json:"sections"`
	Terms     []string `json:"terms"`
}

type legacyPDFGlossary struct {
	FeatureID string `json:"feature_id"`
	PDFTerm   string `json:"pdf_term"`
	Evidence  string `json:"evidence,omitempty"`
}

type legacyCheatsheetEntry struct {
	FeatureID   string   `json:"feature_id"`
	FeatureName string   `json:"feature_name,omitempty"`
	PDFTerms    []string `json:"pdf_terms"`
	Section     string   `json:"section"`
	StartPage   int      `json:"start_page"`
}

// ParsePDFIndexMeta unmarshals index metadata and normalizes legacy v1 JSON.
func ParsePDFIndexMeta(raw string) (*PDFIndexMeta, error) {
	if strings.TrimSpace(raw) == "" {
		return EmptyPDFIndexMeta(), nil
	}
	var legacy legacyPDFIndexMeta
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return nil, err
	}
	meta := convertLegacyIndexMeta(legacy)
	NormalizeIndexMeta(meta)
	return meta, nil
}

func EmptyPDFIndexMeta() *PDFIndexMeta {
	return &PDFIndexMeta{
		Glossary:   []PDFGlossaryEntry{},
		Features:   []string{},
		Cheatsheet: []CheatsheetEntry{},
	}
}

func convertLegacyIndexMeta(raw legacyPDFIndexMeta) *PDFIndexMeta {
	meta := EmptyPDFIndexMeta()
	meta.LLMLearningsSkipped = raw.LLMLearningsSkipped || raw.LLMGlossarySkipped

	if len(raw.Features) > 0 {
		var ids []string
		if json.Unmarshal(raw.Features, &ids) == nil {
			meta.Features = ids
		} else {
			var feats []legacyPDFFeature
			if json.Unmarshal(raw.Features, &feats) == nil {
				for _, f := range feats {
					if id := strings.TrimSpace(f.FeatureID); id != "" {
						meta.Features = append(meta.Features, id)
					}
				}
			}
		}
	}

	if len(raw.Glossary) > 0 {
		var entries []PDFGlossaryEntry
		if json.Unmarshal(raw.Glossary, &entries) == nil && len(entries) > 0 && len(entries[0].Terms) > 0 {
			meta.Glossary = entries
		} else {
			var flat []legacyPDFGlossary
			if json.Unmarshal(raw.Glossary, &flat) == nil {
				byFeature := map[string]map[string]struct{}{}
				for _, g := range flat {
					id := strings.TrimSpace(g.FeatureID)
					term := strings.TrimSpace(g.PDFTerm)
					if id == "" || term == "" {
						continue
					}
					if byFeature[id] == nil {
						byFeature[id] = map[string]struct{}{}
					}
					byFeature[id][term] = struct{}{}
				}
				for id, terms := range byFeature {
					entry := PDFGlossaryEntry{FeatureID: id}
					for t := range terms {
						entry.Terms = append(entry.Terms, t)
					}
					sort.Strings(entry.Terms)
					meta.Glossary = append(meta.Glossary, entry)
				}
				sort.Slice(meta.Glossary, func(i, j int) bool {
					return meta.Glossary[i].FeatureID < meta.Glossary[j].FeatureID
				})
			}
		}
	}

	if len(raw.Cheatsheet) > 0 {
		var entries []CheatsheetEntry
		if json.Unmarshal(raw.Cheatsheet, &entries) == nil && len(entries) > 0 && entries[0].Definition != "" {
			meta.Cheatsheet = entries
		} else {
			var old []legacyCheatsheetEntry
			if json.Unmarshal(raw.Cheatsheet, &old) == nil {
				for _, c := range old {
					id := strings.TrimSpace(c.FeatureID)
					if id == "" {
						continue
					}
					entry := CheatsheetEntry{
						FeatureID:  id,
						Definition: strings.TrimSpace(c.FeatureName),
						Citations:  []CheatsheetCitation{},
					}
					if c.Section != "" {
						entry.Citations = append(entry.Citations, CheatsheetCitation{
							SectionTitle: strings.TrimSpace(c.Section),
							StartPage:    c.StartPage,
						})
					}
					meta.Cheatsheet = append(meta.Cheatsheet, entry)
				}
			}
		}
	}

	return meta
}

// NormalizeIndexMeta trims and deduplicates index metadata fields.
func NormalizeIndexMeta(meta *PDFIndexMeta) {
	if meta == nil {
		return
	}
	if meta.Glossary == nil {
		meta.Glossary = []PDFGlossaryEntry{}
	}
	if meta.Features == nil {
		meta.Features = []string{}
	}
	if meta.Cheatsheet == nil {
		meta.Cheatsheet = []CheatsheetEntry{}
	}

	glossary := meta.Glossary[:0]
	for _, g := range meta.Glossary {
		g.FeatureID = strings.TrimSpace(g.FeatureID)
		if g.FeatureID == "" {
			continue
		}
		seen := map[string]struct{}{}
		var terms []string
		for _, t := range g.Terms {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			key := strings.ToLower(t)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			terms = append(terms, t)
		}
		sort.Strings(terms)
		if len(terms) == 0 {
			continue
		}
		g.Terms = terms
		glossary = append(glossary, g)
	}
	sort.Slice(glossary, func(i, j int) bool {
		return glossary[i].FeatureID < glossary[j].FeatureID
	})
	meta.Glossary = glossary

	featureSet := map[string]struct{}{}
	features := meta.Features[:0]
	for _, id := range meta.Features {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := featureSet[id]; ok {
			continue
		}
		featureSet[id] = struct{}{}
		features = append(features, id)
	}
	sort.Strings(features)
	meta.Features = features

	cheatsheet := meta.Cheatsheet[:0]
	for _, c := range meta.Cheatsheet {
		c.FeatureID = strings.TrimSpace(c.FeatureID)
		c.Definition = strings.TrimSpace(c.Definition)
		if c.FeatureID == "" {
			continue
		}
		if c.Citations == nil {
			c.Citations = []CheatsheetCitation{}
		}
		citations := c.Citations[:0]
		for _, cit := range c.Citations {
			cit.SectionID = strings.TrimSpace(cit.SectionID)
			cit.SectionTitle = strings.TrimSpace(cit.SectionTitle)
			if cit.SectionTitle == "" && cit.StartPage <= 0 {
				continue
			}
			citations = append(citations, cit)
		}
		c.Citations = citations
		cheatsheet = append(cheatsheet, c)
	}
	sort.Slice(cheatsheet, func(i, j int) bool {
		return cheatsheet[i].FeatureID < cheatsheet[j].FeatureID
	})
	meta.Cheatsheet = cheatsheet
}
