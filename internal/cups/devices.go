package cups

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	ipp "github.com/phin1x/go-ipp"
)

// Device es un dispositivo que CUPS encontró al explorar.
type Device struct {
	URI       string
	Info      string
	MakeModel string
	Class     string
}

// PPD es un archivo de descripción de impresora, o sea un driver.
type PPD struct {
	Name      string
	MakeModel string
}

// DriverlessPPD es el "driver" de las impresoras IPP modernas, que se
// describen solas y no necesitan PPD del fabricante.
const DriverlessPPD = "everywhere"

// Devices explora en busca de impresoras. Es lento —CUPS sondea la red— así
// que conviene llamarlo con un contexto con plazo y mostrando que está
// trabajando.
func (c *Client) Devices(ctx context.Context) (devices []Device, err error) {
	defer func() { err = classifyNil(recovered(recover(), err)) }()

	raw, err := c.cups.GetDevicesContext(ctx)
	if err != nil {
		return nil, err
	}

	for uri, a := range raw {
		// CUPS devuelve también los backends disponibles ("smb", "lpd", "beh")
		// como si fueran dispositivos. Sin una URI completa no hay nada que
		// agregar, así que se descartan.
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

// PPDs devuelve los drivers instalados.
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

// MatchPPDs ordena los drivers por cercanía a la consulta y descarta los que no
// tienen nada que ver. La consulta suele ser el modelo que informó el
// dispositivo, así el driver correcto queda arriba.
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

// printerNamePattern es lo que CUPS acepta como nombre de cola: sin espacios,
// sin barras y sin numeral.
var printerNamePattern = regexp.MustCompile(`^[^\s/#]+$`)

// ValidatePrinterName comprueba el nombre antes de mandárselo a CUPS, para dar
// un mensaje claro en vez de un error de protocolo.
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

// NewPrinterSpec describe la impresora que se quiere dar de alta.
type NewPrinterSpec struct {
	Name      string
	DeviceURI string
	PPD       string
	Info      string
	Location  string
}

// AddPrinter da de alta una impresora, habilitada y aceptando trabajos.
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
	// Sin esto la impresora queda creada pero detenida y rechazando trabajos.
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
