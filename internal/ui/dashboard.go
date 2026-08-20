package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/icortes/cupstui/internal/cups"
)

// dashboardView muestra una tarjeta por impresora y el resumen de la cola.
func dashboardView(snap cups.Snapshot, width int) string {
	if len(snap.Printers) == 0 {
		return styleDim.Render("  No hay impresoras configuradas en este CUPS.")
	}

	cards := make([]string, 0, len(snap.Printers))
	for _, p := range snap.Printers {
		cards = append(cards, printerCard(p, jobsFor(snap.Jobs, p.Name)))
	}

	var b strings.Builder
	b.WriteString(joinCards(cards, width))
	b.WriteString("\n\n")
	b.WriteString(queueSummary(snap))
	return b.String()
}

func printerCard(p cups.Printer, jobs int) string {
	lines := []string{
		styleBold.Render(p.Name),
		stateBadge(p),
	}
	if p.IsDefault {
		lines = append(lines, styleKey.Render("por omisión"))
	}
	if !p.Accepting {
		lines = append(lines, styleWarnText.Render("rechaza trabajos"))
	}
	lines = append(lines, styleLabel.Render("en cola: ")+styleValue.Render(fmt.Sprint(jobs)))

	if msg := problemLine(p); msg != "" {
		lines = append(lines, styleWarnText.Render(truncate(msg, 28)))
	}

	style := styleCard
	if p.State == cups.StateStopped {
		style = styleCard.BorderForeground(colorErr)
	}
	return style.Width(30).Render(strings.Join(lines, "\n"))
}

// joinCards acomoda las tarjetas en filas según el ancho de la terminal.
func joinCards(cards []string, width int) string {
	const cardWidth = 33
	perRow := width / cardWidth
	if perRow < 1 {
		perRow = 1
	}

	var rows []string
	for i := 0; i < len(cards); i += perRow {
		end := i + perRow
		if end > len(cards) {
			end = len(cards)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
	}
	return strings.Join(rows, "\n")
}

func queueSummary(snap cups.Snapshot) string {
	var activos int
	for _, j := range snap.Jobs {
		if j.State == cups.JobProcessing {
			activos++
		}
	}

	total := styleBold.Render(fmt.Sprint(len(snap.Jobs)))
	act := styleBold.Render(fmt.Sprint(activos))
	line := fmt.Sprintf("  %s en cola · %s imprimiendo ahora", total, act)
	if len(snap.Jobs) == 0 {
		line = "  " + styleDim.Render("No hay trabajos en cola.")
	}
	return line
}

func jobsFor(jobs []cups.Job, printer string) int {
	var n int
	for _, j := range jobs {
		if j.Printer == printer {
			n++
		}
	}
	return n
}
