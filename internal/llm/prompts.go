package llm

import (
	"os"
	"path/filepath"
	"strings"
)

type PromptID string

const (
	PromptChatSystem        PromptID = "chat-system"
	PromptSearchRewrite     PromptID = "search-rewrite"
	PromptGlossaryExtract   PromptID = "glossary"
	PromptFeatureDetect     PromptID = "feature-detect"
	PromptCheatsheetExtract PromptID = "cheatsheet"
	PromptRPGDomain         PromptID = "rpg-domain"
	PromptJSONOutput        PromptID = "json-output"
)

var defaultPromptFiles = map[PromptID]string{
	PromptChatSystem:        "chat-system.md",
	PromptSearchRewrite:     "search-rewrite-system.md",
	PromptGlossaryExtract:   "glossary-extract-system.md",
	PromptFeatureDetect:     "feature-detect-system.md",
	PromptCheatsheetExtract: "cheatsheet-extract-system.md",
	PromptRPGDomain:         filepath.Join("shared", "rpg-domain.md"),
	PromptJSONOutput:        filepath.Join("shared", "json-output.md"),
}

var defaultPromptEnv = map[PromptID]string{
	PromptChatSystem:        "RPG_HELPER_PROMPT_CHAT",
	PromptSearchRewrite:     "RPG_HELPER_PROMPT_SEARCH_REWRITE",
	PromptGlossaryExtract:   "RPG_HELPER_PROMPT_GLOSSARY",
	PromptFeatureDetect:     "RPG_HELPER_PROMPT_FEATURE_DETECT",
	PromptCheatsheetExtract: "RPG_HELPER_PROMPT_CHEATSHEET",
	PromptRPGDomain:         "RPG_HELPER_PROMPT_RPG_DOMAIN",
}

var embeddedDefaults = map[PromptID]string{
	PromptChatSystem: `{{RPG_DOMAIN}}

You are a helpful tabletop RPG rules assistant. Answer using ONLY the provided rule excerpts and indexed learnings below.
If the answer is not in the excerpts, say you could not find it in the indexed rules.
Format answers in Markdown. Cite excerpts inline as [1], [2], etc. matching the excerpt numbers below.
Preserve dice notation exactly (e.g. 2D6, d8, 2D6-2, 2D) — do not rewrite or expand dice expressions.

{{GLOSSARY}}

{{CHEATSHEET}}

{{GAME_NOTES}}

When cheatsheet entries are provided, treat them as indexed summaries and prefer their cited sections in the rule excerpts below.

Rule excerpts (later books override earlier ones on conflict):
{{EXCERPTS}}
`,
	PromptSearchRewrite: `{{RPG_DOMAIN}}

You rewrite tabletop RPG rules questions into concise keyword lists for full-text search.
Include the original important terms plus closely related words, synonyms, and word stems.
When book terminology is provided below, include PDF-specific glossary terms as keywords when they relate to the question.
Preserve dice notation tokens exactly (e.g. 2D6, d8, 2D6-2, 2D) — do not expand or normalize them.
Reply with ONLY space-separated keywords. No punctuation, labels, or explanation. At most 12 words.

{{GLOSSARY}}`,
	PromptGlossaryExtract: `{{RPG_DOMAIN}}

{{JSON_OUTPUT}}

You map book word-frequency tokens to canonical RPG feature IDs.
You receive a catalog and a TSV of top document tokens with counts.
Reply with this JSON shape: {"glossary":[{"feature_id":"...","terms":["..."]}]}
Use feature_id values from the catalog only. Return terms in natural book casing.`,
	PromptFeatureDetect: `{{RPG_DOMAIN}}

{{JSON_OUTPUT}}

You determine which canonical features an RPG rulebook actually uses from its word-frequency dictionary.
You receive catalog, glossary, optional offline candidates, and a TSV of top document tokens.
Reply with this JSON shape: {"features":["skill_check","wild_die"]}
Use feature_id values from the catalog only.`,
	PromptCheatsheetExtract: `{{RPG_DOMAIN}}

{{JSON_OUTPUT}}

You write concise rules cheatsheet entries for one RPG feature from search-ranked rulebook excerpts.
You receive catalog entry, optional catalog questions to answer, glossary terms, and ranked section excerpts. Summarize in one response; address catalog questions when excerpts support them.
Reply with this JSON shape: {"feature_id":"...","definition":"...","citations":[{"section_title":"...","section_id":"...","start_page":1}]}`,
	PromptJSONOutput: `## Output format

Your entire reply must be one raw JSON value with no surrounding text.

- Do not wrap the JSON in markdown code fences.
- Do not add prose, labels, or explanations before or after the JSON.
- Do not use // or /* */ comments inside the JSON.
- Use standard JSON: double-quoted keys and strings, no trailing commas.
- Keep each string value on one line (use spaces, not literal line breaks).`,
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
	if usesJSONOutputRules(id) {
		out = strings.ReplaceAll(out, "{{JSON_OUTPUT}}", strings.TrimSpace(LoadPrompt(PromptJSONOutput)))
	}
	out = strings.ReplaceAll(out, "{{JSON_OUTPUT}}", "")
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	// Remove unused placeholders.
	for _, key := range []string{"RPG_DOMAIN", "JSON_OUTPUT", "GLOSSARY", "CHEATSHEET", "GAME_NOTES", "EXCERPTS"} {
		out = strings.ReplaceAll(out, "{{"+key+"}}", "")
	}
	return strings.TrimSpace(out) + "\n"
}

func usesJSONOutputRules(id PromptID) bool {
	switch id {
	case PromptGlossaryExtract, PromptFeatureDetect, PromptCheatsheetExtract:
		return true
	default:
		return false
	}
}
