package main

import "testing"

func TestBuildVersion(t *testing.T) {
	cases := []struct {
		name    string
		stamped string
		module  string
		want    string
	}{
		{"a release stamps the version in", "v0.1.6", "v0.1.5", "v0.1.6"},
		{"go install reports the module version", "dev", "v0.1.6", "v0.1.6"},
		{"a build from the tree stays dev", "dev", "(devel)", "dev"},
		{"no build info at all stays dev", "dev", "", "dev"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildVersion(c.stamped, c.module); got != c.want {
				t.Errorf("buildVersion(%q, %q) = %q, want %q",
					c.stamped, c.module, got, c.want)
			}
		})
	}
}
