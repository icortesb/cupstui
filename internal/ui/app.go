// Package ui es la interfaz de terminal, construida sobre bubbletea.
package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/icortes/cupstui/internal/cups"
)

// refreshInterval es cada cuánto se vuelve a consultar CUPS. La consulta corre
// en un tea.Cmd, así que nunca bloquea el dibujado.
const refreshInterval = 3 * time.Second

// requestTimeout evita que un CUPS colgado congele el refresco para siempre.
const requestTimeout = 5 * time.Second

type tab int

const (
	tabDashboard tab = iota
	tabQueue
	tabPrinters
)

var tabNames = []string{"Dashboard", "Cola", "Impresoras"}

type (
	tickMsg     time.Time
	snapshotMsg struct {
		snap cups.Snapshot
		err  error
	}
	actionMsg struct {
		text string
		err  error
	}
)

// confirmation es una acción destructiva esperando el sí del usuario.
type confirmation struct {
	prompt string
	run    func() tea.Msg
}

// Model es el modelo raíz: mantiene la foto de CUPS y reparte teclas y
// dibujado entre las vistas.
type Model struct {
	client *cups.Client

	tab      tab
	snap     cups.Snapshot
	err      error // error del último refresco, se muestra como cartel
	loaded   bool
	lastSync time.Time
	// refreshing evita que los tics se apilen si CUPS se pone lento: sin esto
	// una consulta que tarda más que el intervalo dispara otra encima.
	refreshing bool

	queue    queueModel
	printers printersModel

	confirm     *confirmation
	status      string
	statusIsErr bool
	showHelp    bool

	width, height int
}

// New arma el modelo raíz con un cliente ya conectado.
func New(c *cups.Client) Model {
	return Model{
		client: c,
		queue:  newQueue(),
		width:  80,
		height: 24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refresh(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// refresh consulta CUPS fuera del hilo de la UI.
func (m Model) refresh() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		snap, err := c.Snapshot(ctx)
		return snapshotMsg{snap: snap, err: err}
	}
}

// action envuelve una operación de escritura: informa el resultado y dispara
// un refresco inmediato para que la vista no quede desactualizada.
func action(text string, run func() error) func() tea.Msg {
	return func() tea.Msg {
		return actionMsg{text: text, err: run()}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.queue.setSize(m.width, m.bodyHeight()-2)
		return m, nil

	case tickMsg:
		if m.refreshing {
			return m, tickCmd()
		}
		m.refreshing = true
		return m, tea.Batch(m.refresh(), tickCmd())

	case snapshotMsg:
		m.refreshing = false
		m.err = msg.err
		if msg.err == nil {
			m.snap = msg.snap
			m.loaded = true
			m.lastSync = time.Now()
		}
		m.queue.setJobs(m.snap.Jobs)
		m.printers.clamp(len(m.snap.Printers))
		return m, nil

	case actionMsg:
		m.refreshing = true
		if msg.err != nil {
			m.setStatus(describeError(msg.err), true)
		} else {
			m.setStatus(msg.text, false)
		}
		return m, m.refresh()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Una confirmación pendiente se come todas las teclas.
	if m.confirm != nil {
		switch strings.ToLower(msg.String()) {
		case "y", "s":
			run := m.confirm.run
			m.confirm = nil
			m.status = ""
			return m, func() tea.Msg { return run() }
		case "n", "esc", "q", "ctrl+c":
			m.confirm = nil
			m.setStatus("Cancelado.", false)
			return m, nil
		}
		return m, nil
	}

	// Mientras se escribe el filtro, el campo de texto tiene prioridad.
	if m.tab == tabQueue && m.queue.filtering {
		switch msg.String() {
		case "enter":
			m.queue.stopFiltering(false)
			return m, nil
		case "esc":
			m.queue.stopFiltering(true)
			m.queue.setJobs(m.snap.Jobs)
			return m, nil
		}
		var cmd tea.Cmd
		m.queue.filter, cmd = m.queue.filter.Update(msg)
		m.queue.setJobs(m.snap.Jobs)
		return m, cmd
	}

	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Help):
		m.showHelp = !m.showHelp
		return m, nil

	case key.Matches(msg, keys.Escape):
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if m.tab == tabQueue && m.queue.filter.Value() != "" {
			m.queue.stopFiltering(true)
			m.queue.setJobs(m.snap.Jobs)
		}
		return m, nil

	case key.Matches(msg, keys.Refresh):
		m.setStatus("Refrescando…", false)
		if m.refreshing {
			return m, nil
		}
		m.refreshing = true
		return m, m.refresh()

	case key.Matches(msg, keys.Tab1):
		m.tab = tabDashboard
		return m, nil
	case key.Matches(msg, keys.Tab2):
		m.tab = tabQueue
		return m, nil
	case key.Matches(msg, keys.Tab3):
		m.tab = tabPrinters
		return m, nil

	case key.Matches(msg, keys.NextTab):
		m.tab = (m.tab + 1) % tab(len(tabNames))
		return m, nil
	case key.Matches(msg, keys.PrevTab):
		m.tab = (m.tab + tab(len(tabNames)) - 1) % tab(len(tabNames))
		return m, nil
	}

	if m.showHelp {
		return m, nil
	}

	switch m.tab {
	case tabQueue:
		return m.handleQueueKey(msg)
	case tabPrinters:
		return m.handlePrintersKey(msg)
	}
	return m, nil
}

