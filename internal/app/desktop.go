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
	"github.com/gogpu/ui/core/scrollview"
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
	pdfVersion         state.Signal[int]
	tocVersion         state.Signal[int]
	optionalRefVersion state.Signal[int]
	statusVersion      state.Signal[int]

	gameNameSignal  state.Signal[string]
	gameNotesSignal state.Signal[string]
	gameOptInSignal state.Signal[string]
	pdfTitleSignal  state.Signal[string]
	pdfPathSignal   state.Signal[string]

	loadedGameID string
	loadedPDFID  string
}

type desktopPainters struct {
	button    material3.ButtonPainter
	checkbox  material3.CheckboxPainter
	textfield material3.TextFieldPainter
	dropdown  material3.DropdownPainter
	datatable material3.DataTablePainter
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
		},
		gamesVersion:       state.NewSignal(0),
		pdfVersion:         state.NewSignal(0),
		tocVersion:         state.NewSignal(0),
		optionalRefVersion: state.NewSignal(0),
		statusVersion:      state.NewSignal(0),
		gameNameSignal:     state.NewSignal(""),
		gameNotesSignal:    state.NewSignal(""),
		gameOptInSignal:    state.NewSignal(""),
		pdfTitleSignal:     state.NewSignal(""),
		pdfPathSignal:      state.NewSignal(""),
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
		d.pdfVersion.Set(d.pdfVersion.Get() + 1)
		d.tocVersion.Set(d.tocVersion.Get() + 1)
		d.optionalRefVersion.Set(d.optionalRefVersion.Get() + 1)
		d.statusVersion.Set(d.statusVersion.Get() + 1)
		d.reloadFormsIfSelectionChanged()
	})

	ctrl.RefreshGames()
	uiApp.SetRoot(d.buildRoot())
	return desktop.Run(gogpuApp, uiApp)
}

func (d *Desktop) reloadFormsIfSelectionChanged() {
	d.ctrl.mu.Lock()
	gameID := ""
	pdfID := ""
	if d.ctrl.SelectedGame != nil {
		gameID = d.ctrl.SelectedGame.ID
	}
	if d.ctrl.SelectedPDF != nil {
		pdfID = d.ctrl.SelectedPDF.ID
	}
	game := d.ctrl.SelectedGame
	pdf := d.ctrl.SelectedPDF
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
	header := primitives.HBox(
		primitives.Text("RPG Helper Bot").FontSize(20).Bold(),
		primitives.Expanded(primitives.Box()),
		d.buildFilterDropdown(),
		button.New(
			button.TextOpt("+ New Game"),
			button.OnClick(func() { d.ctrl.BeginNewGame() }),
			button.PainterOpt(d.painters.button),
			button.VariantOpt(button.Filled),
		),
	).Padding(12).Gap(12).Background(widget.RGBA8(245, 245, 245, 255))

	body := primitives.HBox(
		d.buildGameListPane(),
		primitives.Box().Width(1).Background(widget.RGBA8(220, 220, 220, 255)),
		primitives.Expanded(d.buildGameDetailPane()),
		primitives.Box().Width(1).Background(widget.RGBA8(220, 220, 220, 255)),
		primitives.Expanded(d.buildPDFPane()),
	).Background(widget.RGBA8(250, 250, 250, 255))

	status := primitives.HBox(
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

	return primitives.VBox(header, primitives.Expanded(body), status)
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
		listview.ItemCountFn(func() int {
			_ = d.gamesVersion.Get()
			return d.ctrl.GameListCount()
		}),
		listview.FixedItemHeight(44),
		listview.SelectionModeOpt(listview.SelectionSingle),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			_ = d.gamesVersion.Get()
			game, ok := d.ctrl.GameAt(ctx.Index)
			if !ok {
				return primitives.Box()
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
			color := widget.RGBA8(30, 30, 30, 255)
			if game.Archived || isDraft {
				color = widget.RGBA8(120, 120, 120, 255)
			}
			return primitives.Box(
				primitives.Text(label).FontSize(14).Color(color),
			).Padding(12)
		}),
		listview.OnItemClick(func(index int) {
			game, ok := d.ctrl.GameAt(index)
			if !ok {
				return
			}
			d.ctrl.SelectGame(game.ID)
		}),
		listview.PainterOpt(material3.ListViewPainter{Theme: d.theme}),
	)

	return primitives.VBox(
		components.Label("Games"),
		primitives.Expanded(lv),
	).Width(240).Padding(8).Gap(8)
}

