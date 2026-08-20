package cups

import (
	"testing"
	"time"
)

func TestJobFromAttributes(t *testing.T) {
	j := jobFromAttributes(42, attrs(map[string]interface{}{
		"job-name":                  "informe.pdf",
		"job-originating-user-name": "icortes",
		"job-printer-uri":           "ipp://localhost/printers/Epson_L3150",
		"job-state":                 5,
		"time-at-creation":          1755640000,
		"job-k-octets":              128,
	}))

	if j.ID != 42 {
		t.Errorf("ID = %d, want 42", j.ID)
	}
	if j.Name != "informe.pdf" {
		t.Errorf("Name = %q", j.Name)
	}
	if j.User != "icortes" {
		t.Errorf("User = %q", j.User)
	}
	if j.Printer != "Epson_L3150" {
		t.Errorf("Printer = %q, want the name taken from the URI", j.Printer)
	}
	if j.State != JobProcessing {
		t.Errorf("State = %v, want JobProcessing", j.State)
	}
	if !j.Created.Equal(time.Unix(1755640000, 0)) {
		t.Errorf("Created = %v", j.Created)
	}
	if j.KOctets != 128 {
		t.Errorf("KOctets = %d", j.KOctets)
	}
}

func TestJobStateMapping(t *testing.T) {
	cases := []struct {
		value int
		want  JobState
		label string
	}{
		{3, JobPending, "pending"},
		{4, JobHeld, "held"},
		{5, JobProcessing, "printing"},
		{6, JobStopped, "stopped"},
		{7, JobCanceled, "canceled"},
		{8, JobAborted, "aborted"},
		{9, JobCompleted, "completed"},
		{99, JobUnknown, "unknown"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			j := jobFromAttributes(1, attrs(map[string]interface{}{"job-state": c.value}))
			if j.State != c.want {
				t.Errorf("State = %v, want %v", j.State, c.want)
			}
			if j.State.String() != c.label {
				t.Errorf("String() = %q, want %q", j.State.String(), c.label)
			}
		})
	}
}

func TestJobPrinterFallsBackToPrinterURI(t *testing.T) {
	j := jobFromAttributes(1, attrs(map[string]interface{}{
		"printer-uri": "ipp://localhost:631/printers/HP_LaserJet",
	}))
	if j.Printer != "HP_LaserJet" {
		t.Errorf("Printer = %q, want HP_LaserJet", j.Printer)
	}
}

func TestSortJobsPutsNewestFirst(t *testing.T) {
	jobs := []Job{{ID: 7}, {ID: 42}, {ID: 13}}
	sortJobs(jobs)
	want := []int{42, 13, 7}
	for i, id := range want {
		if jobs[i].ID != id {
			t.Fatalf("order = %v, want %v", []Job{jobs[0], jobs[1], jobs[2]}, want)
		}
	}
}
