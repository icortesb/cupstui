package cups

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Policy holds the quota and access rules of one printer.
//
// It is written with lpadmin rather than over IPP: CUPS stores the access list
// as a multi-valued attribute, which the IPP library available here cannot
// encode, and lpadmin already resets the opposite list when one is set.
type Policy struct {
	// PageLimit and KLimit are per quota period; zero means no limit.
	PageLimit int
	KLimit    int
	// QuotaDays is the length of the period. Zero disables the quota.
	QuotaDays int
	// AllowedUsers and DeniedUsers are mutually exclusive; CUPS keeps one list
	// per printer. Both empty means everyone may print.
	AllowedUsers []string
	DeniedUsers  []string
}

const secondsPerDay = 86400

// Summary is the one-line description shown next to the printer.
func (p Policy) Summary() string {
	var parts []string

	if p.PageLimit > 0 {
		parts = append(parts, fmt.Sprintf("%d pages", p.PageLimit))
	}
	if p.KLimit > 0 {
		parts = append(parts, fmt.Sprintf("%d kB", p.KLimit))
	}
	if len(parts) > 0 && p.QuotaDays > 0 {
		parts = append(parts, fmt.Sprintf("per %s", days(p.QuotaDays)))
	}

	switch {
	case len(p.AllowedUsers) > 0:
		parts = append(parts, fmt.Sprintf("%d user(s) allowed", len(p.AllowedUsers)))
	case len(p.DeniedUsers) > 0:
		parts = append(parts, fmt.Sprintf("%d user denied", len(p.DeniedUsers)))
	}

	if len(parts) == 0 {
		return "no limits"
	}
	return strings.Join(parts, " · ")
}

func days(n int) string {
	if n == 1 {
		return "day"
	}
	return strconv.Itoa(n) + " days"
}

// userNamePattern is what a CUPS access list accepts as one entry. A comma or a
// space would split into two entries and change who is allowed or denied.
var userNamePattern = regexp.MustCompile(`^[^\s,]+$`)

func (p Policy) validate() error {
	if len(p.AllowedUsers) > 0 && len(p.DeniedUsers) > 0 {
		return errors.New("a printer holds one access list: allow users or deny users, not both")
	}
	if p.PageLimit < 0 || p.KLimit < 0 || p.QuotaDays < 0 {
		return errors.New("limits cannot be negative")
	}
	for _, list := range [][]string{p.AllowedUsers, p.DeniedUsers} {
		for _, u := range list {
			if !userNamePattern.MatchString(u) {
				return fmt.Errorf("invalid user name %q: no spaces or commas", u)
			}
		}
	}
	return nil
}

// lpadminPolicyArgs builds the lpadmin arguments. Limits are always written,
// including zero, so that clearing a quota works: an omitted option leaves the
// previous value in place.
func lpadminPolicyArgs(printer string, p Policy) ([]string, error) {
	if err := ValidatePrinterName(printer); err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, err
	}

	args := []string{"-p", printer,
		"-o", "job-page-limit=" + strconv.Itoa(p.PageLimit),
		"-o", "job-k-limit=" + strconv.Itoa(p.KLimit),
		"-o", "job-quota-period=" + strconv.Itoa(p.QuotaDays*secondsPerDay),
	}

	switch {
	case len(p.AllowedUsers) > 0:
		args = append(args, "-u", "allow:"+strings.Join(p.AllowedUsers, ","))
	case len(p.DeniedUsers) > 0:
		args = append(args, "-u", "deny:"+strings.Join(p.DeniedUsers, ","))
	default:
		args = append(args, "-u", "allow:all")
	}
	return args, nil
}

// SetPolicy applies quota and access rules to a printer.
func SetPolicy(printer string, p Policy) error {
	args, err := lpadminPolicyArgs(printer, p)
	if err != nil {
		return &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
	}

	out, err := exec.Command("lpadmin", args...).CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return &Error{
				Kind: KindNotFound,
				Hint: "lpadmin command not found — install the cups package",
				Err:  err,
			}
		}
		return &Error{Kind: KindUnknown, Hint: lpError(out, err), Err: err}
	}
	return nil
}

// ParseUserList reads a comma or space separated list as typed in the interface.
func ParseUserList(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	if len(fields) == 0 {
		return nil
	}
	return fields
}
