package cups

import (
	"context"
	"errors"
	"io"
	"testing"

	ipp "github.com/phin1x/go-ipp"
)

// fakeAdapter implementa ipp.Adapter para probar el cliente sin un CUPS vivo.
type fakeAdapter struct {
	responses map[int16]*ipp.Response
	err       error
	seen      []*ipp.Request
}

func (f *fakeAdapter) SendRequest(url string, req *ipp.Request, w io.Writer) (*ipp.Response, error) {
	return f.SendRequestContext(context.Background(), url, req, w)
}

func (f *fakeAdapter) SendRequestContext(_ context.Context, _ string, req *ipp.Request, _ io.Writer) (*ipp.Response, error) {
	f.seen = append(f.seen, req)
	if f.err != nil {
		return nil, f.err
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
		t.Fatalf("Printers = %d, quiero 2", len(snap.Printers))
	}
	if snap.Printers[0].Name != "Epson_L3150" || snap.Printers[1].Name != "HP_LaserJet" {
		t.Errorf("las impresoras deben venir ordenadas por nombre, tengo %v", snap.Printers)
	}
	if !snap.Printers[1].IsDefault {
		t.Error("HP_LaserJet debería estar marcada como default")
	}
	if snap.Printers[0].IsDefault {
		t.Error("Epson_L3150 no es default")
	}
	if snap.Default != "HP_LaserJet" {
		t.Errorf("Default = %q", snap.Default)
	}
	if len(snap.Jobs) != 2 || snap.Jobs[0].ID != 42 {
		t.Errorf("los jobs deben venir ordenados del más nuevo al más viejo, tengo %v", snap.Jobs)
	}
}

func TestSnapshotSurfacesClassifiedError(t *testing.T) {
	f := &fakeAdapter{err: ipp.SocketNotFoundError}

	_, err := newTestClient(f).Snapshot(context.Background())
	var cerr *Error
	if !errors.As(err, &cerr) {
		t.Fatalf("quiero un *cups.Error, tengo %T (%v)", err, err)
	}
	if cerr.Kind != KindDaemonDown {
		t.Errorf("Kind = %v, quiero KindDaemonDown", cerr.Kind)
	}
}

func TestSnapshotDoesNotPanicOnMalformedResponse(t *testing.T) {
	// La librería hace type assertions sin chequear: una respuesta sin
	// printer-name la haría entrar en pánico y matar la TUI entera.
	f := &fakeAdapter{responses: map[int16]*ipp.Response{
		ipp.OperationCupsGetPrinters: {PrinterAttributes: []ipp.Attributes{
			attrs(map[string]interface{}{"printer-state": 3}),
		}},
	}}

	_, err := newTestClient(f).Snapshot(context.Background())
	if err == nil {
		t.Fatal("quiero un error, no un pánico ni un éxito silencioso")
	}
}

func TestCancelJobSendsCancelJobOperation(t *testing.T) {
	f := &fakeAdapter{}
	if err := newTestClient(f).CancelJob(42); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	req := f.sentOperation(ipp.OperationCancelJob)
	if req == nil {
		t.Fatal("no se envió Cancel-Job")
	}
	// La librería identifica el trabajo por job-uri, no por job-id.
	uri, _ := req.OperationAttributes[ipp.AttributeJobURI].(string)
	if printerNameFromURI(uri) != "42" {
		t.Errorf("job-uri = %q, quiero que apunte al trabajo 42", uri)
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
		t.Error("DisablePrinter debería mandar Pause-Printer")
	}
	if f.sentOperation(ipp.OperationResumePrinter) == nil {
		t.Error("EnablePrinter debería mandar Resume-Printer")
	}
}

func TestSetDefaultSendsCupsSetDefaultWithPrinterURI(t *testing.T) {
	f := &fakeAdapter{}
	if err := newTestClient(f).SetDefault("Epson_L3150"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	req := f.sentOperation(ipp.OperationCupsSetDefault)
	if req == nil {
		t.Fatal("no se envió CUPS-Set-Default")
	}
	uri, _ := req.OperationAttributes[ipp.AttributePrinterURI].(string)
	if printerNameFromURI(uri) != "Epson_L3150" {
		t.Errorf("printer-uri = %q, quiero que apunte a Epson_L3150", uri)
	}
}

func TestActionErrorsAreClassified(t *testing.T) {
	f := &fakeAdapter{err: ipp.HTTPError{Code: 403}}
	var cerr *Error
	if err := newTestClient(f).DisablePrinter("x"); !errors.As(err, &cerr) || cerr.Kind != KindForbidden {
		t.Errorf("quiero KindForbidden, tengo %v", err)
	}
}