func (m Model) handleQueueKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Filter):
		return m, m.queue.startFiltering()

	case key.Matches(msg, keys.Cancel):
		job, ok := m.queue.selected()
		if !ok {
			m.setStatus("No hay ningún trabajo seleccionado.", true)
			return m, nil
		}
		c := m.client
		m.confirm = &confirmation{
			prompt: fmt.Sprintf("¿Cancelar el trabajo %d (%s)?", job.ID, job.Name),
			run: action(fmt.Sprintf("Trabajo %d cancelado.", job.ID), func() error {
				return c.CancelJob(job.ID)
			}),
		}
		return m, nil

	case key.Matches(msg, keys.CancelAll):
		if len(m.snap.Jobs) == 0 {
			m.setStatus("La cola ya está vacía.", true)
			return m, nil
		}
		c := m.client
		n := len(m.snap.Jobs)
		m.confirm = &confirmation{
			prompt: fmt.Sprintf("¿Cancelar los %d trabajos de la cola?", n),
			run: action(fmt.Sprintf("%d trabajos cancelados.", n), func() error {
				return c.CancelAllJobs("")
			}),
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.queue.table, cmd = m.queue.table.Update(msg)
	return m, cmd
}

func (m Model) handlePrintersKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.snap.Printers)
	switch {
	case key.Matches(msg, keys.Down):
		m.printers.move(1, n)
		return m, nil
	case key.Matches(msg, keys.Up):
		m.printers.move(-1, n)
		return m, nil
	}

	p, ok := m.printers.selected(m.snap.Printers)
	if !ok {
		return m, nil
	}
	c := m.client

	switch {
	case key.Matches(msg, keys.Toggle):
		if p.State == cups.StateStopped {
			return m, action(fmt.Sprintf("%s habilitada.", p.Name), func() error {
				return c.EnablePrinter(p.Name)
			})
		}
		return m, action(fmt.Sprintf("%s deshabilitada.", p.Name), func() error {
			return c.DisablePrinter(p.Name)
		})

	case key.Matches(msg, keys.Default):
		return m, action(fmt.Sprintf("%s es la impresora por omisión.", p.Name), func() error {
			return c.SetDefault(p.Name)
		})

	case key.Matches(msg, keys.Accepting):
		accept := !p.Accepting
		text := fmt.Sprintf("%s ya no acepta trabajos nuevos.", p.Name)
		if accept {
			text = fmt.Sprintf("%s acepta trabajos nuevos.", p.Name)
		}
		return m, action(text, func() error {
			return c.SetAccepting(p.Name, accept)
		})
	}
	return m, nil
}

func (m *Model) setStatus(text string, isErr bool) {
	m.status = text
	m.statusIsErr = isErr
}

// describeError arma el texto que ve el usuario: la pista accionable si el
// paquete cups supo clasificar el fallo, o el error crudo si no.
func describeError(err error) string {
	var cerr *cups.Error
	if errors.As(err, &cerr) && cerr.Hint != "" {
		return cerr.Hint
	}
	return err.Error()
}

func (m Model) bodyHeight() int {
	// alto total menos encabezado (2), cartel de error (1), pie (1) y aire.
	h := m.height - 6
	if h < 3 {
		h = 3
	}
	return h
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")

	if banner := m.bannerView(); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n")
	}

	body := m.bodyView()
	b.WriteString(styleApp.
		Height(m.bodyHeight()).
		MaxHeight(m.bodyHeight()).
		Render(body))
	b.WriteString("\n")
	b.WriteString(m.footerView())

	// Se pinta la pantalla entera para que no queden huecos por donde se vea el
	// fondo del terminal.
	return paintBackground(b.String(), m.width)
}

func (m Model) headerView() string {
	tabs := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		if tab(i) == m.tab {
			tabs = append(tabs, styleTabActive.Render(name))
		} else {
			tabs = append(tabs, styleTabInactive.Render(name))
		}
	}

	left := lipgloss.JoinHorizontal(lipgloss.Bottom,
		append([]string{styleTitle.Render("cupstui"), " "}, tabs...)...)

	right := styleDim.Render(m.syncLabel())
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return styleHeaderBar.Width(m.width).Render(line)
}

func (m Model) syncLabel() string {
	if !m.loaded {
		return "conectando…"
	}
	return fmt.Sprintf("⟳ %s ", m.lastSync.Format("15:04:05"))
}

func (m Model) bannerView() string {
	if m.err == nil {
		return ""
	}
	text := "⚠ " + describeError(m.err)
	if m.loaded {
		text += " (mostrando la última lectura buena)"
	}
	return styleBanner.Width(m.width).Render(truncate(text, m.width-2))
}

func (m Model) bodyView() string {
	if m.showHelp {
		return helpView()
	}
	if !m.loaded && m.err != nil {
		return styleDim.Render("\n  Sin datos todavía. Se reintenta cada " +
			refreshInterval.String() + ".")
	}
	if !m.loaded {
		return styleDim.Render("\n  Consultando CUPS…")
	}

	switch m.tab {
	case tabQueue:
		return m.queue.view(len(m.snap.Jobs))
	case tabPrinters:
		return m.printers.view(m.snap.Printers, m.width)
	default:
		return dashboardView(m.snap, m.width)
	}
}

func (m Model) footerView() string {
	if m.confirm != nil {
		return styleStatusErr.Width(m.width).Render(
			m.confirm.prompt + "  " + styleKey.Render("y") + " sí  " + styleKey.Render("n") + " no")
	}
	if m.status != "" {
		style := styleStatusOK
		if m.statusIsErr {
			style = styleStatusErr
		}
		return style.Width(m.width).Render(truncate(m.status, m.width-2))
	}
	return styleStatusBar.Width(m.width).Render(shortHelp(m.tab, m.queue.filtering))
}
