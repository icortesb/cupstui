// Package cups speaks IPP to the local CUPS daemon.
//
// It uses IPP directly rather than shelling out to lp/lpstat because the output
// of those tools is localised and free-form, while IPP returns typed
// attributes. The transport is the unix socket (/run/cups/cups.sock): over it
// CUPS authenticates the local user, whereas the same request over TCP to
// localhost:631 answers 401.
package cups

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	ipp "github.com/phin1x/go-ipp"
)

// Client is a CUPS client safe for the interface: it never panics and always
// returns classified errors.
type Client struct {
	server  Server
	user    string
	adapter ipp.Adapter
	cups    *ipp.CUPSClient
	ipp     *ipp.IPPClient
}

// New connects to whichever CUPS the environment points at: CUPS_SERVER, then
// ServerName in the user's client.conf, and the local socket otherwise.
//
// A missing socket or an unreachable server is not an error here: the interface
// starts anyway, shows the notice and recovers on its own once the service
// returns.
func New() (*Client, error) {
	server := ResolveServer()

	var adapter *socketAdapter
	if server.Local {
		adapter = newSocketAdapter(server.Address)
	} else {
		adapter = newRemoteAdapter(server.Address, server.User, "", server)
	}

	c := newWithAdapter(server.User, adapter)
	c.server = server
	return c, nil
}

// Server is the CUPS this client talks to.
func (c *Client) Server() Server { return c.server }

// NeedsPassword reports whether the client is talking to a remote server that
// has not been given a password yet. A local socket never needs one: CUPS
// authenticates by the credentials of the connection.
func (c *Client) NeedsPassword() bool {
	if c.server.Local {
		return false
	}
	a, ok := c.adapter.(*socketAdapter)
	if !ok {
		return false
	}
	_, _, has := a.credentials()
	return !has
}

// SetPassword supplies the password for a remote server.
func (c *Client) SetPassword(password string) {
	if a, ok := c.adapter.(*socketAdapter); ok {
		a.setPassword(password)
	}
}

// Close releases the persistent connection to cupsd.
func (c *Client) Close() {
	if a, ok := c.adapter.(*socketAdapter); ok {
		a.close()
	}
}

func newWithAdapter(username string, a ipp.Adapter) *Client {
	return &Client{
		user:    username,
		adapter: a,
		cups:    ipp.NewCUPSClientWithAdapter(username, a),
		ipp:     ipp.NewIPPClientWithAdapter(username, a),
	}
}

// Snapshot is a complete picture of CUPS at one instant. The interface
// refreshes from a single one of these, so the views never drift apart.
type Snapshot struct {
	Printers []Printer
	Jobs     []Job
	Default  string
}

var printerAttributes = []string{
	"printer-name",
	"printer-state",
	"printer-state-reasons",
	"printer-state-message",
	"printer-is-accepting-jobs",
	"printer-info",
	"printer-location",
	"printer-make-and-model",
	"device-uri",
	"job-page-limit",
	"job-k-limit",
	"job-quota-period",
	"requesting-user-name-allowed",
	"requesting-user-name-denied",
}

var jobAttributes = []string{
	"job-id",
	"job-name",
	"job-originating-user-name",
	"job-printer-uri",
	"job-state",
	"time-at-creation",
	"job-k-octets",
	"job-media-sheets",
	"job-media-sheets-completed",
}

// Snapshot queries printers, queue and default in one pass.
func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	var snap Snapshot

	rawPrinters, err := c.getPrinters(ctx)
	if err != nil {
		return snap, classify(err)
	}
	for name, a := range rawPrinters {
		snap.Printers = append(snap.Printers, printerFromAttributes(name, a))
	}
	sort.Slice(snap.Printers, func(i, j int) bool { return snap.Printers[i].Name < snap.Printers[j].Name })

	rawJobs, err := c.getJobs(ctx)
	if err != nil {
		return snap, classify(err)
	}
	for id, a := range rawJobs {
		snap.Jobs = append(snap.Jobs, jobFromAttributes(id, a))
	}
	sortJobs(snap.Jobs)

	// The default is informational: if it fails, it is not worth discarding the
	// whole snapshot.
	if def, err := c.getDefault(ctx); err == nil {
		snap.Default = def
		for i := range snap.Printers {
			snap.Printers[i].IsDefault = snap.Printers[i].Name == def
		}
	}

	return snap, nil
}

