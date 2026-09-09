package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDir_envOverride(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "custom-data")
	if err := os.MkdirAll(custom, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RPG_HELPER_DATA_DIR", custom)
	resetDataDirCache()

	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(custom)
	if err != nil {
		t.Fatal(err)
	}
	if dir != abs {
		t.Fatalf("expected %q, got %q", abs, dir)
	}
}

func TestDBPath_underDataDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RPG_HELPER_DATA_DIR", root)
	resetDataDirCache()

	db, err := DBPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "rpg-helper-bot.db")
	if db != want {
		t.Fatalf("expected %q, got %q", want, db)
	}
}

func TestMigrateLegacyDataIfNeeded_copiesDatabase(t *testing.T) {
	root := t.TempDir()
	newData := filepath.Join(root, "new-data")
	home := filepath.Join(root, "home")
	oldRoot := filepath.Join(home, ".config", "rpg-helper-bot")
	if err := os.MkdirAll(newData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oldRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "rpg-helper-bot.db"), []byte("legacy-db"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("RPG_HELPER_DATA_DIR", newData)
	resetDataDirCache()

	if err := MigrateLegacyDataIfNeeded(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(newData, "rpg-helper-bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "legacy-db" {
		t.Fatalf("unexpected migrated db contents: %q", got)
	}
}
