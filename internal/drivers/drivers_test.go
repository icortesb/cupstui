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

func TestResolveRejectsUnknownOrAmbiguousInput(t *testing.T) {
	cases := []string{
		"",
		"Unknown",
		"unknown",
		"Generic",
		"Generic PCL 6 printer",
		"Local Printer",
		"Raw Queue",
		"Epson XP-15000",    // real vendor, model not in the database
		"Canon PIXMA G3010", // vendor not in the database at all
		"L3150",             // model alone, no vendor word present
	}

	for _, makeModel := range cases {
		t.Run(makeModel, func(t *testing.T) {
			if info, ok := Resolve(makeModel); ok {
				t.Fatalf("Resolve(%q) = %+v, want no match", makeModel, info)
			}
		})
	}
}
