package cups

import "sort"

// Usage is what one user or one printer accounts for in a report.
type Usage struct {
	Name  string
	Jobs  int
	Pages int
}

// UsageByUser totals the report per user, heaviest first.
func UsageByUser(entries []HistoryEntry) []Usage {
	return totalBy(entries, func(e HistoryEntry) string { return e.User })
}

// UsageByPrinter totals the report per printer, heaviest first.
func UsageByPrinter(entries []HistoryEntry) []Usage {
	return totalBy(entries, func(e HistoryEntry) string { return e.Printer })
}

func totalBy(entries []HistoryEntry, key func(HistoryEntry) string) []Usage {
	index := make(map[string]*Usage)
	for _, e := range entries {
		k := key(e)
		u, ok := index[k]
		if !ok {
			u = &Usage{Name: k}
			index[k] = u
		}
		u.Jobs++
		u.Pages += e.Pages
	}

	out := make([]Usage, 0, len(index))
	for _, u := range index {
		out = append(out, *u)
	}

	// Heaviest first, then by name so that equal rows keep a stable order
	// instead of shuffling on every refresh.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pages != out[j].Pages {
			return out[i].Pages > out[j].Pages
		}
		return out[i].Name < out[j].Name
	})
	return out
}
