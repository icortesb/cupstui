package cups

import (
	"strconv"
	"strings"
)

// field names a job field a search can be scoped to.
type field int

const (
	fieldAny field = iota
	fieldUser
	fieldPrinter
	fieldState
	fieldName
)

// qualifiers are the prefixes that scope a term to a single field.
var qualifiers = map[string]field{
	"user":     fieldUser,
	"printer":  fieldPrinter,
	"state":    fieldState,
	"document": fieldName,
	"doc":      fieldName,
	"name":     fieldName,
}

// term is one search term, already read.
type term struct {
	field field
	text  string
}

// FilterJobs returns the jobs matching the query.
//
// A bare term is looked for in the id, user, document, printer and state; a
// term with a prefix (printer:epson, user:ana, state:held) is looked for in
// that field alone. Terms combine: every one must match.
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
				// "user:" with no value scopes nothing, so it is dropped.
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
