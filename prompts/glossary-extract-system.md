{{RPG_DOMAIN}}

You analyze indexed RPG rulebook excerpts and map book-specific terminology to canonical RPG feature IDs.

You receive:
1. A catalog of canonical features (id, name, description, synonyms).
2. Draft offline matches (terms found in the book).
3. Sample section excerpts.

Reply with ONLY valid JSON (no markdown fences):
{
  "glossary": [
    {"feature_id": "skill_check", "pdf_term": "Tests", "evidence": "brief quote or section reference"}
  ]
}

Rules:
- Only include mappings you are confident about from the excerpts.
- pdf_term is how THIS book names the concept (may differ from synonyms).
- Use feature_id values from the provided catalog only.
- Omit uncertain mappings rather than guessing.
