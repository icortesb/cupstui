package cups

import ipp "github.com/phin1x/go-ipp"

// PrinterState is the IPP printer-state (RFC 8011 §5.4.11), normalised.
type PrinterState int

const (
	StateUnknown PrinterState = iota
	StateIdle
	StatePrinting
	StateStopped
)

func (s PrinterState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StatePrinting:
		return "printing"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Printer is a CUPS printer as the interface shows it.
type Printer struct {
	Name         string
	Info         string
	Location     string
	MakeModel    string
	DeviceURI    string
	State        PrinterState
	StateMessage string
	Reasons      []string
	Accepting    bool
	IsDefault    bool
	Policy       Policy
}

func printerFromAttributes(name string, a ipp.Attributes) Printer {
	p := Printer{
		Name:         name,
		Info:         attrString(a, "printer-info"),
		Location:     attrString(a, "printer-location"),
		MakeModel:    attrString(a, "printer-make-and-model"),
		DeviceURI:    attrString(a, "device-uri"),
		StateMessage: attrString(a, "printer-state-message"),
		Accepting:    attrBool(a, "printer-is-accepting-jobs"),
	}

	switch attrInt(a, "printer-state") {
	case 3:
		p.State = StateIdle
	case 4:
		p.State = StatePrinting
	case 5:
		p.State = StateStopped
	}

	p.Policy = Policy{
		PageLimit:    attrInt(a, "job-page-limit"),
		KLimit:       attrInt(a, "job-k-limit"),
		QuotaDays:    attrInt(a, "job-quota-period") / secondsPerDay,
		AllowedUsers: attrStrings(a, "requesting-user-name-allowed"),
		DeniedUsers:  attrStrings(a, "requesting-user-name-denied"),
	}

	for _, r := range attrStrings(a, "printer-state-reasons") {
		// CUPS sends "none" when there is nothing to report.
		if r != "" && r != "none" {
			p.Reasons = append(p.Reasons, r)
		}
	}
	return p
}
