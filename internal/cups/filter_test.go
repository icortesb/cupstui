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
		{"consulta vacía devuelve todo", "", []int{42, 43, 44}},
		{"por usuario", "ana", []int{43, 44}},
		{"por impresora", "epson", []int{42, 44}},
		{"por documento", "factura", []int{43}},
		{"por estado en castellano", "retenido", []int{44}},
		{"por id", "42", []int{42}},
		{"sin coincidencias", "zzz", []int{}},
		{"ignora mayúsculas", "ICORTES", []int{42}},
		{"ignora espacios sobrantes", "  ana  ", []int{43, 44}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ids(FilterJobs(sampleJobs(), c.query))
			if !equal(got, c.want) {
				t.Errorf("FilterJobs(%q) = %v, quiero %v", c.query, got, c.want)
			}
		})
	}
}

func TestFilterJobsPreservesOrder(t *testing.T) {
	if got := ids(FilterJobs(sampleJobs(), "epson")); !equal(got, []int{42, 44}) {
		t.Errorf("orden = %v, quiero %v", got, []int{42, 44})
	}
}

func TestFilterJobsWithFieldQualifiers(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  []int
	}{
		{"por impresora", "impresora:epson", []int{42, 44}},
		{"por usuario", "usuario:ana", []int{43, 44}},
		{"por estado", "estado:retenido", []int{44}},
		{"dos calificadores se acumulan", "usuario:ana impresora:epson", []int{44}},
		{"calificador más texto libre", "usuario:ana recibo", []int{44}},
		{"alias en inglés", "printer:hp", []int{43}},
		{"calificador sin coincidencias", "usuario:nadie", []int{}},
		{"calificador vacío se ignora", "usuario:", []int{42, 43, 44}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ids(FilterJobs(sampleJobs(), c.query))
			if !equal(got, c.want) {
				t.Errorf("FilterJobs(%q) = %v, quiero %v", c.query, got, c.want)
			}
		})
	}
}

func TestFilterJobsQualifierDoesNotMatchOtherFields(t *testing.T) {
	// "ana" es un usuario, no una impresora: el calificador tiene que acotar
	// la búsqueda, no ampliarla.
	if got := ids(FilterJobs(sampleJobs(), "impresora:ana")); !equal(got, []int{}) {
		t.Errorf("FilterJobs(\"impresora:ana\") = %v, quiero vacío", got)
	}
}
