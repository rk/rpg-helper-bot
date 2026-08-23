package app

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/gogpu/gg/gpu"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/core/checkbox"
	"github.com/gogpu/ui/core/datatable"
	"github.com/gogpu/ui/core/dialog"
	"github.com/gogpu/ui/core/dropdown"
	"github.com/gogpu/ui/core/listview"
	"github.com/gogpu/ui/core/tabview"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/desktop"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/models"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/store"
	"github.com/rpg-helper-bot/rpg-helper-bot/internal/ui/components"
)

type Desktop struct {
	ctrl     *Controller
	gogpuApp *gogpu.App
	uiApp    *app.App
	theme    *material3.Theme
	painters desktopPainters

	gamesVersion       state.Signal[int]
	libraryVersion     state.Signal[int]
	tocVersion         state.Signal[int]
	optionalRefVersion state.Signal[int]
	statusVersion      state.Signal[int]

	gameListCount      state.Signal[int]
	libraryListCount   state.Signal[int]
	tocSectionCount    state.Signal[int]
	gameSelectedIndex  state.Signal[int]
	librarySelectedIndex state.Signal[int]
	selectedTOCIndex   state.Signal[int]

	gameNameSignal  state.Signal[string]
	gameNotesSignal state.Signal[string]
	gameOptInSignal state.Signal[string]
	pdfTitleSignal  state.Signal[string]
	pdfPathSignal   state.Signal[string]

	sectionTitleSignal    state.Signal[string]
	sectionStartSignal    state.Signal[string]
	sectionEndSignal      state.Signal[string]
	sectionOptionalSignal state.Signal[bool]

	loadedGameID   string
	loadedPDFID    string
	loadedTOCForPDF string
	loadedTOCIndex int
}

type desktopPainters struct {
	button    material3.ButtonPainter
	checkbox  material3.CheckboxPainter
	textfield material3.TextFieldPainter
	dropdown  material3.DropdownPainter
	datatable material3.DataTablePainter
	tabview   material3.TabViewPainter
}

func Run(ctrl *Controller) error {
	gogpuApp := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("RPG Helper Bot").
		WithSize(1200, 800))

	m3 := material3.New(widget.Hex(0x6750A4))
	d := &Desktop{
		ctrl:     ctrl,
		gogpuApp: gogpuApp,
		theme:    m3,
		painters: desktopPainters{
			button:    material3.ButtonPainter{Theme: m3},
			checkbox:  material3.CheckboxPainter{Theme: m3},
			textfield: material3.TextFieldPainter{Theme: m3},
			dropdown:  material3.DropdownPainter{Theme: m3},
			datatable: material3.DataTablePainter{Theme: m3},
			tabview:   material3.TabViewPainter{Theme: m3},
		},
		gamesVersion:       state.NewSignal(0),
		libraryVersion:     state.NewSignal(0),
		tocVersion:         state.NewSignal(0),
		optionalRefVersion: state.NewSignal(0),
		statusVersion:      state.NewSignal(0),
		gameListCount:      state.NewSignal(0),
		libraryListCount:   state.NewSignal(0),
		tocSectionCount:    state.NewSignal(0),
		gameSelectedIndex:  state.NewSignal(-1),
		librarySelectedIndex: state.NewSignal(-1),
		selectedTOCIndex:   state.NewSignal(-1),
		gameNameSignal:     state.NewSignal(""),
		gameNotesSignal:    state.NewSignal(""),
		gameOptInSignal:    state.NewSignal(""),
		pdfTitleSignal:     state.NewSignal(""),
		pdfPathSignal:      state.NewSignal(""),
		sectionTitleSignal:    state.NewSignal(""),
		sectionStartSignal:    state.NewSignal("1"),
		sectionEndSignal:      state.NewSignal("1"),
		sectionOptionalSignal: state.NewSignal(false),
		loadedTOCIndex:        -1,
	}

	uiApp := app.New(
		app.WithWindowProvider(gogpuApp),
		app.WithPlatformProvider(gogpuApp),
		app.WithEventSource(gogpuApp.EventSource()),
		app.WithTheme(m3.AsTheme()),
	)
	d.uiApp = uiApp

	ctrl.SetOnChange(func() {
		d.gamesVersion.Set(d.gamesVersion.Get() + 1)
		d.libraryVersion.Set(d.libraryVersion.Get() + 1)
		d.tocVersion.Set(d.tocVersion.Get() + 1)
		d.optionalRefVersion.Set(d.optionalRefVersion.Get() + 1)
		d.statusVersion.Set(d.statusVersion.Get() + 1)
		d.syncListCounts()
		d.syncListSelections()
		d.reloadFormsIfSelectionChanged()
		d.reloadTOCEditorIfSelectionChanged()
	})

	ctrl.RefreshGames()
	ctrl.RefreshLibrary()
	uiApp.SetRoot(d.buildRoot())
	return desktop.Run(gogpuApp, uiApp)
}

