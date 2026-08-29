package llm_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
)

func TestStreamMessages_emitsDataStreamTextLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	client := &llm.Client{BaseURL: srv.URL, Model: "test", HTTP: srv.Client()}
	var out strings.Builder
	sink := llm.PlainTextSink{W: &out}
	if err := client.StreamMessages(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, sink); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Hello world" {
		t.Fatalf("expected plain sink output, got %q", out.String())
	}
}
