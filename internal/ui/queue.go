package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/icortesb/cupstui/internal/cups"
)

// queueModel is the queue view: a table of jobs plus a filter field.
type queueModel struct {
	table     table.Model
	filter    textinput.Model
	filtering bool
	visible   []cups.Job // aligned with the table rows
	width     int
}

func newQueue() queueModel {
	t := table.New(
		table.WithFocused(true),
		table.WithColumns(queueColumns(80)),
	)
	f := textinput.New()
	f.Prompt = "/"
	f.Placeholder = "free text, or printer:epson user:ana state:held"
	f.CharLimit = 80

	q := queueModel{table: t, filter: f}
	q.restyle()
	return q
}

// restyle reapplies the styles, which the table and the filter field copy when
// they are built.
func (q *queueModel) restyle() {
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
	q.table.SetStyles(s)

	q.filter.PromptStyle = styleKey
	q.filter.TextStyle = styleValue
	q.filter.PlaceholderStyle = styleDim
}

// queueColumns shares out the available width, leaving the document name
// whatever is left over.
func queueColumns(width int) []table.Column {
	const fixed = 1 + 6 + 12 + 18 + 13 + 7
	doc := width - fixed - 14
	if doc < 10 {
		doc = 10
	}
	return []table.Column{
		{Title: " ", Width: 1},
		{Title: "ID", Width: 6},
		{Title: "USER", Width: 12},
		{Title: "DOCUMENT", Width: doc},
		{Title: "PRINTER", Width: 18},
		{Title: "STATE", Width: 13},
		{Title: "TIME", Width: 7},
	}
}

func (q *queueModel) setSize(width, height int) {
	q.width = width
	q.table.SetColumns(queueColumns(width))
	q.table.SetWidth(width)
	if height < 3 {
		height = 3
	}
	q.table.SetHeight(height)
}

// setJobs rebuilds the rows under the current filter, keeping the cursor
// position as far as it can.
func (q *queueModel) setJobs(jobs []cups.Job) {
	q.visible = cups.FilterJobs(jobs, q.filter.Value())

	rows := make([]table.Row, 0, len(q.visible))
	for _, j := range q.visible {
		when := ""
		if !j.Created.IsZero() {
			when = j.Created.Format("15:04")
		}
		name := j.Name
		if name == "" {
			name = "(untitled)"
		}
		rows = append(rows, table.Row{
			" ", strconv.Itoa(j.ID), j.User, name, j.Printer, jobState(j), when,
		})
	}

	cursor := q.table.Cursor()
	q.table.SetRows(rows)
	if cursor >= len(rows) {
		cursor = len(rows) - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	q.table.SetCursor(cursor)
	q.markCursor()
}

// markCursor moves the cursor bar. It needs a column of its own because bubbles
// applies the Selected style outside the cells, where a padding shifts the row
// out of line with the header and a background never reaches the text.
func (q *queueModel) markCursor() {
	rows := q.table.Rows()
	for i := range rows {
		rows[i][0] = " "
	}
	if c := q.table.Cursor(); c >= 0 && c < len(rows) {
		rows[c][0] = "▌"
	}
	q.table.SetRows(rows)
}

// jobState is the state cell: a printing job shows how far along it is, which
// is the one thing worth watching in a queue.
func jobState(j cups.Job) string {
	fraction, known := j.Progress()
	if !known {
		return j.State.String()
	}
	return fmt.Sprintf("%s %d%%", miniBar(fraction, 5), int(fraction*100+0.5))
}

// miniBar draws a bar in the width of a table cell, where a full progress
// component would not fit.
func miniBar(fraction float64, width int) string {
	filled := int(fraction*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func (q queueModel) selected() (cups.Job, bool) {
	i := q.table.Cursor()
	if i < 0 || i >= len(q.visible) {
		return cups.Job{}, false
	}
	return q.visible[i], true
}

func (q *queueModel) startFiltering() tea.Cmd {
	q.filtering = true
	q.filter.Focus()
	return textinput.Blink
}

func (q *queueModel) stopFiltering(clear bool) {
	q.filtering = false
	q.filter.Blur()
	if clear {
		q.filter.SetValue("")
	}
}

func (q queueModel) view(total int) string {
	header := styleDim.Render(fmt.Sprintf("%d %s", total, plural(total, "job")))
	if q.filter.Value() != "" {
		header = styleDim.Render(fmt.Sprintf("%d of %d jobs", len(q.visible), total))
	}
	if q.filtering || q.filter.Value() != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Left, q.filter.View(), "  ", header)
	}

	if len(q.visible) == 0 {
		empty := "The queue is empty."
		if q.filter.Value() != "" {
			empty = "No jobs match the filter."
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, "", styleDim.Render("  "+empty))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, q.table.View())
}
