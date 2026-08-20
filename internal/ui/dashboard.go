package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/icortes/cupstui/internal/cups"
)

// dashboardView muestra una tarjeta por impresora y el resumen de la cola.
func dashboardView(snap cups.Snapshot, width int) string {
	if len(snap.Printers) == 0 {
		return styleDim.Render("  No printers configured on this CUPS server.")
	}

	cards := make([]string, 0, len(snap.Printers))
	for _, p := range snap.Printers {
		cards = append(cards, printerCard(p, jobsFor(snap.Jobs, p.Name)))
	}

	var b strings.Builder
	b.WriteString(overview(snap, width))
	b.WriteString("\n\n")
	b.WriteString(joinCards(cards, width))
	return b.String()
}

// overview is the line an operator reads first: whether anything needs
// attention, and how much work is waiting.
func overview(snap cups.Snapshot, width int) string {
	var stopped, rejecting, printing int
	for _, p := range snap.Printers {
		if p.State == cups.StateStopped {
			stopped++
		}
		if !p.Accepting {
			rejecting++
		}
		if p.State == cups.StatePrinting {
			printing++
		}
	}

	items := []string{
		stat(len(snap.Printers), "printer", styleValue),
		stat(len(snap.Jobs), "job queued", styleValue),
	}
	if printing > 0 {
		items = append(items, stat(printing, "printing", styleOKText))
	}
	if stopped > 0 {
		items = append(items, stat(stopped, "stopped", styleErrText))
	}
	if rejecting > 0 {
		items = append(items, stat(rejecting, "rejecting jobs", styleWarnText))
	}

	return "  " + strings.Join(items, styleDim.Render("   "))
}

func stat(value int, label string, style lipgloss.Style) string {
	return style.Bold(true).Render(strconv.Itoa(value)) + " " + styleDim.Render(plural(value, label))
}

// plural adds the s to a counted noun, on the last word so that "job queued"
// becomes "jobs queued" rather than "job queueds".
func plural(n int, label string) string {
	if n == 1 {
		return label
	}
	words := strings.SplitN(label, " ", 2)
	words[0] += "s"
	return strings.Join(words, " ")
}

func printerCard(p cups.Printer, jobs int) string {
	// Name and state share the top line so the card reads as one unit rather
	// than a stack of unrelated facts.
	head := styleBold.Render(truncate(p.Name, 18)) + "  " + stateBadge(p)

	var tags []string
	if p.IsDefault {
		tags = append(tags, styleKey.Render("default"))
	}
	if !p.Accepting {
		tags = append(tags, styleWarnText.Render("rejecting"))
	}

	lines := []string{head}
	if len(tags) > 0 {
		lines = append(lines, strings.Join(tags, styleDim.Render(" · ")))
	}

	detail := p.Info
	if detail == "" {
		detail = p.MakeModel
	}
	if detail != "" {
		lines = append(lines, styleDim.Render(truncate(detail, 28)))
	}

	queued := styleDim.Render("idle queue")
	if jobs > 0 {
		queued = styleValue.Bold(true).Render(fmt.Sprint(jobs)) +
			styleDim.Render(" "+plural(jobs, "job")+" waiting")
	}
	lines = append(lines, queued)

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

func jobsFor(jobs []cups.Job, printer string) int {
	var n int
	for _, j := range jobs {
		if j.Printer == printer {
			n++
		}
	}
	return n
}
