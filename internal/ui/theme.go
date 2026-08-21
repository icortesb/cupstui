package ui

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
)

// Palette. Written in hex rather than as ANSI 256 indices: lipgloss knows what
// the terminal can take and steps the colour down to the nearest one it has, so
// a capable terminal gets the colour that was chosen instead of whichever
// corner of the xterm cube happened to be closest to it.
//
// Every fill colour is dark in the light theme and light in the dark one, which
// is what lets colorInk be the one readable foreground for all of them.
var (
	colorBG     = lipgloss.AdaptiveColor{Light: "#f7f8fa", Dark: "#16181d"}
	colorBGAlt  = lipgloss.AdaptiveColor{Light: "#eceef2", Dark: "#1e2128"}
	colorAccent = lipgloss.AdaptiveColor{Light: "#0a63c2", Dark: "#62c8ff"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#12794a", Dark: "#4fd6a0"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "#9a5a00", Dark: "#f5b661"}
	colorErr    = lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ff6b6b"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#5c6472", Dark: "#8b93a5"}
	colorText   = lipgloss.AdaptiveColor{Light: "#1f2430", Dark: "#e2e6ee"}

	// colorInk is what goes on top of a filled block — the title, the active
	// tab, a badge, the banner. It has to invert with the theme: the fills are
	// dark blues and reds on a light terminal, where near-black text on them
	// was barely there.
	colorInk = lipgloss.AdaptiveColor{Light: "#f7f8fa", Dark: "#0d1016"}

	// titleGradient tints the name in the header across its letters. It only
	// shows on a terminal with 24-bit colour; anywhere else the steps collapse
	// onto each other and the title is drawn flat instead.
	titleGradient = [2]lipgloss.AdaptiveColor{
		colorAccent,
		{Light: "#6d28d9", Dark: "#c4a2ff"},
	}
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
	styleBody        lipgloss.Style
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
		Foreground(colorInk).
		Background(colorAccent).
		Padding(0, 1)

	// The active tab is filled rather than underlined: on a wide bar of six
	// names, an underline is easy to miss.
	styleTabActive = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorInk).
		Background(colorAccent).
		Padding(0, 2)

	styleTabInactive = base().
		Foreground(colorMuted).
		Padding(0, 2)

	styleBadge = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorInk).
		Background(colorWarn).
		Padding(0, 1)

	// No rule of its own: the body panel's top border is the line under the
	// header, and two adjacent rules read as a seam rather than a divider.
	styleHeaderBar = base()

	styleBody = base().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1)

	styleStatusBar = base().Foreground(colorMuted).Padding(0, 1)
	styleStatusOK = base().Foreground(colorOK).Bold(true).Padding(0, 1)
	styleStatusErr = base().Foreground(colorErr).Bold(true).Padding(0, 1)

	styleBanner = base().
		Foreground(colorInk).
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
		styleBody = styleBody.BorderBackground(colorBG)
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

// titleView draws the application name in the header, stepping its background
// from one end of titleGradient to the other across the letters.
//
// A gradient is only drawn where there are enough colours to draw one with: on
// anything below 24-bit every step lands on the same cell of the palette, and
// what should be a fade comes out as a flat block with a seam in it. There the
// title is rendered as it always was.
func titleView(name string) string {
	if lipgloss.ColorProfile() != termenv.TrueColor {
		return styleTitle.Render(name)
	}

	from, err1 := colorful.Hex(resolve(titleGradient[0]))
	to, err2 := colorful.Hex(resolve(titleGradient[1]))
	if err1 != nil || err2 != nil {
		return styleTitle.Render(name)
	}

	// The padding styleTitle would have added is drawn here instead, as the two
	// end colours, so the block keeps its shape.
	runes := []rune(" " + name + " ")
	var b strings.Builder
	for i, r := range runes {
		// Blending in Lab keeps the midpoint from going muddy the way it does
		// when the two ends are mixed channel by channel in RGB.
		pos := float64(i) / float64(len(runes)-1)
		step := from.BlendLab(to, pos).Clamped()
		b.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(colorInk).
			Background(lipgloss.Color(step.Hex())).
			Render(string(r)))
	}
	return b.String()
}

// resolve picks the side of an adaptive colour the terminal is actually using,
// which lipgloss will only do while rendering.
func resolve(c lipgloss.AdaptiveColor) string {
	if lipgloss.HasDarkBackground() {
		return c.Dark
	}
	return c.Light
}
