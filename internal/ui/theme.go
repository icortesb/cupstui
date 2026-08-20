package ui

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Palette. ANSI 256 colours, so it works in any terminal, with a light and a
// dark variant wherever contrast matters.
var (
	colorBG     = lipgloss.AdaptiveColor{Light: "255", Dark: "234"}
	colorBGAlt  = lipgloss.AdaptiveColor{Light: "253", Dark: "236"}
	colorAccent = lipgloss.AdaptiveColor{Light: "26", Dark: "81"}
	colorOK     = lipgloss.AdaptiveColor{Light: "28", Dark: "78"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "130", Dark: "215"}
	colorErr    = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "241", Dark: "247"}
	colorText   = lipgloss.AdaptiveColor{Light: "235", Dark: "253"}
)

// transparent lets the terminal background through instead of painting one.
//
// By default the application paints its own: in a translucent terminal, grey
// text that reads over a dark part of the desktop disappears over a light one,
// and no choice of foreground colour fixes that.
var transparent bool

var (
	styleTitle       lipgloss.Style
	styleTabActive   lipgloss.Style
	styleTabInactive lipgloss.Style
	styleHeaderBar   lipgloss.Style
	styleStatusBar   lipgloss.Style
	styleStatusOK    lipgloss.Style
	styleStatusErr   lipgloss.Style
	styleBanner      lipgloss.Style
	styleCard        lipgloss.Style
	styleLabel       lipgloss.Style
	styleValue       lipgloss.Style
	styleBold        lipgloss.Style
	styleDim         lipgloss.Style
	styleKey         lipgloss.Style
	styleApp         lipgloss.Style
	styleBadge       lipgloss.Style
	styleAccentText  lipgloss.Style
	styleWarnText    lipgloss.Style
	styleOKText      lipgloss.Style
	styleErrText     lipgloss.Style
)

func init() {
	useTerminfoColors()
	SetTransparent(false)
}

// useTerminfoColors asks terminfo what the terminal can do when the name alone
// does not say. lipgloss reads TERM for the word "color", which foot,
// alacritty, wezterm and the rest do not carry: they announce themselves
// through COLORTERM instead, and that one is lost across ssh and inside a
// misconfigured tmux, leaving a perfectly capable terminal drawn in no colour
// at all. terminfo is the database built to answer this, so it gets asked.
func useTerminfoColors() {
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return
	}
	// What lipgloss itself looks at. Asking it directly instead would make it
	// query the terminal for its background before the program owns the input,
	// and the answer would arrive with nobody reading.
	if os.Getenv("COLORTERM") != "" ||
		strings.Contains(term, "color") || strings.Contains(term, "ansi") {
		return
	}

	out, err := exec.Command("tput", "colors").Output()
	if err != nil {
		return
	}
	colors, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return
	}

	switch {
	case colors >= 1<<24:
		lipgloss.SetColorProfile(termenv.TrueColor)
	case colors >= 256:
		lipgloss.SetColorProfile(termenv.ANSI256)
	case colors >= 8:
		lipgloss.SetColorProfile(termenv.ANSI)
	}
}

// SetTransparent rebuilds the styles. Call it before New.
func SetTransparent(v bool) {
	transparent = v
	buildStyles()
}

// base is where every style starts: when the application paints a background,
// each fragment of text has to carry it, because runs with no background colour
// let the terminal's show through and break up the block.
func base() lipgloss.Style {
	s := lipgloss.NewStyle()
	if !transparent {
		s = s.Background(colorBG)
	}
	return s
}

func buildStyles() {
	styleApp = base()

	styleTitle = base().
		Bold(true).
		Foreground(lipgloss.Color("232")).
		Background(colorAccent).
		Padding(0, 1)

	// The active tab is filled rather than underlined: on a wide bar of six
	// names, an underline is easy to miss.
	styleTabActive = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("232")).
		Background(colorAccent).
		Padding(0, 2)

	styleTabInactive = base().
		Foreground(colorMuted).
		Padding(0, 2)

	styleBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("232")).
		Background(colorWarn).
		Padding(0, 1)

	styleHeaderBar = base().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(colorMuted)

	styleStatusBar = base().Foreground(colorMuted).Padding(0, 1)
	styleStatusOK = base().Foreground(colorOK).Bold(true).Padding(0, 1)
	styleStatusErr = base().Foreground(colorErr).Bold(true).Padding(0, 1)

	styleBanner = base().
		Foreground(lipgloss.Color("232")).
		Background(colorErr).
		Bold(true).
		Padding(0, 1)

	styleCard = base().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		MarginRight(1)

	styleLabel = base().Foreground(colorMuted)
	styleValue = base().Foreground(colorText)
	styleBold = base().Bold(true).Foreground(colorText)
	styleDim = base().Foreground(colorMuted)
	styleKey = base().Bold(true).Foreground(colorAccent)

	styleAccentText = base().Foreground(colorAccent)
	styleWarnText = base().Foreground(colorWarn)
	styleOKText = base().Foreground(colorOK)
	styleErrText = base().Foreground(colorErr)

	if !transparent {
		styleCard = styleCard.BorderBackground(colorBG)
		styleHeaderBar = styleHeaderBar.BorderBackground(colorBG)
	}
}

// backgroundSequence returns the ANSI sequence that turns on the application
// background, resolved for the active colour profile and theme.
//
// It is obtained by asking lipgloss to paint a sentinel character and keeping
// what it emits before it, so neither the profile degradation nor the choice
// between light and dark has to be reimplemented here.
func backgroundSequence() string {
	if transparent {
		return ""
	}
	const sentinel = "\x00"
	rendered := lipgloss.NewStyle().Background(colorBG).Render(sentinel)
	i := strings.Index(rendered, sentinel)
	if i <= 0 {
		return "" // a terminal with no colour
	}
	return rendered[:i]
}

// paintBackground forces the application background across the whole screen.
//
// Setting Background on every style is not enough: the spaces and borders
// joined by hand carry no style, and each `\x1b[0m` lipgloss closes a fragment
// with returns the background to the terminal's. In a translucent terminal that
// leaves text sitting on the desktop wallpaper, unreadable as soon as the image
// has a light area.
func paintBackground(view string, width int) string {
	bg := backgroundSequence()
	if bg == "" {
		return view
	}

	lines := strings.Split(view, "\n")
	for i, line := range lines {
		// Repaint after every reset, whether full (0m) or background only (49m).
		line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+bg)
		line = strings.ReplaceAll(line, "\x1b[49m", "\x1b[49m"+bg)

		if pad := width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lines[i] = bg + line + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}
