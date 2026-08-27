package chat

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/rpgconcepts"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

// BuildFeatureIndex returns a compact feature_id → name list for the game's indexed PDFs.
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
		for _, f := range catalog.Features {
			if f.ID != "" {
				featureSet[f.ID] = struct{}{}
			}
		}
	}

	ids := make([]string, 0, len(featureSet))
	for id := range featureSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var b strings.Builder
	b.WriteString("Feature index (use feature_id values in tool calls):\n")
	for _, id := range ids {
		name := id
		if feat, ok := catalog.FeatureByID(id); ok && strings.TrimSpace(feat.Name) != "" {
			name = feat.Name
		}
		fmt.Fprintf(&b, "- %s: %s\n", id, name)
	}
	return strings.TrimSpace(b.String()), nil
}
