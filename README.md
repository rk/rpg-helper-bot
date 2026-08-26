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
* **Hybrid search (Recipe 3)** — FTS5 BM25 candidates, embedding rerank (Ollama with hash fallback), PDF override order on ties.
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