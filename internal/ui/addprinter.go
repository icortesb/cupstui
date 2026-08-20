package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/icortesb/cupstui/internal/cups"
)

// discoveryTimeout bounds the scan: CUPS probes the network and can take a while.
const discoveryTimeout = 30 * time.Second

type addStep int

const (
	stepDevice addStep = iota
	stepURI
	stepDriver
	stepDetails
)

// addModel is the wizard for creating a printer.
type addModel struct {
	active  bool
	step    addStep
	loading bool
	err     error

	spin      spinner.Model
	devices   []cups.Device
	devCursor int
	uri       textinput.Model

	ppds      []cups.PPD
	matches   []cups.PPD
	ppdCursor int
	ppdFilter textinput.Model

	name        textinput.Model
	info        textinput.Model
	location    textinput.Model
	detailFocus int

	chosenURI string
	chosenPPD string

	width, height int
}

func newAdd() addModel {
	mk := func(placeholder string, limit int) textinput.Model {
		t := textinput.New()
		t.Prompt = ""
		t.Placeholder = placeholder
		t.CharLimit = limit
		return t
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	a := addModel{
		spin:      sp,
		uri:       mk("socket://192.168.0.50:9100", 200),
		ppdFilter: mk("filter drivers by model", 60),
		name:      mk("no spaces, / or #", 127),
		info:      mk("description (optional)", 100),
		location:  mk("location (optional)", 100),
	}
	a.restyle()
	return a
}

// restyle reapplies the styles to the text fields.
func (a *addModel) restyle() {
	for _, t := range []*textinput.Model{&a.uri, &a.ppdFilter, &a.name, &a.info, &a.location} {
		styleInput(t)
	}
}

func (a *addModel) setSize(width, height int) {
	a.width, a.height = width, height
	for _, t := range []*textinput.Model{&a.uri, &a.ppdFilter, &a.name, &a.info, &a.location} {
		t.Width = width - 20
	}
}

// start opens the wizard and launches the scan.
func (a *addModel) start(c *cups.Client) tea.Cmd {
	// The size arrives with the WindowSizeMsg, which is already past: keep it
	// before replacing the model with a clean one.
	width, height := a.width, a.height
	*a = newAdd()
	a.setSize(width, height)
	a.active = true
	a.loading = true
	a.spin.Style = styleAccentText
	// The scan takes seconds while CUPS probes the network; without something
	// moving it reads as a hang.
	return tea.Batch(a.spin.Tick, discoverDevices(c), fetchPPDs(c))
}

func (a *addModel) cancel() {
	a.active = false
}

// listRows is how many list rows fit on screen.
func (a addModel) listRows() int {
	rows := a.height - 6
	if rows < 3 {
		rows = 3
	}
	if rows > 12 {
		rows = 12
	}
	return rows
}

func discoverDevices(c *cups.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
		defer cancel()
		devs, err := c.Devices(ctx)
		return devicesMsg{devices: devs, err: err}
	}
}

func fetchPPDs(c *cups.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
		defer cancel()
		ppds, err := c.PPDs(ctx)
		return ppdsMsg{ppds: ppds, err: err}
	}
}

// deviceCount includes the manual URI entry, always last.
func (a addModel) deviceCount() int { return len(a.devices) + 1 }

func (a *addModel) refreshMatches() {
	query := a.ppdFilter.Value()
	a.matches = cups.MatchPPDs(a.ppds, query)
	a.ppdCursor = 0
}

