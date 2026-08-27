package llm

import (
	"os"
	"path/filepath"
	"strings"
)

type PromptID string

const (
	PromptChatSystem      PromptID = "chat-system"
	PromptSearchRewrite   PromptID = "search-rewrite"
	PromptGlossaryExtract PromptID = "glossary"
	PromptRPGDomain       PromptID = "rpg-domain"
)

var defaultPromptFiles = map[PromptID]string{
	PromptChatSystem:      "chat-system.md",
	PromptSearchRewrite:   "search-rewrite-system.md",
	PromptGlossaryExtract: "glossary-extract-system.md",
	PromptRPGDomain:       filepath.Join("shared", "rpg-domain.md"),
}

var defaultPromptEnv = map[PromptID]string{
	PromptChatSystem:      "RPG_HELPER_PROMPT_CHAT",
	PromptSearchRewrite:   "RPG_HELPER_PROMPT_SEARCH_REWRITE",
	PromptGlossaryExtract: "RPG_HELPER_PROMPT_GLOSSARY",
	PromptRPGDomain:       "RPG_HELPER_PROMPT_RPG_DOMAIN",
}

var embeddedDefaults = map[PromptID]string{
	PromptChatSystem: `{{RPG_DOMAIN}}

You are a helpful tabletop RPG rules assistant. Answer using ONLY the provided rule excerpts.
If the answer is not in the excerpts, say you could not find it in the indexed rules.
Format answers in Markdown. Cite excerpts inline as [1], [2], etc. matching the excerpt numbers below.
Preserve dice notation exactly (e.g. 2D6, d8, 2D6-2, 2D) — do not rewrite or expand dice expressions.

{{GLOSSARY}}

{{GAME_NOTES}}

Rule excerpts (later books override earlier ones on conflict):
{{EXCERPTS}}
`,
	PromptSearchRewrite: `{{RPG_DOMAIN}}

You rewrite tabletop RPG rules questions into concise keyword lists for full-text search.
Include the original important terms plus closely related words, synonyms, and word stems.
Preserve dice notation tokens exactly (e.g. 2D6, d8, 2D6-2, 2D) — do not expand or normalize them.
Reply with ONLY space-separated keywords. No punctuation, labels, or explanation. At most 12 words.
`,
	PromptGlossaryExtract: `{{RPG_DOMAIN}}

You analyze indexed RPG rulebook excerpts and map book-specific terminology to canonical RPG feature IDs.
Reply with ONLY valid JSON: {"glossary":[{"feature_id":"...","pdf_term":"...","evidence":"..."}]}
Use feature_id values from the provided catalog only.`,
	PromptRPGDomain: `## Dice notation

Preserve dice notation exactly as written. Do not expand, normalize, or paraphrase dice expressions.
NdS = N dice of S sides; NdS±M applies a modifier; dice-pool Nd may omit size when context defines it.
`,
}

// LoadPrompt returns the prompt template for id, optionally including the shared RPG domain block.
func LoadPrompt(id PromptID) string {
	raw := loadPromptRaw(id)
	if id != PromptRPGDomain {
		domain := loadPromptRaw(PromptRPGDomain)
		raw = strings.ReplaceAll(raw, "{{RPG_DOMAIN}}", strings.TrimSpace(domain))
	}
	raw = strings.ReplaceAll(raw, "{{RPG_DOMAIN}}", "")
	return strings.TrimSpace(raw)
}

func loadPromptRaw(id PromptID) string {
	if envKey, ok := defaultPromptEnv[id]; ok {
		if path := strings.TrimSpace(os.Getenv(envKey)); path != "" {
			if b, err := os.ReadFile(path); err == nil {
				return string(b)
			}
		}
	}
	dir := promptsDir()
	if file, ok := defaultPromptFiles[id]; ok {
		path := filepath.Join(dir, file)
		if b, err := os.ReadFile(path); err == nil {
			return string(b)
		}
	}
	return embeddedDefaults[id]
}

func promptsDir() string {
	if d := strings.TrimSpace(os.Getenv("RPG_HELPER_PROMPTS_DIR")); d != "" {
		return d
	}
	return "prompts"
}

func renderPrompt(id PromptID, vars map[string]string) string {
	out := LoadPrompt(id)
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	// Remove unused placeholders.
	for _, key := range []string{"RPG_DOMAIN", "GLOSSARY", "GAME_NOTES", "EXCERPTS"} {
		out = strings.ReplaceAll(out, "{{"+key+"}}", "")
	}
	return strings.TrimSpace(out) + "\n"
}
