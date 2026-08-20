package cups

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	ipp "github.com/phin1x/go-ipp"
)

// Device is a device CUPS found while scanning.
type Device struct {
	URI       string
	Info      string
	MakeModel string
	Class     string
}

// PPD is a PostScript Printer Description file, that is, a driver.
type PPD struct {
	Name      string
	MakeModel string
}

// DriverlessPPD is the "driver" of modern IPP printers, which describe
// themselves and need no PPD from the maker.
const DriverlessPPD = "everywhere"

// Devices scans for printers. It is slow — CUPS probes the network — so call
// it with a deadline and show that something is happening.
func (c *Client) Devices(ctx context.Context) (devices []Device, err error) {
	defer func() { err = classifyNil(recovered(recover(), err)) }()

	raw, err := c.cups.GetDevicesContext(ctx)
	if err != nil {
		return nil, err
	}

	for uri, a := range raw {
		// CUPS also returns the available backends ("smb", "lpd", "beh") as if
		// they were devices. Without a complete URI there is nothing to add,
		// so they are dropped.
		if !strings.Contains(uri, "://") {
			continue
		}
		devices = append(devices, Device{
			URI:       uri,
			Info:      attrString(a, "device-info"),
			MakeModel: attrString(a, "device-make-and-model"),
			Class:     attrString(a, "device-class"),
		})
	}

	sort.Slice(devices, func(i, j int) bool { return devices[i].URI < devices[j].URI })
	return devices, nil
}

// PPDs returns the installed drivers.
func (c *Client) PPDs(ctx context.Context) (ppds []PPD, err error) {
	defer func() { err = classifyNil(recovered(recover(), err)) }()

	raw, err := c.cups.GetPPDsContext(ctx)
	if err != nil {
		return nil, err
	}
	for name, a := range raw {
		ppds = append(ppds, PPD{Name: name, MakeModel: attrString(a, "ppd-make-and-model")})
	}
	sort.Slice(ppds, func(i, j int) bool { return ppds[i].MakeModel < ppds[j].MakeModel })
	return ppds, nil
}

// MatchPPDs orders the drivers by how close they are to the query and drops
// the unrelated ones. The query is usually the model the device reported, which
// puts the right driver on top.
func MatchPPDs(ppds []PPD, query string) []PPD {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return ppds
	}

	type scored struct {
		ppd   PPD
		score int
	}
	var out []scored
	for _, p := range ppds {
		haystack := strings.ToLower(p.MakeModel + " " + p.Name)
		var score int
		for _, w := range words {
			if strings.Contains(haystack, w) {
				score++
			}
		}
		if score > 0 {
			out = append(out, scored{p, score})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	result := make([]PPD, 0, len(out))
	for _, s := range out {
		result = append(result, s.ppd)
	}
	return result
}

// printerNamePattern is what CUPS accepts as a queue name: no spaces, no
// slashes and no hash.
var printerNamePattern = regexp.MustCompile(`^[^\s/#]+$`)

// ValidatePrinterName checks the name before sending it to CUPS, so the user
// gets a clear message instead of a protocol error.
func ValidatePrinterName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("a printer name is required")
	case len(name) > 127:
		return fmt.Errorf("name must be 127 characters or fewer")
	case !printerNamePattern.MatchString(name):
		return fmt.Errorf("name cannot contain spaces or the / and # characters")
	}
	return nil
}

// NewPrinterSpec describes the printer to be created.
type NewPrinterSpec struct {
	Name      string
	DeviceURI string
	PPD       string
	Info      string
	Location  string
}

// AddPrinter creates a printer, enabled and accepting jobs.
func (c *Client) AddPrinter(ctx context.Context, spec NewPrinterSpec) (err error) {
	if err := ValidatePrinterName(spec.Name); err != nil {
		return &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
	}
	if spec.DeviceURI == "" {
		return &Error{Kind: KindUnknown, Hint: "select a device or enter its URI"}
	}

	defer func() { err = classifyNil(recovered(recover(), err)) }()

	req := ipp.NewRequest(ipp.OperationCupsAddModifyPrinter, 1)
	req.OperationAttributes[ipp.AttributePrinterURI] = printerURI(spec.Name)
	req.OperationAttributes[ipp.AttributeRequestingUserName] = c.user
	if spec.PPD != "" {
		req.OperationAttributes[ipp.AttributePPDName] = spec.PPD
	}
	req.PrinterAttributes[ipp.AttributeDeviceURI] = spec.DeviceURI
	req.PrinterAttributes[ipp.AttributePrinterInfo] = spec.Info
	req.PrinterAttributes[ipp.AttributePrinterLocation] = spec.Location
	// Without these the printer is created stopped and rejecting jobs.
	req.PrinterAttributes[ipp.AttributePrinterIsAcceptingJobs] = true
	req.PrinterAttributes[ipp.AttributePrinterState] = 3 // idle

	_, err = c.adapter.SendRequestContext(ctx, c.adapter.GetHttpUri("admin", nil), req, nil)
	return err
}

// DeletePrinter removes a print queue and any jobs still on it.
func (c *Client) DeletePrinter(ctx context.Context, name string) (err error) {
	if err := ValidatePrinterName(name); err != nil {
		return &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
	}

	defer func() { err = classifyNil(recovered(recover(), err)) }()

	req := ipp.NewRequest(ipp.OperationCupsDeletePrinter, 1)
	req.OperationAttributes[ipp.AttributePrinterURI] = printerURI(name)
	req.OperationAttributes[ipp.AttributeRequestingUserName] = c.user

	_, err = c.adapter.SendRequestContext(ctx, c.adapter.GetHttpUri("admin", nil), req, nil)
	return err
}
