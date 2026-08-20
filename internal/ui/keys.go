package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	NextTab   key.Binding
	PrevTab   key.Binding
	Tab1      key.Binding
	Tab2      key.Binding
	Tab3      key.Binding
	Filter    key.Binding
	Refresh   key.Binding
	Cancel    key.Binding
	CancelAll key.Binding
	Toggle    key.Binding
	Default   key.Binding
	Accepting key.Binding
	Help      key.Binding
	Escape    key.Binding
	Quit      key.Binding
}

var keys = keyMap{
	Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "mover")),
	Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/k", "mover")),
	NextTab:   key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab", "siguiente pestaña")),
	PrevTab:   key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("shift+tab", "pestaña anterior")),
	Tab1:      key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "dashboard")),
	Tab2:      key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "cola")),
	Tab3:      key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "impresoras")),
	Filter:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filtrar")),
	Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refrescar ya")),
	Cancel:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "cancelar trabajo")),
	CancelAll: key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "cancelar todos")),
	Toggle:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "habilitar/deshabilitar")),
	Default:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "marcar por omisión")),
	Accepting: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "aceptar/rechazar trabajos")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "ayuda")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "volver")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "salir")),
}
