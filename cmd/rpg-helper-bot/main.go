package main

import (
	"log"
	"os"
	"path/filepath"

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

	ctrl := app.NewController(sqliteStore)
	if err := app.Run(ctrl); err != nil {
		log.Fatal(err)
	}
}
