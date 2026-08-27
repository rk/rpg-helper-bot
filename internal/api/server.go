package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/indexing"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

type Server struct {
	Store           store.Store
	DataDir         string
	GMPort          int
	PlayerPort      int
	StaticDir       string
	PlayerStaticDir string
	Indexer         *indexing.Service
	Search          *search.Service
	LLM             *llm.Client
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/pdfs", s.handleListPDFs)
	mux.HandleFunc("POST /api/pdfs", s.handleCreatePDF)
	mux.HandleFunc("POST /api/pdfs/upload", s.handleUploadPDF)
	mux.HandleFunc("GET /api/pdfs/{id}", s.handleGetPDF)
	mux.HandleFunc("PATCH /api/pdfs/{id}", s.handlePatchPDF)
	mux.HandleFunc("DELETE /api/pdfs/{id}", s.handleDeletePDF)
	mux.HandleFunc("POST /api/pdfs/{id}/probe", s.handleProbePDF)
	mux.HandleFunc("GET /api/pdfs/{id}/toc", s.handleGetTOC)
	mux.HandleFunc("PUT /api/pdfs/{id}/toc", s.handlePutTOC)

	s.registerTOCRoutes(mux)

	mux.HandleFunc("GET /api/games", s.handleListGames)
	mux.HandleFunc("POST /api/games", s.handleCreateGame)
	mux.HandleFunc("GET /api/games/{id}", s.handleGetGame)
	mux.HandleFunc("PATCH /api/games/{id}", s.handlePatchGame)
	mux.HandleFunc("POST /api/games/{id}/archive", s.handleArchiveGame)
	mux.HandleFunc("POST /api/games/{id}/restore", s.handleRestoreGame)
	mux.HandleFunc("GET /api/games/{id}/pdfs", s.handleGetGamePDFs)
	mux.HandleFunc("PUT /api/games/{id}/pdfs", s.handlePutGamePDFs)
	mux.HandleFunc("POST /api/games/{id}/running", s.handleSetRunning)

	mux.HandleFunc("GET /api/running", s.handleGetRunning)

	s.registerIndexRoutes(mux)
	s.registerIndexMetaRoutes(mux)

	if s.StaticDir != "" {
		fileServer := http.FileServer(http.Dir(s.StaticDir))
		mux.Handle("/", spaHandler(s.StaticDir, fileServer))
	}

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "X-RPG-Sources, X-RPG-Sources-Enc, X-RPG-Search-Debug, X-RPG-Search-Debug-Enc")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func spaHandler(staticDir string, fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/" && !strings.Contains(r.URL.Path, ".") {
			http.ServeFile(w, r, staticDir+"/index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) enrichPDF(p models.PDF) (store.PDFSummary, error) {
	sections, err := s.Store.CountTOCSections(p.ID)
	if err != nil {
		return store.PDFSummary{}, err
	}
	games, err := s.Store.CountPDFGameRefs(p.ID)
	if err != nil {
		return store.PDFSummary{}, err
	}
	return store.PDFSummary{
		PDF:          p,
		PathStatus:   store.ProbePath(p.FilePath),
		SectionCount: sections,
		GameCount:    games,
	}, nil
}

func (s *Server) handleListPDFs(w http.ResponseWriter, r *http.Request) {
	pdfs, err := s.Store.ListPDFs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]store.PDFSummary, 0, len(pdfs))
	for _, p := range pdfs {
		summary, err := s.enrichPDF(p)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, summary)
	}
	writeJSON(w, http.StatusOK, out)
}

type createPDFRequest struct {
	Title             string `json:"title"`
	FilePath          string `json:"file_path"`
	TOCSource          string `json:"toc_source,omitempty"` // "pages" or "bookmarks"
	TOCStartPage       *int   `json:"toc_start_page,omitempty"`
	TOCEndPage         *int   `json:"toc_end_page,omitempty"`
	TOCIncludeChildren *bool  `json:"toc_include_children,omitempty"` // legacy
	TOCBookmarkMaxDepth *int  `json:"toc_bookmark_depth,omitempty"`
}

type createPDFResponse struct {
	store.PDFSummary
	TOCExtractError string `json:"toc_extract_error,omitempty"`
}

func (s *Server) handleCreatePDF(w http.ResponseWriter, r *http.Request) {
	var req createPDFRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p := &models.PDF{Title: req.Title, FilePath: req.FilePath}
	if err := s.Store.SavePDF(p); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	maxDepth := 2
	if req.TOCBookmarkMaxDepth != nil {
		maxDepth = indexing.ResolveBookmarkMaxDepth(req.TOCBookmarkMaxDepth, nil)
	} else if req.TOCIncludeChildren != nil {
		maxDepth = indexing.ResolveBookmarkMaxDepth(nil, req.TOCIncludeChildren)
	}
	start, end := 0, 0
	if req.TOCStartPage != nil {
		start = *req.TOCStartPage
	}
	if req.TOCEndPage != nil {
		end = *req.TOCEndPage
	}
	s.respondCreatePDF(w, p, pdfImportOptions{
		TOCSource:           req.TOCSource,
		TOCStartPage:        start,
		TOCEndPage:          end,
		TOCBookmarkMaxDepth: maxDepth,
	})
}

