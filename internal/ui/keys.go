package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	NextTab     key.Binding
	PrevTab     key.Binding
	Tab1        key.Binding
	Tab2        key.Binding
	Tab3        key.Binding
	Tab4        key.Binding
	Tab5        key.Binding
	NextLog     key.Binding
	Filter      key.Binding
	Refresh     key.Binding
	Cancel      key.Binding
	HoldJob     key.Binding
	CancelAll   key.Binding
	Toggle      key.Binding
	Default     key.Binding
	Accepting   key.Binding
	AddPrinter  key.Binding
	Help        key.Binding
	Transparent key.Binding
	Escape      key.Binding
	Quit        key.Binding
}

var keys = keyMap{
	Up:          key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "mover")),
	Down:        key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/k", "mover")),
	NextTab:     key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab", "siguiente pestaña")),
	PrevTab:     key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("shift+tab", "pestaña anterior")),
	Tab1:        key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "dashboard")),
	Tab2:        key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "cola")),
	Tab3:        key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "impresoras")),
	Tab4:        key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "imprimir")),
	Tab5:        key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "logs")),
	NextLog:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "siguiente registro")),
	Filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filtrar")),
	Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refrescar ya")),
	Cancel:      key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "cancelar trabajo")),
	HoldJob:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pausar/reanudar trabajo")),
	CancelAll:   key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "cancelar todos")),
	Toggle:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "habilitar/deshabilitar")),
	Default:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "marcar por omisión")),
	Accepting:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "aceptar/rechazar trabajos")),
	AddPrinter:  key.NewBinding(key.WithKeys("A", "+"), key.WithHelp("A", "agregar impresora")),
	Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "ayuda")),
	Transparent: key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "fondo propio / transparente")),
	Escape:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "volver")),
	Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "salir")),
}
