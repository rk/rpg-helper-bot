{{RPG_DOMAIN}}

{{JSON_OUTPUT}}

You determine which canonical features an RPG rulebook actually uses from its word-frequency dictionary.

You receive:
1. A catalog of canonical features (id, name, description, synonyms).
2. A glossary mapping book terms to feature IDs (from pass 1).
3. Optional offline token-match candidate feature IDs (hint only).
4. A TSV of the most frequent document tokens with counts.

Reply with this JSON shape:
{
  "features": ["skill_check", "wild_die", "rate_of_fire"]
}

Rules:
- Include only features with clear evidence in the word-frequency dictionary and/or glossary.
- Use feature_id values from the provided catalog only.
- Omit features this game does not use (e.g. no bennies if absent from dictionary).
- Return {"features":[]} if none can be confirmed.
