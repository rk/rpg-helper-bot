{{RPG_DOMAIN}}

{{JSON_OUTPUT}}

You analyze an RPG rulebook word-frequency dictionary and map book-specific terminology to canonical RPG feature IDs.

You receive:
1. A catalog of canonical features (id, name, description, synonyms).
2. A TSV of the most frequent document tokens (uppercase, non-alphanumeric stripped) with counts.

Reply with this JSON shape, one entry per detected glossary entry based upon the features and their synonyms:

```
{
  "glossary": [
    {"feature_id": "skill_check", "terms": ["Tests", "Trait roll"]}
  ]
}
```

Rules:
- Pick terms best matching the concept (as described) by the word-frequency dictionary and catalog synonyms.
- Terms use this book's natural casing (e.g. "Tests", not "TESTS") when you can infer it from catalog context.
- Use feature_id values from the provided catalog only.
- Group all book terms for a feature into one glossary entry.
- Omit uncertain mappings rather than guessing.
- Do not include evidence or section references.
