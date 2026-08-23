package models

import "time"

type Game struct {
	ID                 string
	Name               string
	Notes              string
	OptionalRulesOptIn string
	Archived           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// PDF is a globally configured document in the PDF library.
type PDF struct {
	ID        string
	Title     string
	FilePath  string
	PageCount int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TOCSection struct {
	ID        string
	PDFID     string
	Title     string
	StartPage int
	EndPage   int
	SortOrder int
	Optional  bool
}
