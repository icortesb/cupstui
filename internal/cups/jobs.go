package cups

import (
	"sort"
	"strings"
	"time"

	ipp "github.com/phin1x/go-ipp"
)

// JobState es el job-state de IPP (RFC 8011 §5.3.7).
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

// Job es un trabajo de impresión en la cola.
type Job struct {
	ID      int
	Name    string
	User    string
	Printer string
	State   JobState
	Created time.Time
	KOctets int
}

func jobFromAttributes(id int, a ipp.Attributes) Job {
	j := Job{
		ID:      id,
		Name:    attrString(a, "job-name"),
		User:    attrString(a, "job-originating-user-name"),
		KOctets: attrInt(a, "job-k-octets"),
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

// printerNameFromURI saca "Epson_L3150" de "ipp://localhost/printers/Epson_L3150".
func printerNameFromURI(uri string) string {
	if uri == "" {
		return ""
	}
	return uri[strings.LastIndex(uri, "/")+1:]
}

// sortJobs deja los trabajos más nuevos arriba.
func sortJobs(jobs []Job) {
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID > jobs[j].ID })
}
