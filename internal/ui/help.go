package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type helpSection struct {
	title    string
	bindings []key.Binding
}

func helpView() string {
	sections := []helpSection{
		{"Navegación", []key.Binding{keys.Down, keys.NextTab, keys.PrevTab, keys.Tab1, keys.Tab2, keys.Tab3}},
		{"Cola", []key.Binding{keys.Cancel, keys.CancelAll, keys.Filter}},
		{"Impresoras", []key.Binding{keys.Toggle, keys.Default, keys.Accepting}},
		{"General", []key.Binding{keys.Refresh, keys.Escape, keys.Help, keys.Quit}},
	}

	var b strings.Builder
	for _, s := range sections {
		b.WriteString(styleBold.Render("  " + s.title))
		b.WriteString("\n")
		for _, bind := range s.bindings {
			h := bind.Help()
			fmt.Fprintf(&b, "    %s  %s\n", styleKey.Render(pad(h.Key, 12)), styleValue.Render(h.Desc))
		}
		b.WriteString("\n")
	}
	b.WriteString(styleDim.Render("  Las confirmaciones se responden con y (sí) o n / esc (no)."))
	return b.String()
}

func pad(s string, n int) string {
	for len([]rune(s)) < n {
		s += " "
	}
	return s
}

// shortHelp es la línea de atajos del pie, distinta según la pestaña.
func shortHelp(t tab, filtering bool) string {
	if filtering {
		return hint("enter", "aplicar") + " · " + hint("esc", "limpiar filtro")
	}
	common := hint("tab", "cambiar") + " · " + hint("r", "refrescar") + " · " + hint("?", "ayuda") + " · " + hint("q", "salir")
	switch t {
	case tabQueue:
		return hint("j/k", "mover") + " · " + hint("x", "cancelar") + " · " + hint("X", "cancelar todos") + " · " + hint("/", "filtrar") + " · " + common
	case tabPrinters:
		return hint("j/k", "mover") + " · " + hint("e", "habilitar/deshabilitar") + " · " + hint("d", "por omisión") + " · " + hint("a", "aceptar trabajos") + " · " + common
	default:
		return common
	}
}

func hint(k, desc string) string {
	return styleKey.Render(k) + " " + styleDim.Render(desc)
}
