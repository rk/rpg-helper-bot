package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/api"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func testServer(t *testing.T) (*api.Server, *store.SQLiteStore) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "api.db")
	s, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	return &api.Server{Store: s, PlayerPort: 8766}, s
}

func TestHealth(t *testing.T) {
	server, db := testServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPDFCreateAndProbe(t *testing.T) {
	server, db := testServer(t)
	defer db.Close()

	body, _ := json.Marshal(map[string]string{
		"title":     "Rules",
		"file_path": "/no/such/file.pdf",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pdfs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}

	var pdf store.PDFSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &pdf); err != nil {
		t.Fatal(err)
	}
	if pdf.PathStatus != models.PathStatusMissing {
		t.Fatalf("expected missing path, got %s", pdf.PathStatus)
	}
}

func TestRunningExclusive(t *testing.T) {
	server, db := testServer(t)
	defer db.Close()

	createGame := func(name string) string {
		body, _ := json.Marshal(map[string]string{"name": name})
		req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create game %s: %d %s", name, rec.Code, rec.Body.String())
		}
		var g struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
			t.Fatal(err)
		}
		return g.ID
	}

	g1 := createGame("One")
	g2 := createGame("Two")

	setRunning := func(id string) {
		body, _ := json.Marshal(map[string]bool{"running": true})
		req := httptest.NewRequest(http.MethodPost, "/api/games/"+id+"/running", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("running %s: %d %s", id, rec.Code, rec.Body.String())
		}
	}

	setRunning(g1)
	setRunning(g2)

	req := httptest.NewRequest(http.MethodGet, "/api/running", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	var resp struct {
		Game *models.Game `json:"game"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Game == nil || resp.Game.ID != g2 {
		t.Fatalf("expected g2 running, got %+v", resp.Game)
	}
}

func TestChatAcceptsAISDKMessages(t *testing.T) {
	server, db := testServer(t)
	defer db.Close()

	gameBody, _ := json.Marshal(map[string]string{"name": "Running Table"})
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(gameBody))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	var game struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &game); err != nil {
		t.Fatal(err)
	}

	runBody, _ := json.Marshal(map[string]bool{"running": true})
	req = httptest.NewRequest(http.MethodPost, "/api/games/"+game.ID+"/running", bytes.NewReader(runBody))
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("running status %d: %s", rec.Code, rec.Body.String())
	}

	chatBody := []byte(`{
		"messages": [
			{"id":"msg-1","role":"user","content":"What edges help soak rolls?"}
		]
	}`)
	req = httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(chatBody))
	rec = httptest.NewRecorder()
	server.PlayerHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") == "application/json" {
		t.Fatalf("expected text response, got json error: %s", rec.Body.String())
	}
}
