// Package ui is the terminal interface, built on bubbletea.
package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/icortesb/cupstui/internal/config"
	"github.com/icortesb/cupstui/internal/cups"
)

// refreshInterval is how often CUPS is queried again. The query runs in a
// tea.Cmd, so it never blocks drawing.
const refreshInterval = 3 * time.Second

// requestTimeout keeps a stalled CUPS from freezing the refresh forever.
const requestTimeout = 5 * time.Second

// The footer message shares its line with the key hints, so it has to give the
// line back. Errors linger longer because they are worth reading twice.
const (
	statusTTL    = 4 * time.Second
	statusErrTTL = 10 * time.Second
)

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
	// statusMsg reports something that does not change the state of CUPS, so
	// it is not worth refreshing the snapshot for.
	statusMsg struct {
		text string
		err  error
	}
	// expireStatusMsg retires the footer message it was scheduled for. The
	// sequence number keeps a late expiry from wiping a newer message.
	expireStatusMsg struct {
		seq int
	}
	checkMsg struct {
		result cups.CheckResult
		source chan cups.CheckResult
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
	diagnoseMsg struct {
		result cups.PrinterDiagnosis
		err    error
	}
)

// confirmation is a destructive action waiting for the user to say yes.
type confirmation struct {
	prompt string
	run    func() tea.Msg
}

// Model is the root model: it holds the CUPS snapshot and hands keys and
// drawing to the views.
type Model struct {
	client *cups.Client

	tab      tab
	snap     cups.Snapshot
	err      error // error from the last refresh, shown as a banner
	loaded   bool
	lastSync time.Time
	// refreshing keeps ticks from stacking up when CUPS turns slow: without it
	// a query taking longer than the interval starts another on top.
	refreshing bool

	queue     queueModel
	printers  printersModel
	print     printModel
	history   historyModel
	logs      logsModel
	add       addModel
	policy    policyModel
	diagnose  diagnoseModel
	preflight preflightModel
	password  passwordModel

	confirm     *confirmation
	helpVP      viewport.Model
	status      string
	statusIsErr bool
	statusSeq   int
	showHelp    bool

	width, height int
}

// New builds the root model around a connected client.
func New(c *cups.Client) Model {
	return Model{
		client:    c,
		queue:     newQueue(),
		print:     newPrint(),
		history:   newHistory(),
		logs:      newLogs(),
		add:       newAdd(),
		policy:    newPolicy(),
		diagnose:  newDiagnose(),
		preflight: newPreflight(),
		password:  newPassword(),
		helpVP:    viewport.New(80, 10),
		width:     80,
		height:    24,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.refresh(), tickCmd()}
	if m.preflight.active {
		cmds = append(cmds, m.preflight.start(m.client))
	}
	return tea.Batch(cmds...)
}

// ShowPreflight makes the startup checks run before the interface is entered.
func (m Model) ShowPreflight() Model {
	m.preflight.active = true
	return m
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// refresh queries CUPS off the UI goroutine.
func (m Model) refresh() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		snap, err := c.Snapshot(ctx)
		return snapshotMsg{snap: snap, err: err}
	}
}