func (d *Desktop) syncListCounts() {
	d.gameListCount.Set(d.ctrl.GameListCount())
	d.libraryListCount.Set(d.ctrl.LibraryListCount())
	d.ctrl.mu.Lock()
	d.tocSectionCount.Set(len(d.ctrl.TOCSections))
	d.ctrl.mu.Unlock()
}

func (d *Desktop) syncListSelections() {
	d.ctrl.mu.Lock()
	gameID := ""
	pdfID := ""
	if d.ctrl.SelectedGame != nil {
		gameID = d.ctrl.SelectedGame.ID
	}
	if d.ctrl.SelectedLibraryPDF != nil {
		pdfID = d.ctrl.SelectedLibraryPDF.ID
	}
	d.ctrl.mu.Unlock()

	d.gameSelectedIndex.Set(d.ctrl.GameIndexForID(gameID))
	d.librarySelectedIndex.Set(d.ctrl.LibraryIndexForID(pdfID))

	if pdfID != d.loadedTOCForPDF {
		d.loadedTOCForPDF = pdfID
		d.loadedTOCIndex = -1
		d.selectedTOCIndex.Set(-1)
	}
}

func (d *Desktop) reloadTOCEditorIfSelectionChanged() {
	idx := d.selectedTOCIndex.Get()
	if idx == d.loadedTOCIndex {
		return
	}
	d.loadedTOCIndex = idx

	d.ctrl.mu.Lock()
	var sec models.TOCSection
	hasSection := idx >= 0 && idx < len(d.ctrl.TOCSections)
	if hasSection {
		sec = d.ctrl.TOCSections[idx]
	}
	d.ctrl.mu.Unlock()

	if !hasSection {
		d.sectionTitleSignal.Set("")
		d.sectionStartSignal.Set("1")
		d.sectionEndSignal.Set("1")
		d.sectionOptionalSignal.Set(false)
		return
	}
	d.sectionTitleSignal.Set(sec.Title)
	d.sectionStartSignal.Set(strconv.Itoa(sec.StartPage))
	d.sectionEndSignal.Set(strconv.Itoa(sec.EndPage))
	d.sectionOptionalSignal.Set(sec.Optional)
}

func (d *Desktop) beginNewGame() {
	d.loadedGameID = ""
	d.ctrl.BeginNewGame()
	d.gameSelectedIndex.Set(0)
	d.gameNameSignal.Set("Untitled Game")
	d.gameNotesSignal.Set("")
	d.gameOptInSignal.Set("")
}

func (d *Desktop) beginNewLibraryPDF() {
	d.loadedPDFID = ""
	d.loadedTOCForPDF = ""
	d.loadedTOCIndex = -1
	d.selectedTOCIndex.Set(-1)
	d.ctrl.BeginNewLibraryPDF()
	d.librarySelectedIndex.Set(0)
	d.pdfTitleSignal.Set("Untitled PDF")
	d.pdfPathSignal.Set("")
	d.sectionTitleSignal.Set("")
	d.sectionStartSignal.Set("1")
	d.sectionEndSignal.Set("1")
	d.sectionOptionalSignal.Set(false)
}

