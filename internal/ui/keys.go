package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	NextTab       key.Binding
	PrevTab       key.Binding
	Tab1          key.Binding
	Tab2          key.Binding
	Tab3          key.Binding
	Tab4          key.Binding
	Tab5          key.Binding
	Tab6          key.Binding
	Export        key.Binding
	Summary       key.Binding
	NextLog       key.Binding
	Filter        key.Binding
	Refresh       key.Binding
	Cancel        key.Binding
	HoldJob       key.Binding
	CancelAll     key.Binding
	Toggle        key.Binding
	Default       key.Binding
	Accepting     key.Binding
	AddPrinter    key.Binding
	DeletePrinter key.Binding
	Policy        key.Binding
	Help          key.Binding
	Transparent   key.Binding
	SignIn        key.Binding
	Escape        key.Binding
	Quit          key.Binding
}

var keys = keyMap{
	Up:            key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "move")),
	Down:          key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/k", "move")),
	NextTab:       key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab", "next tab")),
	PrevTab:       key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("shift+tab", "previous tab")),
	Tab1:          key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "dashboard")),
	Tab2:          key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "queue")),
	Tab3:          key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "printers")),
	Tab4:          key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "print")),
	Tab5:          key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "history")),
	Tab6:          key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "logs")),
	Export:        key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "export CSV")),
	Summary:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "totals / rows")),
	NextLog:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next log")),
	Filter:        key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Refresh:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh now")),
	Cancel:        key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "cancel job")),
	HoldJob:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "hold/release job")),
	CancelAll:     key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "cancel all jobs")),
	Toggle:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "enable/disable")),
	Default:       key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "set as default")),
	Accepting:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "accept/reject jobs")),
	AddPrinter:    key.NewBinding(key.WithKeys("A", "+"), key.WithHelp("A", "add printer")),
	DeletePrinter: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "remove printer")),
	Policy:        key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "quotas and access")),
	Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Transparent:   key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "toggle transparent background")),
	SignIn:        key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sign in to a remote server")),
	Escape:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
