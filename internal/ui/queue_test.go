package ui

import (
	"strings"
	"testing"

	"github.com/icortes/cupstui/internal/cups"
)

func TestJobStateShowsProgressOnlyWhilePrinting(t *testing.T) {
	cases := []struct {
		name string
		job  cups.Job
		want string
	}{
		{
			"printing with a known total",
			cups.Job{State: cups.JobProcessing, Sheets: 10, SheetsDone: 5},
			"50%",
		},
		{
			// CUPS reports no total until the job clears the filters; a bar
			// drawn from a missing total would be invented.
			"printing with no total yet",
			cups.Job{State: cups.JobProcessing},
			"printing",
		},
		{"held", cups.Job{State: cups.JobHeld, Sheets: 10}, "held"},
		{"pending", cups.Job{State: cups.JobPending, Sheets: 10}, "pending"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := jobState(c.job); !strings.Contains(got, c.want) {
				t.Errorf("jobState = %q, want it to contain %q", got, c.want)
			}
		})
	}
}

func TestMiniBarFillsInProportion(t *testing.T) {
	cases := []struct {
		fraction float64
		want     string
	}{
		{0, "░░░░"},
		{0.5, "██░░"},
		{1, "████"},
		{1.5, "████"}, // more done than expected must not overflow the cell
	}
	for _, c := range cases {
		if got := miniBar(c.fraction, 4); got != c.want {
			t.Errorf("miniBar(%v) = %q, want %q", c.fraction, got, c.want)
		}
	}
}

func TestMiniBarKeepsItsWidth(t *testing.T) {
	// The bar sits in a fixed table cell; a bar of the wrong width would shift
	// every column after it.
	for _, f := range []float64{0, 0.13, 0.5, 0.87, 1} {
		if got := len([]rune(miniBar(f, 5))); got != 5 {
			t.Errorf("miniBar(%v) is %d cells wide, want 5", f, got)
		}
	}
}