// action wraps a write: it reports the outcome and triggers an immediate
// refresh so the view does not sit stale.
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
		// The log is read only while it is on screen: disk I/O the other tabs
		// need not pay for.
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
			// The banner takes a line: its appearing or leaving changes the
			// height available to the content.
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
			cmd := m.setStatus(describeError(msg.err), true)
			// A remote server refusing means credentials, not group
			// membership: offer the prompt instead of a dead end.
			if signIn := m.offerSignIn(msg.err); signIn != nil {
				return m, tea.Batch(cmd, signIn)
			}
			return m, tea.Batch(cmd, m.refresh())
		}
		return m, tea.Batch(m.setStatus(msg.text, false), m.refresh())

	case statusMsg:
		if msg.err != nil {
			return m, m.setStatus(describeError(msg.err), true)
		}
		return m, m.setStatus(msg.text, false)

	case expireStatusMsg:
		if msg.seq == m.statusSeq {
			m.clearStatus()
		}
		return m, nil

	case historyMsg:
		m.history.setEntries(msg.entries, msg.err)
		return m, nil

	case logsMsg:
		m.logs.setLines(msg.lines, msg.err)
		return m, nil

	case checkMsg:
		return m, m.preflight.record(msg)

	case spinner.TickMsg:
		if m.preflight.active && !m.preflight.done() {
			var cmd tea.Cmd
			m.preflight.spin, cmd = m.preflight.spin.Update(msg)
			return m, cmd
		}
		if m.add.active && m.add.loading {
			var cmd tea.Cmd
			m.add.spin, cmd = m.add.spin.Update(msg)
			return m, cmd
		}
		if m.diagnose.active && m.diagnose.loading {
			var cmd tea.Cmd
			m.diagnose.spin, cmd = m.diagnose.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case devicesMsg:
		m.add.loading = false
		m.add.setDevices(msg.devices)
		m.add.err = msg.err
		return m, nil

	case ppdsMsg:
		// Drivers load in the background while a device is chosen; if that
		// fails, the driverless option remains.
		m.add.ppds = msg.ppds
		m.add.refreshMatches()
		return m, nil

	case diagnoseMsg:
		m.diagnose.loading = false
		m.diagnose.result, m.diagnose.err = msg.result, msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// The file browser talks through messages of its own, the directory
	// listing among them. Without forwarding, it stays empty.
	if m.print.picking {
		return m, m.print.updatePicker(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// A pending confirmation swallows every key.
	if m.confirm != nil {
		switch strings.ToLower(msg.String()) {
		case "y", "s":
			run := m.confirm.run
			m.confirm = nil
			m.clearStatus()
			return m, func() tea.Msg { return run() }
		case "n", "esc", "q", "ctrl+c":
			m.confirm = nil
			return m, m.setStatus("Cancelled.", false)
		}
		return m, nil
	}

	// With a text field focused, or the file browser open, the keys belong to
	// the form: otherwise the global shortcuts (1..6, r, q, /) would eat the
	// letters being typed.
	// The password prompt takes every key: it is a text field.
	if m.password.active {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, m.password.handleKey(msg, m.client)
	}

	// The startup screen takes every key until it is dismissed.
	if m.preflight.active {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter", "esc", " ":
			m.preflight.dismiss()
			return m, rememberSeen()
		}
		return m, nil
	}

	// The add wizard takes every key: it has text fields and a list of its
	// own.
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

	// The diagnosis screen has no text field, but it still takes every key
	// while open: r and esc must not fall through to the global shortcuts.
	if m.diagnose.active {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, m.diagnose.handleKey(msg, m.client)
	}

	// tab and shift+tab always switch tabs: in the form the ←/→ arrows change
	// the field value, so without this there would be no obvious way off the
	// screen.
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
				// Taking focus off the text brings the global shortcuts back.
				m.print.focus = fieldPrinter
				m.print.applyFocus()
				return m, nil
			}
			return m.handlePrintKey(msg)
		}
		// With no text field active the form still takes the navigation and
		// value keys: h/l and ←/→ change the option, not the tab. The rest
		// (1..6, tab, q, r, ?) stays global.
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

	// While the filter is being typed, the text field takes priority.
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

	case key.Matches(msg, keys.SignIn):
		if m.client == nil || m.client.Server().Local {
			return m, m.setStatus("The local CUPS authenticates by connection, no password needed.", true)
		}
		return m, m.password.start(m.client.Server().String(), m.client.Server().Encrypted())

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
		cmd := m.setStatus("Refreshing…", false)
		if m.refreshing {
			return m, cmd
		}
		m.refreshing = true
		return m, tea.Batch(cmd, m.refresh())

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

	// With help open the scrolling keys are its own: it does not fit whole on
	// a short terminal.
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

	case key.Matches(msg, keys.Summary):
		m.history.summary = !m.history.summary
		return m, nil

	case key.Matches(msg, keys.Export):
		if len(m.history.visible) == 0 {
			return m, m.setStatus("Nothing to export.", true)
		}
		return m, exportHistory(m.history.visible)
	}

	var cmd tea.Cmd
	m.history.table, cmd = m.history.table.Update(msg)
	m.history.markCursor()
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

	// On a text field, ←/→ move the cursor within what is typed; they change
	// the value on option fields alone.
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

	// With a text field focused the letters type; otherwise j/k/h/l navigate as
	// they do everywhere else in the application.
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

// formKeys are the keys the print form handles even with no text field
// focused.
var formKeys = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"j": true, "k": true, "h": true, "l": true,
	"enter": true, "ctrl+o": true, "+": true, "-": true,
}

