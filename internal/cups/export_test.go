package cups

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportCSVWritesAHeaderAndEveryEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.csv")
	entries := []HistoryEntry{
		{Printer: "HP", User: "bob", JobID: 7, When: time.Unix(1755640000, 0), Pages: 12, Document: "report.pdf", Host: "ws", Billing: "acct-42"},
		{Printer: "Epson", User: "ana", JobID: 8, When: time.Unix(1755640100, 0), Pages: 1, Document: "note.txt", Host: "localhost"},
	}

	if err := ExportCSV(path, entries); err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("the file is not valid CSV: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want a header plus two entries", len(rows))
	}
	if rows[0][0] != "date" || rows[0][1] != "user" {
		t.Errorf("header = %v", rows[0])
	}
	if rows[1][1] != "bob" || rows[1][4] != "12" {
		t.Errorf("first entry = %v", rows[1])
	}
}

func TestExportCSVQuotesFieldsThatNeedIt(t *testing.T) {
	// Document names carry commas and quotes; written raw they would shift
	// every later column of the row.
	path := filepath.Join(t.TempDir(), "usage.csv")
	entries := []HistoryEntry{
		{Printer: "HP", User: "bob", Document: `invoice, "final".pdf`, Pages: 1, When: time.Unix(1755640000, 0)},
	}

	if err := ExportCSV(path, entries); err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}

	f, _ := os.Open(path)
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("the file is not valid CSV: %v", err)
	}
	if got := rows[1][3]; got != `invoice, "final".pdf` {
		t.Errorf("document = %q, want it to survive the round trip", got)
	}
}

func TestExportCSVOnAnEmptyReportStillWritesTheHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.csv")
	if err := ExportCSV(path, nil); err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "date,user,printer,document,pages") {
		t.Errorf("file = %q", data)
	}
}

func TestExportCSVReportsAnUnwritablePath(t *testing.T) {
	err := ExportCSV(filepath.Join(t.TempDir(), "no", "such", "dir", "x.csv"), nil)
	if err == nil {
		t.Fatal("want an error")
	}
	var cerr *Error
	if !asError(err, &cerr) {
		t.Errorf("want a classified error, got %T", err)
	}
}

func TestExportPathIsDatedAndUnderTheHomeDirectory(t *testing.T) {
	path, err := DefaultExportPath(time.Date(2026, time.August, 19, 22, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DefaultExportPath: %v", err)
	}
	if got := filepath.Base(path); got != "cupstui-usage-2026-08-19.csv" {
		t.Errorf("file name = %q", got)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path = %q, want an absolute path", path)
	}
}