func (s *Server) handleGetPDF(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.Store.GetPDF(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	summary, err := s.enrichPDF(*p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

type patchPDFRequest struct {
	Title     *string `json:"title"`
	FilePath  *string `json:"file_path"`
	PageCount *int    `json:"page_count"`
}

func (s *Server) handlePatchPDF(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.Store.GetPDF(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	var req patchPDFRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Title != nil {
		p.Title = *req.Title
	}
	if req.FilePath != nil {
		p.FilePath = *req.FilePath
	}
	if req.PageCount != nil {
		p.PageCount = *req.PageCount
	}
	if err := s.Store.SavePDF(p); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	summary, err := s.enrichPDF(*p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleDeletePDF(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pdf, err := s.Store.GetPDF(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err := s.Store.DeletePDF(id); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	s.removeManagedPDF(pdf.FilePath)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProbePDF(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.Store.GetPDF(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	status := store.ProbePath(p.FilePath)
	pageCount := p.PageCount
	if status == models.PathStatusOK {
		if err := indexing.ToolsAvailable(); err == nil {
			if n, err := indexing.PageCount(p.FilePath); err == nil && n > 0 {
				pageCount = n
				if p.PageCount != n {
					p.PageCount = n
					_ = s.Store.SavePDF(p)
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path_status": status,
		"page_count":  pageCount,
	})
}

func (s *Server) handleGetTOC(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Store.GetPDF(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	sections, err := s.Store.ListTOCSections(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if sections == nil {
		sections = []models.TOCSection{}
	}
	writeJSON(w, http.StatusOK, sections)
}

type putTOCRequest struct {
	Sections []models.TOCSection `json:"sections"`
}

func (s *Server) handlePutTOC(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Store.GetPDF(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	var req putTOCRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.SaveTOCSections(id, req.Sections); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sections, err := s.Store.ListTOCSections(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sections)
}

func (s *Server) gameSummary(g models.Game) (store.GameSummary, error) {
	pdfs, err := s.Store.ListGamePDFs(g.ID)
	if err != nil {
		return store.GameSummary{}, err
	}
	return store.GameSummary{Game: g, PDFCount: len(pdfs)}, nil
}

func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	filter := store.ListActive
	switch r.URL.Query().Get("filter") {
	case "archived":
		filter = store.ListArchived
	case "all":
		filter = store.ListAll
	}
	games, err := s.Store.ListGames(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]store.GameSummary, 0, len(games))
	for _, g := range games {
		summary, err := s.gameSummary(g)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, summary)
	}
	writeJSON(w, http.StatusOK, out)
}

type createGameRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var req createGameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	g := &models.Game{Name: req.Name}
	if err := s.Store.SaveGame(g); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	summary, err := s.gameSummary(*g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, summary)
}

type gameDetailResponse struct {
	store.GameSummary
	PDFs []store.GamePDFEntry `json:"pdfs"`
}

func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	g, err := s.Store.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	summary, err := s.gameSummary(*g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	pdfs, err := s.Store.ListGamePDFs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if pdfs == nil {
		pdfs = []store.GamePDFEntry{}
	}
	writeJSON(w, http.StatusOK, gameDetailResponse{GameSummary: summary, PDFs: pdfs})
}

type patchGameRequest struct {
	Name  *string `json:"name"`
	Notes *string `json:"notes"`
}

func (s *Server) handlePatchGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	g, err := s.Store.GetGame(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	var req patchGameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name != nil {
		g.Name = *req.Name
	}
	if req.Notes != nil {
		g.Notes = *req.Notes
	}
	if err := s.Store.SaveGame(g); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleArchiveGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Store.ArchiveGame(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	g, err := s.Store.GetGame(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleRestoreGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Store.RestoreGame(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	g, err := s.Store.GetGame(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleGetGamePDFs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Store.GetGame(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	pdfs, err := s.Store.ListGamePDFs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if pdfs == nil {
		pdfs = []store.GamePDFEntry{}
	}
	writeJSON(w, http.StatusOK, pdfs)
}

type putGamePDFsRequest struct {
	PDFIDs []string `json:"pdf_ids"`
}

func (s *Server) handlePutGamePDFs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Store.GetGame(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	var req putGamePDFsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.SetGamePDFs(id, req.PDFIDs); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	pdfs, err := s.Store.ListGamePDFs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, pdfs)
}

type setRunningRequest struct {
	Running bool `json:"running"`
}

type runningResponse struct {
	Game       *models.Game `json:"game"`
	PlayerURL  string       `json:"player_url"`
	PlayerNote string       `json:"player_note"`
}

func (s *Server) handleSetRunning(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req setRunningRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	g, err := s.Store.SetGameRunning(id, req.Running)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, s.runningPayload(g))
}

func (s *Server) handleGetRunning(w http.ResponseWriter, r *http.Request) {
	g, err := s.Store.GetRunningGame()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if g == nil {
		writeJSON(w, http.StatusOK, runningResponse{Game: nil, PlayerURL: "", PlayerNote: ""})
		return
	}
	writeJSON(w, http.StatusOK, s.runningPayload(g))
}

func (s *Server) runningPayload(g *models.Game) runningResponse {
	resp := runningResponse{
		Game:       g,
		PlayerNote: "Player UI available on the LAN player port",
	}
	if g != nil && g.Running {
		resp.PlayerURL = playerURL(s.PlayerPort)
	}
	return resp
}

func playerURL(port int) string {
	ip := lanIP()
	if ip == "" {
		ip = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(ip, strconv.Itoa(port)) + "/"
}

func lanIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip != nil {
				return ip.String()
			}
		}
	}
	return ""
}
