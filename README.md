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
  * Uses chromem-go with the configured embedding model to look up relevant sections based on the included PDF documents. Can use the configured local AI to summarize the most relevant sections.
  * The relevant sections should be presented/linked to the user based upon those search results.