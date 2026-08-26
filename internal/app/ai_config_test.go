package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProvider(t *testing.T) {
	if got := parseProvider("ollama", ProviderOllama); got != ProviderOllama {
		t.Fatalf("got %q", got)
	}
	if got := parseProvider("llamacpp", ProviderLlamaCpp, ProviderOllama); got != ProviderLlamaCpp {
		t.Fatalf("got %q", got)
	}
}

func TestAISettingsFromEnv(t *testing.T) {
	t.Setenv("RPG_HELPER_CHAT_PROVIDER", "ollama")
	t.Setenv("RPG_HELPER_CHAT_MODEL", "mistral")
	t.Setenv("RPG_HELPER_EMBED_PROVIDER", "llama.cpp")
	t.Setenv("RPG_HELPER_EMBED_MODEL", "embed-model")
	t.Setenv("RPG_HELPER_CHAT_URL", "")
	t.Setenv("RPG_HELPER_LLM_URL", "")
	t.Setenv("RPG_HELPER_EMBED_URL", "")

	s := AISettingsFromEnv()
	if s.ChatProvider != ProviderOllama || s.ChatModel != "mistral" {
		t.Fatalf("chat: %+v", s)
	}
	if s.EmbedProvider != ProviderLlamaCpp || s.EmbedModel != "embed-model" {
		t.Fatalf("embed: %+v", s)
	}
	if s.ChatBaseURL != "http://127.0.0.1:11434/v1" {
		t.Fatalf("chat url %q", s.ChatBaseURL)
	}
	if s.EmbedBaseURL != "http://127.0.0.1:8080/v1" {
		t.Fatalf("embed url %q", s.EmbedBaseURL)
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("RPG_HELPER_CHAT_MODEL=from-dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RPG_HELPER_ENV_FILE", envPath)
	os.Unsetenv("RPG_HELPER_CHAT_MODEL")

	LoadDotEnv()
	if os.Getenv("RPG_HELPER_CHAT_MODEL") != "from-dotenv" {
		t.Fatalf("got %q", os.Getenv("RPG_HELPER_CHAT_MODEL"))
	}
}
