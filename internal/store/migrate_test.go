package store_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	_ "modernc.org/sqlite"
)

func TestMigrateFromV2NoDeadlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v2.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE schema_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO schema_meta VALUES ('version', '2');
CREATE TABLE games (id TEXT PRIMARY KEY, name TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '', archived INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE pdfs (id TEXT PRIMARY KEY, title TEXT NOT NULL, file_path TEXT NOT NULL, page_count INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE game_pdfs (game_id TEXT NOT NULL, pdf_id TEXT NOT NULL, PRIMARY KEY (game_id, pdf_id));
CREATE TABLE toc_sections (id TEXT PRIMARY KEY, pdf_id TEXT NOT NULL, title TEXT NOT NULL, start_page INTEGER NOT NULL, end_page INTEGER NOT NULL, sort_order INTEGER NOT NULL);
INSERT INTO games VALUES ('g1', 'Test', '', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
INSERT INTO pdfs VALUES ('p1', 'Book', '/tmp/a.pdf', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
INSERT INTO game_pdfs VALUES ('g1', 'p1');
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		s, err := store.OpenSQLite(path)
		if err != nil {
			done <- err
			return
		}
		done <- s.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-t.Context().Done():
		t.Fatal("migration deadlocked")
	}
}
