package cups

import (
	"testing"

	ipp "github.com/phin1x/go-ipp"
)

// attrs arma un ipp.Attributes como el que devuelve CUPS: cada clave mapea a
// una lista de atributos con Tag/Name/Value.
func attrs(kv map[string]interface{}) ipp.Attributes {
	a := ipp.Attributes{}
	for k, v := range kv {
		a[k] = []ipp.Attribute{{Name: k, Value: v}}
	}
	return a
}

func TestPrinterFromAttributes(t *testing.T) {
	p := printerFromAttributes("Epson_L3150", attrs(map[string]interface{}{
		"printer-state":             3,
		"printer-state-reasons":     "none",
		"printer-state-message":     "",
		"printer-is-accepting-jobs": true,
		"printer-info":              "Epson L3150 WiFi",
		"printer-location":          "Escritorio",
		"printer-make-and-model":    "EPSON L3150 Series",
		"device-uri":                "socket://EPSON7FCD9B.local:9100",
	}))

	if p.Name != "Epson_L3150" {
		t.Errorf("Name = %q, want Epson_L3150", p.Name)
	}
	if p.State != StateIdle {
		t.Errorf("State = %v, want StateIdle", p.State)
	}
	if !p.Accepting {
		t.Error("Accepting = false, want true")
	}
	if p.Info != "Epson L3150 WiFi" {
		t.Errorf("Info = %q", p.Info)
	}
	if p.Location != "Escritorio" {
		t.Errorf("Location = %q", p.Location)
	}
	if p.MakeModel != "EPSON L3150 Series" {
		t.Errorf("MakeModel = %q", p.MakeModel)
	}
	if p.DeviceURI != "socket://EPSON7FCD9B.local:9100" {
		t.Errorf("DeviceURI = %q", p.DeviceURI)
	}
	if len(p.Reasons) != 0 {
		t.Errorf("Reasons = %v, want it empty porque 'none' no es un motivo real", p.Reasons)
	}
}

func TestPrinterStateMapping(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  PrinterState
	}{
		{"idle", 3, StateIdle},
		{"processing", 4, StatePrinting},
		{"stopped", 5, StateStopped},
		{"unknown value", 99, StateUnknown},
		{"missing attribute", nil, StateUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kv := map[string]interface{}{}
			if c.value != nil {
				kv["printer-state"] = c.value
			}
			if got := printerFromAttributes("x", attrs(kv)).State; got != c.want {
				t.Errorf("State = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPrinterReasonsIgnoresNone(t *testing.T) {
	p := printerFromAttributes("x", ipp.Attributes{
		"printer-state-reasons": []ipp.Attribute{
			{Name: "printer-state-reasons", Value: "media-empty-warning"},
			{Name: "printer-state-reasons", Value: "offline-report"},
		},
	})
	want := []string{"media-empty-warning", "offline-report"}
	if len(p.Reasons) != 2 || p.Reasons[0] != want[0] || p.Reasons[1] != want[1] {
		t.Errorf("Reasons = %v, want %v", p.Reasons, want)
	}
}

// ipAttrs builds a multi-valued attribute, the shape CUPS uses for user lists.
func ipAttrs(key string, values ...string) ipp.Attributes {
	a := ipp.Attributes{}
	for _, v := range values {
		a[key] = append(a[key], ipp.Attribute{Name: key, Value: v})
	}
	return a
}
