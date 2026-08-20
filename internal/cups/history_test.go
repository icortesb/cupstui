package cups

import (
	"testing"
	"time"
)

const sampleLog = `Epson_L3150 icortes 5 [19/Aug/2026:20:59:42 -0300] total 0 - localhost testprint - -
Epson_WiFi ana 3 [19/Aug/2026:20:20:50 -0300] total 1 - localhost Test Page - -
HP_LaserJet bob 7 [18/Aug/2026:09:05:00 -0300] total 12 acct-42 workstation informe anual.pdf na_letter_8.5x11in two-sided-long-edge`

func TestParsePageLogReadsEveryField(t *testing.T) {
	entries := ParsePageLog(lines(sampleLog))
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	e := entries[2]
	if e.Printer != "HP_LaserJet" {
		t.Errorf("Printer = %q", e.Printer)
	}
	if e.User != "bob" {
		t.Errorf("User = %q", e.User)
	}
	if e.JobID != 7 {
		t.Errorf("JobID = %d", e.JobID)
	}
	if e.Pages != 12 {
		t.Errorf("Pages = %d", e.Pages)
	}
	if e.Host != "workstation" {
		t.Errorf("Host = %q", e.Host)
	}
	if e.Billing != "acct-42" {
		t.Errorf("Billing = %q", e.Billing)
	}
	want := time.Date(2026, time.August, 18, 9, 5, 0, 0, time.FixedZone("", -3*3600))
	if !e.When.Equal(want) {
		t.Errorf("When = %v, want %v", e.When, want)
	}
}

func TestParsePageLogKeepsDocumentNamesWithSpaces(t *testing.T) {
	// The document name sits between fixed leading fields and the trailing
	// media and sides, so it cannot be read by position from the left alone.
	entries := ParsePageLog(lines(sampleLog))
	if got := entries[1].Document; got != "Test Page" {
		t.Errorf("Document = %q, want \"Test Page\"", got)
	}
	if got := entries[2].Document; got != "informe anual.pdf" {
		t.Errorf("Document = %q, want \"informe anual.pdf\"", got)
	}
}

func TestParsePageLogCountsOnlyTotalLines(t *testing.T) {
	// CUPS writes one line per page plus a "total" summary. Counting both would
	// report every page twice.
	perPage := `Epson_L3150 icortes 9 [19/Aug/2026:21:00:00 -0300] 1 1 - localhost doc.pdf - -
Epson_L3150 icortes 9 [19/Aug/2026:21:00:01 -0300] 2 1 - localhost doc.pdf - -
Epson_L3150 icortes 9 [19/Aug/2026:21:00:02 -0300] total 2 - localhost doc.pdf - -`

	entries := ParsePageLog(lines(perPage))
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want only the total line", len(entries))
	}
	if entries[0].Pages != 2 {
		t.Errorf("Pages = %d, want 2", entries[0].Pages)
	}
}

func TestParsePageLogSkipsUnreadableLines(t *testing.T) {
	mixed := `esto no es una línea de page_log
Epson_L3150 icortes 5 [19/Aug/2026:20:59:42 -0300] total 3 - localhost ok.pdf - -

Epson_L3150 icortes noesnumero [19/Aug/2026:20:59:42 -0300] total 3 - localhost x.pdf - -`

	entries := ParsePageLog(lines(mixed))
	if len(entries) != 1 || entries[0].Document != "ok.pdf" {
		t.Errorf("got %+v, want only the readable line", entries)
	}
}

func TestParsePageLogReportsCancelledJobsWithZeroPages(t *testing.T) {
	entries := ParsePageLog(lines(sampleLog))
	if entries[0].Pages != 0 {
		t.Errorf("Pages = %d, want 0 for a job that printed nothing", entries[0].Pages)
	}
}

func TestFilterHistoryMatchesEveryColumn(t *testing.T) {
	entries := ParsePageLog(lines(sampleLog))
	cases := []struct {
		query string
		want  int
	}{
		{"", 3},
		{"bob", 1},
		{"user:ana", 1},
		{"printer:epson", 2},
		{"document:informe", 1},
		{"user:bob printer:hp", 1},
		{"user:bob printer:epson", 0},
		{"nada", 0},
	}
	for _, c := range cases {
		if got := len(FilterHistory(entries, c.query)); got != c.want {
			t.Errorf("FilterHistory(%q) = %d entries, want %d", c.query, got, c.want)
		}
	}
}

func TestHistoryTotalsSummarisePagesAndJobs(t *testing.T) {
	entries := ParsePageLog(lines(sampleLog))
	jobs, pages := HistoryTotals(entries)
	if jobs != 3 {
		t.Errorf("jobs = %d, want 3", jobs)
	}
	if pages != 13 {
		t.Errorf("pages = %d, want 13", pages)
	}
}

func lines(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}
