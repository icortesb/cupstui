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