func (d *Desktop) selectGameAt(index int) {
	game, ok := d.ctrl.GameAt(index)
	if !ok {
		return
	}
	d.loadedGameID = ""
	d.ctrl.SelectGame(game.ID)
}

func (d *Desktop) selectLibraryPDFAt(index int) {
	pdf, _, ok := d.ctrl.LibraryPDFAt(index)
	if !ok {
		return
	}
	d.loadedPDFID = ""
	d.loadedTOCForPDF = ""
	d.loadedTOCIndex = -1
	d.selectedTOCIndex.Set(-1)
	d.ctrl.SelectLibraryPDF(pdf.ID)
}

func (d *Desktop) reloadFormsIfSelectionChanged() {
	d.ctrl.mu.Lock()
	gameID := ""
	pdfID := ""
	if d.ctrl.SelectedGame != nil {
		gameID = d.ctrl.SelectedGame.ID
	}
	if d.ctrl.SelectedLibraryPDF != nil {
		pdfID = d.ctrl.SelectedLibraryPDF.ID
	}
	game := d.ctrl.SelectedGame
	pdf := d.ctrl.SelectedLibraryPDF
	d.ctrl.mu.Unlock()

	if gameID != d.loadedGameID {
		d.loadedGameID = gameID
		if game != nil {
			d.gameNameSignal.Set(game.Name)
			d.gameNotesSignal.Set(game.Notes)
			d.gameOptInSignal.Set(game.OptionalRulesOptIn)
		} else {
			d.gameNameSignal.Set("")
			d.gameNotesSignal.Set("")
			d.gameOptInSignal.Set("")
		}
	}

	if pdfID != d.loadedPDFID {
		d.loadedPDFID = pdfID
		if pdf != nil {
			d.pdfTitleSignal.Set(pdf.Title)
			d.pdfPathSignal.Set(pdf.FilePath)
		} else {
			d.pdfTitleSignal.Set("")
			d.pdfPathSignal.Set("")
		}
	}
}

func (d *Desktop) ctx() widget.Context {
	return d.uiApp.Window().Context()
}

func (d *Desktop) buildRoot() widget.Widget {
	status := d.buildStatusBar()

	tabs := tabview.New(
		[]tabview.Tab{
			{Label: "Games", Content: d.buildGamesTab()},
			{Label: "PDF Library", Content: d.buildPDFLibraryTab()},
		},
		tabview.PainterOpt(d.painters.tabview),
	)

	return primitives.VBox(
		primitives.Box(
			primitives.Text("RPG Helper Bot").FontSize(20).Bold(),
		).Padding(12).Background(widget.RGBA8(245, 245, 245, 255)),
		primitives.Expanded(tabs),
		status,
	).Background(widget.RGBA8(250, 250, 250, 255))
}

func (d *Desktop) buildStatusBar() widget.Widget {
	return primitives.HBox(
		primitives.Text("").ContentSignal(state.NewComputed(func() string {
			_ = d.statusVersion.Get()
			if d.ctrl.LastError != "" {
				return "Error: " + d.ctrl.LastError
			}
			if d.ctrl.LastInfo != "" {
				return d.ctrl.LastInfo
			}
			d.ctrl.mu.Lock()
			tocDirty := d.ctrl.TOCDirty
			d.ctrl.mu.Unlock()
			if tocDirty {
				return "Unsaved section changes"
			}
			return "Ready"
		})),
	).Padding(8).Background(widget.RGBA8(240, 240, 240, 255))
}

