package drivers

import "testing"

func TestResolveMatchesKnownVariants(t *testing.T) {
	cases := []string{
		"Epson L3150",
		"EPSON L3150 Series",
		"Epson L3150 Series",
		"EPSON L3150",
		"  Epson   L3150  ", // extra whitespace
		"epson l3150",       // case differences
	}

	for _, makeModel := range cases {
		t.Run(makeModel, func(t *testing.T) {
			info, ok := Resolve(makeModel)
			if !ok {
				t.Fatalf("Resolve(%q): want a match, got none", makeModel)
			}
			if info.Package != "epson-inkjet-printer-escpr" {
				t.Errorf("Resolve(%q).Package = %q, want epson-inkjet-printer-escpr", makeModel, info.Package)
			}
		})
	}
}

func TestResolveFindsGutenprintModels(t *testing.T) {
	// Exact model text isn't asserted here: it comes verbatim from
	// gutenprint's own data (see scripts/gen-driverdb.py) and its casing is
	// not this test's business — only that a real, known model resolves to
	// the gutenprint/arch-repo entry.
	cases := []string{
		"Epson Stylus Photo R280",
		"CANON PIXMA iP4000",
		"HP LaserJet 4mL",
		"Brother HL-1040",
	}
	for _, makeModel := range cases {
		t.Run(makeModel, func(t *testing.T) {
			info, ok := Resolve(makeModel)
			if !ok {
				t.Fatalf("Resolve(%q): want a match from gutenprint.json, got none", makeModel)
			}
			if info.Package != "gutenprint" || info.Source != "arch-repo" {
				t.Errorf("Resolve(%q) = %+v, want the gutenprint/arch-repo entry", makeModel, info)
			}
		})
	}
}

// The bulk gutenprint import must not shadow a specific AUR entry for a
// model gutenprint does not actually support — that is exactly why the
// L3150 needed escpr in the first place.
func TestResolvePrefersTheSpecificEntryOverGutenprint(t *testing.T) {
	info, ok := Resolve("EPSON L3150 Series")
	if !ok {
		t.Fatal("Resolve(EPSON L3150 Series): want a match")
	}
	if info.Package != "epson-inkjet-printer-escpr" {
		t.Errorf("Package = %q, want epson-inkjet-printer-escpr, not a gutenprint false positive", info.Package)
	}
}

// A short, generic entry ("Epson Stylus Photo") must not win over a longer,
// more specific one ("Epson Stylus Photo R280") just because it appears
// first — the more specific match is the real answer.
func TestResolvePrefersTheMoreSpecificModel(t *testing.T) {
	info, ok := Resolve("Epson Stylus Photo R280")
	if !ok {
		t.Fatal("Resolve(Epson Stylus Photo R280): want a match")
	}
	if info.Model != "Stylus Photo R280" {
		t.Errorf("Model = %q, want the specific R280 variant, not a shorter prefix match", info.Model)
	}
}

func TestGutenprintDatabaseLoaded(t *testing.T) {
	if len(database) < 1000 {
		t.Errorf("database has %d entries, want the bulk gutenprint import to have loaded", len(database))
	}
}

func TestResolveRejectsUnknownOrAmbiguousInput(t *testing.T) {
	cases := []string{
		"",
		"Unknown",
		"unknown",
		"Generic",
		"Generic PCL 6 printer",
		"Local Printer",
		"Raw Queue",
		"Epson XP-15000",      // real vendor, model not in the database
		"Canon PIXMA ZZ-9999", // real vendor, fictional model
		"L3150",               // model alone, no vendor word present
	}

	for _, makeModel := range cases {
		t.Run(makeModel, func(t *testing.T) {
			if info, ok := Resolve(makeModel); ok {
				t.Fatalf("Resolve(%q) = %+v, want no match", makeModel, info)
			}
		})
	}
}
