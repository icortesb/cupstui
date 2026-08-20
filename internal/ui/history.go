package ui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/icortes/cupstui/internal/cups"
)

// historyEntries caps how much of the page log is kept in memory.
const historyEntries = 2000

// historyModel shows finished jobs, newest first, with a usage summary.
type historyModel struct {
	table     table.Model
	filter    textinput.Model
	filtering bool
	all       []cups.HistoryEntry
	visible   []cups.HistoryEntry
	err       error
	width     int
}

func newHistory() historyModel {
	t := table.New(table.WithFocused(true), table.WithColumns(historyColumns(80)))

	f := textinput.New()
	f.Prompt = "/"
	f.Placeholder = "free text, or printer:epson user:ana document:invoice"
	f.CharLimit = 80

	h := historyModel{table: t, filter: f}
	h.restyle()
	return h
}

func (h *historyModel) restyle() {
	s := table.DefaultStyles()
	s.Header = base().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		BorderBottom(true).
		Bold(true).
		Foreground(colorMuted).
		Padding(0, 1)
	s.Cell = base().Foreground(colorText).Padding(0, 1)
	s.Selected = base().
		Foreground(lipgloss.Color("232")).
		Background(colorAccent).
		Bold(true).
		Padding(0, 1)
	if !transparent {
		s.Header = s.Header.BorderBackground(colorBG)
	}
	h.table.SetStyles(s)

	h.filter.PromptStyle = styleKey
	h.filter.TextStyle = styleValue
	h.filter.PlaceholderStyle = styleDim
}

func historyColumns(width int) []table.Column {
	const fixed = 17 + 12 + 18 + 7
	doc := width - fixed - 12
	if doc < 10 {
		doc = 10
	}
	return []table.Column{
		{Title: "WHEN", Width: 17},
		{Title: "USER", Width: 12},
		{Title: "PRINTER", Width: 18},
		{Title: "DOCUMENT", Width: doc},
		{Title: "PAGES", Width: 7},
	}
}

func (h *historyModel) setSize(width, height int) {
	h.width = width
	h.table.SetColumns(historyColumns(width))
	h.table.SetWidth(width)
	if height < 3 {
		height = 3
	}
	h.table.SetHeight(height)
}

func (h *historyModel) setEntries(entries []cups.HistoryEntry, err error) {
	h.all, h.err = entries, err
	h.apply()
}

// apply rebuilds the rows from the current filter, newest first.
func (h *historyModel) apply() {
	h.visible = cups.FilterHistory(h.all, h.filter.Value())

	rows := make([]table.Row, 0, len(h.visible))
	for i := len(h.visible) - 1; i >= 0; i-- {
		e := h.visible[i]
		document := e.Document
		if document == "" {
			document = "(untitled)"
		}
		rows = append(rows, table.Row{
			e.When.Format("2006-01-02 15:04"),
			e.User,
			e.Printer,
			document,
			strconv.Itoa(e.Pages),
		})
	}

	cursor := h.table.Cursor()
	h.table.SetRows(rows)
	if cursor >= len(rows) {
		cursor = len(rows) - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	h.table.SetCursor(cursor)
}

func (h *historyModel) startFiltering() tea.Cmd {
	h.filtering = true
	h.filter.Focus()
	return textinput.Blink
}

func (h *historyModel) stopFiltering(clear bool) {
	h.filtering = false
	h.filter.Blur()
	if clear {
		h.filter.SetValue("")
	}
}

func (h historyModel) view() string {
	if h.err != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			"  "+styleErrText.Render("Could not read the print history"),
			"  "+styleDim.Render(describeError(h.err)),
			"",
			"  "+styleDim.Render("The history comes from "+cups.LogFiles[2].Path+"."),
		)
	}

	jobs, pages := cups.HistoryTotals(h.visible)
	summary := styleDim.Render(fmt.Sprintf("%d jobs · %d pages", jobs, pages))
	if h.filter.Value() != "" {
		summary = styleDim.Render(fmt.Sprintf("%d of %d jobs · %d pages",
			len(h.visible), len(h.all), pages))
	}

	header := summary
	if h.filtering || h.filter.Value() != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Left, h.filter.View(), "  ", summary)
	}

	if len(h.visible) == 0 {
		empty := "No printing recorded yet."
		if h.filter.Value() != "" {
			empty = "No jobs match the filter."
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, "", styleDim.Render("  "+empty))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, h.table.View())
}

// readHistory loads the page log off the UI goroutine.
func readHistory() tea.Cmd {
	return func() tea.Msg {
		entries, err := cups.History(historyEntries)
		return historyMsg{entries: entries, err: err}
	}
}

// exportHistory writes the rows currently on screen, filter included.
func exportHistory(entries []cups.HistoryEntry) tea.Cmd {
	return func() tea.Msg {
		path, err := cups.DefaultExportPath(time.Now())
		if err != nil {
			return statusMsg{err: err}
		}
		if err := cups.ExportCSV(path, entries); err != nil {
			return statusMsg{err: err}
		}
		return statusMsg{text: fmt.Sprintf("Exported %d rows to %s", len(entries), path)}
	}
}
