package cups

import (
	"strings"
	"testing"
)

func policyArgs(t *testing.T, p Policy) string {
	t.Helper()
	args, err := lpadminPolicyArgs("Epson_L3150", p)
	if err != nil {
		t.Fatalf("lpadminPolicyArgs: %v", err)
	}
	return strings.Join(args, " ")
}

func TestPolicyArgsSetLimitsAndPeriod(t *testing.T) {
	got := policyArgs(t, Policy{PageLimit: 100, QuotaDays: 7})
	for _, want := range []string{
		"-p Epson_L3150",
		"-o job-page-limit=100",
		"-o job-quota-period=604800", // seven days in seconds
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestPolicyArgsClearLimitsWithZero(t *testing.T) {
	// Without this there would be no way to clear a quota already set: an
	// omitted option leaves the previous value in place.
	got := policyArgs(t, Policy{})
	for _, want := range []string{"job-page-limit=0", "job-k-limit=0", "job-quota-period=0"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestPolicyArgsAllowUsers(t *testing.T) {
	got := policyArgs(t, Policy{AllowedUsers: []string{"ana", "bob"}})
	if !strings.Contains(got, "-u allow:ana,bob") {
		t.Errorf("missing the allow list in %q", got)
	}
}

func TestPolicyArgsDenyUsers(t *testing.T) {
	got := policyArgs(t, Policy{DeniedUsers: []string{"eve"}})
	if !strings.Contains(got, "-u deny:eve") {
		t.Errorf("missing the deny list in %q", got)
	}
}

func TestPolicyArgsWithoutAnyListAllowsEveryone(t *testing.T) {
	// CUPS keeps the previous list unless it is explicitly reset.
	got := policyArgs(t, Policy{})
	if !strings.Contains(got, "-u allow:all") {
		t.Errorf("missing the reset to everyone in %q", got)
	}
}

func TestPolicyRejectsBothListsAtOnce(t *testing.T) {
	// CUPS holds one list per printer; sending both would silently drop one.
	_, err := lpadminPolicyArgs("p", Policy{
		AllowedUsers: []string{"ana"},
		DeniedUsers:  []string{"eve"},
	})
	if err == nil {
		t.Fatal("want an error when both lists are set")
	}
}

func TestPolicyRejectsNegativeLimits(t *testing.T) {
	for _, p := range []Policy{{PageLimit: -1}, {KLimit: -5}, {QuotaDays: -2}} {
		if _, err := lpadminPolicyArgs("p", p); err == nil {
			t.Errorf("want an error for %+v", p)
		}
	}
}

func TestPolicyRejectsInvalidUserNames(t *testing.T) {
	// A name with a comma would split into two entries, quietly granting or
	// denying access to something nobody asked for.
	for _, name := range []string{"an,a", "with space", ""} {
		_, err := lpadminPolicyArgs("p", Policy{AllowedUsers: []string{name}})
		if err == nil {
			t.Errorf("the name %q should be rejected", name)
		}
	}
}

func TestPolicyFromAttributesReadsWhatCUPSReports(t *testing.T) {
	p := printerFromAttributes("x", attrs(map[string]interface{}{
		"job-page-limit":   50,
		"job-k-limit":      2048,
		"job-quota-period": 86400,
	}))
	if p.Policy.PageLimit != 50 {
		t.Errorf("PageLimit = %d", p.Policy.PageLimit)
	}
	if p.Policy.KLimit != 2048 {
		t.Errorf("KLimit = %d", p.Policy.KLimit)
	}
	if p.Policy.QuotaDays != 1 {
		t.Errorf("QuotaDays = %d, want 1", p.Policy.QuotaDays)
	}
}

func TestPolicyFromAttributesReadsUserLists(t *testing.T) {
	p := printerFromAttributes("x", ipAttrs("requesting-user-name-denied", "eve", "mallory"))
	if len(p.Policy.DeniedUsers) != 2 || p.Policy.DeniedUsers[0] != "eve" {
		t.Errorf("DeniedUsers = %v", p.Policy.DeniedUsers)
	}
	if len(p.Policy.AllowedUsers) != 0 {
		t.Errorf("AllowedUsers = %v, want empty", p.Policy.AllowedUsers)
	}
}

func TestPolicyDescribesItselfForTheInterface(t *testing.T) {
	if got := (Policy{}).Summary(); got != "no limits" {
		t.Errorf("Summary = %q", got)
	}
	if got := (Policy{PageLimit: 100, QuotaDays: 7}).Summary(); !strings.Contains(got, "100 pages") {
		t.Errorf("Summary = %q", got)
	}
	if got := (Policy{DeniedUsers: []string{"eve"}}).Summary(); !strings.Contains(got, "1 user denied") {
		t.Errorf("Summary = %q", got)
	}
}