func (d *Desktop) buildGameDetailPane() widget.Widget {
	nameField := textfield.New(
		textfield.Placeholder("Game name"),
		textfield.ValueSignal(d.gameNameSignal),
		textfield.DisabledFn(func() bool {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly
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
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly
		}),
		textfield.PainterOpt(d.painters.textfield),
	)

	optInField := textfield.New(
		textfield.Placeholder("Describe which optional rules your table uses"),
		textfield.ValueSignal(d.gameOptInSignal),
		textfield.DisabledFn(func() bool {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly
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
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly
		}),
	)

	optionalTable := datatable.New(
		datatable.Columns([]datatable.Column{
			{Key: "entry", Title: "Optional sections reference", Width: 360},
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

	pdfList := listview.New(
		listview.ItemCountFn(func() int {
			_ = d.pdfVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return len(d.ctrl.PDFs)
		}),
		listview.FixedItemHeight(56),
		listview.SelectionModeOpt(listview.SelectionSingle),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			_ = d.pdfVersion.Get()
			d.ctrl.mu.Lock()
			var title, path, id string
			var counts sectionCounts
			if ctx.Index >= 0 && ctx.Index < len(d.ctrl.PDFs) {
				pdf := d.ctrl.PDFs[ctx.Index]
				id = pdf.ID
				title = pdf.Title
				path = pdf.FilePath
				counts = d.ctrl.PDFCounts[id]
			}
			d.ctrl.mu.Unlock()

			badge := fmt.Sprintf("%d sections", counts.Total)
			if counts.Optional > 0 {
				badge += fmt.Sprintf(", %d optional", counts.Optional)
			}
			return primitives.VBox(
				primitives.Text(title).FontSize(14).Bold(),
				components.Muted(truncate(path, 64)),
				components.Muted(badge),
			).Padding(8).Gap(2)
		}),
		listview.OnItemClick(func(index int) {
			d.ctrl.mu.Lock()
			var id string
			if index >= 0 && index < len(d.ctrl.PDFs) {
				id = d.ctrl.PDFs[index].ID
			}
			d.ctrl.mu.Unlock()
			d.ctrl.SelectPDF(id)
		}),
		listview.PainterOpt(material3.ListViewPainter{Theme: d.theme}),
	)

	archiveBtn := button.New(
		button.TextOpt("Archive"),
		button.OnClick(func() { d.ctrl.ArchiveSelectedGame() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Outlined),
		button.DisabledFn(func() bool {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly || d.ctrl.DraftGame != nil
		}),
	)
	restoreBtn := button.New(
		button.TextOpt("Restore"),
		button.OnClick(func() { d.ctrl.RestoreSelectedGame() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Tonal),
		button.DisabledFn(func() bool {
			_ = d.gamesVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || !d.ctrl.ReadOnly
		}),
	)

	gameEmptyHint := primitives.Text("").ContentSignal(state.NewComputed(func() string {
		_ = d.gamesVersion.Get()
		d.ctrl.mu.Lock()
		defer d.ctrl.mu.Unlock()
		if d.ctrl.SelectedGame == nil {
			return "Select or create a game to edit details."
		}
		return ""
	}))

	detailContent := primitives.VBox(
		gameEmptyHint,
		components.Label("Game detail"),
		components.Label("Name"),
		nameField,
		components.Label("Notes"),
		notesField,
		components.Label("Optional rules in use"),
		optInField,
		saveGameBtn,
		components.Label("Optional sections reference"),
		optionalTable,
		primitives.HBox(archiveBtn, restoreBtn).Gap(8),
		components.Label("PDFs for this game"),
		primitives.Expanded(pdfList),
	).Padding(12).Gap(8)

	return scrollview.New(detailContent)
}

func (d *Desktop) buildPDFPane() widget.Widget {
	titleField := textfield.New(
		textfield.Placeholder("PDF title"),
		textfield.ValueSignal(d.pdfTitleSignal),
		textfield.DisabledFn(func() bool {
			_ = d.pdfVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly || d.ctrl.DraftGame != nil
		}),
		textfield.PainterOpt(d.painters.textfield),
	)

	pathField := textfield.New(
		textfield.Placeholder("/path/to/rules.pdf"),
		textfield.ValueSignal(d.pdfPathSignal),
		textfield.DisabledFn(func() bool {
			_ = d.pdfVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly || d.ctrl.DraftGame != nil
		}),
		textfield.PainterOpt(d.painters.textfield),
	)

	browseBtn := button.New(
		button.TextOpt("Browse…"),
		button.OnClick(func() { d.pickPDFPath() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Outlined),
		button.DisabledFn(func() bool {
			_ = d.pdfVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly || d.ctrl.DraftGame != nil
		}),
	)

	savePDFBtn := button.New(
		button.TextOpt("Save PDF"),
		button.OnClick(func() {
			_ = d.ctrl.SavePDF(d.pdfTitleSignal.Get(), d.pdfPathSignal.Get())
		}),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Filled),
		button.DisabledFn(func() bool {
			_ = d.pdfVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedGame == nil || d.ctrl.ReadOnly || d.ctrl.DraftGame != nil
		}),
	)

	tocList := listview.New(
		listview.ItemCountFn(func() int {
			_ = d.tocVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return len(d.ctrl.TOCSections)
		}),
		listview.ItemHeightFn(func(int) float32 { return 72 }),
		listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
			return d.buildTOCRow(ctx.Index)
		}),
		listview.PainterOpt(material3.ListViewPainter{Theme: d.theme}),
	)

	addSectionBtn := button.New(
		button.TextOpt("+ Add section"),
		button.OnClick(func() { d.ctrl.AddTOCSection() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Tonal),
		button.DisabledFn(func() bool {
			_ = d.tocVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedPDF == nil || d.ctrl.ReadOnly
		}),
	)

	saveSectionsBtn := button.New(
		button.TextOpt("Save Sections"),
		button.OnClick(func() { _ = d.ctrl.SaveTOCSectionsNow() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Filled),
		button.DisabledFn(func() bool {
			_ = d.tocVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedPDF == nil || d.ctrl.ReadOnly
		}),
	)

	removePDFBtn := button.New(
		button.TextOpt("Remove PDF"),
		button.OnClick(func() { d.confirmRemovePDF() }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Outlined),
		button.DisabledFn(func() bool {
			_ = d.pdfVersion.Get()
			d.ctrl.mu.Lock()
			defer d.ctrl.mu.Unlock()
			return d.ctrl.SelectedPDF == nil || d.ctrl.ReadOnly
		}),
	)

	pdfEmptyHint := primitives.Text("").ContentSignal(state.NewComputed(func() string {
		_ = d.pdfVersion.Get()
		d.ctrl.mu.Lock()
		defer d.ctrl.mu.Unlock()
		if d.ctrl.DraftGame != nil {
			return "Save the game before adding PDFs."
		}
		if d.ctrl.SelectedGame == nil {
			return "Select a game to manage PDFs."
		}
		if d.ctrl.SelectedPDF == nil {
			return "Select a PDF from the list, or enter a path and click Save PDF to add one."
		}
		return ""
	}))

	content := primitives.VBox(
		pdfEmptyHint,
		components.Label("PDF and table of contents"),
		components.Label("Title"),
		titleField,
		components.Label("File path"),
		pathField,
		primitives.HBox(browseBtn, savePDFBtn).Gap(8),
		removePDFBtn,
		components.Label("Sections"),
		primitives.HBox(addSectionBtn, saveSectionsBtn).Gap(8),
		primitives.Expanded(tocList),
	).Padding(12).Gap(8)

	return scrollview.New(content)
}

func (d *Desktop) buildTOCRow(index int) widget.Widget {
	_ = d.tocVersion.Get()
	d.ctrl.mu.Lock()
	var sec models.TOCSection
	if index >= 0 && index < len(d.ctrl.TOCSections) {
		sec = d.ctrl.TOCSections[index]
	}
	readOnly := d.ctrl.ReadOnly
	d.ctrl.mu.Unlock()

	titleField := textfield.New(
		textfield.InitialValue(sec.Title),
		textfield.Placeholder("Section title"),
		textfield.OnChange(func(v string) {
			d.ctrl.UpdateTOCSection(index, func(s *models.TOCSection) { s.Title = v })
		}),
		textfield.Disabled(readOnly),
		textfield.PainterOpt(d.painters.textfield),
	)
	startField := textfield.New(
		textfield.InitialValue(strconv.Itoa(sec.StartPage)),
		textfield.Placeholder("Start"),
		textfield.OnChange(func(v string) {
			n, err := ParsePageValue(v)
			if err != nil {
				return
			}
			d.ctrl.UpdateTOCSection(index, func(s *models.TOCSection) { s.StartPage = n })
		}),
		textfield.Disabled(readOnly),
		textfield.PainterOpt(d.painters.textfield),
	)
	endField := textfield.New(
		textfield.InitialValue(strconv.Itoa(sec.EndPage)),
		textfield.Placeholder("End"),
		textfield.OnChange(func(v string) {
			n, err := ParsePageValue(v)
			if err != nil {
				return
			}
			d.ctrl.UpdateTOCSection(index, func(s *models.TOCSection) { s.EndPage = n })
		}),
		textfield.Disabled(readOnly),
		textfield.PainterOpt(d.painters.textfield),
	)

	optCheckbox := checkbox.New(
		checkbox.LabelOpt("Optional"),
		checkbox.Checked(sec.Optional),
		checkbox.OnToggle(func(checked bool) {
			d.ctrl.UpdateTOCSection(index, func(s *models.TOCSection) { s.Optional = checked })
		}),
		checkbox.Disabled(readOnly),
		checkbox.PainterOpt(d.painters.checkbox),
	)

	upBtn := button.New(
		button.TextOpt("Up"),
		button.OnClick(func() { d.ctrl.MoveTOCSection(index, -1) }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.TextOnly),
		button.Disabled(readOnly),
	)
	downBtn := button.New(
		button.TextOpt("Down"),
		button.OnClick(func() { d.ctrl.MoveTOCSection(index, 1) }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.TextOnly),
		button.Disabled(readOnly),
	)
	delBtn := button.New(
		button.TextOpt("Delete"),
		button.OnClick(func() { d.ctrl.RemoveTOCSection(index) }),
		button.PainterOpt(d.painters.button),
		button.VariantOpt(button.Outlined),
		button.Disabled(readOnly),
	)

	return primitives.VBox(
		primitives.HBox(
			optCheckbox,
			primitives.Expanded(titleField),
		).Gap(8),
		primitives.HBox(
			components.Label("Start"),
			startField,
			components.Label("End"),
			endField,
			upBtn,
			downBtn,
			delBtn,
		).Gap(8),
	).Padding(8).Gap(4).Background(widget.RGBA8(255, 255, 255, 255)).Rounded(8)
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

func (d *Desktop) confirmRemovePDF() {
	d.ctrl.mu.Lock()
	pdf := d.ctrl.SelectedPDF
	gameID := ""
	if d.ctrl.SelectedGame != nil {
		gameID = d.ctrl.SelectedGame.ID
	}
	d.ctrl.mu.Unlock()
	if pdf == nil || gameID == "" {
		return
	}
	msg := fmt.Sprintf("Remove %q from this game and delete its sections?", pdf.Title)
	dlg := dialog.Confirm("Remove PDF", msg, func() {}, func() {
		d.ctrl.RemovePDF(pdf.ID)
	})
	dlg.Show(d.ctx())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func fileStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}
