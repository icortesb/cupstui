// Command cupstui is a terminal interface for managing printing on CUPS.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/icortesb/cupstui/internal/config"
	"github.com/icortesb/cupstui/internal/cups"
	"github.com/icortesb/cupstui/internal/ui"
)

func main() {
	transparent := flag.Bool("transparent", false,
		"do not paint a background; let the terminal show through")
	check := flag.Bool("check", false,
		"report what this machine can and cannot do, then continue")
	flag.Parse()

	// The saved preference wins unless the flag is given explicitly. The T key
	// changes it and saves it during the session.
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

	// Without WithAltScreen the interface would leave debris in the scrollback.
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
