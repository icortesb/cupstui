// Command cupstui is a terminal interface for managing printing on CUPS.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/icortesb/cupstui/internal/config"
	"github.com/icortesb/cupstui/internal/cups"
	"github.com/icortesb/cupstui/internal/ui"
)

// version is set at build time by the release pipeline.
var version = "dev"

// buildVersion is the version to report. A release stamps it in; a
// `go install github.com/icortesb/cupstui/cmd/cupstui@latest` build stamps
// nothing, and carries the module version in its build info instead.
func buildVersion(stamped, module string) string {
	if stamped != "dev" {
		return stamped
	}
	// A build straight from the working tree reports "(devel)", which says
	// less than "dev" does.
	if module != "" && module != "(devel)" {
		return module
	}
	return stamped
}

func moduleVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return ""
}

func main() {
	transparent := flag.Bool("transparent", false,
		"do not paint a background; let the terminal show through")
	check := flag.Bool("check", false,
		"report what this machine can and cannot do, then continue")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("cupstui", buildVersion(version, moduleVersion()))
		return
	}

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
