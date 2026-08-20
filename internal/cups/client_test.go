package cups

import (
	"context"
	"errors"
	"io"
	"testing"

	ipp "github.com/phin1x/go-ipp"
)

// fakeAdapter implements ipp.Adapter so the client can be tested with no live CUPS.
type fakeAdapter struct {
	responses map[int16]*ipp.Response
	err       error
	// errs answers one operation with an error while the rest still work.
	errs map[int16]error
	seen []*ipp.Request
}

func (f *fakeAdapter) SendRequest(url string, req *ipp.Request, w io.Writer) (*ipp.Response, error) {
	return f.SendRequestContext(context.Background(), url, req, w)
}

func (f *fakeAdapter) SendRequestContext(_ context.Context, _ string, req *ipp.Request, _ io.Writer) (*ipp.Response, error) {
	f.seen = append(f.seen, req)
	if f.err != nil {
		return nil, f.err
	}
	if err, ok := f.errs[req.Operation]; ok {
		return nil, err
	}
	if resp, ok := f.responses[req.Operation]; ok {
		return resp, nil
	}
	return &ipp.Response{StatusCode: 0}, nil
}

func (f *fakeAdapter) GetHttpUri(namespace string, object interface{}) string { return "http://test/" }
func (f *fakeAdapter) TestConnection() error                                  { return f.err }

func (f *fakeAdapter) sentOperation(op int16) *ipp.Request {
	for _, r := range f.seen {
		if r.Operation == op {
			return r
		}
	}
	return nil
}

func printerResponse(names ...string) *ipp.Response {
	r := &ipp.Response{}
	for i, n := range names {
		r.PrinterAttributes = append(r.PrinterAttributes, attrs(map[string]interface{}{
			"printer-name":              n,
			"printer-state":             3 + i,
			"printer-is-accepting-jobs": true,
		}))
	}
	return r
}

func newTestClient(f *fakeAdapter) *Client {
	return newWithAdapter("icortes", f)
}

func TestSnapshotReturnsPrintersJobsAndDefault(t *testing.T) {
	f := &fakeAdapter{responses: map[int16]*ipp.Response{
		ipp.OperationCupsGetPrinters: printerResponse("Epson_L3150", "HP_LaserJet"),
		ipp.OperationGetJobs: {JobAttributes: []ipp.Attributes{
			attrs(map[string]interface{}{"job-id": 7, "job-name": "viejo.pdf", "job-state": 3}),
			attrs(map[string]interface{}{"job-id": 42, "job-name": "nuevo.pdf", "job-state": 5}),
		}},
		ipp.OperationCupsGetDefault: {PrinterAttributes: []ipp.Attributes{
			attrs(map[string]interface{}{"printer-name": "HP_LaserJet"}),
		}},
	}}

	snap, err := newTestClient(f).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.Printers) != 2 {
		t.Fatalf("Printers = %d, want 2", len(snap.Printers))
	}
	if snap.Printers[0].Name != "Epson_L3150" || snap.Printers[1].Name != "HP_LaserJet" {
		t.Errorf("printers must come sorted by name, got %v", snap.Printers)
	}
	if !snap.Printers[1].IsDefault {
		t.Error("HP_LaserJet should be marked as default")
	}
	if snap.Printers[0].IsDefault {
		t.Error("Epson_L3150 is not the default")
	}
	if snap.Default != "HP_LaserJet" {
		t.Errorf("Default = %q", snap.Default)
	}
	if len(snap.Jobs) != 2 || snap.Jobs[0].ID != 42 {
		t.Errorf("jobs must come newest first, got %v", snap.Jobs)
	}
}

func TestSnapshotSurfacesClassifiedError(t *testing.T) {
	f := &fakeAdapter{err: ipp.SocketNotFoundError}

	_, err := newTestClient(f).Snapshot(context.Background())
	var cerr *Error
	if !errors.As(err, &cerr) {
		t.Fatalf("want a *cups.Error, got %T (%v)", err, err)
	}
	if cerr.Kind != KindDaemonDown {
		t.Errorf("Kind = %v, want KindDaemonDown", cerr.Kind)
	}
}

func TestSnapshotDoesNotPanicOnMalformedResponse(t *testing.T) {
	// The library type asserts without checking: a response with no
	// printer-name would panic and take the whole interface down.
	f := &fakeAdapter{responses: map[int16]*ipp.Response{
		ipp.OperationCupsGetPrinters: {PrinterAttributes: []ipp.Attributes{
			attrs(map[string]interface{}{"printer-state": 3}),
		}},
	}}

	_, err := newTestClient(f).Snapshot(context.Background())
	if err == nil {
		t.Fatal("want an error, not a panic or a silent success")
	}
}

