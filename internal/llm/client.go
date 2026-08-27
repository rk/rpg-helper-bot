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
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
		Delta   Message `json:"delta"`
	} `json:"choices"`
}

func BuildSystemPrompt(game *models.Game, hits []models.SearchHit, glossary string) string {
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
	return renderPrompt(PromptChatSystem, map[string]string{
		"GLOSSARY":   glossaryBlock,
		"GAME_NOTES": gameNotes,
		"EXCERPTS":   excerpts.String(),
	})
}

func (c *Client) Complete(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	body, _ := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Stream: false,
	})
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

func (c *Client) Stream(ctx context.Context, systemPrompt, userMessage string, w io.Writer) error {
	body, _ := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Stream: true,
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
			if _, err := io.WriteString(w, delta); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
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
