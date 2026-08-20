// cupstui es una interfaz de terminal para administrar la impresión en CUPS.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/icortes/cupstui/internal/config"
	"github.com/icortes/cupstui/internal/cups"
	"github.com/icortes/cupstui/internal/ui"
)

func main() {
	transparent := flag.Bool("transparent", false,
		"do not paint a background; let the terminal show through")
	check := flag.Bool("check", false,
		"report what this machine can and cannot do, then continue")
	flag.Parse()

	// La preferencia guardada manda, salvo que se pase el flag explícitamente.
	// La tecla T la cambia y la guarda durante la sesión.
	saved := config.Load()
	setting := saved.Transparent
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "transparent" {
			setting = *transparent
		}
	})
	ui.SetTransparent(setting)

	client, err := cups.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not start the CUPS client:", err)
		os.Exit(1)
	}

	defer client.Close()

	// Sin WithAltScreen la TUI dejaría basura en el scrollback al salir.
	model := ui.New(client)
	// The checks run on the first start, when nothing is known about the
	// machine yet, and on request after that.
	if *check || !saved.Seen {
		model = model.ShowPreflight()
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "cupstui:", err)
		os.Exit(1)
	}
}
