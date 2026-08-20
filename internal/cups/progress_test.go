package cups

import "testing"

func TestJobReadsSheetCounts(t *testing.T) {
	j := jobFromAttributes(42, attrs(map[string]interface{}{
		"job-media-sheets":           10,
		"job-media-sheets-completed": 3,
	}))
	if j.Sheets != 10 {
		t.Errorf("Sheets = %d, want 10", j.Sheets)
	}
	if j.SheetsDone != 3 {
		t.Errorf("SheetsDone = %d, want 3", j.SheetsDone)
	}
}

func TestJobProgressIsAFractionOfTheWork(t *testing.T) {
	cases := []struct {
		name      string
		sheets    int
		done      int
		state     JobState
		want      float64
		wantKnown bool
	}{
		{"halfway", 10, 5, JobProcessing, 0.5, true},
		{"just started", 10, 0, JobProcessing, 0, true},
		{"finished", 10, 10, JobProcessing, 1, true},
		{"total unknown", 0, 3, JobProcessing, 0, false},
		{"not printing yet", 10, 0, JobPending, 0, false},
		{"more done than expected", 10, 12, JobProcessing, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			j := Job{Sheets: c.sheets, SheetsDone: c.done, State: c.state}
			got, known := j.Progress()
			if known != c.wantKnown {
				t.Fatalf("known = %v, want %v", known, c.wantKnown)
			}
			if known && got != c.want {
				t.Errorf("Progress = %v, want %v", got, c.want)
			}
		})
	}
}

func TestLogSeverityIsReadFromTheLineTag(t *testing.T) {
	cases := []struct {
		line string
		want Severity
	}{
		{"E [19/Aug/2026:22:05:28 -0300] Client 285 failed", SeverityError},
		{"W [19/Aug/2026:22:05:28 -0300] CreateProfile failed", SeverityWarning},
		{"I [19/Aug/2026:22:05:28 -0300] Listening", SeverityInfo},
		{"D [19/Aug/2026:22:05:28 -0300] noisy detail", SeverityDebug},
		{"localhost - - [19/Aug/2026] \"POST / HTTP/1.1\" 200", SeverityNone},
		{"", SeverityNone},
		{"Epson_L3150 icortes 5 [19/Aug/2026] total 0", SeverityNone},
	}
	for _, c := range cases {
		if got := LineSeverity(c.line); got != c.want {
			t.Errorf("LineSeverity(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestUsageByUserAddsUpPagesAndJobs(t *testing.T) {
	entries := []HistoryEntry{
		{User: "ana", Printer: "HP", Pages: 3},
		{User: "bob", Printer: "HP", Pages: 10},
		{User: "ana", Printer: "Epson", Pages: 4},
	}

	byUser := UsageByUser(entries)
	if len(byUser) != 2 {
		t.Fatalf("got %d users, want 2", len(byUser))
	}
	// Heaviest user first, so the interesting row is the one on top.
	if byUser[0].Name != "bob" || byUser[0].Pages != 10 {
		t.Errorf("first row = %+v, want bob with 10 pages", byUser[0])
	}
	if byUser[1].Name != "ana" || byUser[1].Pages != 7 || byUser[1].Jobs != 2 {
		t.Errorf("second row = %+v, want ana with 7 pages over 2 jobs", byUser[1])
	}
}

func TestUsageByPrinterAddsUpPagesAndJobs(t *testing.T) {
	entries := []HistoryEntry{
		{User: "ana", Printer: "HP", Pages: 3},
		{User: "bob", Printer: "HP", Pages: 10},
		{User: "ana", Printer: "Epson", Pages: 4},
	}

	byPrinter := UsageByPrinter(entries)
	if len(byPrinter) != 2 {
		t.Fatalf("got %d printers, want 2", len(byPrinter))
	}
	if byPrinter[0].Name != "HP" || byPrinter[0].Pages != 13 || byPrinter[0].Jobs != 2 {
		t.Errorf("first row = %+v, want HP with 13 pages over 2 jobs", byPrinter[0])
	}
}

func TestUsageBreaksTiesByNameSoTheOrderIsStable(t *testing.T) {
	entries := []HistoryEntry{
		{User: "zoe", Pages: 5},
		{User: "adam", Pages: 5},
	}
	got := UsageByUser(entries)
	if got[0].Name != "adam" {
		t.Errorf("ties should sort by name, got %+v", got)
	}
}