func (d *Desktop) buildGamesTab() widget.Widget {
	header := primitives.HBox(
		d.buildFilterDropdown(),
		primitives.Expanded(primitives.Box()),
		button.New(
			button.TextOpt("+ New Game"),
			button.OnClick(func() { d.beginNewGame() }),
			button.PainterOpt(d.painters.button),
			button.VariantOpt(button.Filled),
		),
	).Padding(8).Gap(8)

	return primitives.VBox(
		header,
		primitives.Expanded(primitives.HBox(
			d.buildGameListPane(),
			primitives.Box().Width(1).Background(widget.RGBA8(220, 220, 220, 255)),
			primitives.Expanded(d.buildGameDetailPane()),
		)),
	).Gap(0)
}

func (d *Desktop) buildPDFLibraryTab() widget.Widget {
	header := primitives.HBox(
		components.Label("Configure PDFs once, then attach them to games"),
		primitives.Expanded(primitives.Box()),
		button.New(
			button.TextOpt("+ New PDF"),
			button.OnClick(func() { d.beginNewLibraryPDF() }),
			button.PainterOpt(d.painters.button),
			button.VariantOpt(button.Filled),
		),
	).Padding(8).Gap(8)

	return primitives.VBox(
		header,
		primitives.Expanded(primitives.HBox(
			d.buildLibraryListPane(),
			primitives.Box().Width(1).Background(widget.RGBA8(220, 220, 220, 255)),
			primitives.Expanded(d.buildLibraryEditorPane()),
		)),
	).Gap(0)
}

func (d *Desktop) buildFilterDropdown() widget.Widget {
	return dropdown.New(
		dropdown.Items("Active", "Archived", "All"),
		dropdown.Selected(0),
		dropdown.OnChange(func(index int, _ string) {
			switch index {
			case 1:
				d.ctrl.SetFilter(store.ListArchived)
			case 2:
				d.ctrl.SetFilter(store.ListAll)
			default:
				d.ctrl.SetFilter(store.ListActive)
			}
		}),
		dropdown.PainterOpt(d.painters.dropdown),
	)
}

func (d *Desktop) buildGameListPane() widget.Widget {
	lv := listview.New(
		listview.ItemCountSignal(d.gameListCount),
		listview.SelectedIndexSignal(d.gameSelectedIndex),
		listview.FixedItemHeight(44),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			_ = d.gamesVersion.Get()
			game, ok := d.ctrl.GameAt(ctx.Index)
			if !ok {
				return listRowText("(empty)")
			}
			label := game.Name
			d.ctrl.mu.Lock()
			isDraft := d.ctrl.DraftGame != nil && d.ctrl.DraftGame.ID == game.ID
			d.ctrl.mu.Unlock()
			if isDraft {
				label += " (unsaved)"
			}
			if game.Archived {
				label += " (Archived)"
			}
			return listRowText(label)
		}),
		listview.OnItemClick(func(index int) { d.selectGameAt(index) }),
		listview.PainterOpt(material3.ListViewPainter{Theme: d.theme}),
	)

	return primitives.VBox(
		components.Label("Games"),
		primitives.Expanded(lv),
	).Width(240).Padding(8).Gap(8)
}

func (d *Desktop) buildLibraryListPane() widget.Widget {
	lv := listview.New(
		listview.ItemCountSignal(d.libraryListCount),
		listview.SelectedIndexSignal(d.librarySelectedIndex),
		listview.FixedItemHeight(44),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			_ = d.libraryVersion.Get()
			pdf, counts, ok := d.ctrl.LibraryPDFAt(ctx.Index)
			if !ok {
				return listRowText("(empty)")
			}
			label := fmt.Sprintf("%s (%d sections)", pdf.Title, counts.Total)
			d.ctrl.mu.Lock()
			isDraft := d.ctrl.DraftLibraryPDF != nil && d.ctrl.DraftLibraryPDF.ID == pdf.ID
			d.ctrl.mu.Unlock()
			if isDraft {
				label += " — unsaved"
			}
			return listRowText(label)
		}),
		listview.OnItemClick(func(index int) { d.selectLibraryPDFAt(index) }),
		listview.PainterOpt(material3.ListViewPainter{Theme: d.theme}),
	)

	return primitives.VBox(
		components.Label("PDF Library"),
		primitives.Expanded(lv),
	).Width(280).Padding(8).Gap(8)
}

