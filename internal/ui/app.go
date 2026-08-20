// Package ui es la interfaz de terminal, construida sobre bubbletea.
package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
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
	tabPrint
	tabLogs
)

var tabNames = []string{"Dashboard", "Cola", "Impresoras", "Imprimir", "Logs"}

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
	logsMsg struct {
		lines []string
		err   error
	}
	devicesMsg struct {
		devices []cups.Device
		err     error
	}
	ppdsMsg struct {
		ppds []cups.PPD
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
	print    printModel
	logs     logsModel
	add      addModel

	confirm     *confirmation
	helpVP      viewport.Model
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
		print:  newPrint(),
		logs:   newLogs(),
		add:    newAdd(),
		helpVP: viewport.New(80, 10),
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
		m.logs.setSize(m.width, m.bodyHeight()-2)
		m.print.setSize(m.width, m.bodyHeight()-2)
		m.add.setSize(m.width, m.bodyHeight())
		m.helpVP.Width = m.width
		m.helpVP.Height = m.bodyHeight()
		m.helpVP.SetContent(helpView(m.width))
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tickCmd()}
		// El registro solo se lee cuando se está mirando: es E/S de disco que
		// no hace falta pagar en las otras pestañas.
		if m.tab == tabLogs {
			cmds = append(cmds, readLog(m.logs.current()))
		}
		if !m.refreshing {
			m.refreshing = true
			cmds = append(cmds, m.refresh())
		}
		return m, tea.Batch(cmds...)

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

	case logsMsg:
		m.logs.setLines(msg.lines, msg.err)
		return m, nil

	case devicesMsg:
		m.add.loading = false
		m.add.devices, m.add.err = msg.devices, msg.err
		return m, nil

	case ppdsMsg:
		// Los drivers se cargan de fondo mientras se elige el dispositivo; si
		// fallan, queda la opción sin driver.
		m.add.ppds = msg.ppds
		m.add.refreshMatches()
		return m, nil

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

	// Con un campo de texto enfocado, o el buscador de archivos abierto, las
	// teclas son del formulario: si no, los atajos globales (1..5, r, q, /)
	// se comerían las letras de lo que se está escribiendo.
	// El asistente de alta se queda con todas las teclas: tiene campos de texto
	// y una lista propia.
	if m.add.active {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, m.add.handleKey(msg, m.client)
	}

	if m.tab == tabPrint {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.print.picking || m.print.editing() {
			if msg.String() == "esc" {
				if m.print.picking {
					m.print.picking = false
					return m, nil
				}
				// Sacar el foco del texto devuelve los atajos globales.
				m.print.focus = fieldPrinter
				m.print.applyFocus()
				return m, nil
			}
			return m.handlePrintKey(msg)
		}
		// Sin un campo de texto activo, el formulario igual se queda con las
		// teclas de navegación y de valor: h/l y ←/→ cambian la opción, no de
		// pestaña. El resto (1..5, tab, q, r, ?) sigue siendo global.
		if formKeys[msg.String()] {
			return m.handlePrintKey(msg)
		}
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
		if m.showHelp {
			m.helpVP.SetContent(helpView(m.width))
			m.helpVP.GotoTop()
		}
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
	case key.Matches(msg, keys.Tab4):
		m.tab = tabPrint
		return m, nil
	case key.Matches(msg, keys.Tab5):
		return m.goTo(tabLogs)

	case key.Matches(msg, keys.NextTab):
		return m.goTo((m.tab + 1) % tab(len(tabNames)))
	case key.Matches(msg, keys.PrevTab):
		return m.goTo((m.tab + tab(len(tabNames)) - 1) % tab(len(tabNames)))
	}

	// Con la ayuda abierta, las teclas de desplazamiento son suyas: no entra
	// entera en una terminal corta.
	if m.showHelp {
		var cmd tea.Cmd
		m.helpVP, cmd = m.helpVP.Update(msg)
		return m, cmd
	}

	switch m.tab {
	case tabQueue:
		return m.handleQueueKey(msg)
	case tabPrinters:
		return m.handlePrintersKey(msg)
	case tabPrint:
		return m.handlePrintKey(msg)
	case tabLogs:
		return m.handleLogsKey(msg)
	}
	return m, nil
}

