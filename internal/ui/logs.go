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

func (l *logsModel) setLines(lines []string, err error) {
	l.lines, l.err = lines, err
	l.render()
}

func (l *logsModel) render() {
	body := strings.Join(colourise(l.lines), "\n")
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
	return header + "\n" + l.vp.View()
}

// colourise tints each line by the level cupsd stamped on it, so that an error
// stands out in a wall of routine chatter.
func colourise(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		switch cups.LineSeverity(line) {
		case cups.SeverityError:
			out[i] = styleErrText.Render(line)
		case cups.SeverityWarning:
			out[i] = styleWarnText.Render(line)
		case cups.SeverityInfo:
			out[i] = styleValue.Render(line)
		case cups.SeverityDebug:
			out[i] = styleDim.Render(line)
		default:
			out[i] = styleValue.Render(line)
		}
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
