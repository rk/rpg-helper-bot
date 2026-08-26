package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/api"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/app"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

func main() {
	dbPath, err := app.DBPath()
	if err != nil {
		log.Fatalf("config path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("create config dir: %v", err)
	}

	sqliteStore, err := store.OpenSQLite(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer sqliteStore.Close()

	staticDir := resolveStaticDir()
	gmPort := app.GMPort()
	playerPort := app.PlayerPort()

	server := &api.Server{
		Store:      sqliteStore,
		GMPort:     gmPort,
		PlayerPort: playerPort,
		StaticDir:  staticDir,
	}

	addr := fmt.Sprintf("127.0.0.1:%d", gmPort)
	log.Printf("GM server listening on http://%s", addr)
	openBrowser(fmt.Sprintf("http://%s/library", addr))

	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
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
