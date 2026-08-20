package cups

import (
	"strings"
	"testing"
)

func TestCheckResultsCarryAHintOnlyWhenSomethingIsWrong(t *testing.T) {
	ok := CheckResult{Name: "CUPS", Status: CheckOK}
	if ok.Hint != "" {
		t.Errorf("a passing check needs no hint, got %q", ok.Hint)
	}
}

func TestPreflightSummaryReportsTheWorstStatus(t *testing.T) {
	cases := []struct {
		name    string
		results []CheckResult
		want    CheckStatus
	}{
		{"all good", []CheckResult{{Status: CheckOK}, {Status: CheckOK}}, CheckOK},
		{"one warning", []CheckResult{{Status: CheckOK}, {Status: CheckWarn}}, CheckWarn},
		{"a failure outweighs a warning", []CheckResult{{Status: CheckWarn}, {Status: CheckFail}}, CheckFail},
		{"still running", []CheckResult{{Status: CheckOK}, {Status: CheckRunning}}, CheckRunning},
		{"nothing yet", nil, CheckRunning},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PreflightStatus(c.results); got != c.want {
				t.Errorf("PreflightStatus = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPreflightNamesEveryCheckItWillRun(t *testing.T) {
	// The screen lists the checks before it has any answers, so the names must
	// be known up front.
	names := PreflightNames()
	if len(names) < 4 {
		t.Fatalf("got %d checks, want at least four", len(names))
	}
	joined := strings.ToLower(strings.Join(names, " | "))
	for _, want := range []string{"cups", "printing", "administrative", "drivers"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing a check about %q in %v", want, names)
		}
	}
}

func TestDriverHintNamesWhatToLookFor(t *testing.T) {
	// When no PPD matches, the way out is installing a driver package, and the
	// make of the printer is the search term.
	hint := DriverHint("EPSON L3150 Series")
	if !strings.Contains(strings.ToLower(hint), "epson") {
		t.Errorf("hint = %q, want it to name the make", hint)
	}

	if got := DriverHint(""); got == "" {
		t.Error("want a general hint even with no model")
	}
}

func TestDriverHintFallsBackWhenTheMakeIsUnknown(t *testing.T) {
	hint := DriverHint("Unknown")
	if strings.Contains(strings.ToLower(hint), "unknown") {
		t.Errorf("hint = %q, should not search for the word Unknown", hint)
	}
}

func TestMakeOfReadsTheFirstWord(t *testing.T) {
	cases := map[string]string{
		"EPSON L3150 Series":       "EPSON",
		"HP LaserJet 1200":         "HP",
		"Brother HL-2270DW series": "Brother",
		"":                         "",
		"Unknown":                  "",
		"Local Raw Printer":        "",
	}
	for model, want := range cases {
		if got := makeOf(model); got != want {
			t.Errorf("makeOf(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestChecksThatDependOnCUPSDoNotRepeatItsHint(t *testing.T) {
	// With the daemon down every dependent check fails for the same reason.
	// Repeating the remediation on each line buries the one that matters.
	down := &Error{Kind: KindDaemonDown, Hint: "CUPS is not responding"}

	for _, r := range []CheckResult{
		unavailable(checkAdmin, down),
		unavailable(checkDriver, down),
	} {
		if r.Hint != "" {
			t.Errorf("%s carries a hint %q, want none", r.Name, r.Hint)
		}
		if !strings.Contains(r.Detail, "skipped") {
			t.Errorf("%s detail = %q, want it to say the check was skipped", r.Name, r.Detail)
		}
		if r.Status != CheckWarn {
			t.Errorf("%s status = %v, want CheckWarn", r.Name, r.Status)
		}
	}
}

func TestAnUnrelatedFailureKeepsItsOwnHint(t *testing.T) {
	r := unavailable(checkDriver, &Error{Kind: KindForbidden, Hint: "permission denied"})
	if r.Hint != "permission denied" {
		t.Errorf("Hint = %q, want the error's own hint", r.Hint)
	}
}
