package indexing

import (
	"context"
	"log"
	"sort"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
)

// CompilePDFIndexMeta builds feature list, glossary, and cheatsheet from indexed sections.
func CompilePDFIndexMeta(catalog *rpgconcepts.ConceptCatalog, sections []models.TOCSection) *models.PDFIndexMeta {
	if catalog == nil {
		return &models.PDFIndexMeta{}
	}

	type featureAcc struct {
		sections map[string]struct{}
		terms    map[string]struct{}
	}
	byFeature := map[string]*featureAcc{}
	glossarySeen := map[string]struct{}{}
	var glossary []models.PDFGlossary

	addFeature := func(id, section, term string) {
		acc, ok := byFeature[id]
		if !ok {
			acc = &featureAcc{sections: map[string]struct{}{}, terms: map[string]struct{}{}}
			byFeature[id] = acc
		}
		if section != "" {
			acc.sections[section] = struct{}{}
		}
		if term != "" {
			acc.terms[term] = struct{}{}
		}
	}

	for _, sec := range sections {
		if strings.TrimSpace(sec.PlainText) == "" {
			continue
		}
		body := sec.Title + "\n" + sec.PlainText
		for _, m := range catalog.ScanText(body) {
			addFeature(m.FeatureID, sec.Title, m.Term)
			key := m.FeatureID + "\x00" + strings.ToLower(m.Term)
			if _, ok := glossarySeen[key]; ok {
				continue
			}
			glossarySeen[key] = struct{}{}
			glossary = append(glossary, models.PDFGlossary{
				FeatureID: m.FeatureID,
				PDFTerm:   m.Term,
				Evidence:  sec.Title,
			})
		}
	}

	features := make([]models.PDFFeature, 0, len(byFeature))
	for id, acc := range byFeature {
		f := models.PDFFeature{FeatureID: id}
		for s := range acc.sections {
			f.Sections = append(f.Sections, s)
		}
		sort.Strings(f.Sections)
		for t := range acc.terms {
			f.Terms = append(f.Terms, t)
		}
		sort.Strings(f.Terms)
		features = append(features, f)
	}
	sort.Slice(features, func(i, j int) bool {
		return features[i].FeatureID < features[j].FeatureID
	})
	sort.Slice(glossary, func(i, j int) bool {
		if glossary[i].FeatureID == glossary[j].FeatureID {
			return glossary[i].PDFTerm < glossary[j].PDFTerm
		}
		return glossary[i].FeatureID < glossary[j].FeatureID
	})

	cheatsheet := buildCheatsheet(catalog, features, sections)
	return &models.PDFIndexMeta{
		Features:   features,
		Glossary:   glossary,
		Cheatsheet: cheatsheet,
	}
}

func buildCheatsheet(catalog *rpgconcepts.ConceptCatalog, features []models.PDFFeature, sections []models.TOCSection) []models.CheatsheetEntry {
	sectionPage := map[string]int{}
	for _, sec := range sections {
		sectionPage[sec.Title] = sec.StartPage
	}
	var out []models.CheatsheetEntry
	for _, f := range features {
		if len(f.Sections) == 0 {
			continue
		}
		name := f.FeatureID
		if feat, ok := catalog.FeatureByID(f.FeatureID); ok {
			name = feat.Name
		}
		out = append(out, models.CheatsheetEntry{
			FeatureID:   f.FeatureID,
			FeatureName: name,
			PDFTerms:    append([]string(nil), f.Terms...),
			Section:     f.Sections[0],
			StartPage:   sectionPage[f.Sections[0]],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].FeatureID < out[j].FeatureID
	})
	return out
}

func featureSamples(sections []models.TOCSection, meta *models.PDFIndexMeta, maxPerFeature int) map[string]string {
	if meta == nil || maxPerFeature <= 0 {
		return nil
	}
	byTitle := map[string]string{}
	for _, sec := range sections {
		if sec.PlainText != "" {
			text := sec.PlainText
			if len(text) > 600 {
				text = text[:600] + "..."
			}
			byTitle[sec.Title] = sec.Title + "\n" + text
		}
	}
	samples := map[string]string{}
	for _, feat := range meta.Features {
		var b strings.Builder
		count := 0
		for _, title := range feat.Sections {
			if count >= maxPerFeature {
				break
			}
			if sample, ok := byTitle[title]; ok {
				if b.Len() > 0 {
					b.WriteString("\n\n---\n\n")
				}
				b.WriteString(sample)
				count++
			}
		}
		if b.Len() > 0 {
			samples[feat.FeatureID] = b.String()
		}
	}
	return samples
}

// CompileAndRefinePDFIndexMeta runs offline scan and optional LLM refinement.
func CompileAndRefinePDFIndexMeta(ctx context.Context, llmClient *llm.Client, catalog *rpgconcepts.ConceptCatalog, sections []models.TOCSection) *models.PDFIndexMeta {
	meta := CompilePDFIndexMeta(catalog, sections)
	if llmClient == nil || len(meta.Features) == 0 {
		meta.LLMGlossarySkipped = llmClient == nil
		return meta
	}
	samples := featureSamples(sections, meta, 3)
	refined, err := llmClient.RefinePDFGlossary(ctx, catalog, meta, samples)
	if err != nil {
		log.Printf("index: LLM glossary refinement skipped: %v", err)
		meta.LLMGlossarySkipped = true
		return meta
	}
	return refined
}
