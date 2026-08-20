package cups

import (
	"strings"
	"testing"
)

// The two URIs a real HP Smart Tank answers on. Both reach the same printer, so
// the list has to say what tells them apart.
const (
	hpDNSSD = "dnssd://HP%20Smart%20Tank%20510%20series%20%5BA28FDF%5D._ipp._tcp.local/?uuid=61133018-64d6-5f60-9f80-9de577e48349"
	hpIPPS  = "ipps://HP%20Smart%20Tank%20510%20series%20%5BA28FDF%5D._ipps._tcp.local/"
)

func TestDeviceChoicesPutTheEncryptedConnectionFirst(t *testing.T) {
	choices := DeviceChoices([]Device{
		{URI: hpDNSSD, MakeModel: "HP Smart Tank 510 series"},
		{URI: hpIPPS, MakeModel: "HP Smart Tank 510 series"},
	})

	if len(choices) != 2 {
		t.Fatalf("%d choices, want 2", len(choices))
	}
	if choices[0].URI != hpIPPS {
		t.Errorf("first choice is %q, want the ipps:// one", choices[0].URI)
	}
	if !choices[0].Recommended {
		t.Error("the best connection of a printer answering twice must be recommended")
	}
	if choices[1].Recommended {
		t.Error("only one connection per printer may be recommended")
	}
}

func TestDeviceChoicesDoNotRecommendWhenThereIsNoChoice(t *testing.T) {
	// A single connection needs no badge: there is nothing to pick between, and
	// a "recommended" on every row teaches the user to ignore it.
	choices := DeviceChoices([]Device{{URI: hpIPPS, MakeModel: "HP Smart Tank 510 series"}})
	if choices[0].Recommended {
		t.Error("a lone connection was marked recommended")
	}
}

func TestDeviceChoicesLabelTheTransport(t *testing.T) {
	cases := []struct {
		uri  string
		want string
	}{
		{hpIPPS, "IPP/TLS"},
		{hpDNSSD, "DNS-SD"},
		{"ipp://printer.local/ipp/print", "IPP"},
		{"usb://HP/Smart%20Tank?serial=A28FDF", "USB"},
		{"socket://192.168.0.50:9100", "socket"},
		{"lpd://192.168.0.56:515/PASSTHRU", "LPD"},
		{"smb://server/printer", "SMB"},
		{"dnssd://EPSON%20L3150._pdl-datastream._tcp.local/", "DNS-SD"},
		{"weirdbackend://somewhere/", "weirdbackend"},
	}
	for _, c := range cases {
		got := DeviceChoices([]Device{{URI: c.uri}})[0].Label
		if !strings.Contains(got, c.want) {
			t.Errorf("%s labelled %q, want it to mention %q", c.uri, got, c.want)
		}
	}
}

func TestDeviceChoicesSayWhetherTheConnectionIsEncrypted(t *testing.T) {
	got := DeviceChoices([]Device{{URI: hpIPPS}})[0].Label
	if !strings.Contains(got, "encrypted") || strings.Contains(got, "not encrypted") {
		t.Errorf("ipps:// labelled %q, want it called encrypted", got)
	}
	if got := DeviceChoices([]Device{{URI: hpDNSSD}})[0].Label; !strings.Contains(got, "not encrypted") {
		t.Errorf("the _ipp._tcp service is plain text, but it is labelled %q", got)
	}
}

func TestDeviceChoicesShowAReadableURI(t *testing.T) {
	got := DeviceChoices([]Device{{URI: hpIPPS}})[0].Display
	if strings.Contains(got, "%20") || strings.Contains(got, "%5B") {
		t.Errorf("Display keeps the percent-encoding: %q", got)
	}
	if !strings.Contains(got, "HP Smart Tank 510 series [A28FDF]") {
		t.Errorf("Display = %q, want the decoded service name", got)
	}
	if !strings.HasPrefix(got, "ipps://") {
		t.Errorf("Display = %q, want the scheme kept so the row stays copyable", got)
	}
}

func TestDeviceChoicesKeepOnePrintersConnectionsTogether(t *testing.T) {
	// Interleaving two printers' connections is what makes the list unreadable:
	// every row of one printer must sit next to its siblings.
	choices := DeviceChoices([]Device{
		{URI: "socket://192.168.0.50:9100", MakeModel: "EPSON L3150 Series"},
		{URI: hpDNSSD, MakeModel: "HP Smart Tank 510 series"},
		{URI: "ipp://epson.local/ipp/print", MakeModel: "EPSON L3150 Series"},
		{URI: hpIPPS, MakeModel: "HP Smart Tank 510 series"},
	})

	var order []string
	for _, c := range choices {
		order = append(order, c.MakeModel)
	}
	want := []string{
		"EPSON L3150 Series", "EPSON L3150 Series",
		"HP Smart Tank 510 series", "HP Smart Tank 510 series",
	}
	if strings.Join(order, "|") != strings.Join(want, "|") {
		t.Errorf("order = %v, want the printers grouped: %v", order, want)
	}
	if choices[0].URI != "ipp://epson.local/ipp/print" {
		t.Errorf("the Epson leads with %q, want IPP ahead of the raw socket", choices[0].URI)
	}
}

func TestDeviceChoicesGroupByWhatTheListShows(t *testing.T) {
	// deviceView falls back to device-info when there is no make and model, so
	// the grouping has to use the same name the user reads.
	choices := DeviceChoices([]Device{
		{URI: hpDNSSD, Info: "HP Smart Tank"},
		{URI: hpIPPS, Info: "HP Smart Tank"},
	})
	if !choices[0].Recommended {
		t.Error("two connections of one unnamed printer were not recognised as the same printer")
	}
}

func TestDeviceChoicesToleratesAnEmptyScan(t *testing.T) {
	if got := DeviceChoices(nil); len(got) != 0 {
		t.Errorf("DeviceChoices(nil) = %v, want empty", got)
	}
}
