package cups

import (
	"context"
	"strings"
)

// The rows a printer diagnosis reports, in the order they are shown.
const (
	checkPrinterExists = "Printer"
	checkQueue         = "Queue"
	checkAccepting     = "Accepting jobs"
	checkConnection    = "Device connection"
	checkPrinterState  = "Printer state"
	checkDriverName    = "Driver"
	checkRecentJobs    = "Recent jobs"
)

// PrinterDiagnosis is the outcome of diagnosing one printer.
type PrinterDiagnosis struct {
	Printer string
	Checks  []CheckResult
	Overall CheckStatus
}

// DiagnosePrinter takes a name rather than a Printer value and re-reads its
// live state, same as checkAdministrative: the queue may have changed since
// it was last listed.
func DiagnosePrinter(ctx context.Context, c *Client, name string) (PrinterDiagnosis, error) {
	snap, snapErr := c.Snapshot(ctx)
	printer, found := findPrinter(snap.Printers, name)
	jobs, jobsErr := c.RecentJobs(ctx, name, 3)

	d := PrinterDiagnosis{
		Printer: name,
		Checks: []CheckResult{
			snapshotCheckResult(c, snap, snapErr),
			printerExistsCheck(snapErr, found, printer),
			queueCheck(snapErr, found, printer),
			acceptingCheck(snapErr, found, printer),
			connectionCheck(snapErr, found, printer),
			stateCheck(snapErr, found, printer),
			driverCheck(snapErr, found, printer, jobsErr, jobs),
			recentJobsCheck(jobsErr, jobs),
		},
	}
	d.Overall = PreflightStatus(d.Checks)
	return d, ctx.Err()
}

func findPrinter(printers []Printer, name string) (Printer, bool) {
	for _, p := range printers {
		if p.Name == name {
			return p, true
		}
	}
	return Printer{}, false
}

func printerExistsCheck(snapErr error, found bool, p Printer) CheckResult {
	switch {
	case snapErr != nil:
		return unavailable(checkPrinterExists, snapErr)
	case !found:
		return CheckResult{Name: checkPrinterExists, Status: CheckFail, Detail: "no longer exists on this server"}
	default:
		detail := p.MakeModel
		if detail == "" {
			detail = "recognised"
		}
		return CheckResult{Name: checkPrinterExists, Status: CheckOK, Detail: detail}
	}
}

func queueCheck(snapErr error, found bool, p Printer) CheckResult {
	switch {
	case snapErr != nil:
		return unavailable(checkQueue, snapErr)
	case !found:
		return CheckResult{Name: checkQueue, Status: CheckWarn, Detail: "skipped, printer not found"}
	case p.State == StateStopped:
		return CheckResult{Name: checkQueue, Status: CheckFail, Detail: "disabled", Hint: "press e on the Printers screen to enable it"}
	default:
		return CheckResult{Name: checkQueue, Status: CheckOK, Detail: "enabled"}
	}
}

func acceptingCheck(snapErr error, found bool, p Printer) CheckResult {
	switch {
	case snapErr != nil:
		return unavailable(checkAccepting, snapErr)
	case !found:
		return CheckResult{Name: checkAccepting, Status: CheckWarn, Detail: "skipped, printer not found"}
	case !p.Accepting:
		return CheckResult{Name: checkAccepting, Status: CheckWarn, Detail: "rejecting new jobs", Hint: "press a on the Printers screen to resume"}
	default:
		return CheckResult{Name: checkAccepting, Status: CheckOK, Detail: "accepting jobs"}
	}
}

func connectionCheck(snapErr error, found bool, p Printer) CheckResult {
	switch {
	case snapErr != nil:
		return unavailable(checkConnection, snapErr)
	case !found:
		return CheckResult{Name: checkConnection, Status: CheckWarn, Detail: "skipped, printer not found"}
	}
	if text, status, ok := worstReason(p.Reasons, bucketConnection); ok {
		return CheckResult{Name: checkConnection, Status: status, Detail: text}
	}
	return CheckResult{Name: checkConnection, Status: CheckOK, Detail: "reachable"}
}

func stateCheck(snapErr error, found bool, p Printer) CheckResult {
	switch {
	case snapErr != nil:
		return unavailable(checkPrinterState, snapErr)
	case !found:
		return CheckResult{Name: checkPrinterState, Status: CheckWarn, Detail: "skipped, printer not found"}
	}
	if text, status, ok := worstReason(p.Reasons, bucketState); ok {
		return CheckResult{Name: checkPrinterState, Status: status, Detail: text}
	}
	if p.StateMessage != "" {
		return CheckResult{Name: checkPrinterState, Status: CheckOK, Detail: p.StateMessage}
	}
	return CheckResult{Name: checkPrinterState, Status: CheckOK, Detail: p.State.String()}
}

