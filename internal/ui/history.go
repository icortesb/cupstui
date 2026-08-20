package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/icortesb/cupstui/internal/cups"
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
	summary   bool // show the per-user and per-printer totals instead of the rows
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
	s.Selected = base().Foreground(colorAccent).Bold(true)
	if !transparent {
		s.Header = s.Header.BorderBackground(colorBG)
	}
	h.table.SetStyles(s)

	h.filter.PromptStyle = styleKey
	h.filter.TextStyle = styleValue
	h.filter.PlaceholderStyle = styleDim
}

func historyColumns(width int) []table.Column {
	const fixed = 1 + 17 + 12 + 18 + 7
	doc := width - fixed - 12
	if doc < 10 {
		doc = 10
	}
	return []table.Column{
		{Title: " ", Width: 1},
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
			" ",
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
	h.markCursor()
}

func (h *historyModel) markCursor() {
	rows := h.table.Rows()
	for i := range rows {
		rows[i][0] = " "
	}
	if c := h.table.Cursor(); c >= 0 && c < len(rows) {
		rows[c][0] = "▌"
	}
	h.table.SetRows(rows)
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

	if h.summary {
		return h.summaryView()
	}

	jobs, pages := cups.HistoryTotals(h.visible)
	summary := styleDim.Render(fmt.Sprintf("%d %s · %d %s",
		jobs, plural(jobs, "job"), pages, plural(pages, "page")))
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

// summaryView answers the question an audit starts from: who and what printed
// the most. The rows behind it stay one key away.
func (h historyModel) summaryView() string {
	jobs, pages := cups.HistoryTotals(h.visible)

	head := styleDim.Render(fmt.Sprintf("%d %s · %d %s",
		jobs, plural(jobs, "job"), pages, plural(pages, "page")))
	if h.filter.Value() != "" {
		head = styleDim.Render(fmt.Sprintf("%d of %d jobs · %d pages",
			len(h.visible), len(h.all), pages))
	}

	// Side by side each column gets half the screen. Below the point where a
	// name, a bar and the numbers still fit, they stack instead of wrapping.
	column := h.width/2 - 1
	if column < minUsageColumn {
		body := lipgloss.JoinVertical(lipgloss.Left,
			usageTable("BY USER", cups.UsageByUser(h.visible), pages, h.width),
			"",
			usageTable("BY PRINTER", cups.UsageByPrinter(h.visible), pages, h.width))
		return lipgloss.JoinVertical(lipgloss.Left, head, "", body)
	}

	left := usageTable("BY USER", cups.UsageByUser(h.visible), pages, column)
	right := usageTable("BY PRINTER", cups.UsageByPrinter(h.visible), pages, column)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		styleApp.Width(column+1).Render(left), right)
	return lipgloss.JoinVertical(lipgloss.Left, head, "", body)
}

// minUsageColumn is the narrowest a totals column can be and still hold a
// name, a bar and the counts.
const minUsageColumn = 46

// usageTable lists totals with a share bar, so the distribution reads at a
// glance rather than from comparing numbers. It is drawn to fit width: two of
// these sit side by side and a row that overflows wraps into the other one.
func usageTable(title string, rows []cups.Usage, totalPages, width int) string {
	var b strings.Builder
	b.WriteString("  " + styleBold.Render(title) + "\n")

	if len(rows) == 0 {
		b.WriteString("  " + styleDim.Render("nothing recorded") + "\n")
		return b.String()
	}

	for i, r := range rows {
		if i >= 8 {
			b.WriteString("  " + styleDim.Render(fmt.Sprintf("+%d more", len(rows)-i)) + "\n")
			break
		}

		share := 0.0
		if totalPages > 0 {
			share = float64(r.Pages) / float64(totalPages)
		}
		counts := fmt.Sprintf("%dp · %dj", r.Pages, r.Jobs)
		name, bar := usageWidths(width, len([]rune(counts)))

		row := "  " + styleValue.Render(pad(truncate(r.Name, name), name))
		if bar > 0 {
			row += " " + styleAccentText.Render(miniBar(share, bar))
		}
		b.WriteString(row + " " + styleDim.Render(counts) + "\n")
	}
	return b.String()
}

// usageWidths shares a row out between the name and the share bar. The counts
// are never dropped — they are the report — and the bar goes first when the
// column is too narrow for everything.
func usageWidths(width, counts int) (name, bar int) {
	// Two leading spaces plus one separator before the counts, and one more
	// before the bar when there is a bar.
	available := width - 2 - 1 - counts

	bar = 10
	if available < bar+1+4 {
		bar = 0
	}
	name = available - bar
	if bar > 0 {
		name--
	}

	if name < 3 {
		name = 3
	}
	if name > 20 {
		name = 20
	}
	return name, bar
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
