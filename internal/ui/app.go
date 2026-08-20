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

	"github.com/icortes/cupstui/internal/config"
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
	tabHistory
	tabLogs
)

var tabNames = []string{"Dashboard", "Queue", "Printers", "Print", "History", "Logs"}

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
	// statusMsg informa el resultado de algo que no cambia el estado de CUPS,
	// así que no vale la pena refrescar la foto por él.
	statusMsg struct {
		text string
		err  error
	}
	historyMsg struct {
		entries []cups.HistoryEntry
		err     error
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
	history  historyModel
	logs     logsModel
	add      addModel
	policy   policyModel

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
		client:  c,
		queue:   newQueue(),
		print:   newPrint(),
		history: newHistory(),
		logs:    newLogs(),
		add:     newAdd(),
		policy:  newPolicy(),
		helpVP:  viewport.New(80, 10),
		width:   80,
		height:  24,
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
		m.resize()
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tickCmd()}
		// El registro solo se lee cuando se está mirando: es E/S de disco que
		// no hace falta pagar en las otras pestañas.
		if m.tab == tabLogs {
			cmds = append(cmds, readLog(m.logs.current()))
		}
		if m.tab == tabHistory {
			cmds = append(cmds, readHistory())
		}
		if !m.refreshing {
			m.refreshing = true
			cmds = append(cmds, m.refresh())
		}
		return m, tea.Batch(cmds...)

	case snapshotMsg:
		m.refreshing = false
		hadError := m.err != nil
		m.err = msg.err
		if hadError != (m.err != nil) {
			// El cartel ocupa una línea: al aparecer o desaparecer cambia el
			// alto disponible para el contenido.
			m.resize()
		}
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

	case statusMsg:
		if msg.err != nil {
			m.setStatus(describeError(msg.err), true)
		} else {
			m.setStatus(msg.text, false)
		}
		return m, nil

	case historyMsg:
		m.history.setEntries(msg.entries, msg.err)
		return m, nil

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

	// El buscador de archivos se comunica con mensajes propios (el listado del
	// directorio, entre otros). Si no se le reenvían, se queda vacío.
	if m.print.picking {
		return m, m.print.updatePicker(msg)
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
			m.setStatus("Cancelled.", false)
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

	// The quota editor likewise: it is all text fields.
	if m.policy.active {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, m.policy.handleKey(msg)
	}

	// tab y shift+tab quedan siempre para cambiar de pestaña: en el formulario
	// las flechas ←/→ cambian el valor del campo, así que sin esto no habría
	// forma evidente de salir de la pantalla.
	isTabSwitch := msg.String() == "tab" || msg.String() == "shift+tab"

	if m.tab == tabPrint && !isTabSwitch {
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

	// While the history filter is being typed, the text field takes the keys.
	if m.tab == tabHistory && m.history.filtering {
		switch msg.String() {
		case "enter":
			m.history.stopFiltering(false)
			return m, nil
		case "esc":
			m.history.stopFiltering(true)
			m.history.apply()
			return m, nil
		}
		var cmd tea.Cmd
		m.history.filter, cmd = m.history.filter.Update(msg)
		m.history.apply()
		return m, cmd
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

	case key.Matches(msg, keys.Transparent):
		SetTransparent(!transparent)
		m.restyle()
		return m, rememberTransparency(transparent)

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
		m.setStatus("Refreshing…", false)
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
		return m.goTo(tabHistory)
	case key.Matches(msg, keys.Tab6):
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
	case tabHistory:
		return m.handleHistoryKey(msg)
	case tabLogs:
		return m.handleLogsKey(msg)
	}
	return m, nil
}

func (m Model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Filter):
		return m, m.history.startFiltering()

	case key.Matches(msg, keys.Export):
		if len(m.history.visible) == 0 {
			m.setStatus("Nothing to export.", true)
			return m, nil
		}
		return m, exportHistory(m.history.visible)
	}

	var cmd tea.Cmd
	m.history.table, cmd = m.history.table.Update(msg)
	return m, cmd
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
	}

	// Sobre un campo de texto, ←/→ mueven el cursor dentro de lo escrito; solo
	// cambian el valor en los campos de opciones.
	if !m.print.editing() {
		switch msg.String() {
		case "left", "-":
			m.print.cycle(-1, m.snap.Printers)
			return m, nil
		case "right", "+":
			m.print.cycle(1, m.snap.Printers)
			return m, nil
		}
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
		m.setStatus("Select a file first — ctrl+o to browse.", true)
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
			return actionMsg{text: fmt.Sprintf("%s sent to printer — job %d.", name, id)}
		}
		return actionMsg{text: name + " sent to printer."}
	}
}

