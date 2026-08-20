package cups

import (
	"os"
	"path/filepath"
	"testing"
)

// withClientConf writes a client.conf into a temporary home.
func withClientConf(t *testing.T, contents string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if contents == "" {
		return
	}
	dir := filepath.Join(home, ".cups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "client.conf"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestServerDefaultsToTheLocalSocket(t *testing.T) {
	withClientConf(t, "")
	t.Setenv("CUPS_SERVER", "")

	s := ResolveServer()
	if !s.Local {
		t.Errorf("Server = %+v, want the local socket", s)
	}
	if s.Address == "" {
		t.Error("want a socket path")
	}
}

func TestServerFromEnvironment(t *testing.T) {
	withClientConf(t, "")
	cases := []struct {
		env       string
		wantAddr  string
		wantHost  string
		wantLocal bool
	}{
		{"print.example.org", "print.example.org:631", "print.example.org", false},
		{"print.example.org:1631", "print.example.org:1631", "print.example.org", false},
		{"192.168.0.10", "192.168.0.10:631", "192.168.0.10", false},
		// An absolute path is a domain socket, not a host name.
		{"/run/cups/cups.sock", "/run/cups/cups.sock", "localhost", true},
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			t.Setenv("CUPS_SERVER", c.env)
			s := ResolveServer()
			if s.Address != c.wantAddr {
				t.Errorf("Address = %q, want %q", s.Address, c.wantAddr)
			}
			if s.Host != c.wantHost {
				t.Errorf("Host = %q, want %q", s.Host, c.wantHost)
			}
			if s.Local != c.wantLocal {
				t.Errorf("Local = %v, want %v", s.Local, c.wantLocal)
			}
		})
	}
}

func TestServerFromClientConf(t *testing.T) {
	withClientConf(t, "# a comment\n\nServerName print.example.org:1631\nUser ana\n")
	t.Setenv("CUPS_SERVER", "")

	s := ResolveServer()
	if s.Address != "print.example.org:1631" {
		t.Errorf("Address = %q", s.Address)
	}
	if s.User != "ana" {
		t.Errorf("User = %q, want ana", s.User)
	}
	if s.Local {
		t.Error("a named server is not local")
	}
}

func TestEnvironmentOverridesClientConf(t *testing.T) {
	withClientConf(t, "ServerName from-file.example.org\n")
	t.Setenv("CUPS_SERVER", "from-env.example.org")

	if got := ResolveServer().Host; got != "from-env.example.org" {
		t.Errorf("Host = %q, want the environment to win", got)
	}
}

func TestClientConfDirectivesAreCaseInsensitive(t *testing.T) {
	withClientConf(t, "servername Print.Example.Org\nuser BOB\n")
	t.Setenv("CUPS_SERVER", "")

	s := ResolveServer()
	if s.Host != "Print.Example.Org" {
		t.Errorf("Host = %q", s.Host)
	}
	if s.User != "BOB" {
		t.Errorf("User = %q", s.User)
	}
}

func TestCUPSUserOverridesTheClientConfUser(t *testing.T) {
	withClientConf(t, "ServerName host\nUser ana\n")
	t.Setenv("CUPS_SERVER", "")
	t.Setenv("CUPS_USER", "bob")

	if got := ResolveServer().User; got != "bob" {
		t.Errorf("User = %q, want bob", got)
	}
}

func TestAnUnreadableClientConfFallsBackToTheLocalSocket(t *testing.T) {
	withClientConf(t, "this line means nothing\n")
	t.Setenv("CUPS_SERVER", "")

	if !ResolveServer().Local {
		t.Error("an unusable client.conf must not take the local socket away")
	}
}

func TestServerDescribesItselfForTheInterface(t *testing.T) {
	local := Server{Local: true, Address: "/run/cups/cups.sock"}
	if got := local.String(); got != "local" {
		t.Errorf("String = %q, want local", got)
	}

	remote := Server{Host: "print.example.org", Address: "print.example.org:631"}
	if got := remote.String(); got != "print.example.org:631" {
		t.Errorf("String = %q", got)
	}
}
