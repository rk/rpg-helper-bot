package llm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

type mockExecutor struct {
	calls []string
	hits  []models.SearchHit
}

func (m *mockExecutor) Execute(_ context.Context, name string, argsJSON string) (string, error) {
	m.calls = append(m.calls, name+":"+argsJSON)
	if name == "search" {
		m.hits = []models.SearchHit{{SectionID: "sec-1", PDFTitle: "Core", SectionTitle: "Tests", StartPage: 1, EndPage: 2, Snippet: "roll dice"}}
		return "[1] Core — Tests (pages 1-2)\nroll dice", nil
	}
	return "ok", nil
}

func (m *mockExecutor) SearchHits() []models.SearchHit { return m.hits }
func (m *mockExecutor) LastDebug() models.ChatSearchDebug {
	return models.ChatSearchDebug{OriginalQuery: "skill check"}
}

func TestRunToolLoop_executesToolsThenReturnsMessages(t *testing.T) {
	var thinking []string
	var toolResults []string
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_cheatsheet","arguments":"{\"feature_id\":\"skill_check\"}"}}]}}]}`))
			return
		}
		if call == 2 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_2","type":"function","function":{"name":"search","arguments":"{\"query\":\"skill check\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""}}]}`))
	}))
	defer srv.Close()

	client := &llm.Client{BaseURL: srv.URL, Model: "test", HTTP: srv.Client()}
	exec := &mockExecutor{}
	messages, result, err := client.RunToolLoop(context.Background(), "system", "how do skill checks work?", exec, &llm.ToolLoopOptions{
		OnThinking: func(content string) { thinking = append(thinking, content) },
		OnToolResult: func(name, _ string, result string, _ error) {
			toolResults = append(toolResults, name+":"+result)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ToolsUsed {
		t.Fatal("expected tools to be used")
	}
	if len(result.SearchHits) != 1 {
		t.Fatalf("expected 1 search hit, got %d", len(result.SearchHits))
	}
	if len(exec.calls) != 2 {
		t.Fatalf("expected 2 tool executions, got %d: %v", len(exec.calls), exec.calls)
	}
	if len(toolResults) != 2 {
		t.Fatalf("expected 2 tool results, got %d", len(toolResults))
	}
	if len(messages) < 4 {
		t.Fatalf("expected expanded message history, got %d messages", len(messages))
	}

	call = 0
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Final answer\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	var out strings.Builder
	if err := client.StreamMessages(context.Background(), messages, llm.PlainTextSink{W: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Final answer") {
		t.Fatalf("expected streamed answer, got %q", out.String())
	}
}

func TestCompleteWithTools_parsesToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		tools, _ := body["tools"].([]any)
		if len(tools) != 3 {
			t.Fatalf("expected 3 tools in request, got %v", body["tools"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"search","arguments":"{\"query\":\"test\"}"}}]}}]}`))
	}))
	defer srv.Close()

	client := &llm.Client{BaseURL: srv.URL, Model: "test", HTTP: srv.Client()}
	msg, err := client.CompleteWithTools(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, llm.ChatToolDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "search" {
		t.Fatalf("unexpected tool calls: %+v", msg.ToolCalls)
	}
}

func TestBuildToolChatSystemPrompt(t *testing.T) {
	game := &models.Game{Notes: "Use wild die."}
	prompt := llm.BuildToolChatSystemPrompt(game, "- skill_check: Skill Check")
	if !strings.Contains(prompt, "lookup_cheatsheet") || !strings.Contains(prompt, "skill_check") || !strings.Contains(prompt, "wild die") {
		t.Fatalf("unexpected prompt: %q", prompt)
	}
}