// goTo cambia de pestaña y dispara la carga que esa pestaña necesite.
func (m Model) goTo(t tab) (tea.Model, tea.Cmd) {
	m.tab = t
	switch t {
	case tabLogs:
		return m, readLog(m.logs.current())
	case tabHistory:
		return m, readHistory()
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
			m.setStatus("No job selected.", true)
			return m, nil
		}
		c := m.client
		m.confirm = &confirmation{
			prompt: fmt.Sprintf("Cancel job %d (%s)?", job.ID, job.Name),
			run: action(fmt.Sprintf("Job %d cancelled.", job.ID), func() error {
				return c.CancelJob(job.ID)
			}),
		}
		return m, nil

	case key.Matches(msg, keys.HoldJob):
		job, ok := m.queue.selected()
		if !ok {
			m.setStatus("No job selected.", true)
			return m, nil
		}
		c := m.client
		if job.State == cups.JobHeld {
			return m, action(fmt.Sprintf("Job %d released.", job.ID), func() error {
				return c.ReleaseJob(job.ID)
			})
		}
		return m, action(fmt.Sprintf("Job %d held.", job.ID), func() error {
			return c.HoldJob(job.ID)
		})

	case key.Matches(msg, keys.CancelAll):
		if len(m.snap.Jobs) == 0 {
			m.setStatus("The queue is already empty.", true)
			return m, nil
		}
		c := m.client
		n := len(m.snap.Jobs)
		m.confirm = &confirmation{
			prompt: fmt.Sprintf("Cancel all %d queued jobs?", n),
			run: action(fmt.Sprintf("%d jobs cancelled.", n), func() error {
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
			return m, action(fmt.Sprintf("%s enabled.", p.Name), func() error {
				return c.EnablePrinter(p.Name)
			})
		}
		return m, action(fmt.Sprintf("%s disabled.", p.Name), func() error {
			return c.DisablePrinter(p.Name)
		})

	case key.Matches(msg, keys.Default):
		return m, action(fmt.Sprintf("%s set as default.", p.Name), func() error {
			return c.SetDefault(p.Name)
		})

	case key.Matches(msg, keys.AddPrinter):
		return m, m.add.start(m.client)

	case key.Matches(msg, keys.Policy):
		return m, m.policy.start(p)

	case key.Matches(msg, keys.DeletePrinter):
		c := m.client
		queued := jobsFor(m.snap.Jobs, p.Name)
		prompt := fmt.Sprintf("Remove printer %s?", p.Name)
		if queued > 0 {
			// Deleting a queue discards whatever is still on it, so the count
			// belongs in the prompt rather than in a surprise afterwards.
			prompt = fmt.Sprintf("Remove printer %s and discard %d queued job(s)?", p.Name, queued)
		}
		m.confirm = &confirmation{
			prompt: prompt,
			run: action(fmt.Sprintf("Printer %s removed.", p.Name), func() error {
				ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
				defer cancel()
				return c.DeletePrinter(ctx, p.Name)
			}),
		}
		return m, nil

	case key.Matches(msg, keys.Accepting):
		accept := !p.Accepting
		text := fmt.Sprintf("%s is rejecting new jobs.", p.Name)
		if accept {
			text = fmt.Sprintf("%s is accepting new jobs.", p.Name)
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

// resize reparte el tamaño de la ventana entre las vistas.
func (m *Model) resize() {
	body := m.bodyHeight()
	m.queue.setSize(m.width, body-2)
	m.logs.setSize(m.width, body-2)
	m.print.setSize(m.width, body-2)
	m.history.setSize(m.width, body-2)
	m.add.setSize(m.width, body)
	m.policy.setSize(m.width)
	m.helpVP.Width = m.width
	m.helpVP.Height = body
	m.helpVP.SetContent(helpView(m.width))
}

// rememberTransparency guarda la preferencia para la próxima sesión.
func rememberTransparency(on bool) tea.Cmd {
	return func() tea.Msg {
		text := "Opaque background."
		if on {
			text = "Transparent background."
		}
		if err := config.Save(config.Config{Transparent: on}); err != nil {
			return statusMsg{err: fmt.Errorf("could not save preference: %w", err)}
		}
		return statusMsg{text: text}
	}
}

// restyle vuelve a aplicar los estilos a los componentes que se los guardaron
// al construirse. Hace falta al cambiar el modo transparente en caliente.
func (m *Model) restyle() {
	m.queue.restyle()
	m.history.restyle()
	m.print.restyle()
	m.add.restyle()
	m.policy.restyle()
	m.helpVP.SetContent(helpView(m.width))
}

// bodyHeight es lo que queda para el contenido: el encabezado ocupa dos líneas
// (título y borde), el pie una, y el cartel de error otra cuando está.
func (m Model) bodyHeight() int {
	h := m.height - 3
	if m.err != nil {
		h--
	}
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
		return "connecting…"
	}
	return fmt.Sprintf("⟳ %s ", m.lastSync.Format("15:04:05"))
}

func (m Model) bannerView() string {
	if m.err == nil {
		return ""
	}
	text := "⚠ " + describeError(m.err)
	if m.loaded {
		text += " — showing last known state"
	}
	return styleBanner.Width(m.width).Render(truncate(text, m.width-2))
}

func (m Model) bodyView() string {
	if m.add.active {
		return m.add.view()
	}
	if m.policy.active {
		return m.policy.view()
	}
	if m.showHelp {
		return m.helpVP.View()
	}
	if !m.loaded && m.err != nil {
		return styleDim.Render("\n  No data yet. Retrying every " + refreshInterval.String() + ".")
	}
	if !m.loaded {
		return styleDim.Render("\n  Querying CUPS…")
	}

	switch m.tab {
	case tabQueue:
		return m.queue.view(len(m.snap.Jobs))
	case tabPrinters:
		return m.printers.view(m.snap.Printers, m.width)
	case tabPrint:
		return m.print.view(m.snap.Printers, m.snap.Default)
	case tabHistory:
		return m.history.view()
	case tabLogs:
		return m.logs.view(m.width)
	default:
		return dashboardView(m.snap, m.width)
	}
}

func (m Model) footerView() string {
	if m.confirm != nil {
		return styleStatusErr.Width(m.width).Render(
			m.confirm.prompt + "  " + styleKey.Render("y") + " yes  " + styleKey.Render("n") + " no")
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
			hint("j/k", "scroll") + " · " + hint("?", "close help"))
	}
	return styleStatusBar.Width(m.width).Render(
		shortHelp(m.tab, m.queue.filtering || m.history.filtering))
}