func (m Model) submitPrint() (tea.Model, tea.Cmd) {
	path := m.print.file()
	if path == "" {
		return m, m.setStatus("Select a file first — ctrl+o to browse.", true)
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

// goTo switches tab and starts whatever loading that tab needs.
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
	if key.Matches(msg, keys.LogLevel) {
		m.logs.cycleLevel()
		return m, nil
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
			return m, m.setStatus("No job selected.", true)
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
			return m, m.setStatus("No job selected.", true)
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
			return m, m.setStatus("The queue is already empty.", true)
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
	m.queue.markCursor()
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

	case key.Matches(msg, keys.Diagnose):
		return m, m.diagnose.start(c, p)

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

// setStatus puts a message in the footer and hands back the command that takes
// it away again; without running that command the key hints never return.
func (m *Model) setStatus(text string, isErr bool) tea.Cmd {
	m.status = text
	m.statusIsErr = isErr
	m.statusSeq++
	if text == "" {
		return nil
	}
	ttl, seq := statusTTL, m.statusSeq
	if isErr {
		ttl = statusErrTTL
	}
	return tea.Tick(ttl, func(time.Time) tea.Msg { return expireStatusMsg{seq: seq} })
}

func (m *Model) clearStatus() {
	m.status = ""
	m.statusIsErr = false
	m.statusSeq++
}

// describeError builds what the user sees: the actionable hint when the cups
// package could classify the failure, and the raw error when it could not.
func describeError(err error) string {
	var cerr *cups.Error
	if errors.As(err, &cerr) && cerr.Hint != "" {
		return cerr.Hint
	}
	return err.Error()
}

// resize shares the window size out among the views.
func (m *Model) resize() {
	body := m.contentHeight()
	width := m.contentWidth()
	m.queue.setSize(width, body-2)
	m.logs.setSize(width, body-2)
	m.print.setSize(width, body-2)
	m.history.setSize(width, body-2)
	m.add.setSize(width, body)
	m.diagnose.setSize(width, body)
	m.policy.setSize(width)
	m.preflight.setSize(m.width)
	m.password.setSize(width)
	m.helpVP.Width = width
	m.helpVP.Height = body
	m.helpVP.SetContent(helpView(width))
}

// offerSignIn asks for a password when a remote server refuses and none has
// been given yet. A server that asks for credentials and one that refuses them
// both land here: only the first says a password is what is missing, but a
// server configured to refuse outright would otherwise leave no way to sign in.
func (m *Model) offerSignIn(err error) tea.Cmd {
	var cerr *cups.Error
	if !errors.As(err, &cerr) {
		return nil
	}
	if cerr.Kind != cups.KindUnauthorized && cerr.Kind != cups.KindForbidden {
		return nil
	}
	if m.client == nil || !m.client.NeedsPassword() {
		return nil
	}
	return m.password.start(m.client.Server().String(), m.client.Server().Encrypted())
}

// rememberSeen records that the startup checks have been shown, so they do not
// greet a returning user on every start.
func rememberSeen() tea.Cmd {
	return func() tea.Msg {
		c := config.Load()
		if c.Seen {
			return nil
		}
		c.Seen = true
		if err := config.Save(c); err != nil {
			return statusMsg{err: err}
		}
		return nil
	}
}

// rememberTransparency saves the preference for the next session.
func rememberTransparency(on bool) tea.Cmd {
	return func() tea.Msg {
		text := "Opaque background."
		if on {
			text = "Transparent background."
		}
		c := config.Load()
		c.Transparent = on
		if err := config.Save(c); err != nil {
			return statusMsg{err: fmt.Errorf("could not save preference: %w", err)}
		}
		return statusMsg{text: text}
	}
}

// restyle reapplies the styles to the components that copied them when they
// were built. Needed when transparent mode changes at runtime.
func (m *Model) restyle() {
	m.queue.restyle()
	m.history.restyle()
	m.print.restyle()
	m.add.restyle()
	m.policy.restyle()
	m.password.restyle()
	m.helpVP.SetContent(helpView(m.contentWidth()))
}

// bodyHeight is the height of the body panel, borders included: the header
// takes one line, the footer one, and the banner another when present.
func (m Model) bodyHeight() int {
	h := m.height - 2
	if m.err != nil {
		h--
	}
	if h < 3 {
		h = 3
	}
	return h
}

// contentHeight is what is left inside the panel once its top and bottom
// borders are taken off.
func (m Model) contentHeight() int {
	h := m.bodyHeight() - 2
	if h < 1 {
		h = 1
	}
	return h
}

// contentWidth is the room inside the panel: two columns for the borders and
// two more for the padding that keeps the text off them.
func (m Model) contentWidth() int {
	w := m.width - 4
	if w < 1 {
		// A width not known yet is not a narrow screen; the views are told the
		// screen width and resized as soon as a real one arrives.
		w = m.width
	}
	return w
}

// panelWidth is what lipgloss has to be given for the panel: it counts the
// padding inside the width but adds the border on top of it.
func (m Model) panelWidth() int {
	w := m.width - 2
	if w < 1 {
		w = m.width
	}
	return w
}

// bodyStyle is the panel around the body. Its border carries the focus: while a
// filter is being typed the keys go to the field rather than the list, and the
// border says so.
func (m Model) bodyStyle() lipgloss.Style {
	s := styleBody
	if m.queue.filtering || m.history.filtering {
		s = s.BorderForeground(colorAccent)
	}
	return s
}

// fit keeps the screen within the terminal. A line wider than the screen is
// not cut by the terminal but wrapped onto the next row, which pushes
// everything down until the top scrolls away — and the top is the navigation,
// the one thing that has to stay. Cutting the line instead costs the end of a
// row of key hints and keeps the shape of the screen.
func fit(view string, width, height int) string {
	lines := strings.Split(view, "\n")
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	// A width of zero is a size not known yet, not a screen with no room:
	// cutting to it would blank the interface until the first resize arrives.
	if width > 0 {
		for i, line := range lines {
			if lipgloss.Width(line) > width {
				lines[i] = ansi.Truncate(line, width, "…")
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) View() string {
	// The startup screen replaces the interface rather than overlaying it:
	// until the checks answer, there is nothing trustworthy to show behind.
	if m.preflight.active {
		return paintBackground(m.preflight.view(), m.width)
	}

	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")

	if banner := m.bannerView(); banner != "" {
		b.WriteString(banner)
		b.WriteString("\n")
	}

	body := m.bodyView()
	b.WriteString(m.bodyStyle().
		Width(m.panelWidth()).
		Height(m.contentHeight()).
		MaxHeight(m.bodyHeight()).
		Render(body))
	b.WriteString("\n")
	b.WriteString(m.footerView())

	// The whole screen is painted so no gaps are left for the terminal
	// background to show through.
	return paintBackground(fit(b.String(), m.width, m.height), m.width)
}

func (m Model) headerView() string {
	tabs := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		label := name
		// The queue is the one tab whose contents change on their own, so it
		// carries its count and can be watched from any other screen.
		if tab(i) == tabQueue && len(m.snap.Jobs) > 0 {
			label = fmt.Sprintf("%s %d", name, len(m.snap.Jobs))
		}

		if tab(i) == m.tab {
			tabs = append(tabs, styleTabActive.Render(label))
			continue
		}
		if tab(i) == tabQueue && len(m.snap.Jobs) > 0 {
			tabs = append(tabs, styleTabInactive.Render(name)+styleBadge.Render(strconv.Itoa(len(m.snap.Jobs))))
			continue
		}
		tabs = append(tabs, styleTabInactive.Render(label))
	}

	left := lipgloss.JoinHorizontal(lipgloss.Bottom,
		append([]string{titleView("cupstui"), " "}, tabs...)...)

	right := styleDim.Render(m.syncLabel())
	if server := m.serverLabel(); server != "" {
		right = m.serverStyle().Render(server) + styleDim.Render("  ") + right
	}

	// What goes when the screen is too narrow, in order: the clock and the
	// server name, then the application's own name, then every tab but the one
	// being looked at. Which tab you are on is worth more than all of it.
	fits := func() bool {
		// A width not known yet is not a narrow screen: nothing is dropped
		// until there is a size to judge against.
		return m.width <= 0 || lipgloss.Width(left)+lipgloss.Width(right)+1 <= m.width
	}
	if !fits() {
		right = ""
	}
	if !fits() {
		left = lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	}
	if !fits() {
		left = styleTabActive.Render(tabNames[m.tab])
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return styleHeaderBar.Width(m.width).Render(line)
}

// serverLabel names the server when it is a remote one, so it is never a
// surprise which machine is being administered.
func (m Model) serverLabel() string {
	if m.client == nil {
		return ""
	}
	if server := m.client.Server(); !server.Local {
		return server.String()
	}
	return ""
}

// serverStyle warns when a remote connection is unprotected. Reading a queue
// in the clear is one thing; the password for an administrative operation
// crosses the same wire.
func (m Model) serverStyle() lipgloss.Style {
	server := m.client.Server()
	switch {
	case !server.Encrypted():
		return styleWarnText
	case server.AllowAnyRoot:
		return styleWarnText
	default:
		return styleOKText
	}
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
	if m.diagnose.active {
		return m.diagnose.view()
	}
	if m.password.active {
		return m.password.view()
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
		return m.printers.view(m.snap.Printers, m.contentWidth())
	case tabPrint:
		return m.print.view(m.snap.Printers, m.snap.Default)
	case tabHistory:
		return m.history.view()
	case tabLogs:
		return m.logs.view(m.contentWidth())
	default:
		return dashboardView(m.snap, m.contentWidth())
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
