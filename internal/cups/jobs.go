package cups

import (
	"sort"
	"strings"
	"time"

	ipp "github.com/phin1x/go-ipp"
)

// JobState is the IPP job-state (RFC 8011 §5.3.7).
type JobState int

const (
	JobUnknown JobState = iota
	JobPending
	JobHeld
	JobProcessing
	JobStopped
	JobCanceled
	JobAborted
	JobCompleted
)

func (s JobState) String() string {
	switch s {
	case JobPending:
		return "pending"
	case JobHeld:
		return "held"
	case JobProcessing:
		return "printing"
	case JobStopped:
		return "stopped"
	case JobCanceled:
		return "canceled"
	case JobAborted:
		return "aborted"
	case JobCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

// Job is a print job on the queue.
type Job struct {
	ID      int
	Name    string
	User    string
	Printer string
	State   JobState
	Created time.Time
	KOctets int
	// Sheets is what the job is expected to print, SheetsDone what it has
	// printed so far. CUPS reports both only once the job reaches the printer.
	Sheets     int
	SheetsDone int
	// Reasons is job-state-reasons, cupsd's explanation for the state above —
	// "job-canceled-by-user", "cups-missing-filter-error" and the like.
	Reasons []string
}

// Progress is how far along a printing job is, and whether that is known at
// all: CUPS reports no sheet total until the job has been through the filters,
// and a bar drawn from a missing total would be a lie.
func (j Job) Progress() (float64, bool) {
	if j.State != JobProcessing || j.Sheets <= 0 {
		return 0, false
	}
	done := j.SheetsDone
	if done > j.Sheets {
		done = j.Sheets
	}
	return float64(done) / float64(j.Sheets), true
}

func jobFromAttributes(id int, a ipp.Attributes) Job {
	j := Job{
		ID:         id,
		Name:       attrString(a, "job-name"),
		User:       attrString(a, "job-originating-user-name"),
		KOctets:    attrInt(a, "job-k-octets"),
		Sheets:     attrInt(a, "job-media-sheets"),
		SheetsDone: attrInt(a, "job-media-sheets-completed"),
	}

	for _, r := range attrStrings(a, "job-state-reasons") {
		// CUPS sends "none" when there is nothing to report.
		if r != "" && r != "none" {
			j.Reasons = append(j.Reasons, r)
		}
	}

	uri := attrString(a, "job-printer-uri")
	if uri == "" {
		uri = attrString(a, "printer-uri")
	}
	j.Printer = printerNameFromURI(uri)

	switch attrInt(a, "job-state") {
	case 3:
		j.State = JobPending
	case 4:
		j.State = JobHeld
	case 5:
		j.State = JobProcessing
	case 6:
		j.State = JobStopped
	case 7:
		j.State = JobCanceled
	case 8:
		j.State = JobAborted
	case 9:
		j.State = JobCompleted
	}

	if ts := attrInt(a, "time-at-creation"); ts > 0 {
		j.Created = time.Unix(int64(ts), 0)
	}
	return j
}

// printerNameFromURI takes "Epson_L3150" out of "ipp://localhost/printers/Epson_L3150".
func printerNameFromURI(uri string) string {
	if uri == "" {
		return ""
	}
	return uri[strings.LastIndex(uri, "/")+1:]
}

// sortJobs puts the newest jobs on top.
func sortJobs(jobs []Job) {
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID > jobs[j].ID })
}
