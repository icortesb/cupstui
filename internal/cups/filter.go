package cups

import (
	"strconv"
	"strings"
)

// FilterJobs devuelve los trabajos que coinciden con la consulta, buscando en
// id, usuario, documento, impresora y estado. Una consulta vacía no filtra.
func FilterJobs(jobs []Job, query string) []Job {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return jobs
	}

	out := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		haystack := strings.ToLower(strings.Join([]string{
			strconv.Itoa(j.ID),
			j.User,
			j.Name,
			j.Printer,
			j.State.String(),
		}, " "))
		if strings.Contains(haystack, q) {
			out = append(out, j)
		}
	}
	return out
}
