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

func TestCollapseFoldsARunOfTheSameMessage(t *testing.T) {
	lines := []string{
		"E [19/Aug/2026:22:05:28 -0300] [Client 290] Local authentication certificate not found.",
		"E [19/Aug/2026:22:05:28 -0300] [Client 291] Local authentication certificate not found.",
		"E [19/Aug/2026:22:05:28 -0300] [Client 292] Local authentication certificate not found.",
		"W [19/Aug/2026:22:37:52 -0300] CreateProfile failed",
	}

	got := Collapse(lines)
	if len(got) != 2 {
		t.Fatalf("Collapse = %v, want 2 lines", got)
	}
	// The client tag is what varies, so it goes; the count replaces it.
	if strings.Contains(got[0], "Client") {
		t.Errorf("kept a client tag that misreports the rest: %q", got[0])
	}
	if !strings.Contains(got[0], "(×3)") {
		t.Errorf("Collapse[0] = %q, want a count of 3", got[0])
	}
	if got[1] != lines[3] {
		t.Errorf("Collapse[1] = %q, want it untouched", got[1])
	}
}

func TestCollapseKeepsTheLineWhenOneClientRepeatsItself(t *testing.T) {
	lines := []string{
		"E [19/Aug/2026:22:05:28 -0300] [Client 5] Local authentication certificate not found.",
		"E [19/Aug/2026:22:05:29 -0300] [Client 5] Local authentication certificate not found.",
	}

	got := Collapse(lines)
	if len(got) != 1 {
		t.Fatalf("Collapse = %v, want 1 line", got)
	}
	if !strings.Contains(got[0], "[Client 5]") || !strings.Contains(got[0], "(×2)") {
		t.Errorf("Collapse = %q, want the client kept and a count of 2", got[0])
	}
}

func TestCollapseOnlyFoldsNeighbours(t *testing.T) {
	lines := []string{
		"E [19/Aug/2026:22:05:28 -0300] [Client 1] same",
		"W [19/Aug/2026:22:05:29 -0300] something else",
		"E [19/Aug/2026:22:05:30 -0300] [Client 2] same",
	}

	if got := Collapse(lines); len(got) != 3 {
		t.Errorf("Collapse = %v, want the order of events kept", got)
	}
}

func TestCollapseLeavesLinesWithoutALevelAlone(t *testing.T) {
	lines := []string{
		`localhost - - [19/Aug/2026:22:05:28 -0300] "POST / HTTP/1.1" 200 362`,
		`localhost - - [19/Aug/2026:22:05:29 -0300] "POST / HTTP/1.1" 200 362`,
	}

	if got := Collapse(lines); len(got) != 2 {
		t.Errorf("Collapse = %v, want access_log lines untouched", got)
	}
}

func TestSplitLogLineSeparatesTheTimestampFromTheMessage(t *testing.T) {
	prefix, msg := SplitLogLine("E [19/Aug/2026:22:05:28 -0300] [Client 290] Not found.")
	if prefix != "E [19/Aug/2026:22:05:28 -0300] " {
		t.Errorf("prefix = %q", prefix)
	}
	if msg != "[Client 290] Not found." {
		t.Errorf("msg = %q", msg)
	}

	if prefix, msg := SplitLogLine("no level here"); prefix != "" || msg != "no level here" {
		t.Errorf("SplitLogLine = %q, %q, want it all message", prefix, msg)
	}
}