// handleKey processes one key and returns the resulting command. The wizard
// takes every key while it is open.
func (a *addModel) handleKey(msg tea.KeyMsg, c *cups.Client) tea.Cmd {
	switch msg.String() {
	case "esc":
		if a.step == stepDevice {
			a.cancel()
			return nil
		}
		a.back()
		return nil
	case "enter":
		return a.advance(c)
	}

	switch a.step {
	case stepDevice:
		switch msg.String() {
		case "j", "down":
			a.devCursor = clampIndex(a.devCursor+1, a.deviceCount())
		case "k", "up":
			a.devCursor = clampIndex(a.devCursor-1, a.deviceCount())
		}
	case stepURI:
		var cmd tea.Cmd
		a.uri, cmd = a.uri.Update(msg)
		return cmd
	case stepDriver:
		switch msg.String() {
		case "down":
			a.ppdCursor = clampIndex(a.ppdCursor+1, len(a.matches)+1)
		case "up":
			a.ppdCursor = clampIndex(a.ppdCursor-1, len(a.matches)+1)
		default:
			var cmd tea.Cmd
			a.ppdFilter, cmd = a.ppdFilter.Update(msg)
			a.refreshMatches()
			return cmd
		}
	case stepDetails:
		switch msg.String() {
		case "tab", "down":
			a.detailFocus = clampIndex(a.detailFocus+1, 3)
			a.applyDetailFocus()
		case "shift+tab", "up":
			a.detailFocus = clampIndex(a.detailFocus-1, 3)
			a.applyDetailFocus()
		default:
			return a.updateDetail(msg)
		}
	}
	return nil
}

func (a *addModel) applyDetailFocus() {
	a.name.Blur()
	a.info.Blur()
	a.location.Blur()
	switch a.detailFocus {
	case 0:
		a.name.Focus()
	case 1:
		a.info.Focus()
	case 2:
		a.location.Focus()
	}
}

func (a *addModel) updateDetail(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch a.detailFocus {
	case 0:
		a.name, cmd = a.name.Update(msg)
	case 1:
		a.info, cmd = a.info.Update(msg)
	case 2:
		a.location, cmd = a.location.Update(msg)
	}
	return cmd
}

func (a *addModel) back() {
	switch a.step {
	case stepURI:
		a.step = stepDevice
	case stepDriver:
		if a.chosenURI == a.uri.Value() && a.uri.Value() != "" {
			a.step = stepURI
			a.uri.Focus()
		} else {
			a.step = stepDevice
		}
	case stepDetails:
		a.step = stepDriver
	}
}

// advance moves to the next step and, on the last one, creates the printer.
func (a *addModel) advance(c *cups.Client) tea.Cmd {
	switch a.step {
	case stepDevice:
		if a.devCursor == len(a.devices) { // the manual entry
			a.step = stepURI
			a.uri.Focus()
			return textinput.Blink
		}
		if a.devCursor >= len(a.devices) {
			return nil
		}
		d := a.devices[a.devCursor]
		a.chosenURI = d.URI
		a.ppdFilter.SetValue(d.MakeModel)
		a.refreshMatches()
		a.suggestName(d)
		a.step = stepDriver

	case stepURI:
		uri := strings.TrimSpace(a.uri.Value())
		if uri == "" {
			return nil
		}
		a.chosenURI = uri
		a.uri.Blur()
		a.refreshMatches()
		a.step = stepDriver

	case stepDriver:
		a.chosenPPD = cups.DriverlessPPD
		if a.ppdCursor > 0 && a.ppdCursor-1 < len(a.matches) {
			a.chosenPPD = a.matches[a.ppdCursor-1].Name
		}
		a.step = stepDetails
		a.detailFocus = 0
		a.applyDetailFocus()
		return textinput.Blink

	case stepDetails:
		spec := cups.NewPrinterSpec{
			Name:      strings.TrimSpace(a.name.Value()),
			DeviceURI: a.chosenURI,
			PPD:       a.chosenPPD,
			Info:      strings.TrimSpace(a.info.Value()),
			Location:  strings.TrimSpace(a.location.Value()),
		}
		if err := cups.ValidatePrinterName(spec.Name); err != nil {
			a.err = err
			return nil
		}
		a.cancel()
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
			defer cancel()
			if err := c.AddPrinter(ctx, spec); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "Impresora " + spec.Name + " agregada."}
		}
	}
	return nil
}

// suggestName proposes a valid name from what the device reported, which is
// almost always the one wanted.
func (a *addModel) suggestName(d cups.Device) {
	base := d.MakeModel
	if base == "" || strings.EqualFold(base, "unknown") {
		base = d.Info
	}
	name := strings.Map(func(r rune) rune {
		if r == ' ' || r == '/' || r == '#' {
			return '_'
		}
		return r
	}, strings.TrimSpace(base))
	a.name.SetValue(name)
}

func clampIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	return ((i % n) + n) % n
}

func (a addModel) view() string {
	var b strings.Builder
	b.WriteString(styleBold.Render("  Add printer"))
	b.WriteString(styleDim.Render("   step " + fmt.Sprint(int(a.step)+1) + " of 4 · esc back"))
	b.WriteString("\n\n")

	if a.err != nil {
		b.WriteString("  " + styleErrText.Render(a.err.Error()) + "\n\n")
	}

	switch a.step {
	case stepDevice:
		b.WriteString(a.deviceView())
	case stepURI:
		b.WriteString("  " + styleLabel.Render("Device URI") + "\n  " + a.uri.View() + "\n\n")
		b.WriteString(styleDim.Render("  For example socket://192.168.0.50:9100, ipp://host/ipp/print or usb://…"))
	case stepDriver:
		b.WriteString(a.driverView())
	case stepDetails:
		b.WriteString(a.detailsView())
	}
	return b.String()
}

func (a addModel) deviceView() string {
	if a.loading {
		return "  " + a.spin.View() + styleDim.Render(" Scanning USB and network… this can take a few seconds")
	}

	var b strings.Builder
	rows := a.listRows()
	start := scrollStart(a.devCursor, a.deviceCount(), rows)

	for i := start; i < a.deviceCount() && i < start+rows; i++ {
		label := styleDim.Render("Enter a URI manually…")
		if i < len(a.devices) {
			d := a.devices[i]
			name := d.MakeModel
			if name == "" || strings.EqualFold(name, "unknown") {
				name = d.Info
			}
			label = styleValue.Render(truncate(name, 34)) + "  " + styleDim.Render(truncate(d.URI, a.width-46))
		}
		b.WriteString(cursorLine(i == a.devCursor, label))
	}
	return b.String()
}

func (a addModel) driverView() string {
	var b strings.Builder
	b.WriteString("  " + styleLabel.Render("Search drivers: ") + a.ppdFilter.View() + "\n\n")

	rows := a.listRows() - 2
	total := len(a.matches) + 1
	start := scrollStart(a.ppdCursor, total, rows)

	for i := start; i < total && i < start+rows; i++ {
		label := styleValue.Render("No driver — IPP Everywhere") +
			styleDim.Render("  (for printers that describe themselves)")
		if i > 0 {
			label = styleValue.Render(truncate(a.matches[i-1].MakeModel, a.width-8))
		}
		b.WriteString(cursorLine(i == a.ppdCursor, label))
	}

	if len(a.matches) == 0 {
		b.WriteString("\n  " + styleWarnText.Render(wrapText(cups.DriverHint(a.ppdFilter.Value()), a.width-6)))
	}
	return b.String()
}

func (a addModel) detailsView() string {
	fields := []struct {
		label string
		input textinput.Model
	}{
		{"Name", a.name},
		{"Description", a.info},
		{"Location", a.location},
	}

	var b strings.Builder
	for i, f := range fields {
		marker := "  "
		label := styleLabel.Render(pad(f.label, 13))
		if i == a.detailFocus {
			marker = styleKey.Render("▸ ")
			label = styleAccentText.Bold(true).Render(pad(f.label, 13))
		}
		fmt.Fprintf(&b, "%s%s%s\n", marker, label, f.input.View())
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Device: ") + styleValue.Render(truncate(a.chosenURI, a.width-18)) + "\n")
	b.WriteString(styleDim.Render("  Driver: ") + styleValue.Render(a.chosenPPD) + "\n\n")
	b.WriteString(styleDim.Render("  enter to create"))
	return b.String()
}

func cursorLine(selected bool, label string) string {
	if selected {
		return styleKey.Render("  ▸ ") + label + "\n"
	}
	return "    " + label + "\n"
}

// scrollStart mantiene el cursor visible dentro de una ventana de rows filas.
func scrollStart(cursor, total, rows int) int {
	if total <= rows || cursor < rows/2 {
		return 0
	}
	start := cursor - rows/2
	if start+rows > total {
		start = total - rows
	}
	return start
}
