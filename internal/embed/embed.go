package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/app"
)

type Func func(ctx context.Context, text string) ([]float32, error)

type Config struct {
	Provider app.Provider
	BaseURL  string
	Model    string
	Fallback Func
}

func NewFuncFromEnv() Func {
	s := app.AISettingsFromEnv()
	return NewFunc(Config{
		Provider: s.EmbedProvider,
		BaseURL:  s.EmbedBaseURL,
		Model:    s.EmbedModel,
	})
}

func NewFunc(cfg Config) Func {
	if cfg.Provider == app.ProviderHash {
		return HashEmbed
	}

	fallback := cfg.Fallback
	if fallback == nil {
		fallback = HashEmbed
	}
	client := &http.Client{Timeout: 60 * time.Second}

	return func(ctx context.Context, text string) ([]float32, error) {
		var vec []float32
		var err error
		switch cfg.Provider {
		case app.ProviderLlamaCpp:
			vec, err = openAIEmbed(ctx, client, cfg.BaseURL, cfg.Model, text)
		default:
			vec, err = ollamaEmbed(ctx, client, cfg.BaseURL, cfg.Model, text)
		}
		if err == nil {
			return vec, nil
		}
		return fallback(ctx, text)
	}
}

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func ollamaEmbed(ctx context.Context, client *http.Client, baseURL, model, text string) ([]float32, error) {
	body, _ := json.Marshal(ollamaEmbedRequest{Model: model, Prompt: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embed status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	return out.Embedding, nil
}

type openAIEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type openAIEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func openAIEmbed(ctx context.Context, client *http.Client, baseURL, model, text string) ([]float32, error) {
	body, _ := json.Marshal(openAIEmbedRequest{Model: model, Input: text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embed status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out openAIEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	return out.Data[0].Embedding, nil
}

const hashDims = 256

func HashEmbed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, hashDims)
	for _, token := range tokenize(text) {
		h := fnv(token)
		vec[h%hashDims] += 1
	}
	norm := float32(0)
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		n := float32(math.Sqrt(float64(norm)))
		for i := range vec {
			vec[i] /= n
		}
	}
	return vec, nil
}

func tokenize(text string) []string {
	fields := strings.Fields(strings.ToLower(text))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		var b strings.Builder
		for _, r := range f {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			}
		}
		if s := b.String(); len(s) > 1 {
			out = append(out, s)
		}
	}
	return out
}

func fnv(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i] * b[i])
		na += float64(a[i] * a[i])
		nb += float64(b[i] * b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
