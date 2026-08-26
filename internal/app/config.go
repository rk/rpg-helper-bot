package app

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultGMPort     = 8765
	DefaultPlayerPort = 8766
)

func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "rpg-helper-bot"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "rpg-helper-bot"), nil
}

func DBPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rpg-helper-bot.db"), nil
}

func DataDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "data"), nil
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
