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

	ListPDFs() ([]models.PDF, error)
	GetPDF(id string) (*models.PDF, error)
	SavePDF(p *models.PDF) error
	DeletePDF(id string) error

	ListGamePDFs(gameID string) ([]models.PDF, error)
	AttachPDFToGame(gameID, pdfID string) error
	DetachPDFFromGame(gameID, pdfID string) error

	ListTOCSections(pdfID string) ([]models.TOCSection, error)
	SaveTOCSections(pdfID string, sections []models.TOCSection) error

	ListOptionalSections(gameID string) ([]OptionalSectionRef, error)
	CountTOCSections(pdfID string) (total int, optional int, err error)
}
