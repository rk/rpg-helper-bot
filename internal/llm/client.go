package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/app"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

type Client struct {
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewClient() *Client {
	s := app.AISettingsFromEnv()
	return &Client{
		BaseURL: s.ChatBaseURL,
		Model:   s.ChatModel,
		HTTP:    &http.Client{Timeout: 180 * time.Second},
	}
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatRequest struct {
	Model          string           `json:"model"`
	Messages       []Message        `json:"messages"`
	Stream         bool             `json:"stream"`
	Tools          []ToolDefinition `json:"tools,omitempty"`
	ToolChoice     any              `json:"tool_choice,omitempty"`
	Format         string           `json:"format,omitempty"`
	ResponseFormat *responseFormat  `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
		Delta   Message `json:"delta"`
	} `json:"choices"`
}

func BuildSystemPrompt(game *models.Game, hits []models.SearchHit, glossary, cheatsheet string) string {
	var excerpts strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&excerpts, "\n[%d] %s — %s (pages %d-%d)\n%s\n",
			i+1, h.PDFTitle, h.SectionTitle, h.StartPage, h.EndPage, h.Snippet)
	}
	gameNotes := ""
	if game != nil && strings.TrimSpace(game.Notes) != "" {
		gameNotes = "Table notes / optional rules in use:\n" + game.Notes + "\n"
	}
	glossaryBlock := ""
	if strings.TrimSpace(glossary) != "" {
		glossaryBlock = strings.TrimSpace(glossary) + "\n"
	}
	cheatsheetBlock := ""
	if strings.TrimSpace(cheatsheet) != "" {
		cheatsheetBlock = strings.TrimSpace(cheatsheet) + "\n"
	}
	return renderPrompt(PromptChatSystem, map[string]string{
		"GLOSSARY":   glossaryBlock,
		"CHEATSHEET": cheatsheetBlock,
		"GAME_NOTES": gameNotes,
		"EXCERPTS":   excerpts.String(),
	})
}

func BuildToolChatSystemPrompt(game *models.Game, featureIndex string) string {
	gameNotes := ""
	if game != nil && strings.TrimSpace(game.Notes) != "" {
		gameNotes = "Table notes / optional rules in use:\n" + game.Notes + "\n"
	}
	featureBlock := ""
	if strings.TrimSpace(featureIndex) != "" {
		featureBlock = strings.TrimSpace(featureIndex) + "\n"
	}
	return renderPrompt(PromptChatSystemTools, map[string]string{
		"FEATURE_INDEX": featureBlock,
		"GAME_NOTES":    gameNotes,
	})
}

func (c *Client) Complete(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return c.complete(ctx, systemPrompt, userMessage, false)
}

// CompleteJSON requests structured JSON from the chat API (Ollama format + OpenAI response_format).
func (c *Client) CompleteJSON(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return c.complete(ctx, systemPrompt, userMessage, true)
}

func (c *Client) complete(ctx context.Context, systemPrompt, userMessage string, jsonMode bool) (string, error) {
	reqBody := chatRequest{
		Model: c.Model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Stream: false,
	}
	if jsonMode {
		reqBody.Format = "json"
		reqBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty llm response")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func (c *Client) Stream(ctx context.Context, systemPrompt, userMessage string, sink TextStreamSink) error {
	return c.StreamMessages(ctx, []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}, sink)
}

func (c *Client) StreamMessages(ctx context.Context, messages []Message, sink TextStreamSink) error {
	body, _ := json.Marshal(chatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llm status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var chunk chatResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta != "" {
			if err := sink.WriteText(delta); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// CompleteWithTools sends a non-streaming chat completion with tool definitions.
func (c *Client) CompleteWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	reqBody := chatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return Message{}, fmt.Errorf("llm status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Message{}, err
	}
	if len(out.Choices) == 0 {
		return Message{}, fmt.Errorf("empty llm response")
	}
	return out.Choices[0].Message, nil
}

func FallbackAnswer(userMessage string, hits []models.SearchHit) string {
	if len(hits) == 0 {
		return "I couldn't find relevant rules for that question in the indexed PDFs for this game."
	}
	var b strings.Builder
	b.WriteString("Based on the indexed rules (chat model unavailable — showing retrieval results only):\n\n")
	for i, h := range hits {
		fmt.Fprintf(&b, "%d. **%s — %s** (pages %d-%d)\n   %s\n\n", i+1, h.PDFTitle, h.SectionTitle, h.StartPage, h.EndPage, h.Snippet)
	}
	b.WriteString("\nQuestion: ")
	b.WriteString(userMessage)
	return b.String()
}
