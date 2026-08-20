package config

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate points the configuration at a temporary directory, so the tests do
// not touch the one belonging to whoever runs them.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "cupstui", "config.json")
}

func TestSaveThenLoadKeepsThePreference(t *testing.T) {
	path := isolate(t)

	if err := Save(Config{Transparent: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s was not written: %v", path, err)
	}

	if got := Load(); !got.Transparent {
		t.Errorf("Load = %+v, want Transparent true", got)
	}
}

func TestLoadWithoutAFileReturnsTheDefaults(t *testing.T) {
	isolate(t)
	if got := Load(); got.Transparent {
		t.Errorf("with no file, Load = %+v, want the defaults", got)
	}
}

func TestLoadIgnoresABrokenFile(t *testing.T) {
	// A corrupt file cannot stop the application from starting: a look and
	// feel preference is not worth an error screen.
	path := isolate(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{esto no es json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := Load(); got.Transparent {
		t.Errorf("with the file broken, Load = %+v, want the defaults", got)
	}
}

func TestSaveCreatesTheDirectory(t *testing.T) {
	path := isolate(t)
	if err := Save(Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("the directory was not created: %v", err)
	}
}

func TestSaveOverwritesThePreviousValue(t *testing.T) {
	isolate(t)
	if err := Save(Config{Transparent: true}); err != nil {
		t.Fatal(err)
	}
	if err := Save(Config{Transparent: false}); err != nil {
		t.Fatal(err)
	}
	if Load().Transparent {
		t.Error("the old value stayed saved")
	}
}

func TestPathHonoursXDGConfigHome(t *testing.T) {
	want := isolate(t)
	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
