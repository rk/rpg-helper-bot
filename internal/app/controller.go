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

	// Games view
	Filter           store.ListGamesFilter
	Games            []models.Game
	SelectedGame     *models.Game
	DraftGame        *models.Game
	AttachedPDFs     []models.PDF
	OptionalRefs     []store.OptionalSectionRef
	GameReadOnly     bool

	// PDF library view
	LibraryPDFs         []models.PDF
	SelectedLibraryPDF  *models.PDF
	DraftLibraryPDF     *models.PDF
	TOCSections         []models.TOCSection
	TOCDirty            bool
	PDFSectionCounts    map[string]sectionCounts

	LastError string
	LastInfo  string

	onChange func()
}

type sectionCounts struct {
	Total    int
	Optional int
}

func NewController(s store.Store) *Controller {
	return &Controller{
		Store:            s,
		PDFSectionCounts: map[string]sectionCounts{},
		Filter:           store.ListActive,
		onChange:         func() {},
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

func (c *Controller) RefreshLibrary() {
	pdfs, err := c.Store.ListPDFs()
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
	c.mu.Lock()
	c.LibraryPDFs = pdfs
	c.PDFSectionCounts = counts
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
		c.AttachedPDFs = nil
		c.OptionalRefs = nil
		c.GameReadOnly = false
		c.mu.Unlock()
		c.setError(nil)
		c.Notify()
		return
	}

	c.mu.Lock()
	if c.DraftGame != nil && c.DraftGame.ID == id {
		game := *c.DraftGame
		c.SelectedGame = &game
		c.AttachedPDFs = nil
		c.OptionalRefs = nil
		c.GameReadOnly = false
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
	attached, err := c.Store.ListGamePDFs(id)
	if err != nil {
		c.setError(err)
		return
	}
	refs, err := c.Store.ListOptionalSections(id)
	if err != nil {
		c.setError(err)
		return
	}

	c.mu.Lock()
	c.DraftGame = nil
	c.SelectedGame = game
	c.AttachedPDFs = attached
	c.OptionalRefs = refs
	c.GameReadOnly = game.Archived
	c.mu.Unlock()
	c.setError(nil)
	c.Notify()
}

func (c *Controller) BeginNewGame() {
	now := time.Now().UTC()
	draft := &models.Game{
		ID:        uuid.NewString(),
		Name:      "Untitled Game",
		CreatedAt: now,
		UpdatedAt: now,
	}
	c.mu.Lock()
	c.DraftGame = draft
	c.SelectedGame = draft
	c.AttachedPDFs = nil
	c.OptionalRefs = nil
	c.GameReadOnly = false
	c.mu.Unlock()
	c.setInfo("New game draft — click Save Game to persist.")
	c.Notify()
}

func (c *Controller) SaveGame(name, notes, optIn string) error {
	c.mu.Lock()
	game := c.SelectedGame
	readOnly := c.GameReadOnly
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

func (c *Controller) AttachPDFToGame(pdfID string) {
	c.mu.Lock()
	game := c.SelectedGame
	draft := c.DraftGame
	readOnly := c.GameReadOnly
	gameID := ""
	if game != nil {
		gameID = game.ID
	}
	c.mu.Unlock()

	if gameID == "" || readOnly || draft != nil {
		c.setError(fmt.Errorf("save the game before attaching PDFs"))
		return
	}
	if err := c.Store.AttachPDFToGame(gameID, pdfID); err != nil {
		c.setError(err)
		return
	}
	c.setInfo("PDF attached to game.")
	c.SelectGame(gameID)
}

func (c *Controller) DetachPDFFromGame(pdfID string) {
	c.mu.Lock()
	gameID := ""
	if c.SelectedGame != nil {
		gameID = c.SelectedGame.ID
	}
	readOnly := c.GameReadOnly
	c.mu.Unlock()

	if gameID == "" || readOnly {
		return
	}
	if err := c.Store.DetachPDFFromGame(gameID, pdfID); err != nil {
		c.setError(err)
		return
	}
	c.setInfo("PDF detached from game.")
	c.SelectGame(gameID)
}

func (c *Controller) IsPDFAttachedToGame(pdfID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.AttachedPDFs {
		if p.ID == pdfID {
			return true
		}
	}
	return false
}

func (c *Controller) SelectLibraryPDF(id string) {
	if id == "" {
		c.mu.Lock()
		c.SelectedLibraryPDF = nil
		c.TOCSections = nil
		c.TOCDirty = false
		c.mu.Unlock()
		c.Notify()
		return
	}

	c.mu.Lock()
	if c.DraftLibraryPDF != nil && c.DraftLibraryPDF.ID == id {
		pdf := *c.DraftLibraryPDF
		c.SelectedLibraryPDF = &pdf
		c.TOCSections = nil
		c.TOCDirty = false
		c.mu.Unlock()
		c.Notify()
		return
	}
	c.mu.Unlock()

	pdf, err := c.Store.GetPDF(id)
	if err != nil {
		c.setError(err)
		return
	}
	sections, err := c.Store.ListTOCSections(id)
	if err != nil {
		c.setError(err)
		return
	}

	c.mu.Lock()
	c.DraftLibraryPDF = nil
	c.SelectedLibraryPDF = pdf
	c.TOCSections = sections
	c.TOCDirty = false
	c.mu.Unlock()
	c.setError(nil)
	c.Notify()
}

func (c *Controller) BeginNewLibraryPDF() {
	now := time.Now().UTC()
	draft := &models.PDF{
		ID:        uuid.NewString(),
		Title:     "Untitled PDF",
		CreatedAt: now,
		UpdatedAt: now,
	}
	c.mu.Lock()
	c.DraftLibraryPDF = draft
	c.SelectedLibraryPDF = draft
	c.TOCSections = nil
	c.TOCDirty = false
	c.mu.Unlock()
	c.setInfo("New PDF draft — set path and click Save PDF.")
	c.Notify()
}

func (c *Controller) SaveLibraryPDF(title, filePath string) error {
	c.mu.Lock()
	selected := c.SelectedLibraryPDF
	c.mu.Unlock()

	if selected == nil {
		return fmt.Errorf("select or create a PDF to save")
	}

	saved := *selected
	saved.Title = strings.TrimSpace(title)
	saved.FilePath = strings.TrimSpace(filePath)
	if saved.Title == "" {
		return fmt.Errorf("pdf title is required")
	}
	if saved.FilePath == "" {
		return fmt.Errorf("pdf file path is required")
	}

	if err := c.Store.SavePDF(&saved); err != nil {
		c.setError(err)
		return err
	}

	c.mu.Lock()
	c.DraftLibraryPDF = nil
	c.SelectedLibraryPDF = &saved
	c.mu.Unlock()
	c.setInfo("PDF saved to library.")
	c.RefreshLibrary()
	c.SelectLibraryPDF(saved.ID)
	return nil
}

func (c *Controller) DeleteLibraryPDF(id string) {
	if err := c.Store.DeletePDF(id); err != nil {
		c.setError(err)
		return
	}
	c.mu.Lock()
	if c.SelectedLibraryPDF != nil && c.SelectedLibraryPDF.ID == id {
		c.SelectedLibraryPDF = nil
		c.TOCSections = nil
		c.TOCDirty = false
	}
	c.mu.Unlock()
	c.setInfo("PDF deleted from library.")
	c.RefreshLibrary()
}

func (c *Controller) AddTOCSection() {
	c.mu.Lock()
	if c.SelectedLibraryPDF == nil {
		c.mu.Unlock()
		c.setError(fmt.Errorf("select a PDF before adding sections"))
		return
	}
	pdfID := c.SelectedLibraryPDF.ID
	c.TOCSections = append(c.TOCSections, models.TOCSection{
		ID:        uuid.NewString(),
		PDFID:     pdfID,
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
	if c.SelectedLibraryPDF == nil || index < 0 || index >= len(c.TOCSections) {
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
	if c.SelectedLibraryPDF == nil {
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
	if c.SelectedLibraryPDF == nil || index < 0 || index >= len(c.TOCSections) {
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
	pdf := c.SelectedLibraryPDF
	sections := append([]models.TOCSection{}, c.TOCSections...)
	c.mu.Unlock()

	if pdf == nil {
		return fmt.Errorf("select a PDF to save sections")
	}
	if strings.TrimSpace(pdf.FilePath) == "" {
		return fmt.Errorf("save the PDF before saving sections")
	}

	if err := c.Store.SaveTOCSections(pdf.ID, sections); err != nil {
		c.setError(err)
		return err
	}

	c.mu.Lock()
	c.TOCDirty = false
	c.mu.Unlock()
	c.setInfo("Sections saved.")
	c.RefreshLibrary()
	c.SelectLibraryPDF(pdf.ID)
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

func (c *Controller) LibraryListCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.LibraryPDFs)
	if c.DraftLibraryPDF != nil {
		found := false
		for _, p := range c.LibraryPDFs {
			if p.ID == c.DraftLibraryPDF.ID {
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

func (c *Controller) LibraryPDFAt(index int) (models.PDF, sectionCounts, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.DraftLibraryPDF != nil {
		found := false
		for _, p := range c.LibraryPDFs {
			if p.ID == c.DraftLibraryPDF.ID {
				found = true
				break
			}
		}
		if !found {
			if index == 0 {
				return *c.DraftLibraryPDF, sectionCounts{}, true
			}
			index--
		}
	}
	if index < 0 || index >= len(c.LibraryPDFs) {
		return models.PDF{}, sectionCounts{}, false
	}
	pdf := c.LibraryPDFs[index]
	return pdf, c.PDFSectionCounts[pdf.ID], true
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
