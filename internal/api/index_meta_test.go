package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

func TestIndexMetaGetPut(t *testing.T) {
	server, db := testServer(t)
	defer db.Close()

	pdf := &models.PDF{Title: "Core", FilePath: "/tmp/core.pdf"}
	if err := db.SavePDF(pdf); err != nil {
		t.Fatal(err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/pdfs/"+pdf.ID+"/index-meta", nil)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", getRec.Code, getRec.Body.String())
	}

	meta := models.PDFIndexMeta{
		Glossary: []models.PDFGlossary{{FeatureID: "skill_check", PDFTerm: "Tests", Evidence: "Support Vs. Test"}},
		Features: []models.PDFFeature{{FeatureID: "skill_check", Terms: []string{"test"}, Sections: []string{"Tests"}}},
	}
	body, _ := json.Marshal(meta)
	putReq := httptest.NewRequest(http.MethodPut, "/api/pdfs/"+pdf.ID+"/index-meta", bytes.NewReader(body))
	putRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put status %d: %s", putRec.Code, putRec.Body.String())
	}

	var saved models.PDFIndexMeta
	if err := json.Unmarshal(putRec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Glossary) != 1 || saved.Glossary[0].PDFTerm != "Tests" {
		t.Fatalf("saved glossary: %+v", saved.Glossary)
	}
}
