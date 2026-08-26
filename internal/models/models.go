package models

import "time"

type Game struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	Archived  bool      `json:"archived"`
	Running   bool      `json:"running"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PDF is a globally configured document in the PDF library.
type PDF struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	FilePath  string    `json:"file_path"`
	PageCount int       `json:"page_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TOCSection struct {
	ID        string `json:"id"`
	PDFID     string `json:"pdf_id"`
	Title     string `json:"title"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
	SortOrder int    `json:"sort_order"`
}

type GamePDF struct {
	PDFID     string `json:"pdf_id"`
	SortOrder int    `json:"sort_order"`
}

type PathStatus string

const (
	PathStatusOK      PathStatus = "ok"
	PathStatusMissing PathStatus = "missing"
)
