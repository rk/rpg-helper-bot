package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeJSONResponse_rawObject(t *testing.T) {
	raw := `{"glossary":[]}`
	got := normalizeJSONResponse(raw)
	if got != raw {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeJSONResponse_markdownFence(t *testing.T) {
	raw := "```json\n{\"glossary\":[{\"feature_id\":\"skill_check\",\"terms\":[\"Tests\"]}]}\n```"
	got := normalizeJSONResponse(raw)
	var resp glossaryLLMResponse
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("parse: %v; got %q", err, got)
	}
	if len(resp.Glossary) != 1 || resp.Glossary[0].FeatureID != "skill_check" {
		t.Fatalf("unexpected: %+v", resp.Glossary)
	}
}

func TestNormalizeJSONResponse_fenceSameLine(t *testing.T) {
	raw := "```json {\"glossary\":[{\"feature_id\":\"skill_check\",\"terms\":[\"Tests\"]}]} ```"
	got := normalizeJSONResponse(raw)
	var resp glossaryLLMResponse
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("parse: %v; got %q", err, got)
	}
}

func TestRepairLLMJSON_termsTypo(t *testing.T) {
	broken := `{"glossary":[{"feature_id":"saving_throw","terms:_: ["SAVE","RESIST"]}]}`
	fixed := repairLLMJSON(broken)
	var resp glossaryLLMResponse
	if err := json.Unmarshal([]byte(fixed), &resp); err != nil {
		t.Fatalf("parse repaired: %v; fixed=%q", err, fixed)
	}
}

func TestNormalizeJSONResponse_prosePrefix(t *testing.T) {
	raw := "Here is the updated glossary:\n\n{\"glossary\":[{\"feature_id\":\"skill_check\",\"terms\":[\"Tests\"]}]}"
	got := normalizeJSONResponse(raw)
	var resp glossaryLLMResponse
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("parse: %v; got %q", err, got)
	}
}

