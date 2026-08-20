package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/icortesb/cupstui/internal/cups"
)

func TestUsageTableStaysInsideItsColumn(t *testing.T) {
	// Two of these sit side by side, so a row wider than its half wraps and the
	// totals come apart on a narrow terminal.
	rows := []cups.Usage{
		{Name: "a-very-long-user-name-indeed", Jobs: 10, Pages: 300},
		{Name: "bob", Jobs: 1, Pages: 1},
	}

	for _, width := range []int{30, 44, 60} {
		out := usageTable("BY USER", rows, 301, width)
		for i, line := range strings.Split(out, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Errorf("width %d: line %d is %d wide: %q", width, i, got, line)
			}
		}
	}
}

func TestSummaryViewStacksOnANarrowTerminal(t *testing.T) {
	// Side by side, each column gets half the screen; below a point that is not
	// enough for a name, a bar and the numbers, so they go one above the other.
	h := newHistory()
	h.summary = true
	h.setEntries([]cups.HistoryEntry{
		{User: "icortes", Printer: "Epson_L3150", Pages: 3},
	}, nil)

	for _, width := range []int{60, 80, 120} {
		h.setSize(width, 20)
		for i, line := range strings.Split(h.view(), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Errorf("width %d: line %d is %d wide: %q", width, i, got, line)
			}
		}
	}
}
