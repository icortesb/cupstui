package cups

import (
	"strconv"
	"strings"
)

// field nombra un campo del trabajo por el que se puede acotar la búsqueda.
type field int

const (
	fieldAny field = iota
	fieldUser
	fieldPrinter
	fieldState
	fieldName
)

// qualifiers son los prefijos que acotan un término a un solo campo. Se
// aceptan en castellano y en inglés porque la salida de CUPS y la costumbre
// mezclan los dos.
var qualifiers = map[string]field{
	"usuario":   fieldUser,
	"user":      fieldUser,
	"impresora": fieldPrinter,
	"printer":   fieldPrinter,
	"estado":    fieldState,
	"state":     fieldState,
	"documento": fieldName,
	"doc":       fieldName,
	"name":      fieldName,
}

// term es un término de búsqueda ya interpretado.
type term struct {
	field field
	text  string
}

// FilterJobs devuelve los trabajos que coinciden con la consulta.
//
// Cada término suelto se busca en id, usuario, documento, impresora y estado;
// un término con prefijo (impresora:epson, usuario:ana, estado:retenido) se
// busca solo en ese campo. Los términos se acumulan: todos tienen que coincidir.
func FilterJobs(jobs []Job, query string) []Job {
	terms := parseQuery(query)
	if len(terms) == 0 {
		return jobs
	}

	out := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		if matchesAll(j, terms) {
			out = append(out, j)
		}
	}
	return out
}

func parseQuery(query string) []term {
	var terms []term
	for _, word := range strings.Fields(strings.ToLower(query)) {
		f := fieldAny
		text := word

		if prefix, rest, found := strings.Cut(word, ":"); found {
			if qf, ok := qualifiers[prefix]; ok {
				// "usuario:" sin valor no acota nada, se descarta.
				if rest == "" {
					continue
				}
				f, text = qf, rest
			}
		}
		terms = append(terms, term{field: f, text: text})
	}
	return terms
}

func matchesAll(j Job, terms []term) bool {
	for _, t := range terms {
		if !strings.Contains(haystack(j, t.field), t.text) {
			return false
		}
	}
	return true
}

func haystack(j Job, f field) string {
	switch f {
	case fieldUser:
		return strings.ToLower(j.User)
	case fieldPrinter:
		return strings.ToLower(j.Printer)
	case fieldState:
		return strings.ToLower(j.State.String())
	case fieldName:
		return strings.ToLower(j.Name)
	default:
		return strings.ToLower(strings.Join([]string{
			strconv.Itoa(j.ID), j.User, j.Name, j.Printer, j.State.String(),
		}, " "))
	}
}
