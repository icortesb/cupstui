package ui

import (
	"strings"
	"testing"

	"github.com/icortesb/cupstui/internal/cups"
)

func testAdd(devices ...cups.Device) addModel {
	a := newAdd()
	a.setSize(100, 24)
	a.active = true
	a.setDevices(devices)
	return a
}

// The header numbers the steps, and skipping one reads as a lost screen.
func TestStepNumbersFollowThePathTaken(t *testing.T) {
	a := testAdd(cups.Device{URI: "ipps://hp.local/", MakeModel: "HP Smart Tank 510 series"})

	if got := a.view(); !strings.Contains(got, "step 1 of 3") {
		t.Fatalf("picking a device is step 1 of 3, got:\n%s", got)
	}

	a.advance(nil) // take the discovered printer
	if a.step != stepDriver {
		t.Fatalf("step = %v, want stepDriver", a.step)
	}
	if got := a.view(); !strings.Contains(got, "step 2 of 3") {
		t.Errorf("the driver step must follow step 1, got:\n%s", got)
	}

	a.advance(nil) // take the driverless option
	if got := a.view(); !strings.Contains(got, "step 3 of 3") {
		t.Errorf("the details step is step 3 of 3, got:\n%s", got)
	}
}

func TestTypingAURIByHandStaysOnStepOne(t *testing.T) {
	// Manual entry is how step 1 is finished when the scan came up empty, not a
	// step of its own.
	a := testAdd()

	a.advance(nil)
	if a.step != stepURI {
		t.Fatalf("step = %v, want stepURI", a.step)
	}
	if got := a.view(); !strings.Contains(got, "step 1 of 3") {
		t.Errorf("entering a URI by hand is still step 1, got:\n%s", got)
	}
}

func TestDeviceListTellsTheConnectionsApart(t *testing.T) {
	a := testAdd(
		cups.Device{URI: "dnssd://HP%20Smart%20Tank%20510%20series._ipp._tcp.local/?uuid=61133018", MakeModel: "HP Smart Tank 510 series"},
		cups.Device{URI: "ipps://HP%20Smart%20Tank%20510%20series._ipps._tcp.local/", MakeModel: "HP Smart Tank 510 series"},
	)

	got := a.view()
	for _, want := range []string{"IPP/TLS", "DNS-SD", "recommended"} {
		if !strings.Contains(got, want) {
			t.Errorf("the device list never says %q, so both rows read the same:\n%s", want, got)
		}
	}
}

func TestDeviceListShowsTheURIOfTheHighlightedRowOnly(t *testing.T) {
	a := testAdd(
		cups.Device{URI: "ipps://encrypted.local/", MakeModel: "HP Smart Tank 510 series"},
		cups.Device{URI: "socket://192.168.0.50:9100", MakeModel: "HP Smart Tank 510 series"},
	)

	got := a.view()
	if !strings.Contains(got, "ipps://encrypted.local/") {
		t.Errorf("the highlighted row hides its URI:\n%s", got)
	}
	if strings.Contains(got, "socket://192.168.0.50:9100") {
		t.Errorf("only the highlighted row should spell out its URI:\n%s", got)
	}

	a.devCursor = 1
	if got := a.view(); !strings.Contains(got, "socket://192.168.0.50:9100") {
		t.Errorf("moving the cursor did not reveal the other URI:\n%s", got)
	}
}

func TestDeviceListDecodesTheURI(t *testing.T) {
	a := testAdd(cups.Device{
		URI:       "ipps://HP%20Smart%20Tank%20510%20series%20%5BA28FDF%5D._ipps._tcp.local/",
		MakeModel: "HP Smart Tank 510 series",
	})
	if got := a.view(); strings.Contains(got, "%20") {
		t.Errorf("the URI is shown percent-encoded:\n%s", got)
	}
}

func TestPickingADeviceCarriesItsURIToTheEnd(t *testing.T) {
	// Reordering the scan results must not detach a row from its URI.
	a := testAdd(
		cups.Device{URI: "socket://192.168.0.50:9100", MakeModel: "HP Smart Tank 510 series"},
		cups.Device{URI: "ipps://hp.local/", MakeModel: "HP Smart Tank 510 series"},
	)
	a.devCursor = 1
	a.advance(nil)

	if a.chosenURI != "socket://192.168.0.50:9100" {
		t.Errorf("chosenURI = %q, want the socket row the cursor was on", a.chosenURI)
	}
	if a.name.Value() != "HP_Smart_Tank_510_series" {
		t.Errorf("suggested name = %q", a.name.Value())
	}
}

func TestDeviceListStaysInsideTheScreen(t *testing.T) {
	// The second line under the highlighted row eats into the window; a list
	// that outgrows the screen scrolls the header away.
	var devices []cups.Device
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		devices = append(devices, cups.Device{URI: "socket://" + s + ":9100", MakeModel: "Printer " + s})
	}
	a := testAdd(devices...)
	a.setSize(100, 14)

	if lines := strings.Count(a.view(), "\n"); lines > 14 {
		t.Errorf("the device step draws %d lines into a 14-line screen:\n%s", lines, a.view())
	}
}
