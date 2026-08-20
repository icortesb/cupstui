package ui

import (
	"strings"
	"testing"

	"github.com/icortesb/cupstui/internal/cups"
)

func TestCycleLevelWalksThroughTheLevelsAndBack(t *testing.T) {
	l := newLogs()
	want := []cups.Severity{cups.SeverityInfo, cups.SeverityWarning, cups.SeverityError, cups.SeverityNone}
	for i, w := range want {
		l.cycleLevel()
		if l.min != w {
			t.Fatalf("cycle %d = %v, want %v", i+1, l.min, w)
		}
	}
}

func TestAtLeastHidesWhatIsBelowTheFloor(t *testing.T) {
	lines := []string{
		"D [19/Aug/2026:22:05:28 -0300] debug chatter",
		"I [19/Aug/2026:22:05:28 -0300] routine",
		"W [19/Aug/2026:22:05:28 -0300] a warning",
		"E [19/Aug/2026:22:05:28 -0300] an error",
	}

	got := atLeast(lines, cups.SeverityWarning)
	if len(got) != 2 {
		t.Fatalf("atLeast = %v, want the warning and the error", got)
	}
	if len(atLeast(lines, cups.SeverityNone)) != 4 {
		t.Error("SeverityNone should hide nothing")
	}
}

func TestAtLeastKeepsLinesThatCarryNoLevel(t *testing.T) {
	lines := []string{`localhost - - [19/Aug/2026:22:05:28 -0300] "POST / HTTP/1.1" 200 362`}
	if len(atLeast(lines, cups.SeverityError)) != 1 {
		t.Error("access_log has no level to compare, so it must survive the filter")
	}
}

func TestLogsViewSaysWhenTheFilterIsWhatEmptiedIt(t *testing.T) {
	l := newLogs()
	l.setSize(80, 10)
	l.setLines([]string{"I [19/Aug/2026:22:05:28 -0300] routine"}, nil)
	l.min = cups.SeverityError

	view := l.view(80)
	if !strings.Contains(view, "press v") {
		t.Errorf("view = %q, want it to point at the level key", view)
	}
}

func TestLogsViewNamesTheLevelInTheHeader(t *testing.T) {
	l := newLogs()
	l.setSize(80, 10)
	l.setLines([]string{"E [19/Aug/2026:22:05:28 -0300] an error"}, nil)
	l.min = cups.SeverityWarning

	if !strings.Contains(l.view(80), "warnings+") {
		t.Error("the header has to say the log is being filtered")
	}
}
