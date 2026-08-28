package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/chat"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/indexing"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/textutil"
)

type Services struct {
	Indexer *indexing.Service
	Search  *search.Service
	LLM     *llm.Client
}

func (s *Server) registerIndexRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/pdfs/{id}/index", s.handleIndexPDF)
	mux.HandleFunc("GET /api/pdfs/{id}/index/progress", s.handleIndexProgress)
	mux.HandleFunc("GET /api/pdfs/{id}/thumbnail", s.handlePDFThumbnail)
}

func (s *Server) handleIndexProgress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSON(w, http.StatusOK, indexing.GetIndexProgress(id))
}

func (s *Server) handleIndexPDF(w http.ResponseWriter, r *http.Request) {
	if s.Indexer == nil {
		writeError(w, http.StatusServiceUnavailable, errServiceUnavailable("indexing"))
		return
	}
	id := r.PathValue("id")
	result, err := s.Indexer.IndexPDF(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePDFThumbnail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pdf, err := s.Store.GetPDF(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if pdf.ThumbnailPath == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, pdf.ThumbnailPath)
}

type chatRequestBody struct {
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Parts   []chatPart `json:"parts"`
}

type chatPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (m chatMessage) text() string {
	if strings.TrimSpace(m.Content) != "" {
		return m.Content
	}
	var b strings.Builder
	for _, p := range m.Parts {
		if p.Type == "text" && p.Text != "" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func decodeChatJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func chatLearnings(st store.Store, gameID, userQuery string) search.ChatLearnings {
	learnings, err := search.LearningsForChat(st, gameID, userQuery)
	if err != nil {
		return search.ChatLearnings{}
	}
	return learnings
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	running, err := s.Store.GetRunningGame()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if running == nil {
		writeError(w, http.StatusBadRequest, errNoRunningGame())
		return
	}

	var req chatRequestBody
	if err := decodeChatJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	userMsg := lastUserMessage(req.Messages)
	if userMsg == "" {
		writeError(w, http.StatusBadRequest, errEmptyMessage())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	if s.LLM != nil {
		if ok := s.handleChatWithTools(r.Context(), w, running, userMsg); ok {
			return
		}
	}

	hits, searchResult, err := prefetchChatContext(r.Context(), s, running.ID, userMsg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeChatHeaders(w, hits, searchResult, s.LLM != nil)

	learnings := chatLearnings(s.Store, running.ID, userMsg)
	systemPrompt := llm.BuildSystemPrompt(running, hits, learnings.Glossary, learnings.Cheatsheet)
	if s.LLM != nil {
		if err := s.LLM.Stream(r.Context(), systemPrompt, userMsg, w); err == nil {
			return
		}
	}

	answer := llm.FallbackAnswer(userMsg, hits)
	_, _ = w.Write([]byte(answer))
}

func (s *Server) handleChatWithTools(ctx context.Context, w http.ResponseWriter, running *models.Game, userMsg string) bool {
	featureIndex, err := chat.BuildFeatureIndex(s.Store, running.ID)
	if err != nil {
		return false
	}
	systemPrompt := llm.BuildToolChatSystemPrompt(running, featureIndex)
	tools := chat.NewTools(s.Store, s.Search, running.ID)

	messages, result, err := s.LLM.RunToolLoop(ctx, systemPrompt, userMsg, tools)
	if errors.Is(err, llm.ErrToolsUnsupported) {
		return false
	}
	if err != nil {
		return false
	}
	if !result.ToolsUsed {
		return false
	}

	hits := result.SearchHits
	var searchResult *search.Result
	searchResult = &search.Result{Hits: hits, Debug: result.Debug}
	writeChatHeaders(w, hits, searchResult, true)

	if err := s.LLM.StreamMessages(ctx, messages, w); err != nil {
		return false
	}
	return true
}

func prefetchChatContext(ctx context.Context, s *Server, gameID, userMsg string) ([]models.SearchHit, *search.Result, error) {
	var searchResult *search.Result
	if s.Search != nil {
		var err error
		searchResult, err = s.Search.Search(ctx, gameID, userMsg)
		if err != nil {
			return nil, nil, err
		}
	}

	var hits []models.SearchHit
	if searchResult != nil {
		hits = searchResult.Hits
	}
	if s.Store != nil {
		if citationHits, err := search.CheatsheetCitationHits(s.Store, gameID, userMsg); err == nil && len(citationHits) > 0 {
			hits = search.MergeSearchHits(citationHits, hits, 10)
		}
	}
	return hits, searchResult, nil
}

func writeChatHeaders(w http.ResponseWriter, hits []models.SearchHit, searchResult *search.Result, llmConfigured bool) {
	sources := make([]models.ChatSource, 0, len(hits))
	for _, h := range hits {
		sources = append(sources, models.ChatSource{
			SectionID:    h.SectionID,
			PDFTitle:     h.PDFTitle,
			SectionTitle: h.SectionTitle,
			StartPage:    h.StartPage,
			EndPage:      h.EndPage,
			Snippet:      textutil.NormalizePDFText(h.Snippet),
		})
	}
	if b, err := json.Marshal(sources); err == nil {
		w.Header().Set("X-RPG-Sources", base64.StdEncoding.EncodeToString(b))
		w.Header().Set("X-RPG-Sources-Enc", "base64")
	}

	if searchResult != nil {
		debug := searchResult.Debug
		debug.LLMConfigured = llmConfigured
		if b, err := json.Marshal(debug); err == nil {
			w.Header().Set("X-RPG-Search-Debug", base64.StdEncoding.EncodeToString(b))
			w.Header().Set("X-RPG-Search-Debug-Enc", "base64")
		}
	}
}

func lastUserMessage(messages []chatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].text()
		}
	}
	return ""
}

func (s *Server) PlayerHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/running", s.handleGetRunning)
	mux.HandleFunc("POST /api/chat", s.handleChat)

	if s.PlayerStaticDir != "" {
		fileServer := http.FileServer(http.Dir(s.PlayerStaticDir))
		mux.Handle("/", spaHandler(s.PlayerStaticDir, fileServer))
	}
	return withCORS(mux)
}

func errServiceUnavailable(feature string) error {
	return &apiError{msg: feature + " service unavailable"}
}

func errNoRunningGame() error {
	return &apiError{msg: "no game is currently running"}
}

func errEmptyMessage() error {
	return &apiError{msg: "message is required"}
}

type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }

func resolvePlayerStaticDir() string {
	if dir := os.Getenv("RPG_HELPER_PLAYER_STATIC_DIR"); dir != "" {
		return dir
	}
	candidates := []string{"web-player/dist", filepath.Join("..", "web-player", "dist")}
	for _, dir := range candidates {
		if info, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs
			}
			return dir
		}
	}
	return "web-player/dist"
}