func (d *Desktop) buildGameDetailPane() widget.Widget {
	nameField := textfield.New(
		textfield.Placeholder("Game name"),
		textfield.ValueSignal(d.gameNameSignal),
		textfield.DisabledFn(func() bool {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.GameReadOnly
		}),
		textfield.PainterOpt(d.painters.textfield),
	)
	notesField := textfield.New(
		textfield.Placeholder("Notes"),
		textfield.ValueSignal(d.gameNotesSignal),
		textfield.DisabledFn(func() bool {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.GameReadOnly
		}),
		textfield.PainterOpt(d.painters.textfield),
	)
	optInField := textfield.New(
		textfield.Placeholder("Optional rules in use"),
		textfield.ValueSignal(d.gameOptInSignal),
		textfield.DisabledFn(func() bool {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.GameReadOnly
		}),
		textfield.PainterOpt(d.painters.textfield),
	)

	saveGameBtn := button.New(
		button.TextOpt("Save Game"),
		button.OnClick(func() {
			_ = d.ctrl.SaveGame(d.gameNameSignal.Get(), d.gameNotesSignal.Get(), d.gameOptInSignal.Get())
		}),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Filled),
		button.DisabledFn(func() bool {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.GameReadOnly
		}),
	)

	optionalTable := datatable.New(
		datatable.Columns([]datatable.Column{
			{Key: "entry", Title: "Optional sections reference", Width: 420},
		}),
		datatable.RowCountFn(func() int {
			_ = d.optionalRefVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return len(d.ctrl.OptionalRefs)
		}),
		datatable.CellValue(func(row int, _ string) string {
			_ = d.optionalRefVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			if row < 0 || row >= len(d.ctrl.OptionalRefs) {
				return ""
			}
			ref := d.ctrl.OptionalRefs[row]
			return fmt.Sprintf("%s > %s (pp. %d-%d)", ref.PDFTitle, ref.SectionTitle, ref.StartPage, ref.EndPage)
		}),
		datatable.PainterOpt(d.painters.datatable),
	)

	attachedList := listview.New(
		listview.ItemCountFn(func() int {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return len(d.ctrl.AttachedPDFs)
		}),
		listview.FixedItemHeight(44),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			if ctx.Index < 0 || ctx.Index >= len(d.ctrl.AttachedPDFs) {
				return listRowText("")
			}
			pdf := d.ctrl.AttachedPDFs[ctx.Index]
			return listRowText(pdf.Title + " — click to detach")
		}),
		listview.OnItemClick(func(index int) {
			d.ctrl.mu.Lock()
			if index < 0 || index >= len(d.ctrl.AttachedPDFs) {
				d.ctrl.mu.Unlock()
				return
			}
			pdfID := d.ctrl.AttachedPDFs[index].ID
			d.ctrl.mu.Unlock()
			d.ctrl.DetachPDFFromGame(pdfID)
		}),
		listview.PainterOpt(material3.ListViewPainter{Theme: d.theme}),
	)

	attachList := listview.New(
		listview.ItemCountFn(func() int {
			_ = d.libraryVersion.Get()
			_ = d.gamesVersion.Get()
			return d.ctrl.AttachablePDFCount()
		}),
		listview.FixedItemHeight(44),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			_ = d.libraryVersion.Get()
			_ = d.gamesVersion.Get()
			pdf, ok := d.ctrl.AttachablePDFAt(ctx.Index)
			if !ok {
				return listRowText("")
			}
			return listRowText(pdf.Title + " — click to attach")
		}),
		listview.OnItemClick(func(index int) {
			pdf, ok := d.ctrl.AttachablePDFAt(index)
			if !ok {
				return
			}
			d.ctrl.AttachPDFToGame(pdf.ID)
		}),
		listview.PainterOpt(material3.ListViewPainter{Theme: d.theme}),
	)

	hint := primitives.Text("").ContentSignal(state.NewComputed(func() string {
		_ = d.gamesVersion.Get()
		d.ctrl.mu.Lock()
		defer d.ctrl.mu.Unlock()
		if d.ctrl.SelectedGame == nil {
			return "Select or create a game."
		}
		if d.ctrl.DraftGame != nil {
			return "Save the game before attaching PDFs from the library."
		}
		return "Attach PDFs from the library list below. Click an attached PDF to detach it."
	}))

	return primitives.VBox(
		hint,
		components.Label("Game detail"),
		components.Label("Name"),
		nameField,
		components.Label("Notes"),
		notesField,
		components.Label("Optional rules in use"),
		optInField,
		saveGameBtn,
		primitives.HBox(
			button.New(button.TextOpt("Archive"), button.OnClick(func() { d.ctrl.ArchiveSelectedGame() }), button.PainterOpt(d.painters.button), button.VariantOpt(button.Outlined)),
			button.New(button.TextOpt("Restore"), button.OnClick(func() { d.ctrl.RestoreSelectedGame() }), button.PainterOpt(d.painters.button), button.VariantOpt(button.Tonal)),
		).Gap(8),
		components.Label("Optional sections reference"),
		optionalTable,
		components.Label("Attached PDFs"),
		primitives.Box(attachedList).Height(132),
		components.Label("Attach from library"),
		primitives.Box(attachList).Height(132),
	).Padding(12).Gap(8)
}

