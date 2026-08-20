package cups

import ipp "github.com/phin1x/go-ipp"

// PrinterState es el printer-state de IPP (RFC 8011 §5.4.11) normalizado.
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
		return "inactiva"
	case StatePrinting:
		return "imprimiendo"
	case StateStopped:
		return "detenida"
	default:
		return "desconocido"
	}
}

// Printer es una impresora de CUPS tal como la muestra la UI.
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

	for _, r := range attrStrings(a, "printer-state-reasons") {
		// CUPS manda "none" cuando no hay nada que reportar.
		if r != "" && r != "none" {
			p.Reasons = append(p.Reasons, r)
		}
	}
	return p
}
