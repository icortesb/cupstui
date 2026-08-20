package cups

import (
	"strings"
	"testing"

	ipp "github.com/phin1x/go-ipp"
)

func deviceResponse(uris ...string) *ipp.Response {
	r := &ipp.Response{}
	for _, u := range uris {
		r.PrinterAttributes = append(r.PrinterAttributes, attrs(map[string]interface{}{
			"device-uri":            u,
			"device-info":           "info de " + u,
			"device-make-and-model": "EPSON L3150 Series",
			"device-class":          "network",
		}))
	}
	return r
}

func TestDevicesDropBareBackends(t *testing.T) {
	// CUPS devuelve los backends disponibles (smb, http, lpd, beh) mezclados
	// con los dispositivos que encontró. Un backend pelado no se puede agregar
	// como impresora, así que no tiene por qué aparecer en la lista.
	f := &fakeAdapter{responses: map[int16]*ipp.Response{
		ipp.OperationCupsGetDevices: deviceResponse(
			"smb", "http", "beh",
			"dnssd://EPSON%20L3150._pdl-datastream._tcp.local/",
			"lpd://192.168.0.56:515/PASSTHRU",
			"socket://192.168.0.56:9100",
		),
	}}

	devs, err := newTestClient(f).Devices(t.Context())
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devs) != 3 {
		t.Fatalf("quedaron %d dispositivos, quiero 3: %+v", len(devs), devs)
	}
	for _, d := range devs {
		if !strings.Contains(d.URI, "://") {
			t.Errorf("se coló un backend pelado: %q", d.URI)
		}
	}
}

func TestDevicesComeSortedByURI(t *testing.T) {
	f := &fakeAdapter{responses: map[int16]*ipp.Response{
		ipp.OperationCupsGetDevices: deviceResponse(
			"socket://b:9100", "dnssd://a/", "lpd://c:515/x",
		),
	}}
	devs, _ := newTestClient(f).Devices(t.Context())
	for i := 1; i < len(devs); i++ {
		if devs[i-1].URI > devs[i].URI {
			t.Errorf("desordenado: %q antes de %q", devs[i-1].URI, devs[i].URI)
		}
	}
}

func TestMatchPPDsRanksTheClosestModelFirst(t *testing.T) {
	ppds := []PPD{
		{Name: "a.ppd", MakeModel: "Epson K200, Epson Inkjet Printer Driver"},
		{Name: "b.ppd", MakeModel: "EPSON L3150 Series, Epson Inkjet Printer Driver (ESC/P-R)"},
		{Name: "c.ppd", MakeModel: "HP LaserJet 1200"},
	}

	got := MatchPPDs(ppds, "EPSON L3150 Series")
	if len(got) == 0 || got[0].Name != "b.ppd" {
		t.Errorf("el primero debería ser b.ppd, tengo %+v", got)
	}
}

func TestMatchPPDsWithoutQueryReturnsEverything(t *testing.T) {
	ppds := []PPD{{Name: "a.ppd"}, {Name: "b.ppd"}}
	if got := MatchPPDs(ppds, ""); len(got) != 2 {
		t.Errorf("MatchPPDs sin consulta = %d, quiero 2", len(got))
	}
}

func TestPrinterNameValidation(t *testing.T) {
	invalid := []string{"", "con espacio", "con/barra", "con#numeral", strings.Repeat("x", 128)}
	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := ValidatePrinterName(name); err == nil {
				t.Errorf("%q debería rechazarse", name)
			}
		})
	}

	for _, name := range []string{"Epson_L3150", "hp-laserjet", "impresora2"} {
		if err := ValidatePrinterName(name); err != nil {
			t.Errorf("%q debería aceptarse: %v", name, err)
		}
	}
}

func TestAddPrinterSendsAddModifyWithDeviceAndDriver(t *testing.T) {
	f := &fakeAdapter{}
	err := newTestClient(f).AddPrinter(t.Context(), NewPrinterSpec{
		Name:      "Epson_nueva",
		DeviceURI: "socket://192.168.0.56:9100",
		PPD:       "lsb/usr/epson.ppd",
		Info:      "la del living",
		Location:  "living",
	})
	if err != nil {
		t.Fatalf("AddPrinter: %v", err)
	}

	req := f.sentOperation(ipp.OperationCupsAddModifyPrinter)
	if req == nil {
		t.Fatal("no se envió CUPS-Add-Modify-Printer")
	}
	if got, _ := req.OperationAttributes[ipp.AttributePPDName].(string); got != "lsb/usr/epson.ppd" {
		t.Errorf("ppd-name = %q", got)
	}
	if got, _ := req.PrinterAttributes[ipp.AttributeDeviceURI].(string); got != "socket://192.168.0.56:9100" {
		t.Errorf("device-uri = %q", got)
	}
	// Una impresora recién creada que no acepta trabajos ni está habilitada no
	// le sirve a nadie.
	if got := req.PrinterAttributes[ipp.AttributePrinterIsAcceptingJobs]; got != true {
		t.Errorf("printer-is-accepting-jobs = %v, quiero true", got)
	}
	if got := req.PrinterAttributes[ipp.AttributePrinterState]; got != 3 {
		t.Errorf("printer-state = %v, quiero 3 (idle)", got)
	}
}

func TestAddPrinterRejectsABadName(t *testing.T) {
	f := &fakeAdapter{}
	err := newTestClient(f).AddPrinter(t.Context(), NewPrinterSpec{
		Name: "nombre con espacios", DeviceURI: "socket://x:9100",
	})
	if err == nil {
		t.Fatal("quiero un error de validación")
	}
	if f.sentOperation(ipp.OperationCupsAddModifyPrinter) != nil {
		t.Error("no se tiene que enviar nada si el nombre es inválido")
	}
}

func TestAddPrinterRequiresADeviceURI(t *testing.T) {
	f := &fakeAdapter{}
	if err := newTestClient(f).AddPrinter(t.Context(), NewPrinterSpec{Name: "ok"}); err == nil {
		t.Fatal("quiero un error si falta la URI del dispositivo")
	}
}

func TestDeletePrinterSendsDeletePrinterOperation(t *testing.T) {
	f := &fakeAdapter{}
	if err := newTestClient(f).DeletePrinter(t.Context(), "Epson_L3150"); err != nil {
		t.Fatalf("DeletePrinter: %v", err)
	}

	req := f.sentOperation(ipp.OperationCupsDeletePrinter)
	if req == nil {
		t.Fatal("CUPS-Delete-Printer was not sent")
	}
	uri, _ := req.OperationAttributes[ipp.AttributePrinterURI].(string)
	if printerNameFromURI(uri) != "Epson_L3150" {
		t.Errorf("printer-uri = %q, want it to point at Epson_L3150", uri)
	}
}

func TestDeletePrinterRejectsAnEmptyName(t *testing.T) {
	// An empty name yields the URI of the printer collection rather than of one
	// printer, and CUPS would answer something unhelpful.
	f := &fakeAdapter{}
	if err := newTestClient(f).DeletePrinter(t.Context(), ""); err == nil {
		t.Fatal("want an error for an empty name")
	}
	if f.sentOperation(ipp.OperationCupsDeletePrinter) != nil {
		t.Error("nothing should be sent when the name is invalid")
	}
}

func TestDeletePrinterErrorsAreClassified(t *testing.T) {
	f := &fakeAdapter{err: ipp.HTTPError{Code: 403}}
	var cerr *Error
	if err := newTestClient(f).DeletePrinter(t.Context(), "x"); !asError(err, &cerr) || cerr.Kind != KindForbidden {
		t.Errorf("want KindForbidden, got %v", err)
	}
}