// driverCheck only fails on direct evidence, never on the absence of an
// internal/drivers recommendation — that knowledge base is advisory and
// composed in the UI, not fed into this check.
func driverCheck(snapErr error, found bool, p Printer, jobsErr error, jobs []Job) CheckResult {
	if found {
		if text, status, ok := worstReason(p.Reasons, bucketDriver); ok {
			return CheckResult{Name: checkDriverName, Status: status, Detail: text, Hint: "reinstall or reconfigure the printer's driver"}
		}
	}
	if jobsErr == nil && len(jobs) > 0 {
		if text, status, ok := worstReason(jobs[0].Reasons, bucketDriver); ok {
			return CheckResult{Name: checkDriverName, Status: status, Detail: text, Hint: "reinstall or reconfigure the printer's driver"}
		}
	}
	if snapErr != nil && jobsErr != nil {
		return unavailable(checkDriverName, snapErr)
	}
	return CheckResult{Name: checkDriverName, Status: CheckOK, Detail: "no missing-filter errors reported"}
}

func recentJobsCheck(jobsErr error, jobs []Job) CheckResult {
	if jobsErr != nil {
		return unavailable(checkRecentJobs, jobsErr)
	}
	if len(jobs) == 0 {
		return CheckResult{Name: checkRecentJobs, Status: CheckOK, Detail: "no recent jobs"}
	}

	last := jobs[0] // RecentJobs sorts newest first
	when := last.Created.Local().Format("Jan 2 15:04")

	switch last.State {
	case JobCompleted:
		return CheckResult{Name: checkRecentJobs, Status: CheckOK, Detail: "completed " + when}
	case JobAborted:
		detail := "aborted " + when
		if text, _, ok := worstOfAll(last.Reasons); ok {
			detail = text + ", " + when
		}
		return CheckResult{Name: checkRecentJobs, Status: CheckFail, Detail: detail, Hint: "check the driver and the document format"}
	case JobCanceled:
		if containsReason(last.Reasons, "job-canceled-by-user") {
			return CheckResult{Name: checkRecentJobs, Status: CheckOK, Detail: "canceled by the user " + when}
		}
		detail := "canceled " + when
		if text, _, ok := worstOfAll(last.Reasons); ok {
			detail = text + ", " + when
		}
		return CheckResult{Name: checkRecentJobs, Status: CheckWarn, Detail: detail}
	default:
		return CheckResult{Name: checkRecentJobs, Status: CheckOK, Detail: last.State.String() + " " + when}
	}
}

// The buckets a state reason falls into, so it is reported under one row
// rather than repeated across several.
const (
	bucketConnection = "connection"
	bucketDriver     = "driver"
	bucketState      = "state"
)

func reasonBucket(reason string) string {
	switch {
	case strings.Contains(reason, "missing-filter"), strings.Contains(reason, "document-format"), strings.Contains(reason, "compression"):
		return bucketDriver
	case strings.Contains(reason, "offline"), strings.Contains(reason, "connecting-to-device"), strings.Contains(reason, "timed-out"):
		return bucketConnection
	default:
		return bucketState
	}
}

// criticalReports are "-report" reasons serious enough to fail the check
// even though that suffix is otherwise informational — cupsd uses
// offline-report for a device that stopped answering entirely, not for
// routine status.
var criticalReports = map[string]bool{
	"offline-report": true,
}

// reasonSeverity reads the suffix cupsd's own reason codes always carry
// ("-error", "-warning", "-report"); anything shaped differently is treated
// as a warning rather than silently as fine.
func reasonSeverity(reason string) CheckStatus {
	switch {
	case strings.HasSuffix(reason, "-error"), criticalReports[reason]:
		return CheckFail
	default:
		return CheckWarn
	}
}

// reasonText covers the common codes; explainReason falls back to a
// readable version of the code itself for the rest, never the raw attribute.
var reasonText = map[string]string{
	"media-empty":          "out of paper",
	"media-jam":            "a paper jam",
	"media-low":            "low on paper",
	"toner-empty":          "out of toner or ink",
	"toner-low":            "low on toner or ink",
	"cover-open":           "a cover is open",
	"door-open":            "a door is open",
	"offline-report":       "the printer did not respond over its connection",
	"paused":               "paused",
	"cups-missing-filter":  "the filter needed to process jobs is missing",
	"connecting-to-device": "still trying to reach the printer",
}

func explainReason(reason string) string {
	base := strings.TrimSuffix(reason, "-error")
	base = strings.TrimSuffix(base, "-warning")
	base = strings.TrimSuffix(base, "-report")
	if text, ok := reasonText[base]; ok {
		return text
	}
	return strings.ReplaceAll(base, "-", " ")
}

func worstReason(reasons []string, bucket string) (text string, status CheckStatus, ok bool) {
	var filtered []string
	for _, r := range reasons {
		if reasonBucket(r) == bucket {
			filtered = append(filtered, r)
		}
	}
	return worstOfAll(filtered)
}

// worstOfAll ignores the bucket, for a job's own failure, where any cause is relevant.
func worstOfAll(reasons []string) (text string, status CheckStatus, ok bool) {
	for _, r := range reasons {
		if s := reasonSeverity(r); !ok || s > status {
			status, text, ok = s, explainReason(r), true
		}
	}
	return text, status, ok
}

func containsReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
