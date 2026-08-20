package cups

import (
	"context"
	"strings"
	"testing"

	ipp "github.com/phin1x/go-ipp"
)

func diagnosisPrinterAttrs(state int, accepting bool, makeModel string, reasons ...string) ipp.Attributes {
	a := attrs(map[string]interface{}{
		"printer-name":              "Epson_L3150",
		"printer-state":             state,
		"printer-is-accepting-jobs": accepting,
		"printer-make-and-model":    makeModel,
	})
	for _, r := range reasons {
		a["printer-state-reasons"] = append(a["printer-state-reasons"], ipp.Attribute{Name: "printer-state-reasons", Value: r})
	}
	return a
}

func diagnosisJobAttrs(id, state int, reasons ...string) ipp.Attributes {
	a := attrs(map[string]interface{}{
		"job-id":    id,
		"job-state": state,
	})
	for _, r := range reasons {
		a["job-state-reasons"] = append(a["job-state-reasons"], ipp.Attribute{Name: "job-state-reasons", Value: r})
	}
	return a
}

// diagnosisAdapter answers both the printer list and the recent-jobs query.
// Snapshot's own jobs query shares an IPP operation code with RecentJobs, so
// both get the same canned job list — harmless, since no check reads
// Snapshot.Jobs.
func diagnosisAdapter(printer ipp.Attributes, jobs ...ipp.Attributes) *fakeAdapter {
	return &fakeAdapter{responses: map[int16]*ipp.Response{
		ipp.OperationCupsGetPrinters: {PrinterAttributes: []ipp.Attributes{printer}},
		ipp.OperationGetJobs:         {JobAttributes: jobs},
	}}
}

func findCheck(t *testing.T, checks []CheckResult, name string) CheckResult {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %q check in %v", name, checks)
	return CheckResult{}
}

func TestDiagnosePrinterHealthy(t *testing.T) {
	f := diagnosisAdapter(
		diagnosisPrinterAttrs(3, true, "EPSON L3150 Series"),
		diagnosisJobAttrs(1, 9), // completed
	)

	d, err := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	if err != nil {
		t.Fatalf("DiagnosePrinter: %v", err)
	}
	if d.Overall != CheckOK {
		t.Fatalf("Overall = %v, want CheckOK; checks: %+v", d.Overall, d.Checks)
	}
	for _, name := range []string{checkPrinterExists, checkQueue, checkAccepting, checkConnection, checkPrinterState, checkDriverName, checkRecentJobs} {
		if c := findCheck(t, d.Checks, name); c.Status != CheckOK {
			t.Errorf("%s = %v, want CheckOK (%s)", name, c.Status, c.Detail)
		}
	}
}

func TestDiagnosePrinterReportsStoppedQueue(t *testing.T) {
	f := diagnosisAdapter(diagnosisPrinterAttrs(5, true, "EPSON L3150 Series"))

	d, err := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	if err != nil {
		t.Fatalf("DiagnosePrinter: %v", err)
	}
	if c := findCheck(t, d.Checks, checkQueue); c.Status != CheckFail {
		t.Errorf("Queue = %v, want CheckFail", c.Status)
	}
	if d.Overall != CheckFail {
		t.Errorf("Overall = %v, want CheckFail", d.Overall)
	}
}

func TestDiagnosePrinterReportsNotAccepting(t *testing.T) {
	f := diagnosisAdapter(diagnosisPrinterAttrs(3, false, "EPSON L3150 Series"))

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	if c := findCheck(t, d.Checks, checkAccepting); c.Status != CheckWarn {
		t.Errorf("Accepting jobs = %v, want CheckWarn", c.Status)
	}
}

func TestDiagnosePrinterHumanizesStateReasons(t *testing.T) {
	f := diagnosisAdapter(diagnosisPrinterAttrs(3, true, "EPSON L3150 Series", "media-empty-warning"))

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	c := findCheck(t, d.Checks, checkPrinterState)
	if c.Status != CheckWarn {
		t.Errorf("Printer state = %v, want CheckWarn", c.Status)
	}
	if c.Detail != "out of paper" {
		t.Errorf("Detail = %q, want a plain explanation, not the raw reason code", c.Detail)
	}
}

