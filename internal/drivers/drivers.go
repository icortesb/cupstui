// Package drivers is a small, read-only knowledge base mapping a printer's
// reported make and model to the driver package it needs. It only reports —
// it never installs anything, runs a shell, or asks for a password. Turning
// a recommendation into an installed driver is the user's decision and the
// user's command.
package drivers

import "strings"

// Info describes a driver recommendation for a known printer model.
type Info struct {
	Vendor      string
	Model       string
	Package     string
	Source      string // "arch-repo", "AUR", "vendor"
	InstallHint string // shown, never run
}

// database is deliberately small: a wrong recommendation is worse than none,
// so an entry is only added once its package name and source are verified,
// not guessed from a vendor's product line.
var database = []Info{
	{
		Vendor:      "Epson",
		Model:       "L3150",
		Package:     "epson-inkjet-printer-escpr",
		Source:      "AUR",
		InstallHint: "yay -S epson-inkjet-printer-escpr",
	},
}

// unresolvable is what CUPS reports when it has nothing useful to say about a
// printer's make — matching against these would be a guess, not a lookup.
// Mirrors the exclusion list internal/cups uses for the same reason.
var unresolvable = map[string]bool{
	"unknown": true,
	"generic": true,
	"local":   true,
	"raw":     true,
}

// Resolve looks up a known driver for a make-and-model string such as
// "EPSON L3150 Series". ok is false when there is no confident match, which
// is preferable to recommending the wrong driver.
func Resolve(makeModel string) (Info, bool) {
	words := normalize(makeModel)
	if len(words) == 0 || unresolvable[words[0]] {
		return Info{}, false
	}

	present := make(map[string]bool, len(words))
	for _, w := range words {
		present[w] = true
	}

	for _, entry := range database {
		if containsAll(present, normalize(entry.Vendor)) && containsAll(present, normalize(entry.Model)) {
			return entry, true
		}
	}
	return Info{}, false
}

func containsAll(present map[string]bool, words []string) bool {
	if len(words) == 0 {
		return false
	}
	for _, w := range words {
		if !present[w] {
			return false
		}
	}
	return true
}

// normalize lowercases and splits into words. A match only requires every
// vendor and model word to be present somewhere in the input, so case,
// spacing, word order and extra words CUPS adds — "Series", for one — are
// already harmless without needing special-casing here.
func normalize(s string) []string {
	return strings.Fields(strings.ToLower(s))
}
