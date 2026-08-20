package cups

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// csvHeader names the columns of the usage report.
var csvHeader = []string{"date", "user", "printer", "document", "pages", "job_id", "host", "billing"}

// ExportCSV writes a usage report. The header is written even for an empty
// report, so the file can be opened by a spreadsheet either way.
func ExportCSV(path string, entries []HistoryEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return classifyFileError(err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(csvHeader); err != nil {
		return classifyFileError(err)
	}

	for _, e := range entries {
		row := []string{
			e.When.Format(time.RFC3339),
			e.User,
			e.Printer,
			e.Document,
			strconv.Itoa(e.Pages),
			strconv.Itoa(e.JobID),
			e.Host,
			e.Billing,
		}
		if err := w.Write(row); err != nil {
			return classifyFileError(err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return classifyFileError(err)
	}
	return f.Close()
}

// DefaultExportPath is where a report goes when no path is given: the user's
// home directory, named for the day it was produced.
func DefaultExportPath(now time.Time) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", classifyFileError(err)
	}
	name := "cupstui-usage-" + now.Format("2006-01-02") + ".csv"
	return filepath.Join(home, name), nil
}
