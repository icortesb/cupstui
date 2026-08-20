package cups

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "error_log")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTailReturnsTheLastLines(t *testing.T) {
	path := writeTemp(t, "uno\ndos\ntres\ncuatro\ncinco\n")

	got, err := Tail(path, 3)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	want := []string{"tres", "cuatro", "cinco"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Tail = %v, quiero %v", got, want)
	}
}

func TestTailReturnsEverythingWhenTheFileIsShorter(t *testing.T) {
	got, err := Tail(writeTemp(t, "uno\ndos\n"), 10)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Tail = %v, quiero 2 líneas", got)
	}
}

func TestTailHandlesAFileWithoutTrailingNewline(t *testing.T) {
	got, err := Tail(writeTemp(t, "uno\ndos"), 5)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) != 2 || got[1] != "dos" {
		t.Errorf("Tail = %v, quiero [uno dos]", got)
	}
}

func TestTailOnAnEmptyFileReturnsNothing(t *testing.T) {
	got, err := Tail(writeTemp(t, ""), 5)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Tail = %v, quiero vacío", got)
	}
}

func TestTailReadsOnlyTheEndOfALargeFile(t *testing.T) {
	// El error_log de CUPS crece sin límite; leerlo entero en cada refresco
	// sería tirar memoria y disco a la basura.
	var b strings.Builder
	for i := 0; i < 200000; i++ {
		b.WriteString("línea de relleno bastante larga para inflar el archivo\n")
	}
	b.WriteString("la última\n")
	path := writeTemp(t, b.String())

	fi, _ := os.Stat(path)
	if fi.Size() < 2*tailWindow {
		t.Fatalf("el archivo de prueba (%d bytes) tiene que superar la ventana de lectura", fi.Size())
	}

	got, err := Tail(path, 2)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] != "la última" {
		t.Errorf("no se leyó el final del archivo: %v", got)
	}
}

func TestTailClassifiesAMissingFile(t *testing.T) {
	_, err := Tail(filepath.Join(t.TempDir(), "no-existe"), 5)
	if err == nil {
		t.Fatal("quiero un error")
	}
	var cerr *Error
	if !asError(err, &cerr) || cerr.Kind != KindNotFound {
		t.Errorf("quiero KindNotFound, tengo %v", err)
	}
}

func TestLogFilesNamesTheUsualCUPSLogs(t *testing.T) {
	names := make([]string, 0, len(LogFiles))
	for _, l := range LogFiles {
		names = append(names, l.Name)
	}
	joined := strings.Join(names, " ")
	for _, want := range []string{"error_log", "access_log", "page_log"} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta %s en %v", want, names)
		}
	}
}
