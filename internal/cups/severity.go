package cups

// Severity is the level cupsd stamps at the start of each error_log line.
type Severity int

const (
	SeverityNone Severity = iota // a line with no level, such as access_log
	SeverityDebug
	SeverityInfo
	SeverityWarning
	SeverityError
)

// LineSeverity reads the level of a log line.
//
// cupsd writes the level as a single letter followed by a space, so a line that
// does not start that way — access_log and page_log entries — has no level.
func LineSeverity(line string) Severity {
	if len(line) < 2 || line[1] != ' ' {
		return SeverityNone
	}
	switch line[0] {
	case 'E', 'X': // error, emergency
		return SeverityError
	case 'W':
		return SeverityWarning
	case 'I', 'N': // info, notice
		return SeverityInfo
	case 'D', 'd': // debug, debug2
		return SeverityDebug
	default:
		return SeverityNone
	}
}
