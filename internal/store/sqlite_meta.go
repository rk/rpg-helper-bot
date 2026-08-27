package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

func (s *SQLiteStore) GetPDFIndexMeta(pdfID string) (*models.PDFIndexMeta, error) {
	var raw sql.NullString
	err := s.db.QueryRow(`SELECT index_meta FROM pdfs WHERE id = ?`, pdfID).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pdf not found")
	}
	if err != nil {
		return nil, err
	}
	if !raw.Valid || raw.String == "" {
		return models.EmptyPDFIndexMeta(), nil
	}
	return models.ParsePDFIndexMeta(raw.String)
}

func (s *SQLiteStore) SavePDFIndexMeta(pdfID string, meta *models.PDFIndexMeta) error {
	if meta == nil {
		meta = &models.PDFIndexMeta{}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE pdfs SET index_meta = ?, updated_at = ? WHERE id = ?`, string(b), formatTime(time.Now().UTC()), pdfID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pdf not found")
	}
	return nil
}

func (s *SQLiteStore) ListPDFIndexMeta(pdfIDs []string) (map[string]models.PDFIndexMeta, error) {
	out := make(map[string]models.PDFIndexMeta, len(pdfIDs))
	if len(pdfIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(pdfIDs))
	args := make([]any, len(pdfIDs))
	for i, id := range pdfIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT id, index_meta FROM pdfs WHERE id IN (%s)`, joinPlaceholders(len(pdfIDs)))
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var raw sql.NullString
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		meta := models.PDFIndexMeta{
			Glossary:   []models.PDFGlossaryEntry{},
			Features:   []string{},
			Cheatsheet: []models.CheatsheetEntry{},
		}
		if raw.Valid && raw.String != "" {
			if parsed, err := models.ParsePDFIndexMeta(raw.String); err == nil && parsed != nil {
				meta = *parsed
			}
		}
		out[id] = meta
	}
	return out, rows.Err()
}

func joinPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	s := "?"
	for i := 1; i < n; i++ {
		s += ",?"
	}
	return s
}
