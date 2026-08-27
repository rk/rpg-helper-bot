package llm

import (
	"encoding/json"
	"testing"
)

func TestCompleteJSONRequestBody(t *testing.T) {
	c := &Client{Model: "test-model"}
	body, err := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "user"},
		},
		Stream:         false,
		Format:         "json",
		ResponseFormat: &responseFormat{Type: "json_object"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if m["format"] != "json" {
		t.Fatalf("format: %v", m["format"])
	}
	rf, ok := m["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_object" {
		t.Fatalf("response_format: %v", m["response_format"])
	}
}
