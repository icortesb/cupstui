package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/icortesb/cupstui/internal/cups"
)

// Fields of the form, in the order they are visited.
const (
	fieldFile = iota
	fieldPrinter
	fieldCopies
	fieldRanges
	fieldDuplex
	fieldColor
	fieldOrientation
	fieldCount
)

var fieldLabels = [fieldCount]string{
	"File", "Printer", "Copies", "Pages", "Duplex", "Color", "Orientation",
}

// printModel is the form for sending a file to print.
type printModel struct {
	focus  int
	path   textinput.Model
	ranges textinput.Model

	printer     int // index into the printers; -1 uses the default one
	copies      int
	duplex      cups.Duplex
	color       cups.ColorMode
	orientation cups.Orientation

	picker  filepicker.Model
	picking bool

	width int
}

func newPrint() printModel {
	path := textinput.New()
	path.Prompt = ""
	path.Placeholder = "file path — ctrl+o to browse"
	path.CharLimit = 300

	ranges := textinput.New()
	ranges.Prompt = ""
	ranges.Placeholder = "all (e.g. 1-5,8)"
	ranges.CharLimit = 40

	fp := filepicker.New()
	// Starts in the home directory, but h or ← walks up as far as the root, so
	// anywhere on the machine is reachable.
	fp.CurrentDirectory, _ = os.UserHomeDir()
	fp.ShowHidden = false
	fp.AutoHeight = false

	m := printModel{
		path:    path,
		ranges:  ranges,
		printer: -1,
		copies:  1,
		picker:  fp,
	}
	m.applyFocus()
	m.restyle()
	return m
}

// restyle reapplies the styles to the text fields.
func (p *printModel) restyle() {
	styleInput(&p.path)
	styleInput(&p.ranges)
}

// styleInput puts the current styles on a text field.
func styleInput(t *textinput.Model) {
	t.PromptStyle = styleKey
	t.TextStyle = styleValue
	t.PlaceholderStyle = styleDim
	t.Cursor.Style = styleAccentText
}

func (p *printModel) setSize(width, height int) {
	p.width = width
	p.path.Width = width - 20
	p.ranges.Width = 24
	if height > 4 {
		p.picker.SetHeight(height - 3)
	}
}

// applyFocus activates the matching text field and blurs the rest, so the
// cursor shows in one place only.
func (p *printModel) applyFocus() {
	p.path.Blur()
	p.ranges.Blur()
	switch p.focus {
	case fieldFile:
		p.path.Focus()
	case fieldRanges:
		p.ranges.Focus()
	}
}

func (p *printModel) move(delta int) {
	p.focus = (p.focus + delta + fieldCount) % fieldCount
	p.applyFocus()
}

// editing reports whether the active field consumes typing; with one of these
// focused, j/k write rather than navigate.
func (p printModel) editing() bool {
	return p.focus == fieldFile || p.focus == fieldRanges
}

// cycle changes the value of the active field. delta is +1 or -1.
func (p *printModel) cycle(delta int, printers []cups.Printer) {
	switch p.focus {
	case fieldPrinter:
		n := len(printers) + 1 // +1 for the "printer default" entry
		p.printer = ((p.printer+1+delta)+n)%n - 1
	case fieldCopies:
		p.copies += delta
		if p.copies < 1 {
			p.copies = 1
		}
		if p.copies > 999 {
			p.copies = 999
		}
	case fieldDuplex:
		p.duplex = cups.Duplex(wrap(int(p.duplex)+delta, 4))
	case fieldColor:
		p.color = cups.ColorMode(wrap(int(p.color)+delta, 3))
	case fieldOrientation:
		p.orientation = cups.Orientation(wrap(int(p.orientation)+delta, 3))
	}
}

func wrap(v, n int) int {
	return ((v % n) + n) % n
}

// options builds the print options from what the form holds.
func (p printModel) options(printers []cups.Printer) cups.PrintOptions {
	o := cups.PrintOptions{
		Copies:      p.copies,
		PageRanges:  strings.TrimSpace(p.ranges.Value()),
		Duplex:      p.duplex,
		Color:       p.color,
		Orientation: p.orientation,
	}
	if p.printer >= 0 && p.printer < len(printers) {
		o.Printer = printers[p.printer].Name
	}
	return o
}

// file returns the path with ~ expanded.
func (p printModel) file() string {
	path := strings.TrimSpace(p.path.Value())
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func (p *printModel) openPicker() tea.Cmd {
	p.picking = true
	return p.picker.Init()
}

// update hands the message to the active component.
func (p *printModel) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch p.focus {
	case fieldFile:
		p.path, cmd = p.path.Update(msg)
	case fieldRanges:
		p.ranges, cmd = p.ranges.Update(msg)
	}
	return cmd
}

func (p *printModel) updatePicker(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.picker, cmd = p.picker.Update(msg)
	if ok, path := p.picker.DidSelectFile(msg); ok {
		p.path.SetValue(path)
		p.picking = false
		p.focus = fieldPrinter
		p.applyFocus()
	}
	return cmd
}

func (p printModel) view(printers []cups.Printer, defaultPrinter string) string {
	if p.picking {
		dir := p.picker.CurrentDirectory
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, home) {
			dir = "~" + strings.TrimPrefix(dir, home)
		}
		header := "  " + styleLabel.Render("In ") + styleValue.Render(truncate(dir, p.width-8))
		hints := "  " + styleDim.Render("j/k move · l or → open · h or ← up · enter select · esc cancel")
		return strings.Join([]string{header, hints, "", p.picker.View()}, "\n")
	}

	var b strings.Builder
	for i := 0; i < fieldCount; i++ {
		marker := "  "
		label := styleLabel.Render(pad(fieldLabels[i], 12))
		if i == p.focus {
			marker = styleKey.Render("▸ ")
			label = styleAccentText.Bold(true).Render(pad(fieldLabels[i], 12))
		}
		fmt.Fprintf(&b, "%s%s%s\n", marker, label, p.valueOf(i, printers, defaultPrinter))
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  ←/→ change value · ctrl+o browse · enter print"))
	return b.String()
}

func (p printModel) valueOf(field int, printers []cups.Printer, defaultPrinter string) string {
	switch field {
	case fieldFile:
		return p.path.View()
	case fieldPrinter:
		if p.printer < 0 || p.printer >= len(printers) {
			name := defaultPrinter
			if name == "" {
				name = "none configured"
			}
			return styleValue.Render(name) + styleDim.Render("  (default)")
		}
		return styleValue.Render(printers[p.printer].Name)
	case fieldCopies:
		return styleValue.Render(strconv.Itoa(p.copies))
	case fieldRanges:
		return p.ranges.View()
	case fieldDuplex:
		return styleValue.Render(p.duplex.String())
	case fieldColor:
		return styleValue.Render(p.color.String())
	case fieldOrientation:
		return styleValue.Render(p.orientation.String())
	}
	return ""
}
