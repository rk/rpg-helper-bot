package store

import (
	"time"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

type ListGamesFilter int

const (
	ListActive ListGamesFilter = iota
	ListArchived
	ListAll
)

type PDFSummary struct {
	models.PDF
	PathStatus models.PathStatus `json:"path_status"`
	SectionCount int             `json:"section_count"`
	GameCount    int             `json:"game_count"`
}

type GamePDFEntry struct {
	models.PDF
	SortOrder    int               `json:"sort_order"`
	PathStatus   models.PathStatus `json:"path_status"`
	SectionCount int               `json:"section_count"`
}

type GameSummary struct {
	models.Game
	PDFCount int `json:"pdf_count"`
}

type Store interface {
	ListGames(filter ListGamesFilter) ([]models.Game, error)
	GetGame(id string) (*models.Game, error)
	SaveGame(g *models.Game) error
	ArchiveGame(id string) error
	RestoreGame(id string) error
	SetGameRunning(id string, running bool) (*models.Game, error)
	GetRunningGame() (*models.Game, error)

	ListPDFs() ([]models.PDF, error)
	GetPDF(id string) (*models.PDF, error)
	SavePDF(p *models.PDF) error
	DeletePDF(id string) error
	CountPDFGameRefs(pdfID string) (int, error)

	ListGamePDFs(gameID string) ([]GamePDFEntry, error)
	SetGamePDFs(gameID string, pdfIDs []string) error

	ListTOCSections(pdfID string) ([]models.TOCSection, error)
	SaveTOCSections(pdfID string, sections []models.TOCSection) error
	CountTOCSections(pdfID string) (int, error)

	SetPDFThumbnail(pdfID, path string) error
	SetPDFIndexStatus(pdfID string, status models.IndexStatus, indexedAt *time.Time) error
	SaveSectionText(sectionID, plainText string) error
	ListIndexedSections() ([]models.TOCSection, error)
	FTSSearch(pdfIDs []string, query string, limit int) ([]FTSCandidate, error)
}
