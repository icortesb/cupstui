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

	// Mezcla deliberada: texto crudo sin estilo, texto estilado (que termina
	// en un reset) y líneas de distinto largo.
	view := "hola\n" + styleKey.Render("x") + " corto\n" + strings.Repeat("─", 5)
	const width = 20

	out := paintBackground(view, width)
	bg := backgroundSequence()
	if bg == "" {
		t.Fatal("sin secuencia de fondo no hay nada que verificar")
	}

	for i, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, bg) {
			t.Errorf("línea %d no arranca pintada: %q", i, line)
		}
		if w := lipgloss.Width(line); w != width {
			t.Errorf("línea %d mide %d, quiero %d", i, w, width)
		}
	}
}

func TestPaintBackgroundRepaintsAfterEveryReset(t *testing.T) {
	withColor(t)

	// Un reset a mitad de línea deja el resto con el fondo del terminal, que es
	// justo lo que hace ilegible el texto en una terminal translúcida.
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
				t.Errorf("tras %q el fondo no se repinta: %q", reset, rest)
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
		t.Errorf("en modo transparente no se toca la vista, tengo %q", got)
	}
}