func TestPreviewLLMText_collapsesWhitespace(t *testing.T) {
	got := previewLLMText("  hello\n\nworld  ", 80)
	if got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractJSON_emptyFeaturesArrayWithFence(t *testing.T) {
	raw := "[ ] ```"
	got := normalizeJSONResponse(raw)
	var resp featuresLLMResponse
	if err := parseLLMJSON(raw, llmParseContext{Pass: "features", ChunkNum: 1, ChunkTotal: 1}, &resp); err != nil {
		t.Fatalf("parse: %v; normalized=%q", err, got)
	}
	if len(resp.Features) != 0 {
		t.Fatalf("expected empty features, got %v", resp.Features)
	}
}

func TestTruncateWords(t *testing.T) {
	text := strings.Repeat("word ", 1500)
	got := truncateWords(text, 1000)
	words := strings.Fields(strings.TrimSuffix(got, "..."))
	if len(words) != 1000 {
		t.Fatalf("expected 1000 words, got %d: %q", len(words), got)
	}
}

func TestParseLLMJSON_errorMentionsFullDump(t *testing.T) {
	var resp glossaryLLMResponse
	err := parseLLMJSON("not json at all", llmParseContext{
		Pass: "glossary", ChunkNum: 1, ChunkTotal: 1,
	}, &resp)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "see log for full raw and extracted JSON") {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestStripJSONComments_lineComments(t *testing.T) {
	broken := `{
  "section_id": "abc",  // not valid json
  "start_page": 9
}`
	fixed := stripJSONComments(broken)
	if strings.Contains(fixed, "//") {
		t.Fatalf("comment not stripped: %q", fixed)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(fixed), &m); err != nil {
		t.Fatalf("parse: %v; fixed=%q", err, fixed)
	}
}

func TestParseCheatsheetJSON_lineCommentsInCitations(t *testing.T) {
	raw := `{
  "feature_id": "attribute",
  "definition": "An Attribute represents innate traits.",
  "citations": [
    {
      "section_title": "Making Tests",
      "section_id": "1dadf38e-27a3-4cb8-a4f6-3e124dab0223",  // Assuming this is correct
      "start_page": 42,
      "end_page": 43
    },
    {
      "section_title": "Fantasy Ancestries: Aquarians",
      "section_id": "98ec5a0a-12b5-4171-bfbd-2e06-937db147", // ancestral rules
      "start_page": 12,
      "end_page": 12
    }
  ]
}`
	resp, err := parseCheatsheetJSON(raw, llmParseContext{Pass: "cheatsheet", ChunkNum: 2, ChunkTotal: 101, FeatureID: "attribute"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Citations) != 2 {
		t.Fatalf("expected 2 citations, got %d", len(resp.Citations))
	}
}

func TestRepairJSONStringLiterals_newlinesInDefinition(t *testing.T) {
	broken := `{
  "feature_id": "attribute",
  "definition": "First paragraph.

  Second paragraph.",
  "citations": []
}`
	fixed := repairLLMJSON(broken)
	var raw flexCheatsheetResponse
	if err := json.Unmarshal([]byte(fixed), &raw); err != nil {
		t.Fatalf("parse repaired: %v; fixed=%q", err, fixed)
	}
	def := parseFlexibleDefinition(raw.Definition)
	if !strings.Contains(def, "First paragraph.") || !strings.Contains(def, "Second paragraph.") {
		t.Fatalf("unexpected definition: %q", def)
	}
}

func TestParseCheatsheetJSON_literalNewlineInDefinition(t *testing.T) {
	raw := `{
  "feature_id": "attribute",
  "definition": "**Attributes** reflect core traits.

  Ancestral lineages also modify baseline traits.",
  "citations": [
    {"section_title": "Cultural Packages", "section_id": "1dadf38e-27a3-4cb8-a4f6-3e124dab0223", "start_page": 9, "end_page": 9}
  ]
}`
	resp, err := parseCheatsheetJSON(raw, llmParseContext{Pass: "cheatsheet", ChunkNum: 3, ChunkTotal: 151, FeatureID: "attribute"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Definition, "Ancestral lineages") {
		t.Fatalf("unexpected definition: %q", resp.Definition)
	}
	if len(resp.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(resp.Citations))
	}
}

func TestParseCheatsheetJSON_sectionIDArray(t *testing.T) {
	raw := `{
  "feature_id": "attribute",
  "definition": "Attributes define trait dice.",
  "citations": [
    {"section_title": "Fantasy Ancestries", "section_id": ["id-1", "id-2"], "start_page": 10, "end_page": 12, "note": "ignored"}
  ]
}`
	resp, err := parseCheatsheetJSON(raw, llmParseContext{Pass: "cheatsheet", ChunkNum: 3, ChunkTotal: 150, FeatureID: "attribute"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Citations) != 2 {
		t.Fatalf("expected 2 citations, got %d: %+v", len(resp.Citations), resp.Citations)
	}
	if resp.Citations[0].SectionID != "id-1" || resp.Citations[1].SectionID != "id-2" {
		t.Fatalf("unexpected ids: %+v", resp.Citations)
	}
}

func TestParseCheatsheetJSON_nestedDefinitionObject(t *testing.T) {
	raw := `{
  "feature_id": "target_number",
  "definition": {
    "definition": "In this system, a **target number** is a fixed numerical threshold."
  },
  "citations": [
    {"section_title": "Skill Checks", "section_id": "41c5a717-b8ce-483f-8a94-ecebb26b9a5d", "start_page": 51, "end_page": 51}
  ]
}`
	resp, err := parseCheatsheetJSON(raw, llmParseContext{Pass: "cheatsheet", ChunkNum: 1, ChunkTotal: 1, FeatureID: "target_number"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.FeatureID != "target_number" {
		t.Fatalf("unexpected feature_id: %q", resp.FeatureID)
	}
	if !strings.Contains(resp.Definition, "target number") {
		t.Fatalf("unexpected definition: %q", resp.Definition)
	}
	if len(resp.Citations) != 1 || resp.Citations[0].SectionID != "41c5a717-b8ce-483f-8a94-ecebb26b9a5d" {
		t.Fatalf("unexpected citations: %+v", resp.Citations)
	}
}

func TestParseCheatsheetJSON_flexiblePageFields(t *testing.T) {
	raw := `{
  "feature_id": "feat",
  "definition": "Edges are special abilities acquired through hindrances.",
  "citations": [
    {
      "section_title": "Traits",
      "section_id": "1aa047a-8309-46b-0a571-251e41caet42",
      "start_page": 9,
      "end_page": 9
    },
    {
      "section_title": "Hindrances",
      "section_id": "58ec9f4a-6f12-4909-8ec2-a9368fe84259",
      "start_page": 57,
      "end_page": 57
    },
    {
      "section_id": null,
      "section_title": "Edges",
      "section_id": "",
      "start_page": 0,
      "end_page": ""
    }
  ]
}`
	resp, err := parseCheatsheetJSON(raw, llmParseContext{Pass: "cheatsheet", ChunkNum: 1, ChunkTotal: 1, FeatureID: "feat"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Definition, "Edges") {
		t.Fatalf("unexpected definition: %q", resp.Definition)
	}
	if len(resp.Citations) != 3 {
		t.Fatalf("expected 3 citations, got %d: %+v", len(resp.Citations), resp.Citations)
	}
	if resp.Citations[0].StartPage != 9 || resp.Citations[1].EndPage != 57 {
		t.Fatalf("unexpected page numbers: %+v", resp.Citations)
	}
	if resp.Citations[2].SectionTitle != "Edges" || resp.Citations[2].EndPage != 0 {
		t.Fatalf("expected title-only citation for Edges, got %+v", resp.Citations[2])
	}
}

func TestParseLLMJSON_repairsTermsTypo(t *testing.T) {
	raw := "{\"glossary\":[{\"feature_id\":\"saving_throw\",\"terms:_: [\"SAVE\"]}]}"
	var resp glossaryLLMResponse
	if err := parseLLMJSON(raw, llmParseContext{Pass: "glossary", ChunkNum: 1, ChunkTotal: 1}, &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Glossary) != 1 || resp.Glossary[0].FeatureID != "saving_throw" {
		t.Fatalf("unexpected: %+v", resp.Glossary)
	}
}

func TestRenderPrompt_learningsIncludeJSONOutputRules(t *testing.T) {
	for _, id := range []PromptID{PromptGlossaryExtract, PromptFeatureDetect, PromptCheatsheetExtract} {
		out := renderPrompt(id, nil)
		if strings.Contains(out, "{{JSON_OUTPUT}}") {
			t.Fatalf("unexpanded placeholder in %s", id)
		}
		if !strings.Contains(out, "Do not wrap the JSON in markdown code fences") {
			t.Fatalf("missing json output rules in %s", id)
		}
	}
}
