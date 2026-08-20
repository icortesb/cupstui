package ui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/icortesb/cupstui/internal/cups"
	"github.com/icortesb/cupstui/internal/drivers"
)

// diagnoseModel is the per-printer diagnosis screen. It never installs
// anything: the install command it shows is for the user to run.
type diagnoseModel struct {
	active  bool
	loading bool
	err     error

	spin    spinner.Model
	printer cups.Printer
	result  cups.PrinterDiagnosis

	advice    drivers.Info
	hasAdvice bool

	width, height int
}

func newDiagnose() diagnoseModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return diagnoseModel{spin: sp}
}

func (d *diagnoseModel) setSize(width, height int) {
	d.width, d.height = width, height
}

// start opens the screen for one printer and launches the checks. The
// driver advice is resolved immediately — it is a local lookup, no CUPS call
// — so it is already on screen while the checks are still loading.
func (d *diagnoseModel) start(c *cups.Client, p cups.Printer) tea.Cmd {
	width, height := d.width, d.height
	*d = newDiagnose()
	d.setSize(width, height)

	d.active = true
	d.loading = true
	d.printer = p
	d.advice, d.hasAdvice = drivers.Resolve(p.MakeModel)
	d.spin.Style = styleAccentText

	return tea.Batch(d.spin.Tick, runDiagnosis(c, p.Name))
}

func runDiagnosis(c *cups.Client, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		result, err := cups.DiagnosePrinter(ctx, c, name)
		return diagnoseMsg{result: result, err: err}
	}
}

func (d *diagnoseModel) cancel() { d.active = false }

// handleKey takes every key while the screen is open: it has no text field
// of its own to lose keys to.
func (d *diagnoseModel) handleKey(msg tea.KeyMsg, c *cups.Client) tea.Cmd {
	switch msg.String() {
	case "esc":
		d.cancel()
		return nil
	case "r":
		if d.loading {
			return nil
		}
		d.loading = true
		d.spin.Style = styleAccentText
		return tea.Batch(d.spin.Tick, runDiagnosis(c, d.printer.Name))
	}
	return nil
}

func (d diagnoseModel) view() string {
	var b strings.Builder
	b.WriteString(styleBold.Render("  Diagnose — " + d.printer.Name))
	b.WriteString(styleDim.Render("   esc back"))
	b.WriteString("\n\n")

	if d.err != nil {
		b.WriteString("  " + styleErrText.Render(describeError(d.err)) + "\n\n")
	}

	switch {
	case d.loading:
		b.WriteString("  " + d.spin.View() + styleDim.Render(" Diagnosing…"))
	default:
		for _, check := range d.result.Checks {
			label := styleApp.Width(20).Render(styleValue.Render(check.Name))
			detail := styleDim.Render("  " + check.Detail)
			b.WriteString("  " + statusMark(check.Status) + " " + label + detail + "\n")
			if check.Hint != "" {
				b.WriteString("      " + hintStyle(check.Status).Render(wrapText(check.Hint, d.width-10)) + "\n")
			}
		}
	}

	if d.hasAdvice {
		b.WriteString("\n")
		b.WriteString("  " + styleLabel.Render("Recommended driver") + "\n")
		b.WriteString("    " + styleValue.Render(d.advice.Vendor+" "+d.advice.Model) + "\n")
		b.WriteString("    " + styleDim.Render("Package: ") + styleValue.Render(d.advice.Package) +
			styleDim.Render("  ("+d.advice.Source+")") + "\n")
		b.WriteString("    " + styleDim.Render("Install: ") + styleValue.Render(d.advice.InstallHint) + "\n")
		b.WriteString("    " + styleDim.Render(wrapText("cupstui never runs this — copy it and run it yourself.", d.width-6)) + "\n")
	}

	b.WriteString("\n")
	if d.loading {
		b.WriteString(styleDim.Render("  esc back"))
	} else {
		b.WriteString(styleDim.Render("  r recheck · esc back"))
	}
	return b.String()
}