func TestDiagnosePrinterTreatsOfflineAsAConnectionFailure(t *testing.T) {
	f := diagnosisAdapter(diagnosisPrinterAttrs(3, true, "EPSON L3150 Series", "offline-report"))

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	c := findCheck(t, d.Checks, checkConnection)
	if c.Status != CheckFail {
		t.Errorf("Device connection = %v, want CheckFail", c.Status)
	}
	// The offline reason must not also be repeated under Printer state.
	if state := findCheck(t, d.Checks, checkPrinterState); state.Status != CheckOK {
		t.Errorf("Printer state = %v, want CheckOK (offline-report belongs to Connection only)", state.Status)
	}
}

func TestDiagnosePrinterDetectsMissingFilterOnThePrinter(t *testing.T) {
	f := diagnosisAdapter(diagnosisPrinterAttrs(3, true, "EPSON L3150 Series", "cups-missing-filter-error"))

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	if c := findCheck(t, d.Checks, checkDriverName); c.Status != CheckFail {
		t.Errorf("Driver = %v, want CheckFail", c.Status)
	}
}

func TestDiagnosePrinterDetectsMissingFilterOnTheLastJob(t *testing.T) {
	f := diagnosisAdapter(
		diagnosisPrinterAttrs(3, true, "EPSON L3150 Series"),
		diagnosisJobAttrs(9, 8, "cups-missing-filter-error"), // aborted
	)

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	if c := findCheck(t, d.Checks, checkDriverName); c.Status != CheckFail {
		t.Errorf("Driver = %v, want CheckFail", c.Status)
	}
	if c := findCheck(t, d.Checks, checkRecentJobs); c.Status != CheckFail {
		t.Errorf("Recent jobs = %v, want CheckFail", c.Status)
	}
}

func TestDiagnosePrinterTreatsUserCancellationAsFine(t *testing.T) {
	f := diagnosisAdapter(
		diagnosisPrinterAttrs(3, true, "EPSON L3150 Series"),
		diagnosisJobAttrs(9, 7, "job-canceled-by-user"), // canceled
	)

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	c := findCheck(t, d.Checks, checkRecentJobs)
	if c.Status != CheckOK {
		t.Errorf("Recent jobs = %v, want CheckOK", c.Status)
	}
	if !strings.Contains(c.Detail, "canceled by the user") {
		t.Errorf("Detail = %q, want it to say the user canceled it", c.Detail)
	}
}

func TestDiagnosePrinterHandlesNoRecentJobs(t *testing.T) {
	f := diagnosisAdapter(diagnosisPrinterAttrs(3, true, "EPSON L3150 Series"))

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	c := findCheck(t, d.Checks, checkRecentJobs)
	if c.Status != CheckOK {
		t.Errorf("Recent jobs = %v, want CheckOK when there is nothing to report", c.Status)
	}
}

func TestDiagnosePrinterReportsAPrinterThatNoLongerExists(t *testing.T) {
	f := &fakeAdapter{responses: map[int16]*ipp.Response{
		ipp.OperationCupsGetPrinters: {PrinterAttributes: []ipp.Attributes{}},
	}}

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	if c := findCheck(t, d.Checks, checkPrinterExists); c.Status != CheckFail {
		t.Errorf("Printer = %v, want CheckFail", c.Status)
	}
}

func TestDiagnosePrinterDegradesGracefullyWhenCupsIsUnreachable(t *testing.T) {
	f := &fakeAdapter{err: ipp.SocketNotFoundError}

	d, _ := DiagnosePrinter(context.Background(), newTestClient(f), "Epson_L3150")
	if c := findCheck(t, d.Checks, checkDaemon); c.Status != CheckFail {
		t.Errorf("CUPS service = %v, want CheckFail", c.Status)
	}
	for _, name := range []string{checkPrinterExists, checkQueue, checkAccepting, checkConnection, checkPrinterState, checkDriverName, checkRecentJobs} {
		c := findCheck(t, d.Checks, name)
		if c.Status != CheckWarn {
			t.Errorf("%s = %v, want CheckWarn (skipped), got detail %q", name, c.Status, c.Detail)
		}
	}
	if d.Overall != CheckFail {
		t.Errorf("Overall = %v, want CheckFail", d.Overall)
	}
}
