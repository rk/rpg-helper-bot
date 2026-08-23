package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	_ "modernc.org/sqlite"
)

const schemaVersion = 2

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS schema_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);`); err != nil {
		return err
	}

	version, _ := s.schemaVersion()
	if version == 0 {
		if err := s.createV2Schema(); err != nil {
			return err
		}
		return s.setSchemaVersion(schemaVersion)
	}
	if version < schemaVersion {
		if err := s.migrateToV2(); err != nil {
			return err
		}
		return s.setSchemaVersion(schemaVersion)
	}
	return nil
}

func (s *SQLiteStore) schemaVersion() (int, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'version'`).Scan(&value)
	if err == sql.ErrNoRows {
		if s.tableExists("pdfs") && s.columnExists("pdfs", "game_id") {
			return 1, nil
		}
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var version int
	_, err = fmt.Sscanf(value, "%d", &version)
	return version, err
}

func (s *SQLiteStore) setSchemaVersion(v int) error {
	_, err := s.db.Exec(`
INSERT INTO schema_meta (key, value) VALUES ('version', ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`, fmt.Sprintf("%d", v))
	return err
}

func (s *SQLiteStore) tableExists(name string) bool {
	row := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name)
	var n string
	return row.Scan(&n) == nil
}

func (s *SQLiteStore) columnExists(table, column string) bool {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func (s *SQLiteStore) createV2Schema() error {
	schema := `
CREATE TABLE IF NOT EXISTS games (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  optional_rules_opt_in TEXT NOT NULL DEFAULT '',
  archived INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pdfs (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  file_path TEXT NOT NULL,
  page_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS game_pdfs (
  game_id TEXT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  pdf_id TEXT NOT NULL REFERENCES pdfs(id) ON DELETE CASCADE,
  PRIMARY KEY (game_id, pdf_id)
);

CREATE TABLE IF NOT EXISTS toc_sections (
  id TEXT PRIMARY KEY,
  pdf_id TEXT NOT NULL REFERENCES pdfs(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  start_page INTEGER NOT NULL,
  end_page INTEGER NOT NULL,
  sort_order INTEGER NOT NULL,
  optional INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_toc_pdf ON toc_sections(pdf_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_game_pdfs_game ON game_pdfs(game_id);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) migrateToV2() error {
	if !s.tableExists("pdfs") || !s.columnExists("pdfs", "game_id") {
		return s.createV2Schema()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS pdfs_v2 (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  file_path TEXT NOT NULL,
  page_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS game_pdfs (
  game_id TEXT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  pdf_id TEXT NOT NULL REFERENCES pdfs(id) ON DELETE CASCADE,
  PRIMARY KEY (game_id, pdf_id)
);`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
INSERT OR IGNORE INTO pdfs_v2 (id, title, file_path, page_count, created_at, updated_at)
SELECT id, title, file_path, page_count, created_at, updated_at FROM pdfs`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
INSERT OR IGNORE INTO game_pdfs (game_id, pdf_id)
SELECT game_id, id FROM pdfs WHERE game_id IS NOT NULL AND game_id != ''`); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE pdfs`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE pdfs_v2 RENAME TO pdfs`); err != nil {
		return err
	}

	return tx.Commit()
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(v string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, v)
}

func (s *SQLiteStore) ListGames(filter ListGamesFilter) ([]models.Game, error) {
	query := `SELECT id, name, notes, optional_rules_opt_in, archived, created_at, updated_at FROM games`
	switch filter {
	case ListActive:
		query += ` WHERE archived = 0`
	case ListArchived:
		query += ` WHERE archived = 1`
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func scanGame(scanner interface {
	Scan(dest ...any) error
}) (models.Game, error) {
	var g models.Game
	var archived int
	var createdAt, updatedAt string
	if err := scanner.Scan(&g.ID, &g.Name, &g.Notes, &g.OptionalRulesOptIn, &archived, &createdAt, &updatedAt); err != nil {
		return models.Game{}, err
	}
	g.Archived = archived != 0
	var err error
	g.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return models.Game{}, err
	}
	g.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return models.Game{}, err
	}
	return g, nil
}

