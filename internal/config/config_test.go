package config

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate manda la configuración a un directorio temporal, para no tocar la del
// usuario que corre los tests.
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
		t.Fatalf("no se escribió %s: %v", path, err)
	}

	if got := Load(); !got.Transparent {
		t.Errorf("Load = %+v, quiero Transparent true", got)
	}
}

func TestLoadWithoutAFileReturnsTheDefaults(t *testing.T) {
	isolate(t)
	if got := Load(); got.Transparent {
		t.Errorf("sin archivo, Load = %+v, quiero los valores por omisión", got)
	}
}

func TestLoadIgnoresABrokenFile(t *testing.T) {
	// Un archivo corrupto no puede impedir que la aplicación arranque: una
	// preferencia de aspecto no vale una pantalla de error.
	path := isolate(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{esto no es json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := Load(); got.Transparent {
		t.Errorf("con el archivo roto, Load = %+v, quiero los valores por omisión", got)
	}
}

func TestSaveCreatesTheDirectory(t *testing.T) {
	path := isolate(t)
	if err := Save(Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("no se creó el directorio: %v", err)
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
		t.Error("quedó guardado el valor viejo")
	}
}

func TestPathHonoursXDGConfigHome(t *testing.T) {
	want := isolate(t)
	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != want {
		t.Errorf("Path = %q, quiero %q", got, want)
	}
}
