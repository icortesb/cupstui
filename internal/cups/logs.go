package cups

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

// LogFile is one of the logs cupsd writes.
type LogFile struct {
	Name string
	Path string
	Desc string
}

// LogFiles are the standard CUPS logs. The paths are the default ones; if one
// is missing, the interface says so.
var LogFiles = []LogFile{
	{Name: "error_log", Path: "/var/log/cups/error_log", Desc: "daemon errors and warnings"},
	{Name: "access_log", Path: "/var/log/cups/access_log", Desc: "HTTP/IPP requests served"},
	{Name: "page_log", Path: "/var/log/cups/page_log", Desc: "pages printed per job"},
}

// tailWindow is how much is read from the end of the file. error_log grows
// without bound, so a window is read rather than the whole file.
const tailWindow int64 = 256 * 1024

// Tail returns the last n lines of the file.
func Tail(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, classifyFileError(err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, classifyFileError(err)
	}

	size := fi.Size()
	offset := size - tailWindow
	if offset < 0 {
		offset = 0
	}

	buf := make([]byte, size-offset)
	if _, err := f.ReadAt(buf, offset); err != nil && !errors.Is(err, os.ErrClosed) {
		// ReadAt returns io.EOF if the file shrank between the Stat and the
		// read; what was read up to there is still good.
		if len(buf) == 0 {
			return nil, classifyFileError(err)
		}
	}

	lines := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	// When reading started mid-file, the first line may be cut in half.
	if offset > 0 && len(lines) > 1 {
		lines = lines[1:]
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func classifyFileError(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &Error{Kind: KindNotFound, Hint: "log file not found", Err: err}
	case errors.Is(err, fs.ErrPermission):
		return &Error{Kind: KindForbidden, Hint: "permission denied reading the CUPS log", Err: err}
	default:
		return &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
	}
}

// clientTag matches the "[Client 42] " cupsd writes in front of a message. The
// number changes with every connection, so two runs of the same message look
// different when they are not.
var clientTag = regexp.MustCompile(`^\[Client \d+\] `)

// SplitLogLine separates the part of a cupsd line that varies between
// otherwise identical entries — the level letter and the timestamp — from the
// message itself. A line not in that shape, as in access_log, is all message.
func SplitLogLine(line string) (prefix, msg string) {
	if LineSeverity(line) == SeverityNone {
		return "", line
	}
	end := strings.Index(line, "] ")
	if end < 0 || !strings.HasPrefix(line[2:], "[") {
		return line[:2], line[2:]
	}
	return line[:end+2], line[end+2:]
}

// Collapse folds a run of the same message into one line carrying a count.
// cupsd repeats some messages once per client connection — screens of
// "Local authentication certificate not found." at a time — and reading the
// log means finding whatever else happened in between. Only neighbours are
// folded, so the order of events survives, and nothing is dropped: the count
// says how many there were.
func Collapse(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		prefix, msg := SplitLogLine(lines[i])
		key := clientTag.ReplaceAllString(msg, "")

		// Walk the run of lines saying the same thing, noting whether they all
		// came from the one client or the tag is part of what varies.
		j, oneClient := i+1, true
		for ; j < len(lines); j++ {
			_, next := SplitLogLine(lines[j])
			if clientTag.ReplaceAllString(next, "") != key {
				break
			}
			oneClient = oneClient && next == msg
		}

		switch n := j - i; {
		case n == 1:
			out = append(out, lines[i])
		case oneClient:
			// One client said it n times: its line is still the whole truth.
			out = append(out, fmt.Sprintf("%s (×%d)", lines[i], n))
		default:
			// Several clients did, so the tag of the first would misreport the
			// rest; the message without it is what they have in common.
			out = append(out, fmt.Sprintf("%s%s (×%d)", prefix, key, n))
		}
		i = j
	}
	return out
}