func (d *Desktop) buildLibraryEditorPane() widget.Widget {
	titleField := textfield.New(
		textfield.Placeholder("PDF title"),
		textfield.ValueSignal(d.pdfTitleSignal),
		textfield.PainterOpt(d.painters.textfield),
	)
	pathField := textfield.New(
		textfield.Placeholder("/path/to/rules.pdf"),
		textfield.ValueSignal(d.pdfPathSignal),
		textfield.PainterOpt(d.painters.textfield),
	)

	savePDFBtn := button.New(
		button.TextOpt("Save PDF"),
		button.OnClick(func() {
			_ = d.ctrl.SaveLibraryPDF(d.pdfTitleSignal.Get(), d.pdfPathSignal.Get())
		}),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Filled),
	)
	browseBtn := button.New(
		button.TextOpt("Browse…"),
		button.OnClick(func() { d.pickPDFPath() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Outlined),
	)
	deleteBtn := button.New(
		button.TextOpt("Delete PDF"),
		button.OnClick(func() { d.confirmDeleteLibraryPDF() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Outlined),
	)

	tocList := listview.New(
		listview.ItemCountSignal(d.tocSectionCount),
		listview.SelectedIndexSignal(d.selectedTOCIndex),
		listview.FixedItemHeight(44),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			_ = d.tocVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			if ctx.Index < 0 || ctx.Index >= len(d.ctrl.TOCSections) {
				return listRowText("(empty)")
			}
			sec := d.ctrl.TOCSections[ctx.Index]
			label := sec.Title
			if label == "" {
				label = "(untitled section)"
			}
			if sec.Optional {
				label += " [optional]"
			}
			label += fmt.Sprintf(" — pp. %d-%d", sec.StartPage, sec.EndPage)
			return listRowText(label)
		}),
		listview.OnItemClick(func(index int) {
			d.loadedTOCIndex = -1
			d.selectedTOCIndex.Set(index)
			d.reloadTOCEditorIfSelectionChanged()
		}),
		listview.PainterOpt(material3.ListViewPainter{Theme: d.theme}),
	)

	sectionTitleField := textfield.New(
		textfield.Placeholder("Section title"),
		textfield.ValueSignal(d.sectionTitleSignal),
		textfield.OnChange(func(v string) {
			idx := d.selectedTOCIndex.Get()
			if idx < 0 {
				return
			}
			d.ctrl.UpdateTOCSection(idx, func(s *models.TOCSection) { s.Title = v })
		}),
		textfield.DisabledFn(func() bool {
			_ = d.tocVersion.Get()
			return d.selectedTOCIndex.Get() < 0
		}),
		textfield.PainterOpt(d.painters.textfield),
	)
	sectionStartField := textfield.New(
		textfield.Placeholder("Start page"),
		textfield.ValueSignal(d.sectionStartSignal),
		textfield.OnChange(func(v string) {
			idx := d.selectedTOCIndex.Get()
			if idx < 0 {
				return
			}
			n, err := ParsePageValue(v)
			if err == nil {
				d.ctrl.UpdateTOCSection(idx, func(s *models.TOCSection) { s.StartPage = n })
			}
		}),
		textfield.DisabledFn(func() bool {
			_ = d.tocVersion.Get()
			return d.selectedTOCIndex.Get() < 0
		}),
		textfield.PainterOpt(d.painters.textfield),
	)
	sectionEndField := textfield.New(
		textfield.Placeholder("End page"),
		textfield.ValueSignal(d.sectionEndSignal),
		textfield.OnChange(func(v string) {
			idx := d.selectedTOCIndex.Get()
			if idx < 0 {
				return
			}
			n, err := ParsePageValue(v)
			if err == nil {
				d.ctrl.UpdateTOCSection(idx, func(s *models.TOCSection) { s.EndPage = n })
			}
		}),
		textfield.DisabledFn(func() bool {
			_ = d.tocVersion.Get()
			return d.selectedTOCIndex.Get() < 0
		}),
		textfield.PainterOpt(d.painters.textfield),
	)
	sectionOptionalCheckbox := checkbox.New(
		checkbox.LabelOpt("Optional section"),
		checkbox.CheckedSignal(d.sectionOptionalSignal),
		checkbox.OnToggle(func(checked bool) {
			idx := d.selectedTOCIndex.Get()
			if idx < 0 {
				return
			}
			d.ctrl.UpdateTOCSection(idx, func(s *models.TOCSection) { s.Optional = checked })
		}),
		checkbox.DisabledFn(func() bool {
			_ = d.tocVersion.Get()
			return d.selectedTOCIndex.Get() < 0
		}),
		checkbox.PainterOpt(d.painters.checkbox),
	)

	sectionActions := primitives.HBox(
		button.New(
			button.TextOpt("Move up"),
			button.OnClick(func() {
				idx := d.selectedTOCIndex.Get()
				if idx > 0 {
					d.ctrl.MoveTOCSection(idx, -1)
					d.loadedTOCIndex = -1
					d.selectedTOCIndex.Set(idx - 1)
					d.reloadTOCEditorIfSelectionChanged()
				}
			}),
			button.PainterOpt(d.painters.button),
			button.VariantOpt(button.Outlined),
			button.DisabledFn(func() bool { return d.selectedTOCIndex.Get() <= 0 }),
		),
		button.New(
			button.TextOpt("Move down"),
			button.OnClick(func() {
				idx := d.selectedTOCIndex.Get()
				d.ctrl.mu.Lock()
				count := len(d.ctrl.TOCSections)
				d.ctrl.mu.Unlock()
				if idx >= 0 && idx < count-1 {
					d.ctrl.MoveTOCSection(idx, 1)
					d.loadedTOCIndex = -1
					d.selectedTOCIndex.Set(idx + 1)
					d.reloadTOCEditorIfSelectionChanged()
				}
			}),
			button.PainterOpt(d.painters.button),
			button.VariantOpt(button.Outlined),
			button.DisabledFn(func() bool {
				idx := d.selectedTOCIndex.Get()
				d.ctrl.mu.Lock()
				count := len(d.ctrl.TOCSections)
				d.ctrl.mu.Unlock()
				return idx < 0 || idx >= count-1
			}),
		),
		button.New(
			button.TextOpt("Delete section"),
			button.OnClick(func() {
				idx := d.selectedTOCIndex.Get()
				if idx < 0 {
					return
				}
				d.ctrl.RemoveTOCSection(idx)
				d.loadedTOCIndex = -1
				d.selectedTOCIndex.Set(-1)
				d.reloadTOCEditorIfSelectionChanged()
			}),
			button.PainterOpt(d.painters.button),
			button.VariantOpt(button.Outlined),
			button.DisabledFn(func() bool { return d.selectedTOCIndex.Get() < 0 }),
		),
	).Gap(8)

	addSectionBtn := button.New(
		button.TextOpt("+ Add section"),
		button.OnClick(func() {
			newIndex := d.ctrl.AddTOCSection()
			if newIndex >= 0 {
				d.loadedTOCIndex = -1
				d.selectedTOCIndex.Set(newIndex)
				d.reloadTOCEditorIfSelectionChanged()
			}
		}),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Tonal),
		button.DisabledFn(func() bool {
			_ = d.libraryVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedLibraryPDF == nil
		}),
	)
	saveSectionsBtn := button.New(
		button.TextOpt("Save Sections"),
		button.OnClick(func() { _ = d.ctrl.SaveTOCSectionsNow() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Filled),
	)

	hint := primitives.Text("").ContentSignal(state.NewComputed(func() string {
		_ = d.libraryVersion.Get()
		d.ctrl.mu.Lock()
		defer d.ctrl.mu.Unlock()
		if d.ctrl.SelectedLibraryPDF == nil {
			return "Select or create a PDF from the library list."
		}
		return ""
	}))

	return primitives.VBox(
		hint,
		components.Label("PDF details"),
		components.Label("Title"),
		titleField,
		components.Label("File path"),
		pathField,
		primitives.HBox(browseBtn, savePDFBtn, deleteBtn).Gap(8),
		components.Label("Table of contents"),
		primitives.HBox(addSectionBtn, saveSectionsBtn).Gap(8),
		primitives.Box(tocList).Height(180),
		components.Label("Section editor"),
		sectionOptionalCheckbox,
		components.Label("Title"),
		sectionTitleField,
		primitives.HBox(
			components.Label("Start"),
			sectionStartField,
			components.Label("End"),
			sectionEndField,
		).Gap(8),
		sectionActions,
	).Padding(12).Gap(8)
}

