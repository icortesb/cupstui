package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func withColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(prev)
		SetTransparent(false)
	})
	SetTransparent(false)
}

func TestPaintBackgroundCoversEveryLine(t *testing.T) {
	withColor(t)

	// A deliberate mix: raw unstyled text, styled text (which ends in a reset)
	// and lines of differing length.
	view := "hola\n" + styleKey.Render("x") + " corto\n" + strings.Repeat("─", 5)
	const width = 20

	out := paintBackground(view, width)
	bg := backgroundSequence()
	if bg == "" {
		t.Fatal("without the background sequence there is nothing to check")
	}

	for i, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, bg) {
			t.Errorf("line %d does not start painted: %q", i, line)
		}
		if w := lipgloss.Width(line); w != width {
			t.Errorf("line %d is %d wide, want %d", i, w, width)
		}
	}
}

func TestPaintBackgroundRepaintsAfterEveryReset(t *testing.T) {
	withColor(t)

	// A reset mid-line leaves the rest on the terminal background, which is
	// exactly what makes text unreadable in a translucent terminal.
	view := styleKey.Render("tecla") + " descripción"
	out := paintBackground(view, 40)
	bg := backgroundSequence()

	for _, reset := range []string{"\x1b[0m", "\x1b[49m"} {
		idx := 0
		for {
			j := strings.Index(out[idx:], reset)
			if j < 0 {
				break
			}
			at := idx + j + len(reset)
			rest := out[at:]
			if rest != "" && !strings.HasPrefix(rest, bg) && !strings.HasPrefix(rest, "\n") {
				t.Errorf("after %q the background is not repainted: %q", reset, rest)
			}
			idx = at
		}
	}
}

func TestPaintBackgroundLeavesViewAloneWhenTransparent(t *testing.T) {
	withColor(t)
	SetTransparent(true)

	view := "hola\nmundo"
	if got := paintBackground(view, 40); got != view {
		t.Errorf("in transparent mode the view is untouched, got %q", got)
	}
}

// withProfile pins the colour profile for one test and puts back what was
// there, so the tests do not depend on the terminal running them.
func withProfile(t *testing.T, p termenv.Profile) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(p)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(prev)
		SetTransparent(false)
	})
	SetTransparent(false)
}

func TestTitleKeepsItsWidthWhateverTheTerminalCanColour(t *testing.T) {
	// The title sits at the left of the tab bar, so a title that measured
	// differently per terminal would shift every tab along with it.
	const want = len(" cupstui ")
	for _, p := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.ANSI, termenv.Ascii} {
		t.Run(p.Name(), func(t *testing.T) {
			withProfile(t, p)
			if got := lipgloss.Width(titleView("cupstui")); got != want {
				t.Errorf("width = %d, want %d", got, want)
			}
		})
	}
}

func TestTitleFadesOnlyWhereThereAreColoursToFadeThrough(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	faded := titleView("cupstui")

	withProfile(t, termenv.ANSI256)
	flat := titleView("cupstui")

	if strings.Count(faded, "\x1b[") <= strings.Count(flat, "\x1b[") {
		t.Error("24-bit colour should give the title a step per letter")
	}
	if flat != styleTitle.Render("cupstui") {
		t.Error("below 24-bit the title should be the plain filled block")
	}
}

func TestInkInvertsWithTheTheme(t *testing.T) {
	// Every filled block — title, active tab, badge, banner — puts colorInk on
	// a fill colour. The fills are dark on a light terminal and light on a dark
	// one, so ink that did not invert would vanish into one of them.
	if colorInk.Light == colorInk.Dark {
		t.Fatal("colorInk has to differ between the themes")
	}
	if !strings.HasPrefix(colorInk.Light, "#f") || !strings.HasPrefix(colorInk.Dark, "#0") {
		t.Errorf("colorInk = %+v, want light ink on the light theme's dark fills and the reverse", colorInk)
	}
}
