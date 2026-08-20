package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

type helpSection struct {
	title    string
	bindings []key.Binding
}

func helpView(width int) string {
	sections := []helpSection{
		{"Navegación", []key.Binding{keys.Down, keys.NextTab, keys.PrevTab, keys.Tab1, keys.Tab2, keys.Tab3, keys.Tab4, keys.Tab5}},
		{"Cola", []key.Binding{keys.HoldJob, keys.Cancel, keys.CancelAll, keys.Filter}},
		{"Impresoras", []key.Binding{keys.Toggle, keys.Default, keys.Accepting, keys.AddPrinter}},
		{"Imprimir", []key.Binding{
			key.NewBinding(key.WithKeys("down"), key.WithHelp("↑↓ j/k", "cambiar de campo")),
			key.NewBinding(key.WithKeys("left"), key.WithHelp("←→ h/l", "cambiar el valor")),
			key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "buscar archivo")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "imprimir")),
		}},
		{"Logs", []key.Binding{keys.NextLog}},
		{"General", []key.Binding{keys.Refresh, keys.Escape, keys.Help, keys.Quit}},
	}

	rendered := make([]string, 0, len(sections))
	for _, sec := range sections {
		rendered = append(rendered, renderSection(sec))
	}

	// En dos columnas la ayuda entra completa en una terminal corta; en una
	// terminal angosta se apilan.
	body := strings.Join(rendered, "\n")
	if width >= 74 {
		half := (len(rendered) + 1) / 2
		left := lipgloss.JoinVertical(lipgloss.Left, rendered[:half]...)
		right := lipgloss.JoinVertical(lipgloss.Left, rendered[half:]...)
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			styleApp.Width(width/2).Render(left), right)
	}

	return body + "\n" + filterHelp()
}

func renderSection(sec helpSection) string {
	var b strings.Builder
	b.WriteString(styleBold.Render("  " + sec.title))
	b.WriteString("\n")
	for _, bind := range sec.bindings {
		h := bind.Help()
		fmt.Fprintf(&b, "    %s  %s\n", styleKey.Render(pad(h.Key, 11)), styleValue.Render(h.Desc))
	}
	return b.String()
}

func filterHelp() string {
	var b strings.Builder
	b.WriteString(styleBold.Render("  Filtro de la cola"))
	b.WriteString("\n    ")
	b.WriteString(styleDim.Render("texto libre, o acotado a un campo: "))
	b.WriteString(styleKey.Render("impresora:") + styleDim.Render("epson  "))
	b.WriteString(styleKey.Render("usuario:") + styleDim.Render("ana  "))
	b.WriteString(styleKey.Render("estado:") + styleDim.Render("retenido"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("    Las confirmaciones se responden con y (sí) o n / esc (no)."))
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
		return hint("j/k", "mover") + " · " + hint("p", "pausar") + " · " + hint("x", "cancelar") + " · " + hint("X", "todos") + " · " + hint("/", "filtrar") + " · " + common
	case tabPrint:
		return hint("↑↓", "campo") + " · " + hint("←→", "valor") + " · " + hint("ctrl+o", "buscar") + " · " + hint("enter", "imprimir") + " · " + common
	case tabLogs:
		return hint("j/k", "desplazar") + " · " + hint("G", "ir al final") + " · " + hint("n", "otro registro") + " · " + common
	case tabPrinters:
		return hint("j/k", "mover") + " · " + hint("e", "habilitar/deshabilitar") + " · " + hint("d", "por omisión") + " · " + hint("a", "aceptar") + " · " + hint("A", "agregar") + " · " + common
	default:
		return common
	}
}

func hint(k, desc string) string {
	return styleKey.Render(k) + " " + styleDim.Render(desc)
}
