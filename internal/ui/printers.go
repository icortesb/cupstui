package ui

import (
	"fmt"
	"strings"

	"github.com/icortes/cupstui/internal/cups"
)

// printersModel es la vista de impresoras: una lista navegable con acciones.
type printersModel struct {
	cursor int
}

func (p *printersModel) clamp(n int) {
	if p.cursor >= n {
		p.cursor = n - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *printersModel) move(delta, n int) {
	p.cursor += delta
	p.clamp(n)
}

func (p printersModel) selected(printers []cups.Printer) (cups.Printer, bool) {
	if p.cursor < 0 || p.cursor >= len(printers) {
		return cups.Printer{}, false
	}
	return printers[p.cursor], true
}

func (p printersModel) view(printers []cups.Printer, width int) string {
	if len(printers) == 0 {
		return styleDim.Render("  No hay impresoras configuradas en este CUPS.")
	}

	var b strings.Builder
	for i, pr := range printers {
		cursor := "  "
		name := styleBold.Render(pr.Name)
		if i == p.cursor {
			cursor = styleKey.Render("▸ ")
			name = styleAccentText.Bold(true).Render(pr.Name)
		}

		badges := []string{stateBadge(pr)}
		if pr.IsDefault {
			badges = append(badges, styleKey.Render("[por omisión]"))
		}
		if !pr.Accepting {
			badges = append(badges, styleWarnText.Render("[rechaza trabajos]"))
		}

		fmt.Fprintf(&b, "%s%s  %s\n", cursor, name, strings.Join(badges, " "))

		detail := pr.Info
		if detail == "" {
			detail = pr.MakeModel
		}
		if detail != "" {
			fmt.Fprintf(&b, "    %s\n", styleDim.Render(truncate(detail, width-6)))
		}
		if pr.DeviceURI != "" {
			fmt.Fprintf(&b, "    %s\n", styleDim.Render(truncate(pr.DeviceURI, width-6)))
		}
		if msg := problemLine(pr); msg != "" {
			fmt.Fprintf(&b, "    %s\n", styleWarnText.Render(truncate(msg, width-6)))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func stateBadge(p cups.Printer) string {
	style := styleDim
	symbol := "○"
	switch p.State {
	case cups.StateIdle:
		style, symbol = styleOKText, "●"
	case cups.StatePrinting:
		style, symbol = styleAccentText, "◐"
	case cups.StateStopped:
		style, symbol = styleErrText, "■"
	}
	return style.Render(symbol + " " + p.State.String())
}

// problemLine junta el mensaje de estado con los motivos que reporta CUPS.
func problemLine(p cups.Printer) string {
	var parts []string
	if p.StateMessage != "" {
		parts = append(parts, p.StateMessage)
	}
	if len(p.Reasons) > 0 {
		parts = append(parts, strings.Join(p.Reasons, ", "))
	}
	return strings.Join(parts, " · ")
}

func truncate(s string, max int) string {
	if max < 4 {
		max = 4
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
