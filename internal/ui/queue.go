package ui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/icortes/cupstui/internal/cups"
)

// queueModel es la vista de cola: una tabla de trabajos más un campo de filtro.
type queueModel struct {
	table     table.Model
	filter    textinput.Model
	filtering bool
	visible   []cups.Job // trabajos que se ven, alineados con las filas de la tabla
	width     int
}

func newQueue() queueModel {
	t := table.New(
		table.WithFocused(true),
		table.WithColumns(queueColumns(80)),
	)
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
	t.SetStyles(s)

	f := textinput.New()
	f.PromptStyle = styleKey
	f.TextStyle = styleValue
	f.PlaceholderStyle = styleDim
	f.Prompt = "/"
	f.Placeholder = "usuario, documento, impresora o estado"
	f.CharLimit = 60

	return queueModel{table: t, filter: f}
}

// queueColumns reparte el ancho disponible dejando que el nombre del documento
// se quede con lo que sobra.
func queueColumns(width int) []table.Column {
	const fixed = 6 + 12 + 18 + 13 + 7 // id, usuario, impresora, estado, hora
	doc := width - fixed - 14          // 14 = padding de las celdas
	if doc < 10 {
		doc = 10
	}
	return []table.Column{
		{Title: "ID", Width: 6},
		{Title: "USUARIO", Width: 12},
		{Title: "DOCUMENTO", Width: doc},
		{Title: "IMPRESORA", Width: 18},
		{Title: "ESTADO", Width: 13},
		{Title: "HORA", Width: 7},
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

// setJobs vuelve a armar las filas aplicando el filtro vigente, conservando la
// posición del cursor en la medida de lo posible.
func (q *queueModel) setJobs(jobs []cups.Job) {
	q.visible = cups.FilterJobs(jobs, q.filter.Value())

	rows := make([]table.Row, 0, len(q.visible))
	for _, j := range q.visible {
		hora := ""
		if !j.Created.IsZero() {
			hora = j.Created.Format("15:04")
		}
		name := j.Name
		if name == "" {
			name = "(sin nombre)"
		}
		rows = append(rows, table.Row{
			strconv.Itoa(j.ID), j.User, name, j.Printer, j.State.String(), hora,
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
	header := styleDim.Render(fmt.Sprintf("%d de %d trabajos", len(q.visible), total))
	if q.filtering || q.filter.Value() != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Left, q.filter.View(), "  ", header)
	}

	if len(q.visible) == 0 {
		empty := "La cola está vacía."
		if q.filter.Value() != "" {
			empty = "Ningún trabajo coincide con el filtro."
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, "", styleDim.Render("  "+empty))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, q.table.View())
}
