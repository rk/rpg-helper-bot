package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	Filter           store.ListGamesFilter
	Games            []models.Game
	SelectedGame     *models.Game
	PDFs             []models.PDF
	PDFCounts        map[string]sectionCounts
	SelectedPDF      *models.PDF
	TOCSections      []models.TOCSection
	OptionalRefs     []store.OptionalSectionRef
	LastError        string
	ReadOnly         bool

	gameSaveTimer *time.Timer
	tocSaveTimer  *time.Timer

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
		c.LastError = ""
		return
	}
	c.LastError = err.Error()
	log.Printf("error: %v", err)
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
		c.PDFs = nil
		c.PDFCounts = map[string]sectionCounts{}
		c.SelectedPDF = nil
		c.TOCSections = nil
		c.OptionalRefs = nil
		c.ReadOnly = false
		c.mu.Unlock()
		c.Notify()
		return
	}

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
	c.SelectedGame = game
	c.PDFs = pdfs
	c.PDFCounts = counts
	c.OptionalRefs = refs
	c.ReadOnly = game.Archived
	if c.SelectedPDF != nil && c.SelectedPDF.GameID != id {
		c.SelectedPDF = nil
		c.TOCSections = nil
	}
	c.mu.Unlock()
	c.setError(nil)
	c.Notify()
}

func (c *Controller) CreateGame() {
	now := time.Now().UTC()
	game := models.Game{
		ID:        uuid.NewString(),
		Name:      "Untitled Game",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := c.Store.SaveGame(&game); err != nil {
		c.setError(err)
		return
	}
	c.RefreshGames()
	c.SelectGame(game.ID)
}

func (c *Controller) UpdateGameField(mutate func(*models.Game)) {
	c.mu.Lock()
	if c.SelectedGame == nil {
		c.mu.Unlock()
		return
	}
	game := *c.SelectedGame
	c.mu.Unlock()
	mutate(&game)

	if c.gameSaveTimer != nil {
		c.gameSaveTimer.Stop()
	}
	c.gameSaveTimer = time.AfterFunc(300*time.Millisecond, func() {
		if err := c.Store.SaveGame(&game); err != nil {
			c.setError(err)
			c.Notify()
			return
		}
		c.mu.Lock()
		if c.SelectedGame != nil && c.SelectedGame.ID == game.ID {
			c.SelectedGame = &game
		}
		c.mu.Unlock()
		c.RefreshGames()
		c.SelectGame(game.ID)
	})
}

func (c *Controller) ArchiveSelectedGame() {
	c.mu.Lock()
	id := ""
	if c.SelectedGame != nil {
		id = c.SelectedGame.ID
	}
	c.mu.Unlock()
	if id == "" {
		return
	}
	if err := c.Store.ArchiveGame(id); err != nil {
		c.setError(err)
		c.Notify()
		return
	}
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
		c.Notify()
		return
	}
	c.RefreshGames()
	c.SelectGame(id)
}

func (c *Controller) SelectPDF(id string) {
	if id == "" {
		c.mu.Lock()
		c.SelectedPDF = nil
		c.TOCSections = nil
		c.mu.Unlock()
		c.Notify()
		return
	}

	c.mu.Lock()
	var pdf *models.PDF
	for i := range c.PDFs {
		if c.PDFs[i].ID == id {
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
		c.Notify()
		return
	}

	c.mu.Lock()
	c.SelectedPDF = pdf
	c.TOCSections = sections
	c.mu.Unlock()
	c.setError(nil)
	c.Notify()
}

func (c *Controller) AddPDF(filePath string) {
	c.mu.Lock()
	game := c.SelectedGame
	readOnly := c.ReadOnly
	c.mu.Unlock()
	if game == nil || readOnly {
		return
	}

	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	now := time.Now().UTC()
	pdf := models.PDF{
		ID:        uuid.NewString(),
		GameID:    game.ID,
		Title:     title,
		FilePath:  filePath,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := c.Store.AddPDF(&pdf); err != nil {
		c.setError(err)
		c.Notify()
		return
	}
	c.SelectGame(game.ID)
	c.SelectPDF(pdf.ID)
}

func (c *Controller) UpdatePDFTitle(title string) {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly {
		c.mu.Unlock()
		return
	}
	pdf := *c.SelectedPDF
	c.mu.Unlock()
	pdf.Title = title
	if err := c.Store.UpdatePDF(pdf); err != nil {
		c.setError(err)
		c.Notify()
		return
	}
	c.SelectGame(pdf.GameID)
	c.SelectPDF(pdf.ID)
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
	if err := c.Store.RemovePDF(id); err != nil {
		c.setError(err)
		c.Notify()
		return
	}
	c.SelectGame(gameID)
}

func (c *Controller) AddTOCSection() {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly {
		c.mu.Unlock()
		return
	}
	pdfID := c.SelectedPDF.ID
	sections := append([]models.TOCSection{}, c.TOCSections...)
	c.mu.Unlock()

	sections = append(sections, models.TOCSection{
		ID:        uuid.NewString(),
		PDFID:     pdfID,
		Title:     "",
		StartPage: 1,
		EndPage:   1,
		SortOrder: len(sections),
	})
	c.setTOCSections(pdfID, sections, true)
}

func (c *Controller) UpdateTOCSection(index int, mutate func(*models.TOCSection)) {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly || index < 0 || index >= len(c.TOCSections) {
		c.mu.Unlock()
		return
	}
	pdfID := c.SelectedPDF.ID
	sections := append([]models.TOCSection{}, c.TOCSections...)
	c.mu.Unlock()

	mutate(&sections[index])
	c.setTOCSections(pdfID, sections, true)
}

func (c *Controller) MoveTOCSection(index, delta int) {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly {
		c.mu.Unlock()
		return
	}
	pdfID := c.SelectedPDF.ID
	sections := append([]models.TOCSection{}, c.TOCSections...)
	c.mu.Unlock()

	newIndex := index + delta
	if index < 0 || index >= len(sections) || newIndex < 0 || newIndex >= len(sections) {
		return
	}
	sections[index], sections[newIndex] = sections[newIndex], sections[index]
	c.setTOCSections(pdfID, sections, true)
}

func (c *Controller) RemoveTOCSection(index int) {
	c.mu.Lock()
	if c.SelectedPDF == nil || c.ReadOnly || index < 0 || index >= len(c.TOCSections) {
		c.mu.Unlock()
		return
	}
	pdfID := c.SelectedPDF.ID
	sections := append([]models.TOCSection{}, c.TOCSections...)
	c.mu.Unlock()

	sections = append(sections[:index], sections[index+1:]...)
	c.setTOCSections(pdfID, sections, true)
}

func (c *Controller) setTOCSections(pdfID string, sections []models.TOCSection, debounce bool) {
	c.mu.Lock()
	c.TOCSections = sections
	gameID := ""
	if c.SelectedGame != nil {
		gameID = c.SelectedGame.ID
	}
	c.mu.Unlock()
	c.Notify()

	save := func() {
		if err := c.Store.SaveTOCSections(pdfID, sections); err != nil {
			c.setError(err)
			c.Notify()
			return
		}
		if gameID != "" {
			c.SelectGame(gameID)
		}
		c.SelectPDF(pdfID)
	}

	if !debounce {
		save()
		return
	}
	if c.tocSaveTimer != nil {
		c.tocSaveTimer.Stop()
	}
	c.tocSaveTimer = time.AfterFunc(300*time.Millisecond, save)
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
