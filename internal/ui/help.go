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
		{"Navigation", []key.Binding{keys.Down, keys.NextTab, keys.PrevTab, keys.Tab1, keys.Tab2, keys.Tab3, keys.Tab4, keys.Tab5, keys.Tab6}},
		{"Queue", []key.Binding{keys.HoldJob, keys.Cancel, keys.CancelAll, keys.Filter}},
		{"Printers", []key.Binding{keys.Toggle, keys.Default, keys.Accepting, keys.AddPrinter, keys.DeletePrinter, keys.Policy}},
		{"Print", []key.Binding{
			key.NewBinding(key.WithKeys("down"), key.WithHelp("↑↓ j/k", "change field")),
			key.NewBinding(key.WithKeys("left"), key.WithHelp("←→ h/l", "change value")),
			key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "browse files")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "print")),
		}},
		{"History", []key.Binding{keys.Summary, keys.Filter, keys.Export}},
		{"Logs", []key.Binding{keys.NextLog}},
		{"General", []key.Binding{keys.Refresh, keys.Transparent, keys.Escape, keys.Help, keys.Quit}},
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
	b.WriteString(styleBold.Render("  Queue filter"))
	b.WriteString("\n    ")
	b.WriteString(styleDim.Render("free text, or scoped to one field: "))
	b.WriteString(styleKey.Render("printer:") + styleDim.Render("epson  "))
	b.WriteString(styleKey.Render("user:") + styleDim.Render("ana  "))
	b.WriteString(styleKey.Render("state:") + styleDim.Render("held"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("    Terms combine: every one must match. Confirm prompts answer y or n."))
	return b.String()
}

func pad(s string, n int) string {
	for len([]rune(s)) < n {
		s += " "
	}
	return s
}

// shortHelp is the footer line of shortcuts, different for each tab.
func shortHelp(t tab, filtering bool) string {
	if filtering {
		return hint("enter", "apply") + " · " + hint("esc", "clear filter")
	}
	common := hint("tab", "switch") + " · " + hint("r", "refresh") + " · " + hint("?", "help") + " · " + hint("q", "quit")
	switch t {
	case tabQueue:
		return hint("j/k", "move") + " · " + hint("p", "hold") + " · " + hint("x", "cancel") + " · " + hint("X", "all") + " · " + hint("/", "filter") + " · " + common
	case tabPrint:
		return hint("↑↓", "field") + " · " + hint("←→", "value") + " · " + hint("ctrl+o", "browse") + " · " + hint("enter", "print") + " · " + hint("tab", "switch tab")
	case tabHistory:
		return hint("s", "totals") + " · " + hint("j/k", "move") + " · " + hint("/", "filter") + " · " + hint("E", "export CSV") + " · " + common
	case tabLogs:
		return hint("j/k", "scroll") + " · " + hint("G", "jump to end") + " · " + hint("n", "next log") + " · " + common
	case tabPrinters:
		return hint("j/k", "move") + " · " + hint("e", "enable") + " · " + hint("d", "default") + " · " + hint("a", "accept") + " · " + hint("A", "add") + " · " + hint("x", "remove") + " · " + hint("u", "quotas") + " · " + common
	default:
		return common
	}
}

func hint(k, desc string) string {
	return styleKey.Render(k) + " " + styleDim.Render(desc)
}