func listRowText(label string) widget.Widget {
	if label == "" {
		label = " "
	}
	return primitives.Box(
		primitives.Text(label).FontSize(14).Color(widget.RGBA8(30, 30, 30, 255)),
	).Padding(12).Height(44)
}

func (d *Desktop) pickPDFPath() {
	paths, err := d.gogpuApp.ShowOpenFileDialog(gogpu.FileDialogOptions{
		Title: "Select PDF",
		Filters: []gogpu.FileTypeFilter{{
			Name:       "PDF files",
			Extensions: []string{"pdf"},
		}},
	})
	if err != nil {
		log.Printf("file dialog: %v", err)
		d.ctrl.setError(err)
		return
	}
	if len(paths) == 0 {
		return
	}
	d.pdfPathSignal.Set(paths[0])
	if d.pdfTitleSignal.Get() == "" {
		d.pdfTitleSignal.Set(fileStem(paths[0]))
	}
	d.statusVersion.Set(d.statusVersion.Get() + 1)
}

func (d *Desktop) confirmDeleteLibraryPDF() {
	d.ctrl.mu.Lock()
	pdf := d.ctrl.SelectedLibraryPDF
	d.ctrl.mu.Unlock()
	if pdf == nil {
		return
	}
	msg := fmt.Sprintf("Delete %q from the library? It must be detached from all games first.", pdf.Title)
	dlg := dialog.Confirm("Delete PDF", msg, func() {}, func() {
		d.ctrl.DeleteLibraryPDF(pdf.ID)
	})
	dlg.Show(d.ctx())
}

func fileStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}
