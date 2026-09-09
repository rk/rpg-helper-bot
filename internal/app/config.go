package app

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	DefaultGMPort     = 8765
	DefaultPlayerPort = 8766
)

var (
	resolvedDataRoot string
	resolveDataOnce  sync.Once
)

// DataDir returns the app data directory (PDFs, thumbnails, SQLite DB).
// Default: data/ next to the working directory or executable.
// Override with RPG_HELPER_DATA_DIR.
func DataDir() (string, error) {
	resolveDataOnce.Do(func() {
		resolvedDataRoot = resolveDataRoot()
	})
	return resolvedDataRoot, nil
}

func DBPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rpg-helper-bot.db"), nil
}

func resolveDataRoot() string {
	if d := strings.TrimSpace(os.Getenv("RPG_HELPER_DATA_DIR")); d != "" {
		if abs, err := filepath.Abs(d); err == nil {
			return abs
		}
		return d
	}
	candidates := []string{"data", filepath.Join("..", "data")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "data"))
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs
			}
			return dir
		}
	}
	if abs, err := filepath.Abs("data"); err == nil {
		return abs
	}
	return "data"
}

func resetDataDirCache() {
	resolvedDataRoot = ""
	resolveDataOnce = sync.Once{}
}

// legacyConfigDir is the pre-v2 user config location (~/.config/rpg-helper-bot).
func legacyConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "rpg-helper-bot"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "rpg-helper-bot"), nil
}

// MigrateLegacyDataIfNeeded copies database and uploaded files from the old
// dot-config location into the app-relative data directory when the new DB is missing.
func MigrateLegacyDataIfNeeded() error {
	newDB, err := DBPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(newDB); err == nil {
		return nil
	}
	oldRoot, err := legacyConfigDir()
	if err != nil {
		return err
	}
	oldDB := filepath.Join(oldRoot, "rpg-helper-bot.db")
	if _, err := os.Stat(oldDB); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	newData, err := DataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(newData, 0o755); err != nil {
		return err
	}
	if err := copyFile(oldDB, newDB); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	oldData := filepath.Join(oldRoot, "data")
	for _, sub := range []string{"pdfs", "thumbnails"} {
		src := filepath.Join(oldData, sub)
		dst := filepath.Join(newData, sub)
		if err := copyDirIfEmpty(dst, src); err != nil {
			return fmt.Errorf("migrate %s: %w", sub, err)
		}
	}
	log.Printf("Migrated data from %s to %s", oldRoot, newData)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDirIfEmpty(dst, src string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if existing, err := os.ReadDir(dst); err == nil && len(existing) > 0 {
		return nil
	}
	return copyDir(src, dst)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func GMPort() int {
	return envPort("RPG_HELPER_GM_PORT", DefaultGMPort)
}

func PlayerPort() int {
	return envPort("RPG_HELPER_PLAYER_PORT", DefaultPlayerPort)
}

func envPort(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var port int
	if _, err := fmt.Sscanf(v, "%d", &port); err != nil || port <= 0 || port > 65535 {
		return fallback
	}
	return port
}
