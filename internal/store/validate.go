package store

import (
	"fmt"
	"os"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
)

const maxGameNameLen = 120

func ValidateGame(g models.Game) error {
	name := strings.TrimSpace(g.Name)
	if name == "" {
		return fmt.Errorf("game name is required")
	}
	if len(name) > maxGameNameLen {
		return fmt.Errorf("game name must be at most %d characters", maxGameNameLen)
	}
	return nil
}

func ValidatePDF(p models.PDF, requireExistingFile bool) error {
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("pdf title is required")
	}
	if strings.TrimSpace(p.FilePath) == "" {
		return fmt.Errorf("pdf file path is required")
	}
	if requireExistingFile {
		if _, err := os.Stat(p.FilePath); err != nil {
			return fmt.Errorf("pdf file does not exist: %s", p.FilePath)
		}
	}
	return nil
}

func ValidateTOCSections(sections []models.TOCSection, pageCount int) error {
	for i, s := range sections {
		if strings.TrimSpace(s.Title) == "" {
			return fmt.Errorf("section %d: title is required", i+1)
		}
		if s.StartPage < 1 {
			return fmt.Errorf("section %d: start page must be at least 1", i+1)
		}
		if s.EndPage < s.StartPage {
			return fmt.Errorf("section %d: end page must be >= start page", i+1)
		}
		if pageCount > 0 {
			if s.StartPage > pageCount || s.EndPage > pageCount {
				return fmt.Errorf("section %d: page range exceeds document page count (%d)", i+1, pageCount)
			}
		}
	}
	return nil
}
