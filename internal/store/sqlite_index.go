package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

type FTSCandidate struct {
	SectionID string
	PDFID     string
	Title     string
	StartPage int
	EndPage   int
	PlainText string
	Rank      float64
}

const pdfSelectCols = `id, title, file_path, page_count, thumbnail_path, index_status, indexed_at, created_at, updated_at`

func scanPDFRow(scanner interface {
	Scan(dest ...any) error
}) (models.PDF, error) {
	var p models.PDF
	var createdAt, updatedAt string
	var thumbPath sql.NullString
	var indexStatus string
	var indexedAt sql.NullString
	if err := scanner.Scan(&p.ID, &p.Title, &p.FilePath, &p.PageCount, &thumbPath, &indexStatus, &indexedAt, &createdAt, &updatedAt); err != nil {
		return models.PDF{}, err
	}
	p.ThumbnailPath = thumbPath.String
	p.IndexStatus = models.IndexStatus(indexStatus)
	if indexedAt.Valid {
		t, err := parseTime(indexedAt.String)
		if err != nil {
			return models.PDF{}, err
		}
		p.IndexedAt = &t
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
	if p.IndexStatus == "" {
		p.IndexStatus = models.IndexStatusNone
	}
	return p, nil
}

func (s *SQLiteStore) SetPDFThumbnail(pdfID, path string) error {
	now := formatTime(time.Now().UTC())
	res, err := s.db.Exec(`UPDATE pdfs SET thumbnail_path = ?, updated_at = ? WHERE id = ?`, path, now, pdfID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pdf not found")
	}
	return nil
}

func (s *SQLiteStore) SetPDFIndexStatus(pdfID string, status models.IndexStatus, indexedAt *time.Time) error {
	now := formatTime(time.Now().UTC())
	var idx any
	if indexedAt != nil {
		idx = formatTime(*indexedAt)
	}
	res, err := s.db.Exec(`UPDATE pdfs SET index_status = ?, indexed_at = ?, updated_at = ? WHERE id = ?`, string(status), idx, now, pdfID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pdf not found")
	}
	return nil
}

func (s *SQLiteStore) SaveSectionText(sectionID, plainText string) error {
	now := formatTime(time.Now().UTC())
	res, err := s.db.Exec(`UPDATE toc_sections SET plain_text = ?, indexed_at = ? WHERE id = ?`, plainText, now, sectionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("section not found")
	}

	var pdfID, title string
	if err := s.db.QueryRow(`SELECT pdf_id, title FROM toc_sections WHERE id = ?`, sectionID).Scan(&pdfID, &title); err != nil {
		return err
	}

	_, _ = s.db.Exec(`DELETE FROM toc_sections_fts WHERE section_id = ?`, sectionID)
	_, err = s.db.Exec(`
INSERT INTO toc_sections_fts(title, content, pdf_id, section_id) VALUES (?, ?, ?, ?)`,
		title, plainText, pdfID, sectionID)
	return err
}

func (s *SQLiteStore) ListIndexedSections() ([]models.TOCSection, error) {
	rows, err := s.db.Query(`
SELECT id, pdf_id, title, start_page, end_page, sort_order, plain_text
FROM toc_sections WHERE plain_text != '' ORDER BY pdf_id, sort_order`)
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
		sec.Indexed = true
		sections = append(sections, sec)
	}
	return sections, rows.Err()
}

func (s *SQLiteStore) FTSSearch(pdfIDs []string, query string, limit int) ([]FTSCandidate, error) {
	if len(pdfIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(pdfIDs))
	placeholders = placeholders[:len(placeholders)-1]

	sqlQuery := fmt.Sprintf(`
SELECT f.section_id, f.pdf_id, f.title, t.start_page, t.end_page, t.plain_text,
       bm25(toc_sections_fts) AS rank
FROM toc_sections_fts f
JOIN toc_sections t ON t.id = f.section_id
WHERE toc_sections_fts MATCH ? AND f.pdf_id IN (%s)
ORDER BY rank
LIMIT ?`, placeholders)

	args := []any{ftsQuery(query)}
	for _, id := range pdfIDs {
		args = append(args, id)
	}
	args = append(args, limit)

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FTSCandidate
	for rows.Next() {
		var c FTSCandidate
		if err := rows.Scan(&c.SectionID, &c.PDFID, &c.Title, &c.StartPage, &c.EndPage, &c.PlainText, &c.Rank); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func ftsQuery(q string) string {
	parts := strings.Fields(q)
	if len(parts) == 0 {
		return `""`
	}
	for i, p := range parts {
		parts[i] = `"` + strings.ReplaceAll(p, `"`, "") + `"*`
	}
	return strings.Join(parts, " OR ")
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
