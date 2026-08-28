package chat

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

// BuildFeatureIndex returns catalog feature entries detected in the game's indexed PDFs.
// Only features present in index_meta.Features are included; Questions and Synonyms are omitted
// so tools can resolve book-specific terminology from indexed glossary data.
func BuildFeatureIndex(st store.Store, gameID string) (string, error) {
	pdfs, err := st.ListGamePDFs(gameID)
	if err != nil {
		return "", err
	}
	catalog, err := rpgconcepts.LoadConcepts(rpgconcepts.DefaultConceptsPath())
	if err != nil {
		return "", err
	}

	featureSet := map[string]struct{}{}
	if len(pdfs) > 0 {
		ids := make([]string, len(pdfs))
		for i, p := range pdfs {
			ids[i] = p.ID
		}
		metaByPDF, err := st.ListPDFIndexMeta(ids)
		if err != nil {
			return "", err
		}
		for _, meta := range metaByPDF {
			for _, id := range meta.Features {
				if id != "" {
					featureSet[id] = struct{}{}
				}
			}
		}
	}

	if len(featureSet) == 0 {
		return "No features detected in this game's indexed PDFs yet.", nil
	}

	featureIDs := make([]string, 0, len(featureSet))
	for id := range featureSet {
		featureIDs = append(featureIDs, id)
	}
	sort.Strings(featureIDs)

	var b strings.Builder
	b.WriteString("Feature index (use feature_id values in tool calls):\n")
	for _, id := range featureIDs {
		if feat, ok := catalog.FeatureByID(id); ok {
			b.WriteString(formatFeatureForChat(feat))
			continue
		}
		fmt.Fprintf(&b, "- %s\n", id)
	}
	return strings.TrimSpace(b.String()), nil
}

func formatFeatureForChat(f rpgconcepts.Feature) string {
	name := strings.TrimSpace(f.Name)
	if name == "" {
		name = f.ID
	}
	var b strings.Builder
	fmt.Fprintf(&b, "- %s (%s)", f.ID, name)
	if desc := strings.TrimSpace(f.Description); desc != "" {
		fmt.Fprintf(&b, ": %s", desc)
	}
	b.WriteByte('\n')
	return b.String()
}
