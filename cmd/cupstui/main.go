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
		"no pintar fondo propio y dejar ver el del terminal (puede afectar la legibilidad)")
	flag.Parse()

	// La preferencia guardada manda, salvo que se pase el flag explícitamente.
	// La tecla T la cambia y la guarda durante la sesión.
	setting := config.Load().Transparent
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "transparent" {
			setting = *transparent
		}
	})
	ui.SetTransparent(setting)

	client, err := cups.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo iniciar el cliente de CUPS:", err)
		os.Exit(1)
	}

	defer client.Close()

	// Sin WithAltScreen la TUI dejaría basura en el scrollback al salir.
	p := tea.NewProgram(ui.New(client), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "cupstui:", err)
		os.Exit(1)
	}
}
