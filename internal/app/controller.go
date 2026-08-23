package app

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
)

type Controller struct {
	Store store.Store

	mu sync.Mutex

	Filter       store.ListGamesFilter
	Games        []models.Game
	SelectedGame *models.Game
	PDFs         []models.PDF
	PDFCounts    map[string]sectionCounts
	SelectedPDF  *models.PDF
	TOCSections  []models.TOCSection
	TOCDirty     bool
	OptionalRefs []store.OptionalSectionRef
	LastError    string
	LastInfo     string
	ReadOnly     bool

	// Unsaved new game draft (not yet in database).
	DraftGame *models.Game

	onChange func()
}

type sectionCounts struct {
	Total    int
	Optional int
}

func NewController(s store.Store) *Controller {
	return &Controller{
		Store:     s,
		PDFCounts: map[string]sectionCounts{},
		Filter:    store.ListActive,
		onChange:  func() {},
	}
}

func (c *Controller) SetOnChange(fn func()) {
	c.onChange = fn
}

func (c *Controller) Notify() {
	if c.onChange != nil {
		c.onChange()
	}
}

func (c *Controller) setError(err error) {
	if err == nil {
		if c.LastError != "" {
			c.LastError = ""
			c.Notify()
		}
		return
	}
	c.LastError = err.Error()
	c.LastInfo = ""
	log.Printf("error: %v", err)
	c.Notify()
}

func (c *Controller) setInfo(msg string) {
	c.LastInfo = msg
	c.LastError = ""
	c.Notify()
}

func (c *Controller) RefreshGames() {
	games, err := c.Store.ListGames(c.Filter)
	if err != nil {
		c.setError(err)
		return
	}
	c.mu.Lock()
	c.Games = games
	c.mu.Unlock()
	c.setError(nil)
	c.Notify()
}

func (c *Controller) SetFilter(filter store.ListGamesFilter) {
	c.Filter = filter
	c.RefreshGames()
}

func (c *Controller) SelectGame(id string) {
	if id == "" {
		c.mu.Lock()
		c.SelectedGame = nil
		c.DraftGame = nil
		c.PDFs = nil
		c.PDFCounts = map[string]sectionCounts{}
		c.SelectedPDF = nil
		c.TOCSections = nil
		c.TOCDirty = false
		c.OptionalRefs = nil
		c.ReadOnly = false
		c.mu.Unlock()
		c.setError(nil)
		c.Notify()
		return
	}

	c.mu.Lock()
	if c.DraftGame != nil && c.DraftGame.ID == id {
		game := *c.DraftGame
		c.SelectedGame = &game
		c.PDFs = nil
		c.PDFCounts = map[string]sectionCounts{}
		c.SelectedPDF = nil
		c.TOCSections = nil
		c.TOCDirty = false
		c.OptionalRefs = nil
		c.ReadOnly = false
		c.mu.Unlock()
		c.setError(nil)
		c.Notify()
		return
	}
	c.mu.Unlock()

	game, err := c.Store.GetGame(id)
	if err != nil {
		c.setError(err)
		return
	}
	pdfs, err := c.Store.ListPDFs(id)
	if err != nil {
		c.setError(err)
		return
	}
	counts := map[string]sectionCounts{}
	for _, pdf := range pdfs {
		total, optional, err := c.Store.CountTOCSections(pdf.ID)
		if err != nil {
			c.setError(err)
			return
		}
		counts[pdf.ID] = sectionCounts{Total: total, Optional: optional}
	}
	refs, err := c.Store.ListOptionalSections(id)
	if err != nil {
		c.setError(err)
		return
	}

	c.mu.Lock()
	c.DraftGame = nil
	c.SelectedGame = game
	c.PDFs = pdfs
	c.PDFCounts = counts
	c.OptionalRefs = refs
	c.ReadOnly = game.Archived
	if c.SelectedPDF != nil && c.SelectedPDF.GameID != id {
		c.SelectedPDF = nil
		c.TOCSections = nil
		c.TOCDirty = false
	}
	c.mu.Unlock()
	c.setError(nil)
	c.Notify()
}

func (c *Controller) BeginNewGame() {
	now := timeNow()
	draft := &models.Game{
		ID:        uuid.NewString(),
		Name:      "Untitled Game",
		CreatedAt: now,
		UpdatedAt: now,
	}
	c.mu.Lock()
	c.DraftGame = draft
	game := *draft
	c.SelectedGame = &game
	c.PDFs = nil
	c.PDFCounts = map[string]sectionCounts{}
	c.SelectedPDF = nil
	c.TOCSections = nil
	c.TOCDirty = false
	c.OptionalRefs = nil
	c.ReadOnly = false
	c.mu.Unlock()
	c.setInfo("New game draft — click Save Game to persist.")
	c.Notify()
}

