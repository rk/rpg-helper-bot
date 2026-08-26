package models

import "time"

type IndexStatus string

const (
	IndexStatusNone      IndexStatus = "none"
	IndexStatusIndexing  IndexStatus = "indexing"
	IndexStatusIndexed   IndexStatus = "indexed"
	IndexStatusError     IndexStatus = "error"
)

type Game struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	Archived  bool      `json:"archived"`
	Running   bool      `json:"running"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PDF struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	FilePath      string      `json:"file_path"`
	PageCount     int         `json:"page_count"`
	ThumbnailPath string      `json:"thumbnail_path,omitempty"`
	IndexStatus   IndexStatus `json:"index_status"`
	IndexedAt     *time.Time  `json:"indexed_at,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type TOCSection struct {
	ID        string `json:"id"`
	PDFID     string `json:"pdf_id"`
	Title     string `json:"title"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
	SortOrder int    `json:"sort_order"`
	PlainText string `json:"plain_text,omitempty"`
	Indexed   bool   `json:"indexed"`
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

type SearchHit struct {
	SectionID    string  `json:"section_id"`
	PDFID        string  `json:"pdf_id"`
	PDFTitle     string  `json:"pdf_title"`
	SectionTitle string  `json:"section_title"`
	StartPage    int     `json:"start_page"`
	EndPage      int     `json:"end_page"`
	PDFSortOrder int     `json:"pdf_sort_order"`
	Score        float64 `json:"score"`
	Snippet      string  `json:"snippet"`
}

type ChatSource struct {
	SectionID    string `json:"section_id"`
	PDFTitle     string `json:"pdf_title"`
	SectionTitle string `json:"section_title"`
	StartPage    int    `json:"start_page"`
	EndPage      int    `json:"end_page"`
	Snippet      string `json:"snippet,omitempty"`
}
