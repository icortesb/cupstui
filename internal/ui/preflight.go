package ui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/icortesb/cupstui/internal/cups"
)

// preflightModel is the startup screen: it reports what the machine can and
// cannot do before the interface is entered, so a missing group membership or
// a stopped service is not discovered one keystroke at a time.
type preflightModel struct {
	active  bool
	spin    spinner.Model
	names   []string
	results map[string]cups.CheckResult
	cancel  context.CancelFunc
	width   int
}

func newPreflight() preflightModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return preflightModel{
		spin:    sp,
		names:   cups.PreflightNames(),
		results: make(map[string]cups.CheckResult),
	}
}

func (p *preflightModel) setSize(width int) { p.width = width }

// start launches every check at once and returns the commands that feed the
// screen while they run.
func (p *preflightModel) start(c *cups.Client) tea.Cmd {
	p.active = true
	p.results = make(map[string]cups.CheckResult)
	p.spin.Style = styleAccentText

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	out := make(chan cups.CheckResult, len(p.names))
	cups.Preflight(ctx, c, out)

	return tea.Batch(p.spin.Tick, waitForCheck(out))
}

// waitForCheck delivers one result and then queues itself again, so the screen
// fills in as answers arrive rather than all at once at the end.
func waitForCheck(out chan cups.CheckResult) tea.Cmd {
	return func() tea.Msg {
		return checkMsg{result: <-out, source: out}
	}
}

func (p *preflightModel) record(msg checkMsg) tea.Cmd {
	p.results[msg.result.Name] = msg.result
	if len(p.results) >= len(p.names) {
		return nil
	}
	return waitForCheck(msg.source)
}

// done reports whether every check has answered.
func (p preflightModel) done() bool {
	return len(p.results) >= len(p.names)
}

func (p *preflightModel) dismiss() {
	if p.cancel != nil {
		p.cancel()
	}
	p.active = false
}

// blocking reports whether something failed outright, in which case the
// interface is not much use yet.
func (p preflightModel) blocking() bool {
	for _, r := range p.results {
		if r.Status == cups.CheckFail {
			return true
		}
	}
	return false
}

func (p preflightModel) view() string {
	var b strings.Builder

	b.WriteString("\n  " + styleTitle.Render("cupstui") + "\n")
	b.WriteString("  " + styleDim.Render("checking this machine") + "\n\n")

	for _, name := range p.names {
		result, answered := p.results[name]

		mark := p.spin.View()
		label := styleDim.Render(name)
		detail := ""

		if answered {
			mark, label = statusMark(result.Status), styleValue.Render(name)
			detail = styleDim.Render("  " + result.Detail)
		}

		// The label is padded with lipgloss, which measures the visible width:
		// pad would count the colour codes as characters and misalign the
		// column as soon as a check answers.
		b.WriteString("  " + mark + " " + styleApp.Width(26).Render(label) + detail + "\n")

		if answered && result.Hint != "" {
			b.WriteString("      " + hintStyle(result.Status).Render(
				wrapText(result.Hint, p.width-10)) + "\n")
		}
	}

	b.WriteString("\n")
	if !p.done() {
		return b.String()
	}

	switch {
	case p.blocking():
		b.WriteString("  " + styleErrText.Render("CUPS is not available.") + " " +
			styleDim.Render("enter to continue anyway · q to quit"))
	default:
		b.WriteString("  " + styleDim.Render("enter to continue"))
	}
	return b.String()
}

func statusMark(s cups.CheckStatus) string {
	switch s {
	case cups.CheckOK:
		return styleOKText.Bold(true).Render("✓")
	case cups.CheckWarn:
		return styleWarnText.Bold(true).Render("!")
	case cups.CheckFail:
		return styleErrText.Bold(true).Render("✗")
	default:
		return styleDim.Render("·")
	}
}

func hintStyle(s cups.CheckStatus) lipgloss.Style {
	if s == cups.CheckFail {
		return styleErrText
	}
	return styleDim
}

// wrapText breaks a hint into lines that fit, so a long remediation note does not
// run off the screen.
func wrapText(text string, width int) string {
	if width < 20 {
		width = 20
	}

	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n      ")
}
