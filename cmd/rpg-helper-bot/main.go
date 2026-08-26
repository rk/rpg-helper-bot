package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/api"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/app"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/embed"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/indexing"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/llm"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/search"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/vectors"
)

func main() {
	app.LoadDotEnv()
	ai := app.AISettingsFromEnv()
	app.LogAISettings(ai)

	dbPath, err := app.DBPath()
	if err != nil {
		log.Fatalf("config path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("create config dir: %v", err)
	}

	dataDir, err := app.DataDir()
	if err != nil {
		log.Fatalf("data dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	sqliteStore, err := store.OpenSQLite(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer sqliteStore.Close()

	embedFn := embed.NewFuncFromEnv()
	vectorStore, err := vectors.New(embedFn)
	if err != nil {
		log.Fatalf("vectors: %v", err)
	}

	ctx := context.Background()
	indexerSvc := &indexing.Service{Store: sqliteStore, Vectors: vectorStore, DataDir: dataDir}
	if err := indexerSvc.RebuildVectors(ctx); err != nil {
		log.Printf("rebuild vectors: %v", err)
	}

	llmClient := llm.NewClient()

	searchSvc := &search.Service{
		Store:        sqliteStore,
		Embed:        embedFn,
		RewriteQuery: llmClient.RewriteSearchQuery,
	}

	server := &api.Server{
		Store:           sqliteStore,
		DataDir:         dataDir,
		GMPort:          app.GMPort(),
		PlayerPort:      app.PlayerPort(),
		StaticDir:       resolveStaticDir(),
		PlayerStaticDir: resolvePlayerStaticDir(),
		Indexer:         indexerSvc,
		Search:          searchSvc,
		LLM:             llmClient,
	}

	gmAddr := fmt.Sprintf("127.0.0.1:%d", server.GMPort)
	playerAddr := fmt.Sprintf("0.0.0.0:%d", server.PlayerPort)

	go func() {
		log.Printf("Player server listening on http://%s", playerAddr)
		if err := http.ListenAndServe(playerAddr, server.PlayerHandler()); err != nil {
			log.Fatalf("player server: %v", err)
		}
	}()

	log.Printf("GM server listening on http://%s", gmAddr)
	openBrowser(fmt.Sprintf("http://%s/library", gmAddr))

	if err := http.ListenAndServe(gmAddr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func resolveStaticDir() string {
	if dir := os.Getenv("RPG_HELPER_STATIC_DIR"); dir != "" {
		return dir
	}
	candidates := []string{
		"web/dist",
		filepath.Join("..", "web", "dist"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "web", "dist"))
	}
	for _, dir := range candidates {
		if info, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs
			}
			return dir
		}
	}
	return "web/dist"
}

func resolvePlayerStaticDir() string {
	if dir := os.Getenv("RPG_HELPER_PLAYER_STATIC_DIR"); dir != "" {
		return dir
	}
	candidates := []string{"web-player/dist", filepath.Join("..", "web-player", "dist")}
	for _, dir := range candidates {
		if info, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs
			}
			return dir
		}
	}
	return "web-player/dist"
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("open browser: %v", err)
	}
}
