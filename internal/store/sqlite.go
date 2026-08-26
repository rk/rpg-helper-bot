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

const schemaVersion = 4

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

	version, err := s.schemaVersion()
	if err != nil {
		return err
	}
	if version == 0 {
		if err := s.createSchema(); err != nil {
			return err
		}
		return s.setSchemaVersion(schemaVersion)
	}
	if version < schemaVersion {
		if err := s.migrateForward(version); err != nil {
			return err
		}
		return s.setSchemaVersion(schemaVersion)
	}
	if version > schemaVersion {
		return fmt.Errorf("unsupported schema version %d", version)
	}
	return nil
}

func (s *SQLiteStore) schemaVersion() (int, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM schema_meta WHERE key = 'version'`).Scan(&value)
	if err == sql.ErrNoRows {
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

func (s *SQLiteStore) createSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS games (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  archived INTEGER NOT NULL DEFAULT 0,
  running INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pdfs (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  file_path TEXT NOT NULL,
  page_count INTEGER NOT NULL DEFAULT 0,
  thumbnail_path TEXT,
  index_status TEXT NOT NULL DEFAULT 'none',
  indexed_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS game_pdfs (
  game_id TEXT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  pdf_id TEXT NOT NULL REFERENCES pdfs(id) ON DELETE CASCADE,
  sort_order INTEGER NOT NULL,
  PRIMARY KEY (game_id, pdf_id),
  UNIQUE (game_id, sort_order)
);

CREATE TABLE IF NOT EXISTS toc_sections (
  id TEXT PRIMARY KEY,
  pdf_id TEXT NOT NULL REFERENCES pdfs(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  start_page INTEGER NOT NULL,
  end_page INTEGER NOT NULL,
  sort_order INTEGER NOT NULL,
  plain_text TEXT NOT NULL DEFAULT '',
  indexed_at TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS toc_sections_fts USING fts5(
  title,
  content,
  pdf_id UNINDEXED,
  section_id UNINDEXED
);

CREATE INDEX IF NOT EXISTS idx_toc_pdf ON toc_sections(pdf_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_game_pdfs_game ON game_pdfs(game_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_games_running ON games(running);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) migrateForward(from int) error {
	if from == 2 {
		if err := s.migrateToV3(from); err != nil {
			return err
		}
		from = 3
	}
	if from == 3 {
		return s.migrateToV4()
	}
	if from < 3 {
		return fmt.Errorf("unsupported migration from version %d", from)
	}
	return nil
}

func (s *SQLiteStore) migrateToV4() error {
	if !s.columnExists("pdfs", "index_status") {
		if _, err := s.db.Exec(`ALTER TABLE pdfs ADD COLUMN thumbnail_path TEXT`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`ALTER TABLE pdfs ADD COLUMN index_status TEXT NOT NULL DEFAULT 'none'`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`ALTER TABLE pdfs ADD COLUMN indexed_at TEXT`); err != nil {
			return err
		}
	}
	if !s.columnExists("toc_sections", "plain_text") {
		if _, err := s.db.Exec(`ALTER TABLE toc_sections ADD COLUMN plain_text TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`ALTER TABLE toc_sections ADD COLUMN indexed_at TEXT`); err != nil {
			return err
		}
	}
	if !s.tableExists("toc_sections_fts") {
		if _, err := s.db.Exec(`
CREATE VIRTUAL TABLE toc_sections_fts USING fts5(
  title,
  content,
  pdf_id UNINDEXED,
  section_id UNINDEXED
);`); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) migrateToV3(from int) error {
	if from == 2 {
		if !s.columnExists("games", "running") {
			if _, err := s.db.Exec(`ALTER TABLE games ADD COLUMN running INTEGER NOT NULL DEFAULT 0`); err != nil {
				return err
			}
		}
		if !s.columnExists("game_pdfs", "sort_order") {
			if _, err := s.db.Exec(`ALTER TABLE game_pdfs ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0`); err != nil {
				return err
			}
			rows, err := s.db.Query(`SELECT DISTINCT game_id FROM game_pdfs ORDER BY game_id`)
			if err != nil {
				return err
			}
			var gameIDs []string
			for rows.Next() {
				var gameID string
				if err := rows.Scan(&gameID); err != nil {
					rows.Close()
					return err
				}
				gameIDs = append(gameIDs, gameID)
			}
			if err := rows.Close(); err != nil {
				return err
			}
			if err := rows.Err(); err != nil {
				return err
			}

			for _, gameID := range gameIDs {
				pdfRows, err := s.db.Query(`SELECT pdf_id FROM game_pdfs WHERE game_id = ? ORDER BY rowid`, gameID)
				if err != nil {
					return err
				}
				var pdfIDs []string
				for pdfRows.Next() {
					var pdfID string
					if err := pdfRows.Scan(&pdfID); err != nil {
						pdfRows.Close()
						return err
					}
					pdfIDs = append(pdfIDs, pdfID)
				}
				if err := pdfRows.Close(); err != nil {
					return err
				}
				if err := pdfRows.Err(); err != nil {
					return err
				}

				for i, pdfID := range pdfIDs {
					if _, err := s.db.Exec(`UPDATE game_pdfs SET sort_order = ? WHERE game_id = ? AND pdf_id = ?`, i, gameID, pdfID); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	return fmt.Errorf("unsupported migration from version %d", from)
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

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

func scanGame(scanner interface {
	Scan(dest ...any) error
}) (models.Game, error) {
	var g models.Game
	var archived, running int
	var createdAt, updatedAt string
	if err := scanner.Scan(&g.ID, &g.Name, &g.Notes, &archived, &running, &createdAt, &updatedAt); err != nil {
		return models.Game{}, err
	}
	g.Archived = archived != 0
	g.Running = running != 0
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

func (s *SQLiteStore) ListPDFs() ([]models.PDF, error) {
	rows, err := s.db.Query(`SELECT ` + pdfSelectCols + ` FROM pdfs ORDER BY title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pdfs []models.PDF
	for rows.Next() {
		p, err := scanPDFRow(rows)
		if err != nil {
			return nil, err
		}
		pdfs = append(pdfs, p)
	}
	return pdfs, rows.Err()
}

func (s *SQLiteStore) GetPDF(id string) (*models.PDF, error) {
	row := s.db.QueryRow(`SELECT `+pdfSelectCols+` FROM pdfs WHERE id = ?`, id)
	p, err := scanPDFRow(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pdf not found")
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *SQLiteStore) SavePDF(p *models.PDF) error {
	if p == nil {
		return fmt.Errorf("pdf is nil")
	}
	if err := ValidatePDF(*p); err != nil {
		return err
	}
	now := time.Now().UTC()
	if p.ID == "" {
		p.ID = uuid.NewString()
		p.CreatedAt = now
		p.IndexStatus = models.IndexStatusNone
	}
	p.UpdatedAt = now
	if p.IndexStatus == "" {
		p.IndexStatus = models.IndexStatusNone
	}

	var indexedAt any
	if p.IndexedAt != nil {
		indexedAt = formatTime(*p.IndexedAt)
	}

	_, err := s.db.Exec(`
INSERT INTO pdfs (id, title, file_path, page_count, thumbnail_path, index_status, indexed_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  title = excluded.title,
  file_path = excluded.file_path,
  page_count = excluded.page_count,
  thumbnail_path = excluded.thumbnail_path,
  index_status = excluded.index_status,
  indexed_at = excluded.indexed_at,
  updated_at = excluded.updated_at
`, p.ID, strings.TrimSpace(p.Title), p.FilePath, p.PageCount, nullStr(p.ThumbnailPath), string(p.IndexStatus), indexedAt, formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	return err
}

func (s *SQLiteStore) ListGames(filter ListGamesFilter) ([]models.Game, error) {
	query := `SELECT id, name, notes, archived, running, created_at, updated_at FROM games`
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

func (s *SQLiteStore) GetGame(id string) (*models.Game, error) {
	row := s.db.QueryRow(`SELECT id, name, notes, archived, running, created_at, updated_at FROM games WHERE id = ?`, id)
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
INSERT INTO games (id, name, notes, archived, running, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  notes = excluded.notes,
  archived = excluded.archived,
  running = excluded.running,
  updated_at = excluded.updated_at
`, g.ID, strings.TrimSpace(g.Name), g.Notes, boolToInt(g.Archived), boolToInt(g.Running), formatTime(g.CreatedAt), formatTime(g.UpdatedAt))
	return err
}

func (s *SQLiteStore) ArchiveGame(id string) error {
	now := formatTime(time.Now().UTC())
	res, err := s.db.Exec(`UPDATE games SET archived = 1, running = 0, updated_at = ? WHERE id = ?`, now, id)
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

func (s *SQLiteStore) SetGameRunning(id string, running bool) (*models.Game, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM games WHERE id = ?`, id).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, fmt.Errorf("game not found")
	}

	now := formatTime(time.Now().UTC())
	if running {
		if _, err := tx.Exec(`UPDATE games SET running = 0, updated_at = ? WHERE running = 1`, now); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE games SET running = 1, archived = 0, updated_at = ? WHERE id = ?`, now, id); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.Exec(`UPDATE games SET running = 0, updated_at = ? WHERE id = ?`, now, id); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetGame(id)
}

func (s *SQLiteStore) GetRunningGame() (*models.Game, error) {
	row := s.db.QueryRow(`SELECT id, name, notes, archived, running, created_at, updated_at FROM games WHERE running = 1 LIMIT 1`)
	g, err := scanGame(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *SQLiteStore) DeletePDF(id string) error {
	refs, err := s.CountPDFGameRefs(id)
	if err != nil {
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

func (s *SQLiteStore) CountPDFGameRefs(pdfID string) (int, error) {
	var refs int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM game_pdfs WHERE pdf_id = ?`, pdfID).Scan(&refs)
	return refs, err
}

func (s *SQLiteStore) ListGamePDFs(gameID string) ([]GamePDFEntry, error) {
	rows, err := s.db.Query(`
SELECT p.id, p.title, p.file_path, p.page_count, p.thumbnail_path, p.index_status, p.indexed_at, p.created_at, p.updated_at, gp.sort_order
FROM pdfs p
JOIN game_pdfs gp ON gp.pdf_id = p.id
WHERE gp.game_id = ?
ORDER BY gp.sort_order ASC`, gameID)
	if err != nil {
		return nil, err
	}

	type row struct {
		entry     GamePDFEntry
		createdAt string
		updatedAt string
		indexedAt sql.NullString
	}
	var scanned []row
	for rows.Next() {
		var r row
		var thumbPath sql.NullString
		var indexStatus string
		if err := rows.Scan(&r.entry.ID, &r.entry.Title, &r.entry.FilePath, &r.entry.PageCount, &thumbPath, &indexStatus, &r.indexedAt, &r.createdAt, &r.updatedAt, &r.entry.SortOrder); err != nil {
			rows.Close()
			return nil, err
		}
		r.entry.ThumbnailPath = thumbPath.String
		r.entry.IndexStatus = models.IndexStatus(indexStatus)
		if r.entry.IndexStatus == "" {
			r.entry.IndexStatus = models.IndexStatusNone
		}
		if r.indexedAt.Valid {
			t, err := parseTime(r.indexedAt.String)
			if err != nil {
				rows.Close()
				return nil, err
			}
			r.entry.IndexedAt = &t
		}
		scanned = append(scanned, r)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var entries []GamePDFEntry
	for _, r := range scanned {
		entry := r.entry
		var err error
		entry.CreatedAt, err = parseTime(r.createdAt)
		if err != nil {
			return nil, err
		}
		entry.UpdatedAt, err = parseTime(r.updatedAt)
		if err != nil {
			return nil, err
		}
		entry.PathStatus = ProbePath(entry.FilePath)
		count, err := s.CountTOCSections(entry.ID)
		if err != nil {
			return nil, err
		}
		entry.SectionCount = count
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *SQLiteStore) SetGamePDFs(gameID string, pdfIDs []string) error {
	if _, err := s.GetGame(gameID); err != nil {
		return err
	}

	seen := map[string]struct{}{}
	for _, id := range pdfIDs {
		if id == "" {
			return fmt.Errorf("pdf id is required")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate pdf id in list")
		}
		seen[id] = struct{}{}
		if _, err := s.GetPDF(id); err != nil {
			return err
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM game_pdfs WHERE game_id = ?`, gameID); err != nil {
		return err
	}

	for i, pdfID := range pdfIDs {
		if _, err := tx.Exec(`
INSERT INTO game_pdfs (game_id, pdf_id, sort_order) VALUES (?, ?, ?)`, gameID, pdfID, i); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) ListTOCSections(pdfID string) ([]models.TOCSection, error) {
	rows, err := s.db.Query(`
SELECT id, pdf_id, title, start_page, end_page, sort_order, plain_text
FROM toc_sections WHERE pdf_id = ? ORDER BY sort_order ASC`, pdfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sections []models.TOCSection
	for rows.Next() {
		var sec models.TOCSection
		if err := rows.Scan(&sec.ID, &sec.PDFID, &sec.Title, &sec.StartPage, &sec.EndPage, &sec.SortOrder, &sec.PlainText); err != nil {
			return nil, err
		}
		sec.Indexed = sec.PlainText != ""
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
INSERT INTO toc_sections (id, pdf_id, title, start_page, end_page, sort_order)
VALUES (?, ?, ?, ?, ?, ?)`,
			sec.ID, sec.PDFID, strings.TrimSpace(sec.Title), sec.StartPage, sec.EndPage, sec.SortOrder); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) CountTOCSections(pdfID string) (int, error) {
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM toc_sections WHERE pdf_id = ?`, pdfID).Scan(&total)
	return total, err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