func (s *SQLiteStore) GetGame(id string) (*models.Game, error) {
	row := s.db.QueryRow(`SELECT id, name, notes, optional_rules_opt_in, archived, created_at, updated_at FROM games WHERE id = ?`, id)
	g, err := scanGame(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("game not found")
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *SQLiteStore) SaveGame(g *models.Game) error {
	if g == nil {
		return fmt.Errorf("game is nil")
	}
	g.Name = strings.TrimSpace(g.Name)
	if err := ValidateGame(*g); err != nil {
		return err
	}
	now := time.Now().UTC()
	if g.ID == "" {
		g.ID = uuid.NewString()
		g.CreatedAt = now
	}
	g.UpdatedAt = now

	_, err := s.db.Exec(`
INSERT INTO games (id, name, notes, optional_rules_opt_in, archived, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  notes = excluded.notes,
  optional_rules_opt_in = excluded.optional_rules_opt_in,
  archived = excluded.archived,
  updated_at = excluded.updated_at
`, g.ID, g.Name, g.Notes, g.OptionalRulesOptIn, boolToInt(g.Archived), formatTime(g.CreatedAt), formatTime(g.UpdatedAt))
	return err
}

func (s *SQLiteStore) ArchiveGame(id string) error {
	now := formatTime(time.Now().UTC())
	res, err := s.db.Exec(`UPDATE games SET archived = 1, updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("game not found")
	}
	return nil
}

func (s *SQLiteStore) RestoreGame(id string) error {
	now := formatTime(time.Now().UTC())
	res, err := s.db.Exec(`UPDATE games SET archived = 0, updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("game not found")
	}
	return nil
}

func (s *SQLiteStore) ListPDFs() ([]models.PDF, error) {
	rows, err := s.db.Query(`
SELECT id, title, file_path, page_count, created_at, updated_at
FROM pdfs ORDER BY title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pdfs []models.PDF
	for rows.Next() {
		p, err := scanPDF(rows)
		if err != nil {
			return nil, err
		}
		pdfs = append(pdfs, p)
	}
	return pdfs, rows.Err()
}

func (s *SQLiteStore) GetPDF(id string) (*models.PDF, error) {
	row := s.db.QueryRow(`
SELECT id, title, file_path, page_count, created_at, updated_at
FROM pdfs WHERE id = ?`, id)
	p, err := scanPDF(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pdf not found")
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanPDF(scanner interface {
	Scan(dest ...any) error
}) (models.PDF, error) {
	var p models.PDF
	var createdAt, updatedAt string
	if err := scanner.Scan(&p.ID, &p.Title, &p.FilePath, &p.PageCount, &createdAt, &updatedAt); err != nil {
		return models.PDF{}, err
	}
	var err error
	p.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return models.PDF{}, err
	}
	p.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return models.PDF{}, err
	}
	return p, nil
}

func (s *SQLiteStore) SavePDF(p *models.PDF) error {
	if p == nil {
		return fmt.Errorf("pdf is nil")
	}
	if err := ValidatePDF(*p, true); err != nil {
		return err
	}
	now := time.Now().UTC()
	if p.ID == "" {
		p.ID = uuid.NewString()
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	_, err := s.db.Exec(`
INSERT INTO pdfs (id, title, file_path, page_count, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  title = excluded.title,
  file_path = excluded.file_path,
  page_count = excluded.page_count,
  updated_at = excluded.updated_at
`, p.ID, p.Title, p.FilePath, p.PageCount, formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	return err
}

func (s *SQLiteStore) DeletePDF(id string) error {
	var refs int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM game_pdfs WHERE pdf_id = ?`, id).Scan(&refs); err != nil {
		return err
	}
	if refs > 0 {
		return fmt.Errorf("pdf is attached to %d game(s); detach it first", refs)
	}
	res, err := s.db.Exec(`DELETE FROM pdfs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pdf not found")
	}
	return nil
}

func (s *SQLiteStore) ListGamePDFs(gameID string) ([]models.PDF, error) {
	rows, err := s.db.Query(`
SELECT p.id, p.title, p.file_path, p.page_count, p.created_at, p.updated_at
FROM pdfs p
JOIN game_pdfs gp ON gp.pdf_id = p.id
WHERE gp.game_id = ?
ORDER BY p.title ASC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pdfs []models.PDF
	for rows.Next() {
		p, err := scanPDF(rows)
		if err != nil {
			return nil, err
		}
		pdfs = append(pdfs, p)
	}
	return pdfs, rows.Err()
}

func (s *SQLiteStore) AttachPDFToGame(gameID, pdfID string) error {
	if _, err := s.GetGame(gameID); err != nil {
		return err
	}
	if _, err := s.GetPDF(pdfID); err != nil {
		return err
	}
	_, err := s.db.Exec(`
INSERT OR IGNORE INTO game_pdfs (game_id, pdf_id) VALUES (?, ?)`, gameID, pdfID)
	return err
}

func (s *SQLiteStore) DetachPDFFromGame(gameID, pdfID string) error {
	res, err := s.db.Exec(`DELETE FROM game_pdfs WHERE game_id = ? AND pdf_id = ?`, gameID, pdfID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pdf is not attached to this game")
	}
	return nil
}

func (s *SQLiteStore) ListTOCSections(pdfID string) ([]models.TOCSection, error) {
	rows, err := s.db.Query(`
SELECT id, pdf_id, title, start_page, end_page, sort_order, optional
FROM toc_sections WHERE pdf_id = ? ORDER BY sort_order ASC`, pdfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sections []models.TOCSection
	for rows.Next() {
		var sec models.TOCSection
		var optional int
		if err := rows.Scan(&sec.ID, &sec.PDFID, &sec.Title, &sec.StartPage, &sec.EndPage, &sec.SortOrder, &optional); err != nil {
			return nil, err
		}
		sec.Optional = optional != 0
		sections = append(sections, sec)
	}
	return sections, rows.Err()
}

func (s *SQLiteStore) SaveTOCSections(pdfID string, sections []models.TOCSection) error {
	var pageCount int
	if err := s.db.QueryRow(`SELECT page_count FROM pdfs WHERE id = ?`, pdfID).Scan(&pageCount); err != nil {
		return fmt.Errorf("pdf not found")
	}
	if err := ValidateTOCSections(sections, pageCount); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM toc_sections WHERE pdf_id = ?`, pdfID); err != nil {
		return err
	}

	for i, sec := range sections {
		sec.PDFID = pdfID
		sec.SortOrder = i
		if sec.ID == "" {
			sec.ID = uuid.NewString()
		}
		if _, err := tx.Exec(`
INSERT INTO toc_sections (id, pdf_id, title, start_page, end_page, sort_order, optional)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			sec.ID, sec.PDFID, strings.TrimSpace(sec.Title), sec.StartPage, sec.EndPage, sec.SortOrder, boolToInt(sec.Optional)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) ListOptionalSections(gameID string) ([]OptionalSectionRef, error) {
	rows, err := s.db.Query(`
SELECT p.id, t.id, p.title, t.title, t.start_page, t.end_page
FROM toc_sections t
JOIN pdfs p ON p.id = t.pdf_id
JOIN game_pdfs gp ON gp.pdf_id = p.id
WHERE gp.game_id = ? AND t.optional = 1
ORDER BY p.title ASC, t.sort_order ASC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []OptionalSectionRef
	for rows.Next() {
		var ref OptionalSectionRef
		if err := rows.Scan(&ref.PDFID, &ref.SectionID, &ref.PDFTitle, &ref.SectionTitle, &ref.StartPage, &ref.EndPage); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (s *SQLiteStore) CountTOCSections(pdfID string) (total int, optional int, err error) {
	err = s.db.QueryRow(`
SELECT COUNT(*), COALESCE(SUM(CASE WHEN optional = 1 THEN 1 ELSE 0 END), 0)
FROM toc_sections WHERE pdf_id = ?`, pdfID).Scan(&total, &optional)
	return total, optional, err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
