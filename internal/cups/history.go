package cups

import (
	"strconv"
	"strings"
	"time"
)

// HistoryEntry is one finished job, as recorded in the CUPS page log.
type HistoryEntry struct {
	Printer  string
	User     string
	JobID    int
	When     time.Time
	Pages    int
	Billing  string
	Host     string
	Document string
}

// pageLogTime is how cupsd stamps each line: [19/Aug/2026:20:59:42 -0300].
const pageLogTime = "02/Jan/2006:15:04:05 -0700"

// Fixed leading fields of a page log line, before the document name:
// printer, user, job id, two date tokens, page, copies, billing, host.
const pageLogLeading = 9

// Fixed trailing fields, after the document name: media and sides.
const pageLogTrailing = 2

// ParsePageLog reads the lines of the CUPS page log.
//
// cupsd writes one line per printed page and then a "total" summary line for
// the job; only the summary is kept, otherwise every page would be counted
// twice. Lines it cannot read are skipped: the log is append-only and may hold
// entries from other CUPS versions or a customised PageLogFormat.
func ParsePageLog(lines []string) []HistoryEntry {
	var entries []HistoryEntry

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < pageLogLeading+pageLogTrailing {
			continue
		}
		if fields[5] != "total" {
			continue
		}

		jobID, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		pages, err := strconv.Atoi(fields[6])
		if err != nil {
			continue
		}

		stamp := strings.Trim(fields[3]+" "+fields[4], "[]")
		when, err := time.Parse(pageLogTime, stamp)
		if err != nil {
			continue
		}

		// The document name can contain spaces, so it is read as everything
		// between the leading fields and the trailing media and sides.
		document := strings.Join(fields[pageLogLeading:len(fields)-pageLogTrailing], " ")

		entries = append(entries, HistoryEntry{
			Printer:  fields[0],
			User:     fields[1],
			JobID:    jobID,
			When:     when,
			Pages:    pages,
			Billing:  emptyIfDash(fields[7]),
			Host:     fields[8],
			Document: document,
		})
	}
	return entries
}

// emptyIfDash turns the "-" cupsd writes for an absent value into "".
func emptyIfDash(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

// History reads the last entries of the CUPS page log, newest last.
func History(max int) ([]HistoryEntry, error) {
	// One line per page plus a summary, so the window is read generously and
	// trimmed after parsing.
	raw, err := Tail(pageLogPath(), max*8)
	if err != nil {
		return nil, err
	}

	entries := ParsePageLog(raw)
	if len(entries) > max {
		entries = entries[len(entries)-max:]
	}
	return entries, nil
}

func pageLogPath() string {
	for _, l := range LogFiles {
		if l.Name == "page_log" {
			return l.Path
		}
	}
	return "/var/log/cups/page_log"
}

// FilterHistory applies the same query syntax as the queue filter.
func FilterHistory(entries []HistoryEntry, query string) []HistoryEntry {
	terms := parseQuery(query)
	if len(terms) == 0 {
		return entries
	}

	out := make([]HistoryEntry, 0, len(entries))
	for _, e := range entries {
		if historyMatches(e, terms) {
			out = append(out, e)
		}
	}
	return out
}

func historyMatches(e HistoryEntry, terms []term) bool {
	for _, t := range terms {
		if !strings.Contains(historyHaystack(e, t.field), t.text) {
			return false
		}
	}
	return true
}

func historyHaystack(e HistoryEntry, f field) string {
	switch f {
	case fieldUser:
		return strings.ToLower(e.User)
	case fieldPrinter:
		return strings.ToLower(e.Printer)
	case fieldName:
		return strings.ToLower(e.Document)
	case fieldState:
		return "" // finished jobs have no state to match on
	default:
		return strings.ToLower(strings.Join([]string{
			strconv.Itoa(e.JobID), e.User, e.Printer, e.Document, e.Host, e.Billing,
		}, " "))
	}
}

// HistoryTotals counts jobs and pages, for the summary line.
func HistoryTotals(entries []HistoryEntry) (jobs, pages int) {
	for _, e := range entries {
		jobs++
		pages += e.Pages
	}
	return jobs, pages
}
