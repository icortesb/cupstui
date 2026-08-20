package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/icortesb/cupstui/internal/cups"
)

// logLines is how many lines from the end of the log are kept in memory.
const logLines = 500

// logsModel shows one CUPS log, following it live.
type logsModel struct {
	vp    viewport.Model
	file  int // index into cups.LogFiles
	lines []string
	err   error
	// follow keeps the view pinned to the end; it turns off when scrolling up
	// so a line can be read without the refresh dragging the screen away.
	follow bool
	// min hides anything cupsd stamped below it. At SeverityNone nothing is
	// hidden, which is where it starts: the log says what it says.
	min cups.Severity
}

func newLogs() logsModel {
	return logsModel{vp: viewport.New(80, 10), follow: true}
}

func (l *logsModel) setSize(width, height int) {
	if height < 3 {
		height = 3
	}
	l.vp.Width = width
	l.vp.Height = height
	l.render()
}

func (l logsModel) current() cups.LogFile {
	return cups.LogFiles[l.file]
}

func (l *logsModel) nextFile() {
	l.file = (l.file + 1) % len(cups.LogFiles)
	l.lines, l.err = nil, nil
	l.follow = true
	l.render()
}

// levels are what cycleLevel walks through, quietest last.
var levels = []cups.Severity{cups.SeverityNone, cups.SeverityInfo, cups.SeverityWarning, cups.SeverityError}

func (l *logsModel) cycleLevel() {
	for i, s := range levels {
		if s == l.min {
			l.min = levels[(i+1)%len(levels)]
			l.render()
			return
		}
	}
	l.min = cups.SeverityNone
	l.render()
}

// atLeast drops the lines below min. A line cupsd stamped with no level at all
// — every line of access_log and page_log — has nothing to compare and stays,
// so the filter only ever thins the log it was meant for.
func atLeast(lines []string, min cups.Severity) []string {
	if min == cups.SeverityNone {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if s := cups.LineSeverity(line); s == cups.SeverityNone || s >= min {
			out = append(out, line)
		}
	}
	return out
}

// levelName is how the header names the current floor.
func levelName(s cups.Severity) string {
	switch s {
	case cups.SeverityInfo:
		return "info+"
	case cups.SeverityWarning:
		return "warnings+"
	case cups.SeverityError:
		return "errors"
	default:
		return "all"
	}
}

func (l *logsModel) setLines(lines []string, err error) {
	l.lines, l.err = lines, err
	l.render()
}

func (l *logsModel) render() {
	// Filter first, then collapse: hiding the chatter in between often leaves
	// two identical lines next to each other that are worth folding.
	body := strings.Join(colourise(cups.Collapse(atLeast(l.lines, l.min))), "\n")
	if l.err != nil {
		body = ""
	}
	l.vp.SetContent(body)
	if l.follow {
		l.vp.GotoBottom()
	}
}

func (l *logsModel) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	l.vp, cmd = l.vp.Update(msg)
	// Keep following only while the end is in view.
	l.follow = l.vp.AtBottom()
	return cmd
}

func (l logsModel) view(width int) string {
	f := l.current()

	title := styleBold.Render(f.Name) + styleDim.Render("  "+f.Desc)
	state := styleDim.Render("paused — G to resume")
	if l.follow {
		state = styleOKText.Render("following")
	}
	if l.min != cups.SeverityNone {
		state = styleWarnText.Render(levelName(l.min)) + "  " + state
	}
	gap := width - lipgloss.Width(title) - lipgloss.Width(state) - 2
	if gap < 1 {
		gap = 1
	}
	header := " " + title + strings.Repeat(" ", gap) + state

	if l.err != nil {
		return strings.Join([]string{
			header, "",
			"  " + styleErrText.Render("Could not read "+f.Path),
			"  " + styleDim.Render(describeError(l.err)),
		}, "\n")
	}
	if len(l.lines) == 0 {
		return header + "\n\n" + styleDim.Render("  The log is empty.")
	}
	if len(atLeast(l.lines, l.min)) == 0 {
		return header + "\n\n" + styleDim.Render(
			"  Nothing at "+levelName(l.min)+" — press v for the rest.")
	}
	return header + "\n" + l.vp.View()
}

// colourise tints each line by the level cupsd stamped on it, so that an error
// stands out in a wall of routine chatter.
func colourise(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		var style lipgloss.Style
		switch cups.LineSeverity(line) {
		case cups.SeverityError:
			style = styleErrText
		case cups.SeverityWarning:
			style = styleWarnText
		case cups.SeverityDebug:
			style = styleDim
		default:
			style = styleValue
		}
		// The level letter and timestamp are the same thirty characters on
		// every line; keeping them dim leaves the colour to mean the message.
		prefix, msg := cups.SplitLogLine(line)
		out[i] = styleDim.Render(prefix) + style.Render(msg)
	}
	return out
}

// readLog reads the log off the UI goroutine.
func readLog(file cups.LogFile) tea.Cmd {
	return func() tea.Msg {
		lines, err := cups.Tail(file.Path, logLines)
		return logsMsg{lines: lines, err: err}
	}
}