func (c *Controller) SaveGame(name, notes, optIn string) error {
	c.mu.Lock()
	game := c.SelectedGame
	draft := c.DraftGame
	readOnly := c.ReadOnly
	c.mu.Unlock()

	if game == nil || readOnly {
		return fmt.Errorf("select an active game to save")
	}

	saved := *game
	saved.Name = strings.TrimSpace(name)
	saved.Notes = notes
	saved.OptionalRulesOptIn = optIn
	if saved.Name == "" {
		return fmt.Errorf("game name is required")
	}

	isDraft := draft != nil && draft.ID == saved.ID
	if isDraft {
		if err := c.Store.SaveGame(&saved); err != nil {
			c.setError(err)
			return err
		}
		c.mu.Lock()
		c.DraftGame = nil
		c.SelectedGame = &saved
		c.mu.Unlock()
		c.setInfo("Game saved.")
		c.RefreshGames()
		c.SelectGame(saved.ID)
		return nil
	}

	if err := c.Store.SaveGame(&saved); err != nil {
		c.setError(err)
		return err
	}
	c.mu.Lock()
	c.SelectedGame = &saved
	c.mu.Unlock()
	c.setInfo("Game saved.")
	c.RefreshGames()
	c.SelectGame(saved.ID)
	return nil
}

func (c *Controller) ArchiveSelectedGame() {
	c.mu.Lock()
	id := ""
	if c.SelectedGame != nil && c.DraftGame == nil {
		id = c.SelectedGame.ID
	}
	c.mu.Unlock()
	if id == "" {
		return
	}
	if err := c.Store.ArchiveGame(id); err != nil {
		c.setError(err)
		return
	}
	c.setInfo("Game archived.")
	c.RefreshGames()
	c.SelectGame(id)
}

func (c *Controller) RestoreSelectedGame() {
	c.mu.Lock()
	id := ""
	if c.SelectedGame != nil {
		id = c.SelectedGame.ID
	}
	c.mu.Unlock()
	if id == "" {
		return
	}
	if err := c.Store.RestoreGame(id); err != nil {
		c.setError(err)
		return
	}
	c.setInfo("Game restored.")
	c.RefreshGames()
	c.SelectGame(id)
}

func (c *Controller) SelectPDF(id string) {
	if id == "" {
		c.mu.Lock()
		c.SelectedPDF = nil
		c.TOCSections = nil
		c.TOCDirty = false
		c.mu.Unlock()
		c.Notify()
		return
	}

	c.mu.Lock()
	var pdf *models.PDF
	gameID := ""
	if c.SelectedGame != nil {
		gameID = c.SelectedGame.ID
	}
	for i := range c.PDFs {
		if c.PDFs[i].ID == id && c.PDFs[i].GameID == gameID {
			pdf = &c.PDFs[i]
			break
		}
	}
	c.mu.Unlock()
	if pdf == nil {
		return
	}

	sections, err := c.Store.ListTOCSections(id)
	if err != nil {
		c.setError(err)
		return
	}

	c.mu.Lock()
	c.SelectedPDF = pdf
	c.TOCSections = sections
	c.TOCDirty = false
	c.mu.Unlock()
	c.setError(nil)
	c.Notify()
}

