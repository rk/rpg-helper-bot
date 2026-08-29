package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatStreamWriter_statusAndText(t *testing.T) {
	rec := httptest.NewRecorder()
	stream, err := beginChatStream(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.WriteStatus("Analyzing question…"); err != nil {
		t.Fatal(err)
	}
	if err := stream.WriteActivityStep(map[string]any{
		"kind": "tool-call",
		"call": toolCallArgs("search", `{"query":"soak"}`),
	}); err != nil {
		t.Fatal(err)
	}
	if err := stream.WriteText("Hello"); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"chat-status"`) || !strings.Contains(body, "Analyzing question") {
		t.Fatalf("expected status part: %q", body)
	}
	if !strings.Contains(body, `"kind":"tool-call"`) {
		t.Fatalf("expected tool-call activity part: %q", body)
	}
	if !strings.Contains(body, "0:\"Hello\"\n") {
		t.Fatalf("expected text part: %q", body)
	}
	if rec.Header().Get("X-Vercel-AI-Data-Stream") != "v1" {
		t.Fatalf("expected data stream header, got %q", rec.Header().Get("X-Vercel-AI-Data-Stream"))
	}
}