func TestCancelJobSendsCancelJobOperation(t *testing.T) {
	f := &fakeAdapter{}
	if err := newTestClient(f).CancelJob(42); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	req := f.sentOperation(ipp.OperationCancelJob)
	if req == nil {
		t.Fatal("Cancel-Job was not sent")
	}
	// The library identifies the job by job-uri, not by job-id.
	uri, _ := req.OperationAttributes[ipp.AttributeJobURI].(string)
	if printerNameFromURI(uri) != "42" {
		t.Errorf("job-uri = %q, want it to point at job 42", uri)
	}
}

func TestDisableAndEnablePrinterSendPauseAndResume(t *testing.T) {
	f := &fakeAdapter{}
	c := newTestClient(f)
	if err := c.DisablePrinter("Epson_L3150"); err != nil {
		t.Fatalf("DisablePrinter: %v", err)
	}
	if err := c.EnablePrinter("Epson_L3150"); err != nil {
		t.Fatalf("EnablePrinter: %v", err)
	}
	if f.sentOperation(ipp.OperationPausePrinter) == nil {
		t.Error("DisablePrinter should send Pause-Printer")
	}
	if f.sentOperation(ipp.OperationResumePrinter) == nil {
		t.Error("EnablePrinter should send Resume-Printer")
	}
}

func TestSetDefaultSendsCupsSetDefaultWithPrinterURI(t *testing.T) {
	f := &fakeAdapter{}
	if err := newTestClient(f).SetDefault("Epson_L3150"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	req := f.sentOperation(ipp.OperationCupsSetDefault)
	if req == nil {
		t.Fatal("CUPS-Set-Default was not sent")
	}
	uri, _ := req.OperationAttributes[ipp.AttributePrinterURI].(string)
	if printerNameFromURI(uri) != "Epson_L3150" {
		t.Errorf("printer-uri = %q, want it to point at Epson_L3150", uri)
	}
}

func TestActionErrorsAreClassified(t *testing.T) {
	f := &fakeAdapter{err: ipp.HTTPError{Code: 403}}
	var cerr *Error
	if err := newTestClient(f).DisablePrinter("x"); !errors.As(err, &cerr) || cerr.Kind != KindForbidden {
		t.Errorf("want KindForbidden, got %v", err)
	}
}

func TestHoldJobSendsHoldJobOperation(t *testing.T) {
	f := &fakeAdapter{}
	if err := newTestClient(f).HoldJob(42); err != nil {
		t.Fatalf("HoldJob: %v", err)
	}
	req := f.sentOperation(ipp.OperationHoldJob)
	if req == nil {
		t.Fatal("Hold-Job was not sent")
	}
	uri, _ := req.OperationAttributes[ipp.AttributeJobURI].(string)
	if printerNameFromURI(uri) != "42" {
		t.Errorf("job-uri = %q, want it to point at job 42", uri)
	}
}

func TestReleaseJobSendsReleaseJobOperation(t *testing.T) {
	f := &fakeAdapter{}
	if err := newTestClient(f).ReleaseJob(42); err != nil {
		t.Fatalf("ReleaseJob: %v", err)
	}
	if f.sentOperation(ipp.OperationReleaseJob) == nil {
		t.Fatal("Release-Job was not sent")
	}
}

func TestJobActionErrorsAreClassified(t *testing.T) {
	f := &fakeAdapter{err: ipp.HTTPError{Code: 403}}
	var cerr *Error
	if err := newTestClient(f).HoldJob(1); !errors.As(err, &cerr) || cerr.Kind != KindForbidden {
		t.Errorf("want KindForbidden, got %v", err)
	}

	f = &fakeAdapter{err: ipp.HTTPError{Code: 401}}
	if err := newTestClient(f).HoldJob(1); !errors.As(err, &cerr) || cerr.Kind != KindUnauthorized {
		t.Errorf("want KindUnauthorized, got %v", err)
	}
}

// TestSnapshotEmptyCUPS covers a machine with no printers yet. cupsd answers
// CUPS-Get-Printers with client-error-not-found rather than an empty list, and
// taking that at face value made the whole interface refuse to start on a
// fresh install — the one case where the user most needs it to.
func TestSnapshotEmptyCUPS(t *testing.T) {
	f := &fakeAdapter{errs: map[int16]error{
		ipp.OperationCupsGetPrinters: ipp.IPPError{Status: ipp.StatusErrorNotFound, Message: "No destinations added."},
		ipp.OperationCupsGetDefault:  ipp.IPPError{Status: ipp.StatusErrorNotFound, Message: "No default printer."},
	}}

	snap, err := newTestClient(f).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot on a CUPS with no printers: %v", err)
	}
	if len(snap.Printers) != 0 {
		t.Errorf("got %d printers, want none", len(snap.Printers))
	}
	if snap.Default != "" {
		t.Errorf("Default = %q, want empty", snap.Default)
	}
}
