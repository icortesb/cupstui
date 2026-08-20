package cups

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CheckStatus is how one preflight check came out.
type CheckStatus int

const (
	CheckRunning CheckStatus = iota
	CheckOK
	CheckWarn // usable, but something is missing
	CheckFail // the application cannot do its job
)

// CheckResult is the outcome of one check, with what to do about it.
type CheckResult struct {
	Name   string
	Status CheckStatus
	Detail string // what was found
	Hint   string // what to do, when something is wrong
}

// The checks, named up front so the screen can list them before it has any
// answers.
const (
	checkDaemon = "CUPS service"
	checkPrint  = "Printing tools"
	checkAdmin  = "Administrative access"
	checkDriver = "Printer drivers"
	checkLink   = "Connection"
)

// PreflightNames lists the checks in the order they are shown.
func PreflightNames() []string {
	return []string{checkDaemon, checkLink, checkPrint, checkAdmin, checkDriver}
}

// PreflightStatus folds the results into one status: anything still running
// keeps the whole set running, and otherwise the worst outcome wins. Whether
// every check has reported yet is the caller's business, so an empty set counts
// as still running.
func PreflightStatus(results []CheckResult) CheckStatus {
	if len(results) == 0 {
		return CheckRunning
	}

	worst := CheckOK
	for _, r := range results {
		if r.Status == CheckRunning {
			return CheckRunning
		}
		if r.Status > worst {
			worst = r.Status
		}
	}
	return worst
}

// Preflight runs every check. Each is independent, so they run together and
// results arrive as they finish.
func Preflight(ctx context.Context, c *Client, out chan<- CheckResult) {
	checks := []func(context.Context, *Client) CheckResult{
		checkService, checkLinkSecurity, checkTools, checkAdministrative, checkDrivers,
	}

	for _, check := range checks {
		go func(run func(context.Context, *Client) CheckResult) {
			select {
			case out <- run(ctx, c):
			case <-ctx.Done():
			}
		}(check)
	}
}

func checkService(ctx context.Context, c *Client) CheckResult {
	snap, err := c.Snapshot(ctx)
	if err != nil {
		return CheckResult{
			Name:   checkDaemon,
			Status: CheckFail,
			Detail: "not responding at " + c.Server().String(),
			Hint:   describe(err),
		}
	}
	return CheckResult{
		Name:   checkDaemon,
		Status: CheckOK,
		Detail: fmt.Sprintf("%s, %d %s configured", c.Server(), len(snap.Printers), plural(len(snap.Printers), "printer")),
	}
}

// checkLinkSecurity reports whether what crosses the network is protected. A
// local socket never leaves the machine, so there is nothing to protect.
func checkLinkSecurity(_ context.Context, c *Client) CheckResult {
	server := c.Server()

	switch {
	case server.Local:
		return CheckResult{Name: checkLink, Status: CheckOK, Detail: "local socket"}
	case !server.Encrypted():
		return CheckResult{
			Name:   checkLink,
			Status: CheckWarn,
			Detail: "not encrypted",
			Hint:   "everything, passwords included, crosses the network in the clear — set Encryption Required in ~/.cups/client.conf, or CUPS_ENCRYPTION=Required",
		}
	case server.AllowAnyRoot:
		return CheckResult{
			Name:   checkLink,
			Status: CheckWarn,
			Detail: "encrypted, certificate not checked",
			Hint:   "the server is taken at its word — anything able to answer in its place would be believed",
		}
	default:
		// This reports the configuration, not the outcome: whether the
		// certificate actually checks out shows up on the service check.
		return CheckResult{Name: checkLink, Status: CheckOK, Detail: "encrypted, certificate checked"}
	}
}

func checkTools(context.Context, *Client) CheckResult {
	var missing []string
	for _, tool := range []string{"lp", "lpadmin"} {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:   checkPrint,
			Status: CheckWarn,
			Detail: "missing " + strings.Join(missing, " and "),
			Hint:   "they ship with CUPS itself: install the cups package (cups-client on Debian and Ubuntu)",
		}
	}
	return CheckResult{Name: checkPrint, Status: CheckOK, Detail: "lp and lpadmin available"}
}

// checkAdministrative probes a real administrative operation rather than
// guessing from group membership: the group that grants it is named in
// cups-files.conf, which is not readable by an ordinary user.
func checkAdministrative(ctx context.Context, c *Client) CheckResult {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	_, err := c.Devices(ctx)
	if err == nil {
		return CheckResult{Name: checkAdmin, Status: CheckOK, Detail: "granted"}
	}

	var cerr *Error
	if errors.As(err, &cerr) && cerr.Kind == KindForbidden {
		return CheckResult{
			Name:   checkAdmin,
			Status: CheckWarn,
			Detail: "denied",
			Hint:   "reading works; enabling printers, quotas and removal need membership in the CUPS SystemGroup (wheel on Arch and Fedora, lpadmin on Debian and Ubuntu)",
		}
	}
	return unavailable(checkAdmin, err)
}

// unavailable reports a check that could not run. When the cause is the daemon
// being down, the remediation already appears on the service check and is left
// off here: repeating it on every dependent line buries the one that matters.
func unavailable(name string, err error) CheckResult {
	var cerr *Error
	if errors.As(err, &cerr) && (cerr.Kind == KindDaemonDown || cerr.Kind == KindUnreachable) {
		return CheckResult{
			Name:   name,
			Status: CheckWarn,
			Detail: "skipped, CUPS is not responding",
		}
	}
	return CheckResult{
		Name:   name,
		Status: CheckWarn,
		Detail: "could not be determined",
		Hint:   describe(err),
	}
}

func checkDrivers(ctx context.Context, c *Client) CheckResult {
	ppds, err := c.PPDs(ctx)
	if err != nil {
		return unavailable(checkDriver, err)
	}

	if len(ppds) == 0 {
		return CheckResult{
			Name:   checkDriver,
			Status: CheckWarn,
			Detail: "none installed",
			Hint:   "modern network printers work without one; for the rest, install a driver package such as gutenprint, hplip or your printer maker's",
		}
	}
	return CheckResult{
		Name:   checkDriver,
		Status: CheckOK,
		Detail: fmt.Sprintf("%d available", len(ppds)),
	}
}

// DriverHint is what to do when no installed driver matches a printer.
func DriverHint(model string) string {
	general := "Modern network printers work without a driver — pick IPP Everywhere. " +
		"Otherwise install a driver package, such as gutenprint or hplip."

	if make := makeOf(model); make != "" {
		return fmt.Sprintf("No installed driver mentions %s. ", make) + general
	}
	return general
}

// makeOf takes the manufacturer from a reported model. CUPS writes "Unknown"
// and generic labels when it has nothing, which are no use as a search term.
func makeOf(model string) string {
	fields := strings.Fields(model)
	if len(fields) == 0 {
		return ""
	}

	switch strings.ToLower(fields[0]) {
	case "unknown", "generic", "local", "raw":
		return ""
	}
	return fields[0]
}

func describe(err error) string {
	var cerr *Error
	if errors.As(err, &cerr) && cerr.Hint != "" {
		return cerr.Hint
	}
	return err.Error()
}

// plural adds the s to a counted noun.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