func (c *Client) getPrinters(ctx context.Context) (printers map[string]ipp.Attributes, err error) {
	defer func() { err = recovered(recover(), err) }()
	return c.cups.GetPrintersContext(ctx, printerAttributes)
}

func (c *Client) getJobs(ctx context.Context) (jobs map[int]ipp.Attributes, err error) {
	defer func() { err = recovered(recover(), err) }()
	// whichJobs "not-completed" is the live queue; myJobs=false shows every
	// user's jobs.
	return c.ipp.GetJobsContext(ctx, "", "", "not-completed", false, 0, 0, jobAttributes)
}

func (c *Client) getDefault(ctx context.Context) (name string, err error) {
	defer func() { err = recovered(recover(), err) }()

	req := ipp.NewRequest(ipp.OperationCupsGetDefault, 1)
	req.OperationAttributes[ipp.AttributeRequestedAttributes] = []string{"printer-name"}

	resp, err := c.adapter.SendRequestContext(ctx, c.adapter.GetHttpUri("", nil), req, nil)
	if err != nil {
		return "", err
	}
	if len(resp.PrinterAttributes) == 0 {
		return "", fmt.Errorf("CUPS reported no default printer")
	}
	return attrString(resp.PrinterAttributes[0], "printer-name"), nil
}

// recovered turns a panic from the IPP library into an error. go-ipp type
// asserts server responses without checking, and a panic there would take the
// whole interface down.
func recovered(r interface{}, err error) error {
	if r == nil {
		return err
	}
	return fmt.Errorf("unexpected IPP response: %v", r)
}

// CancelJob cancels one job by id.
func (c *Client) CancelJob(id int) error {
	return classifyNil(c.ipp.CancelJob(id, false))
}

// CancelAllJobs cancels every job on a printer; with an empty printer, on all
// of them.
func (c *Client) CancelAllJobs(printer string) error {
	return classifyNil(c.ipp.CancelAllJob(printer, false))
}

// HoldJob holds a job: it stays queued but does not print.
func (c *Client) HoldJob(id int) error {
	return c.sendJob(ipp.OperationHoldJob, id)
}

// ReleaseJob returns a held job to the active queue.
func (c *Client) ReleaseJob(id int) error {
	return c.sendJob(ipp.OperationReleaseJob, id)
}

func (c *Client) sendJob(op int16, id int) (err error) {
	defer func() { err = classifyNil(recovered(recover(), err)) }()

	req := ipp.NewRequest(op, 1)
	req.OperationAttributes[ipp.AttributeJobURI] = jobURI(id)
	req.OperationAttributes[ipp.AttributeRequestingUserName] = c.user

	_, err = c.adapter.SendRequest(c.adapter.GetHttpUri("jobs", id), req, nil)
	return err
}

// EnablePrinter resumes a stopped printer, as cupsenable does.
func (c *Client) EnablePrinter(name string) error {
	return classifyNil(c.ipp.ResumePrinter(name))
}

// DisablePrinter stops a printer, as cupsdisable does. Jobs stay queued.
func (c *Client) DisablePrinter(name string) error {
	return classifyNil(c.ipp.PausePrinter(name))
}

// SetAccepting decides whether the printer takes new jobs.
func (c *Client) SetAccepting(name string, accepting bool) error {
	op := ipp.OperationCupsRejectJobs
	if accepting {
		op = ipp.OperationCupsAcceptJobs
	}
	return c.sendAdmin(op, name)
}

// SetDefault makes the printer the system default destination.
func (c *Client) SetDefault(name string) error {
	return c.sendAdmin(ipp.OperationCupsSetDefault, name)
}

func (c *Client) sendAdmin(op int16, printer string) (err error) {
	defer func() { err = classifyNil(recovered(recover(), err)) }()

	req := ipp.NewRequest(op, 1)
	req.OperationAttributes[ipp.AttributePrinterURI] = printerURI(printer)
	req.OperationAttributes[ipp.AttributeRequestingUserName] = c.user

	_, err = c.adapter.SendRequest(c.adapter.GetHttpUri("admin", nil), req, nil)
	return err
}

func printerURI(name string) string {
	return "ipp://localhost/printers/" + name
}

func jobURI(id int) string {
	return "ipp://localhost/jobs/" + strconv.Itoa(id)
}

func classifyNil(err error) error {
	if e := classify(err); e != nil {
		return e
	}
	return nil
}
