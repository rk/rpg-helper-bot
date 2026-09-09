# rpg-helper-bot
A helpful bot that crawls your PDF rules and helps answer questions from your players.

## Base Application

The main executable exposes a small webserver API for a simple React web app (running on Deno) to consume. Together they provide basic management features for the Game Master (GM).

The GM can manage:

* Adding and archiving Games.
  * Games are considered Active unless archived.
  * Each game can include one or more PDF.
* Adding and indexing PDFs.
  * Each PDF will be composed of a Table of Contents that is user defined, which is an array of records with a title, start page number, and end page number.
  * Each Table of Contents section will have its content extracted by pdftotext, stored as both plain text and a vector store using chromem-go.
  * Each PDF will have a thumbnail rendered from the first page around 200px maximum size.
* The GM can mark a Game as Running, which exposes a small web front-end:
  * Uses the Vercel AI SDK to present a chatbot.
  * Uses llama.cpp for local AI.
  * Uses sqlite (for full text searching) and chromem-go with the configured embedding model to look up relevant sections based on the included PDF documents. Can use the configured local AI to summarize the most relevant sections.
  * The relevant sections should be presented/linked to the user based upon those search results.

## Roadmap

Implemented in the current prototype:

* **PDF indexing** — `pdftotext` extraction per TOC section; plain text in SQLite FTS5 + chromem-go vector collection.
* **Thumbnails** — first-page PNG preview (~200px) via `pdftoppm`; served at `/api/pdfs/{id}/thumbnail`.
* **Vector search (exploratory branch)** — chromem-go nearest-neighbor retrieval over indexed section embeddings; optional LLM query expansion; heuristic rerank + PDF override order on ties.
* **Running-game player UI** — LAN player app on port 8766 (`web-player/`).
* **Rules Q&A chat** — Vercel AI SDK player UI; streams from `/api/chat` with llama.cpp OpenAI-compatible API (fallback when unavailable).
* **Answer presentation** — `X-RPG-Sources` header + source list in player UI; answers cite matching sections with page ranges.

### Run

Run all `make` commands from the **repository root** (`rpg-helper-bot/`), not from `web/`.

```bash
# Requires poppler (pdftotext, pdftoppm, pdfinfo)
make build   # GM UI + player UI + Go binary
make run     # build, then start the server
```

`make build` reporting "Nothing to be done" means outputs are already up to date; use `make clean && make build` to force a full rebuild.

Optional: run [llama.cpp server](https://github.com/ggerganov/llama.cpp) and/or [Ollama](https://ollama.com). Copy [`.env.example`](.env.example) to `.env` to choose providers and models:

```env
RPG_HELPER_CHAT_PROVIDER=llama.cpp
RPG_HELPER_CHAT_MODEL=local-model
RPG_HELPER_EMBED_PROVIDER=ollama
RPG_HELPER_EMBED_MODEL=nomic-embed-text
```

Chat and embedding providers are configured independently (`llama.cpp`, `ollama`, or `hash` for offline embeddings).

## RPG concept cheatsheet

[`data/rpg-concepts.yaml`](data/rpg-concepts.yaml) defines canonical RPG features, synonym families, and optional **questions** the cheatsheet pass should try to answer (skill check, rate of fire, wild die, etc.). Extend this file as you encounter new cross-book terminology patterns.

During PDF indexing, the app runs a **3-pass LLM pipeline** (when the chat LLM is configured):

1. **Glossary** — builds a word-frequency dictionary from the full indexed text (offline), then one LLM call maps top tokens to catalog feature terms.
2. **Feature detection** — one LLM call over word-frequency TSV + glossary (offline token-match candidates as hints).
3. **Cheatsheet** — per detected feature, a concise definition plus citation list (section title + page range).

Results are stored in `pdfs.index_meta` (SQLite) as:

- **Glossary:** `[{ feature_id, terms[] }]`
- **Features:** `string[]` of feature IDs present in this PDF
- **Cheatsheet:** `[{ feature_id, definition, citations[] }]`

If the LLM is unavailable, learnings are empty and `llm_learnings_skipped` is set.

Glossary and feature detection each use a single LLM call over the top **1000** word-frequency tokens. Cheatsheet uses **hybrid search** (FTS + embedding rerank) to find the best sections per feature, then **one LLM call per feature** to summarize. Each LLM call has a **120s** timeout.

**Re-index PDFs** after deploying schema or prompt changes to regenerate learnings.

Override the concepts file path with `RPG_HELPER_CONCEPTS_FILE`.

## Search behavior

| Stage | Query used |
|-------|------------|
| Embedding similarity (rerank) | Original user question only |
| Title boost / table penalty | Original user question only |
| FTS keyword retrieval | Original + LLM rewrite (with glossary context) + post-rewrite glossary expansion + preserved dice tokens |
| Snippet highlighting | Original user question |

Dice notation (`2D6`, `d8`, `2D6-2`, dice-pool `2D`) is preserved in FTS queries and passed to LLMs without normalization.

## LLM system prompts

All chat-model system prompts are customizable. Defaults live in [`prompts/`](prompts/):

| Prompt | Default file | Env override | When used |
|--------|--------------|--------------|-----------|
| RPG domain (shared) | `prompts/shared/rpg-domain.md` | `RPG_HELPER_PROMPT_RPG_DOMAIN` | Prepended to all prompts below |
| Chat / rules Q&A | `prompts/chat-system.md` | `RPG_HELPER_PROMPT_CHAT` | `/api/chat` streaming answers |
| Search rewrite | `prompts/search-rewrite-system.md` | `RPG_HELPER_PROMPT_SEARCH_REWRITE` | FTS keyword expansion; receives indexed glossary terms for PDF-specific synonyms |
| Glossary extract | `prompts/glossary-extract-system.md` | `RPG_HELPER_PROMPT_GLOSSARY` | Index pass 1: book terminology |
| Feature detect | `prompts/feature-detect-system.md` | `RPG_HELPER_PROMPT_FEATURE_DETECT` | Index pass 2: feature presence |
| Cheatsheet extract | `prompts/cheatsheet-extract-system.md` | `RPG_HELPER_PROMPT_CHEATSHEET` | Index pass 3: search-ranked sections + one LLM summarize per feature |

Set `RPG_HELPER_PROMPTS_DIR` to use an alternate prompts directory. Individual env vars override specific files. Embedded defaults apply if files are missing.

Chat prompt placeholders: `{{RPG_DOMAIN}}`, `{{GLOSSARY}}`, `{{GAME_NOTES}}`, `{{EXCERPTS}}`.
