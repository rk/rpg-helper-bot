package llm

import (
	"context"
	"errors"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

const maxToolRounds = 5

// ToolExecutor runs a named tool with JSON arguments.
type ToolExecutor interface {
	Execute(ctx context.Context, name string, argsJSON string) (string, error)
}

// SearchToolAccumulator exposes search hits accumulated during tool calls.
type SearchToolAccumulator interface {
	ToolExecutor
	SearchHits() []models.SearchHit
	LastDebug() models.ChatSearchDebug
}

// ChatToolsResult holds accumulated search metadata from a tool chat session.
type ChatToolsResult struct {
	SearchHits []models.SearchHit
	Debug      models.ChatSearchDebug
	ToolsUsed  bool
}

// ErrToolsUnsupported indicates the LLM rejected tool calling.
var ErrToolsUnsupported = errors.New("llm tools unsupported")

// ChatToolDefinitions returns OpenAI-compatible tool schemas for rules chat.
func ChatToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "lookup_cheatsheet",
				Description: "Look up indexed cheatsheet summaries for a canonical feature. Call this first to understand how this game implements the feature before searching.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"feature_id": map[string]any{
							"type":        "string",
							"description": "Canonical feature_id from the feature index (e.g. skill_check).",
						},
					},
					"required": []string{"feature_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "lookup_glossary",
				Description: "Look up book-specific terminology and catalog synonyms for a feature or term. Use before search to enrich keywords.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"feature_id": map[string]any{
							"type":        "string",
							"description": "Canonical feature_id from the feature index.",
						},
						"term": map[string]any{
							"type":        "string",
							"description": "Ambiguous book term to resolve to feature(s) and synonyms.",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "search",
				Description: "Search indexed rule PDFs for relevant sections. Returns up to 5 excerpts. Call after cheatsheet and glossary lookups with an enriched query.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Search query using glossary synonyms and question terms.",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}
}

// RunToolLoop executes tool rounds and returns the message history ready for final streaming.
func (c *Client) RunToolLoop(ctx context.Context, systemPrompt, userMessage string, executor ToolExecutor) ([]Message, ChatToolsResult, error) {
	result := ChatToolsResult{}
	tools := ChatToolDefinitions()
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	for round := 0; round < maxToolRounds; round++ {
		assistant, err := c.CompleteWithTools(ctx, messages, tools)
		if err != nil {
			if isToolsUnsupportedError(err) {
				return nil, result, ErrToolsUnsupported
			}
			return nil, result, err
		}
		if len(assistant.ToolCalls) == 0 {
			collectSearchResult(executor, &result)
			return messages, result, nil
		}

		result.ToolsUsed = true
		messages = append(messages, assistant)
		for _, call := range assistant.ToolCalls {
			toolResult, execErr := executor.Execute(ctx, call.Function.Name, call.Function.Arguments)
			if execErr != nil {
				toolResult = "Error: " + execErr.Error()
			}
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    toolResult,
			})
		}
	}

	collectSearchResult(executor, &result)
	return messages, result, nil
}

func collectSearchResult(executor ToolExecutor, result *ChatToolsResult) {
	if acc, ok := executor.(SearchToolAccumulator); ok {
		result.SearchHits = acc.SearchHits()
		result.Debug = acc.LastDebug()
	}
}

func isToolsUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status 400") ||
		strings.Contains(msg, "tools") && strings.Contains(msg, "not supported") ||
		strings.Contains(msg, "tool_use") && strings.Contains(msg, "unsupported")
}