func (c *Controller) SavePDF(title, filePath string) error {
	c.mu.Lock()
	game := c.SelectedGame
	selected := c.SelectedPDF
	readOnly := c.ReadOnly
	draft := c.DraftGame
	c.mu.Unlock()

	if game == nil || readOnly || draft != nil {
		return fmt.Errorf("save the game before adding PDFs")
	}

	title = strings.TrimSpace(title)
	filePath = strings.TrimSpace(filePath)
	if title == "" {
		return fmt.Errorf("pdf title is required")
	}
	if filePath == "" {
		return fmt.Errorf("pdf file path is required")
	}

	now := timeNow()
	if selected == nil {
		pdf := &models.PDF{
			ID:        uuid.NewString(),
			GameID:    game.ID,
			Title:     title,
			FilePath:  filePath,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := c.Store.AddPDF(pdf); err != nil {
			c.setError(err)
			return err
		}
		c.setInfo("PDF added.")
		c.SelectGame(game.ID)
		c.SelectPDF(pdf.ID)
		return nil
	}

	pdf := *selected
	pdf.Title = title
	pdf.FilePath = filePath
	pdf.UpdatedAt = now
	if err := c.Store.UpdatePDF(pdf, game.ID); err != nil {
		c.setError(err)
		return err
	}
	c.setInfo("PDF saved.")
	c.SelectGame(game.ID)
	c.SelectPDF(pdf.ID)
	return nil
}

func (c *Controller) RemovePDF(id string) {
	c.mu.Lock()
	gameID := ""
	if c.SelectedGame != nil {
		gameID = c.SelectedGame.ID
	}
	readOnly := c.ReadOnly
	c.mu.Unlock()
	if gameID == "" || readOnly {
		return
	}
	if err := c.Store.RemovePDF(id, gameID); err != nil {
		c.setError(err)
		return
	}
	c.setInfo("PDF removed.")
	c.SelectGame(gameID)
}

func (c *Controller) AddTOCSection() {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly {
		c.mu.Unlock()
		c.setError(fmt.Errorf("select a PDF before adding sections"))
		return
	}
	c.TOCSections = append(c.TOCSections, models.TOCSection{
		ID:        uuid.NewString(),
		PDFID:     c.SelectedPDF.ID,
		Title:     "",
		StartPage: 1,
		EndPage:   1,
		SortOrder: len(c.TOCSections),
	})
	c.TOCDirty = true
	c.mu.Unlock()
	c.setInfo("Section added — click Save Sections when ready.")
	c.Notify()
}

func (c *Controller) UpdateTOCSection(index int, mutate func(*models.TOCSection)) {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly || index < 0 || index >= len(c.TOCSections) {
		c.mu.Unlock()
		return
	}
	mutate(&c.TOCSections[index])
	c.TOCDirty = true
	c.mu.Unlock()
	c.Notify()
}

func (c *Controller) MoveTOCSection(index, delta int) {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly {
		c.mu.Unlock()
		return
	}
	newIndex := index + delta
	if index < 0 || index >= len(c.TOCSections) || newIndex < 0 || newIndex >= len(c.TOCSections) {
		c.mu.Unlock()
		return
	}
	c.TOCSections[index], c.TOCSections[newIndex] = c.TOCSections[newIndex], c.TOCSections[index]
	c.TOCDirty = true
	c.mu.Unlock()
	c.Notify()
}

func (c *Controller) RemoveTOCSection(index int) {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly || index < 0 || index >= len(c.TOCSections) {
		c.mu.Unlock()
		return
	}
	c.TOCSections = append(c.TOCSections[:index], c.TOCSections[index+1:]...)
	c.TOCDirty = true
	c.mu.Unlock()
	c.Notify()
}

func (c *Controller) SaveTOCSectionsNow() error {
	c.mu.Lock()
	pdf := c.SelectedPDF
	sections := append([]models.TOCSection{}, c.TOCSections...)
	gameID := ""
	if c.SelectedGame != nil {
		gameID = c.SelectedGame.ID
	}
	readOnly := c.ReadOnly
	c.mu.Unlock()

	if pdf == nil || readOnly {
		return fmt.Errorf("select a PDF to save sections")
	}

	if err := c.Store.SaveTOCSections(pdf.ID, pdf.GameID, sections); err != nil {
		c.setError(err)
		return err
	}

	c.mu.Lock()
	c.TOCDirty = false
	c.mu.Unlock()
	c.setInfo("Sections saved.")
	if gameID != "" {
		c.SelectGame(gameID)
	}
	c.SelectPDF(pdf.ID)
	return nil
}

func (c *Controller) GameListCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.Games)
	if c.DraftGame != nil {
		found := false
		for _, g := range c.Games {
			if g.ID == c.DraftGame.ID {
				found = true
				break
			}
		}
		if !found {
			n++
		}
	}
	return n
}

func (c *Controller) GameAt(index int) (models.Game, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.DraftGame != nil {
		found := false
		for _, g := range c.Games {
			if g.ID == c.DraftGame.ID {
				found = true
				break
			}
		}
		if !found {
			if index == 0 {
				return *c.DraftGame, true
			}
			index--
		}
	}
	if index < 0 || index >= len(c.Games) {
		return models.Game{}, false
	}
	return c.Games[index], true
}

func ParsePageValue(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("page number required")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid page number")
	}
	return n, nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func timeNow() time.Time {
	return time.Now().UTC()
}
