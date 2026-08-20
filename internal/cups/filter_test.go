package cups

import "testing"

func sampleJobs() []Job {
	return []Job{
		{ID: 42, Name: "informe anual.pdf", User: "icortes", Printer: "Epson_L3150", State: JobProcessing},
		{ID: 43, Name: "factura.pdf", User: "ana", Printer: "HP_LaserJet", State: JobPending},
		{ID: 44, Name: "recibo.txt", User: "ana", Printer: "Epson_L3150", State: JobHeld},
	}
}

func ids(jobs []Job) []int {
	out := make([]int, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.ID)
	}
	return out
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFilterJobs(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  []int
	}{
		{"an empty query returns everything", "", []int{42, 43, 44}},
		{"by user", "ana", []int{43, 44}},
		{"by printer", "epson", []int{42, 44}},
		{"by document", "factura", []int{43}},
		{"by state", "held", []int{44}},
		{"by id", "42", []int{42}},
		{"no match", "zzz", []int{}},
		{"case is ignored", "ICORTES", []int{42}},
		{"surrounding spaces are ignored", "  ana  ", []int{43, 44}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ids(FilterJobs(sampleJobs(), c.query))
			if !equal(got, c.want) {
				t.Errorf("FilterJobs(%q) = %v, want %v", c.query, got, c.want)
			}
		})
	}
}

func TestFilterJobsPreservesOrder(t *testing.T) {
	if got := ids(FilterJobs(sampleJobs(), "epson")); !equal(got, []int{42, 44}) {
		t.Errorf("order = %v, want %v", got, []int{42, 44})
	}
}

func TestFilterJobsWithFieldQualifiers(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  []int
	}{
		{"by printer", "printer:epson", []int{42, 44}},
		{"by user", "user:ana", []int{43, 44}},
		{"by qualified state", "state:held", []int{44}},
		{"two qualifiers combine", "user:ana printer:epson", []int{44}},
		{"qualifier plus free text", "user:ana recibo", []int{44}},
		{"document alias", "doc:factura", []int{43}},
		{"qualifier with no match", "user:nadie", []int{}},
		{"an empty qualifier is ignored", "user:", []int{42, 43, 44}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ids(FilterJobs(sampleJobs(), c.query))
			if !equal(got, c.want) {
				t.Errorf("FilterJobs(%q) = %v, want %v", c.query, got, c.want)
			}
		})
	}
}

func TestFilterJobsQualifierDoesNotMatchOtherFields(t *testing.T) {
	// "ana" is a user, not a printer: the qualifier must narrow the search,
	// not widen it.
	if got := ids(FilterJobs(sampleJobs(), "printer:ana")); !equal(got, []int{}) {
		t.Errorf("FilterJobs(\"printer:ana\") = %v, want it empty", got)
	}
}
