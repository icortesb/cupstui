package cups

import (
	"errors"
	"io/fs"
	"os"
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
