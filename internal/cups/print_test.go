package cups

import (
	"strings"
	"testing"
)

func argsOf(t *testing.T, o PrintOptions) string {
	t.Helper()
	args, err := lpArgs("/casa/mi archivo.pdf", o)
	if err != nil {
		t.Fatalf("lpArgs: %v", err)
	}
	return strings.Join(args, " ")
}

func TestPrintArgsWithDefaultsStayMinimal(t *testing.T) {
	// With no options given, none are sent: the printer's own settings apply,
	// which is almost always what is wanted.
	got := argsOf(t, PrintOptions{Printer: "Epson_L3150", Copies: 1})
	want := "-d Epson_L3150 -n 1 -t mi archivo.pdf -- /casa/mi archivo.pdf"
	if got != want {
		t.Errorf("args = %q\nwant    %q", got, want)
	}
}

func TestPrintArgsCarryEveryOption(t *testing.T) {
	got := argsOf(t, PrintOptions{
		Printer:     "HP_LaserJet",
		Copies:      3,
		PageRanges:  "1-5,8",
		Duplex:      DuplexLongEdge,
		Color:       ColorMono,
		Orientation: OrientationLandscape,
	})
	for _, want := range []string{
		"-d HP_LaserJet",
		"-n 3",
		"-o page-ranges=1-5,8",
		"-o sides=two-sided-long-edge",
		"-o print-color-mode=monochrome",
		"-o orientation-requested=4",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q en %q", want, got)
		}
	}
}

func TestPrintArgsSeparateOptionsFromTheFilename(t *testing.T) {
	// Without the "--", lp would read a file whose name begins with a dash as an option.
	args, err := lpArgs("/casa/-raro.pdf", PrintOptions{Printer: "p", Copies: 1})
	if err != nil {
		t.Fatalf("lpArgs: %v", err)
	}
	last := args[len(args)-1]
	if last != "/casa/-raro.pdf" {
		t.Errorf("the file must come last, got %q", last)
	}
	if args[len(args)-2] != "--" {
		t.Errorf("the -- separator before the file is missing: %v", args)
	}
}

func TestPrintArgsOmitDefaultPrinterFlagWhenNoneIsChosen(t *testing.T) {
	if got := argsOf(t, PrintOptions{Copies: 1}); strings.Contains(got, "-d ") {
		t.Errorf("with no printer chosen there is no -d, got %q", got)
	}
}

func TestPrintArgsDuplexVariants(t *testing.T) {
	cases := map[Duplex]string{
		DuplexNone:      "sides=one-sided",
		DuplexLongEdge:  "sides=two-sided-long-edge",
		DuplexShortEdge: "sides=two-sided-short-edge",
	}
	for d, want := range cases {
		got := argsOf(t, PrintOptions{Copies: 1, Duplex: d})
		if d == DuplexDefault {
			continue
		}
		if !strings.Contains(got, want) {
			t.Errorf("duplex %v: missing %q in %q", d, want, got)
		}
	}
}

func TestPrintOptionsValidation(t *testing.T) {
	cases := []struct {
		name string
		opts PrintOptions
	}{
		{"zero copies", PrintOptions{Copies: 0}},
		{"negative copies", PrintOptions{Copies: -3}},
		{"too many copies", PrintOptions{Copies: 1000}},
		{"range with letters", PrintOptions{Copies: 1, PageRanges: "uno-dos"}},
		{"range with a semicolon", PrintOptions{Copies: 1, PageRanges: "1;2"}},
		{"reversed range", PrintOptions{Copies: 1, PageRanges: "5-2"}},
		{"range starting at zero", PrintOptions{Copies: 1, PageRanges: "0-3"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := lpArgs("/x.pdf", c.opts); err == nil {
				t.Errorf("want an error for %+v", c.opts)
			}
		})
	}
}

func TestPrintOptionsAcceptValidRanges(t *testing.T) {
	for _, r := range []string{"", "1", "1-5", "2,4,6", "1-3,7,10-12"} {
		if _, err := lpArgs("/x.pdf", PrintOptions{Copies: 1, PageRanges: r}); err != nil {
			t.Errorf("the range %q should be accepted: %v", r, err)
		}
	}
}

func TestPrintRejectsAMissingFile(t *testing.T) {
	_, err := Submit("/no/existe.pdf", PrintOptions{Copies: 1})
	if err == nil {
		t.Fatal("want an error")
	}
	var cerr *Error
	if !asError(err, &cerr) || cerr.Kind != KindNotFound {
		t.Errorf("want KindNotFound, got %v", err)
	}
}