func (m Model) handlePrintKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.print.picking {
		if msg.String() == "esc" {
			m.print.picking = false
			return m, nil
		}
		return m, m.print.updatePicker(msg)
	}

	switch msg.String() {
	case "ctrl+o":
		return m, m.print.openPicker()

	case "enter":
		return m.submitPrint()

	case "up":
		m.print.move(-1)
		return m, nil
	case "down":
		m.print.move(1)
		return m, nil
	case "left", "-":
		m.print.cycle(-1, m.snap.Printers)
		return m, nil
	case "right", "+":
		m.print.cycle(1, m.snap.Printers)
		return m, nil
	}

	// Con un campo de texto enfocado, las letras escriben; si no, j/k/h/l
	// navegan como en el resto de la aplicación.
	if !m.print.editing() {
		switch msg.String() {
		case "j":
			m.print.move(1)
			return m, nil
		case "k":
			m.print.move(-1)
			return m, nil
		case "h":
			m.print.cycle(-1, m.snap.Printers)
			return m, nil
		case "l":
			m.print.cycle(1, m.snap.Printers)
			return m, nil
		}
	}

	return m, m.print.update(msg)
}

// formKeys son las teclas que el formulario de impresión atiende aunque no
// haya un campo de texto enfocado.
var formKeys = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"j": true, "k": true, "h": true, "l": true,
	"enter": true, "ctrl+o": true, "+": true, "-": true,
}

func (m Model) submitPrint() (tea.Model, tea.Cmd) {
	path := m.print.file()
	if path == "" {
		m.setStatus("Elegí un archivo primero (ctrl+o para buscar).", true)
		return m, nil
	}

	opts := m.print.options(m.snap.Printers)
	name := filepath.Base(path)
	return m, func() tea.Msg {
		id, err := cups.Submit(path, opts)
		if err != nil {
			return actionMsg{err: err}
		}
		if id > 0 {
			return actionMsg{text: fmt.Sprintf("%s enviado a imprimir (trabajo %d).", name, id)}
		}
		return actionMsg{text: name + " enviado a imprimir."}
	}
}

// goTo cambia de pestaña y dispara la carga que esa pestaña necesite.
func (m Model) goTo(t tab) (tea.Model, tea.Cmd) {
	m.tab = t
	if t == tabLogs {
		return m, readLog(m.logs.current())
	}
	return m, nil
}

func (m Model) handleLogsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.NextLog) {
		m.logs.nextFile()
		return m, readLog(m.logs.current())
	}
	return m, m.logs.update(msg)
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

	case key.Matches(msg, keys.HoldJob):
		job, ok := m.queue.selected()
		if !ok {
			m.setStatus("No hay ningún trabajo seleccionado.", true)
			return m, nil
		}
		c := m.client
		if job.State == cups.JobHeld {
			return m, action(fmt.Sprintf("Trabajo %d reanudado.", job.ID), func() error {
				return c.ReleaseJob(job.ID)
			})
		}
		return m, action(fmt.Sprintf("Trabajo %d pausado.", job.ID), func() error {
			return c.HoldJob(job.ID)
		})

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

	case key.Matches(msg, keys.AddPrinter):
		return m, m.add.start(m.client)

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
	if m.add.active {
		return m.add.view()
	}
	if m.showHelp {
		return m.helpVP.View()
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
	case tabPrint:
		return m.print.view(m.snap.Printers, m.snap.Default)
	case tabLogs:
		return m.logs.view(m.width)
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
	if m.showHelp {
		return styleStatusBar.Width(m.width).Render(
			hint("j/k", "desplazar") + " · " + hint("?", "cerrar la ayuda"))
	}
	return styleStatusBar.Width(m.width).Render(shortHelp(m.tab, m.queue.filtering))
}
