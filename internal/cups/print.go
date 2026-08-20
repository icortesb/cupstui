package cups

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Jobs are submitted with lp rather than over IPP, unlike the rest of this
// package. The IPP library available here sends copies twice (it adds its own
// default alongside the requested value), cannot encode page-ranges as a
// rangeOfInteger — it sends text, which the filter may ignore without saying so
// — and the job ends up attributed to root instead of the real user. lp gets
// all three right because it knows the type of each option.
//
// Verified against CUPS 2.4.19 by comparing the attributes each route leaves
// stored on the job.

// Duplex is two-sided printing.
type Duplex int

const (
	DuplexDefault Duplex = iota // whatever the printer is configured for
	DuplexNone
	DuplexLongEdge
	DuplexShortEdge
)

func (d Duplex) String() string {
	switch d {
	case DuplexNone:
		return "one-sided"
	case DuplexLongEdge:
		return "two-sided, long edge"
	case DuplexShortEdge:
		return "two-sided, short edge"
	default:
		return "printer default"
	}
}

func (d Duplex) value() string {
	switch d {
	case DuplexNone:
		return "one-sided"
	case DuplexLongEdge:
		return "two-sided-long-edge"
	case DuplexShortEdge:
		return "two-sided-short-edge"
	default:
		return ""
	}
}

// ColorMode chooses between colour and black and white.
type ColorMode int

const (
	ColorDefault ColorMode = iota
	ColorColor
	ColorMono
)

func (c ColorMode) String() string {
	switch c {
	case ColorColor:
		return "color"
	case ColorMono:
		return "black and white"
	default:
		return "printer default"
	}
}

func (c ColorMode) value() string {
	switch c {
	case ColorColor:
		return "color"
	case ColorMono:
		return "monochrome"
	default:
		return ""
	}
}

// Orientation is how the page is laid out.
type Orientation int

const (
	OrientationDefault Orientation = iota
	OrientationPortrait
	OrientationLandscape
)

func (o Orientation) String() string {
	switch o {
	case OrientationPortrait:
		return "portrait"
	case OrientationLandscape:
		return "landscape"
	default:
		return "printer default"
	}
}

// value is the IPP orientation-requested enum (3 portrait, 4 landscape).
func (o Orientation) value() string {
	switch o {
	case OrientationPortrait:
		return "3"
	case OrientationLandscape:
		return "4"
	default:
		return ""
	}
}

// PrintOptions are the options of a new job. The zero value of each field
// means whatever the printer is configured for.
type PrintOptions struct {
	Printer     string
	Copies      int
	PageRanges  string
	Duplex      Duplex
	Color       ColorMode
	Orientation Orientation
	Hold        bool // encolar retenido, sin imprimir
}

const maxCopies = 999

// pageRangePattern accepts the forms CUPS understands: 1, 1-5, 2,4,6-8.
var pageRangePattern = regexp.MustCompile(`^\d+(-\d+)?(,\d+(-\d+)?)*$`)

func (o PrintOptions) validate() error {
	if o.Copies < 1 {
		return fmt.Errorf("copies must be at least 1")
	}
	if o.Copies > maxCopies {
		return fmt.Errorf("too many copies (maximum %d)", maxCopies)
	}
	if o.PageRanges != "" {
		if err := validatePageRanges(o.PageRanges); err != nil {
			return err
		}
	}
	return nil
}

func validatePageRanges(ranges string) error {
	if !pageRangePattern.MatchString(ranges) {
		return fmt.Errorf("invalid page range %q — expected 1, 1-5 or 2,4,6-8", ranges)
	}
	for _, part := range strings.Split(ranges, ",") {
		from, to, hasDash := strings.Cut(part, "-")
		first, _ := strconv.Atoi(from)
		if first < 1 {
			return fmt.Errorf("pages are numbered from 1, not from %d", first)
		}
		if hasDash {
			last, _ := strconv.Atoi(to)
			if last < first {
				return fmt.Errorf("range %q ends before it starts", part)
			}
		}
	}
	return nil
}

// lpArgs builds the arguments for lp. They are returned as a list and run
// without a shell, so a file name holding spaces or quotes cannot turn into
// something else.
func lpArgs(path string, o PrintOptions) ([]string, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}

	var args []string
	if o.Printer != "" {
		args = append(args, "-d", o.Printer)
	}
	args = append(args, "-n", strconv.Itoa(o.Copies))
	if o.Hold {
		args = append(args, "-H", "hold")
	}
	for _, opt := range []struct{ name, value string }{
		{"page-ranges", o.PageRanges},
		{"sides", o.Duplex.value()},
		{"print-color-mode", o.Color.value()},
		{"orientation-requested", o.Orientation.value()},
	} {
		if opt.value != "" {
			args = append(args, "-o", opt.name+"="+opt.value)
		}
	}

	args = append(args, "-t", filepath.Base(path))
	// The "--" stops a file whose name begins with a dash from being read as
	// one more option.
	return append(args, "--", path), nil
}

// jobIDPattern pulls the number out of "request id is Epson_L3150-42 (1 file(s))".
var jobIDPattern = regexp.MustCompile(`-(\d+)\s`)

// Submit sends a file to print and returns the job id.
func Submit(path string, o PrintOptions) (int, error) {
	if fi, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, &Error{Kind: KindNotFound, Hint: "file not found: " + path, Err: err}
		}
		return 0, classifyFileError(err)
	} else if fi.IsDir() {
		return 0, &Error{Kind: KindUnknown, Hint: path + " is a directory, not a file"}
	}

	args, err := lpArgs(path, o)
	if err != nil {
		return 0, &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
	}

	out, err := exec.Command("lp", args...).CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return 0, &Error{
				Kind: KindNotFound,
				Hint: "lp command not found — install the cups-client package",
				Err:  err,
			}
		}
		return 0, &Error{Kind: KindUnknown, Hint: lpError(out, err), Err: err}
	}

	if m := jobIDPattern.FindStringSubmatch(string(out)); m != nil {
		id, _ := strconv.Atoi(m[1])
		return id, nil
	}
	return 0, nil // accepted, but the output held no id we recognise
}

// lpError keeps the output of lp, which explains the problem better than the
// exit code does.
func lpError(out []byte, err error) string {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return err.Error()
	}
	// lp prefixes each line with its own name, which is noise here.
	msg = strings.TrimPrefix(msg, "lp: ")
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	return msg
}
