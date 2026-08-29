package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ChatStreamWriter emits Vercel AI SDK v4 data stream parts.
type ChatStreamWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func beginChatStream(w http.ResponseWriter) (*ChatStreamWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Vercel-AI-Data-Stream", "v1")
	w.WriteHeader(http.StatusOK)
	return &ChatStreamWriter{w: w, flusher: flusher}, nil
}

func (s *ChatStreamWriter) WriteStatus(message string) error {
	_ = s.WriteActivityStep(map[string]any{
		"kind":    "status",
		"message": message,
	})
	return s.WriteData(map[string]string{
		"type":    "chat-status",
		"message": message,
	})
}

func (s *ChatStreamWriter) WriteActivityStep(step map[string]any) error {
	return s.WriteData(map[string]any{
		"type": "chat-activity",
		"step": step,
	})
}

func (s *ChatStreamWriter) WriteText(delta string) error {
	if delta == "" {
		return nil
	}
	line, err := json.Marshal(delta)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "0:%s\n", line); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *ChatStreamWriter) WriteData(part any) error {
	payload, err := json.Marshal([]any{part})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "2:%s\n", payload); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func toolStatusMessage(toolName string) string {
	switch toolName {
	case "lookup_glossary":
		return "Looking up glossary…"
	case "lookup_cheatsheet":
		return "Reading cheatsheet…"
	case "search":
		return "Searching rules…"
	default:
		return "Working…"
	}
}

func toolCallArgs(name, argsJSON string) map[string]any {
	args := map[string]any{}
	if argsJSON != "" {
		_ = json.Unmarshal([]byte(argsJSON), &args)
	}
	if len(args) == 0 {
		args["raw"] = argsJSON
	}
	return map[string]any{
		"tool": name,
		"args": args,
	}
}

func truncateActivityPreview(text string, max int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "…"
}
