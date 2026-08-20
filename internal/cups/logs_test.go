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
		t.Errorf("Tail = %v, want %v", got, want)
	}
}

func TestTailReturnsEverythingWhenTheFileIsShorter(t *testing.T) {
	got, err := Tail(writeTemp(t, "uno\ndos\n"), 10)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Tail = %v, want 2 lines", got)
	}
}

func TestTailHandlesAFileWithoutTrailingNewline(t *testing.T) {
	got, err := Tail(writeTemp(t, "uno\ndos"), 5)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) != 2 || got[1] != "dos" {
		t.Errorf("Tail = %v, want [uno dos]", got)
	}
}

func TestTailOnAnEmptyFileReturnsNothing(t *testing.T) {
	got, err := Tail(writeTemp(t, ""), 5)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Tail = %v, want it empty", got)
	}
}

func TestTailReadsOnlyTheEndOfALargeFile(t *testing.T) {
	// The CUPS error_log grows without bound; reading it whole on every
	// refresh would throw memory and disk away.
	var b strings.Builder
	for i := 0; i < 200000; i++ {
		b.WriteString("padding line long enough to inflate the file\n")
	}
	b.WriteString("la última\n")
	path := writeTemp(t, b.String())

	fi, _ := os.Stat(path)
	if fi.Size() < 2*tailWindow {
		t.Fatalf("the test file (%d bytes) must exceed the read window", fi.Size())
	}

	got, err := Tail(path, 2)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] != "la última" {
		t.Errorf("the end of the file was not read: %v", got)
	}
}

func TestTailClassifiesAMissingFile(t *testing.T) {
	_, err := Tail(filepath.Join(t.TempDir(), "no-existe"), 5)
	if err == nil {
		t.Fatal("want an error")
	}
	var cerr *Error
	if !asError(err, &cerr) || cerr.Kind != KindNotFound {
		t.Errorf("want KindNotFound, got %v", err)
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
			t.Errorf("missing %s en %v", want, names)
		}
	}
}
