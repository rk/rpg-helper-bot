package app

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Provider string

const (
	ProviderLlamaCpp Provider = "llama.cpp"
	ProviderOllama   Provider = "ollama"
	ProviderHash     Provider = "hash"
)

type AISettings struct {
	ChatProvider Provider
	ChatModel    string
	ChatBaseURL  string

	EmbedProvider Provider
	EmbedModel    string
	EmbedBaseURL  string
}

const (
	defaultLlamaCppURL = "http://127.0.0.1:8080/v1"
	defaultOllamaURL   = "http://127.0.0.1:11434"
)

// LoadDotEnv loads the first .env file found. Missing files are ignored.
func LoadDotEnv() {
	if path := os.Getenv("RPG_HELPER_ENV_FILE"); path != "" {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
	_ = godotenv.Load(".env")
}

func AISettingsFromEnv() AISettings {
	chatProvider := parseProvider(envOr("RPG_HELPER_CHAT_PROVIDER", string(ProviderLlamaCpp)), ProviderLlamaCpp, ProviderOllama)
	embedProvider := parseProvider(envOr("RPG_HELPER_EMBED_PROVIDER", string(ProviderOllama)), ProviderOllama, ProviderLlamaCpp, ProviderHash)

	chatModel := envOr("RPG_HELPER_CHAT_MODEL", defaultChatModel(chatProvider))
	embedModel := envOr("RPG_HELPER_EMBED_MODEL", defaultEmbedModel(embedProvider))

	return AISettings{
		ChatProvider:  chatProvider,
		ChatModel:     chatModel,
		ChatBaseURL:   chatBaseURL(chatProvider),
		EmbedProvider: embedProvider,
		EmbedModel:    embedModel,
		EmbedBaseURL:  embedBaseURL(embedProvider),
	}
}

func LogAISettings(s AISettings) {
	log.Printf("AI chat: provider=%s model=%s url=%s", s.ChatProvider, s.ChatModel, s.ChatBaseURL)
	log.Printf("AI embed: provider=%s model=%s url=%s", s.EmbedProvider, s.EmbedModel, s.EmbedBaseURL)
	if d := strings.TrimSpace(os.Getenv("RPG_HELPER_PROMPTS_DIR")); d != "" {
		log.Printf("AI prompts dir: %s (from RPG_HELPER_PROMPTS_DIR)", d)
	}
}

func parseProvider(raw string, allowed ...Provider) Provider {
	n := normalizeProvider(raw)
	for _, p := range allowed {
		if n == string(p) || normalizeProvider(string(p)) == n {
			return p
		}
	}
	return allowed[0]
}

func normalizeProvider(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "_", ".")
	raw = strings.ReplaceAll(raw, " ", "")
	switch raw {
	case "llamacpp", "llama":
		return string(ProviderLlamaCpp)
	case "local", "none", "offline":
		return string(ProviderHash)
	default:
		return raw
	}
}

func defaultChatModel(p Provider) string {
	switch p {
	case ProviderOllama:
		return "llama3.2"
	default:
		return "local-model"
	}
}

func defaultEmbedModel(p Provider) string {
	switch p {
	case ProviderLlamaCpp:
		return "local-embed"
	default:
		return "nomic-embed-text"
	}
}

func chatBaseURL(p Provider) string {
	if v := os.Getenv("RPG_HELPER_CHAT_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	// Legacy override
	if v := os.Getenv("RPG_HELPER_LLM_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	switch p {
	case ProviderOllama:
		return defaultOllamaURL + "/v1"
	default:
		return defaultLlamaCppURL
	}
}

func embedBaseURL(p Provider) string {
	if v := os.Getenv("RPG_HELPER_EMBED_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	switch p {
	case ProviderLlamaCpp:
		return defaultLlamaCppURL
	default:
		return defaultOllamaURL
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// Legacy helpers kept for callers that still use env directly.
func LLMBaseURL() string  { return AISettingsFromEnv().ChatBaseURL }
func LLMModel() string     { return AISettingsFromEnv().ChatModel }
func EmbedBaseURL() string { return AISettingsFromEnv().EmbedBaseURL }
func EmbedModel() string   { return AISettingsFromEnv().EmbedModel }
