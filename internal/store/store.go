package store

import "github.com/rpg-helper-bot/rpg-helper-bot/internal/models"

type ListGamesFilter int

const (
	ListActive ListGamesFilter = iota
	ListArchived
	ListAll
)

type OptionalSectionRef struct {
	PDFID        string
	SectionID    string
	PDFTitle     string
	SectionTitle string
	StartPage    int
	EndPage      int
}

type Store interface {
	ListGames(filter ListGamesFilter) ([]models.Game, error)
	GetGame(id string) (*models.Game, error)
	SaveGame(g *models.Game) error
	ArchiveGame(id string) error
	RestoreGame(id string) error

	ListPDFs(gameID string) ([]models.PDF, error)
	AddPDF(p *models.PDF) error
	UpdatePDF(p models.PDF, gameID string) error
	RemovePDF(id, gameID string) error

	ListTOCSections(pdfID string) ([]models.TOCSection, error)
	SaveTOCSections(pdfID, gameID string, sections []models.TOCSection) error

	ListOptionalSections(gameID string) ([]OptionalSectionRef, error)
	CountTOCSections(pdfID string) (total int, optional int, err error)
}
