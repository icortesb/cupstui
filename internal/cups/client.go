// Package cups habla IPP con el demonio local de CUPS.
//
// Se usa IPP directo en lugar de shellear lp/lpstat porque la salida de esas
// herramientas está localizada y es de formato libre, mientras que IPP devuelve
// atributos tipados. El transporte es el socket unix (/run/cups/cups.sock): por
// ahí CUPS autentica al usuario local, mientras que el mismo pedido por
// TCP a localhost:631 responde 401.
package cups

import (
	"context"
	"fmt"
	"os/user"
	"sort"
	"strconv"

	ipp "github.com/phin1x/go-ipp"
)

// Client es un cliente de CUPS seguro para la UI: nunca entra en pánico y
// devuelve siempre errores clasificados.
type Client struct {
	user    string
	adapter ipp.Adapter
	cups    *ipp.CUPSClient
	ipp     *ipp.IPPClient
}

// New conecta con el CUPS local por su socket unix.
//
// Que el socket no exista todavía no es un error: la UI arranca igual, muestra
// el aviso y se recupera sola cuando el servicio vuelve, así que se apunta al
// path canónico y se deja fallar cada pedido.
func New() (*Client, error) {
	name := "root"
	if u, err := user.Current(); err == nil {
		name = u.Username
	}

	socket, err := findSocket(socketSearchPaths)
	if err != nil {
		socket = socketSearchPaths[0]
	}

	c := newWithAdapter(name, newSocketAdapter(socket))
	return c, nil
}

// Close libera la conexión persistente con cupsd.
func (c *Client) Close() {
	if a, ok := c.adapter.(*socketAdapter); ok {
		a.close()
	}
}

func newWithAdapter(username string, a ipp.Adapter) *Client {
	return &Client{
		user:    username,
		adapter: a,
		cups:    ipp.NewCUPSClientWithAdapter(username, a),
		ipp:     ipp.NewIPPClientWithAdapter(username, a),
	}
}

// Snapshot es una foto completa del estado de CUPS en un instante. La UI se
// refresca a partir de una sola de estas, así que las vistas nunca se
// desincronizan entre sí.
type Snapshot struct {
	Printers []Printer
	Jobs     []Job
	Default  string
}

var printerAttributes = []string{
	"printer-name",
	"printer-state",
	"printer-state-reasons",
	"printer-state-message",
	"printer-is-accepting-jobs",
	"printer-info",
	"printer-location",
	"printer-make-and-model",
	"device-uri",
}

var jobAttributes = []string{
	"job-id",
	"job-name",
	"job-originating-user-name",
	"job-printer-uri",
	"job-state",
	"time-at-creation",
	"job-k-octets",
}

// Snapshot consulta impresoras, cola y default en una sola pasada.
func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	var snap Snapshot

	rawPrinters, err := c.getPrinters(ctx)
	if err != nil {
		return snap, classify(err)
	}
	for name, a := range rawPrinters {
		snap.Printers = append(snap.Printers, printerFromAttributes(name, a))
	}
	sort.Slice(snap.Printers, func(i, j int) bool { return snap.Printers[i].Name < snap.Printers[j].Name })

	rawJobs, err := c.getJobs(ctx)
	if err != nil {
		return snap, classify(err)
	}
	for id, a := range rawJobs {
		snap.Jobs = append(snap.Jobs, jobFromAttributes(id, a))
	}
	sortJobs(snap.Jobs)

	// El default es informativo: si falla, no vale la pena tirar abajo toda
	// la foto.
	if def, err := c.getDefault(ctx); err == nil {
		snap.Default = def
		for i := range snap.Printers {
			snap.Printers[i].IsDefault = snap.Printers[i].Name == def
		}
	}

	return snap, nil
}

func (c *Client) getPrinters(ctx context.Context) (printers map[string]ipp.Attributes, err error) {
	defer func() { err = recovered(recover(), err) }()
	return c.cups.GetPrintersContext(ctx, printerAttributes)
}

func (c *Client) getJobs(ctx context.Context) (jobs map[int]ipp.Attributes, err error) {
	defer func() { err = recovered(recover(), err) }()
	// whichJobs "not-completed" es la cola real; myJobs=false para ver la de
	// todos los usuarios.
	return c.ipp.GetJobsContext(ctx, "", "", "not-completed", false, 0, 0, jobAttributes)
}

func (c *Client) getDefault(ctx context.Context) (name string, err error) {
	defer func() { err = recovered(recover(), err) }()

	req := ipp.NewRequest(ipp.OperationCupsGetDefault, 1)
	req.OperationAttributes[ipp.AttributeRequestedAttributes] = []string{"printer-name"}

	resp, err := c.adapter.SendRequestContext(ctx, c.adapter.GetHttpUri("", nil), req, nil)
	if err != nil {
		return "", err
	}
	if len(resp.PrinterAttributes) == 0 {
		return "", fmt.Errorf("CUPS no informó impresora por omisión")
	}
	return attrString(resp.PrinterAttributes[0], "printer-name"), nil
}

// recovered convierte un pánico de la librería IPP en un error. go-ipp hace
// type assertions sin verificar sobre las respuestas del servidor, y un pánico
// ahí se llevaría puesta la TUI entera.
func recovered(r interface{}, err error) error {
	if r == nil {
		return err
	}
	return fmt.Errorf("respuesta IPP inesperada: %v", r)
}

// CancelJob cancela un trabajo por id.
func (c *Client) CancelJob(id int) error {
	return classifyNil(c.ipp.CancelJob(id, false))
}

// CancelAllJobs cancela todos los trabajos de una impresora; con printer vacío,
// los de todas.
func (c *Client) CancelAllJobs(printer string) error {
	return classifyNil(c.ipp.CancelAllJob(printer, false))
}

// HoldJob retiene un trabajo: queda en la cola pero no se imprime.
func (c *Client) HoldJob(id int) error {
	return c.sendJob(ipp.OperationHoldJob, id)
}

// ReleaseJob libera un trabajo retenido para que vuelva a la cola activa.
func (c *Client) ReleaseJob(id int) error {
	return c.sendJob(ipp.OperationReleaseJob, id)
}

func (c *Client) sendJob(op int16, id int) (err error) {
	defer func() { err = classifyNil(recovered(recover(), err)) }()

	req := ipp.NewRequest(op, 1)
	req.OperationAttributes[ipp.AttributeJobURI] = jobURI(id)
	req.OperationAttributes[ipp.AttributeRequestingUserName] = c.user

	_, err = c.adapter.SendRequest(c.adapter.GetHttpUri("jobs", id), req, nil)
	return err
}

// EnablePrinter reanuda una impresora detenida (equivale a cupsenable).
func (c *Client) EnablePrinter(name string) error {
	return classifyNil(c.ipp.ResumePrinter(name))
}

// DisablePrinter detiene una impresora (equivale a cupsdisable). Los trabajos
// quedan en cola.
func (c *Client) DisablePrinter(name string) error {
	return classifyNil(c.ipp.PausePrinter(name))
}

// SetAccepting decide si la impresora admite trabajos nuevos.
func (c *Client) SetAccepting(name string, accepting bool) error {
	op := ipp.OperationCupsRejectJobs
	if accepting {
		op = ipp.OperationCupsAcceptJobs
	}
	return c.sendAdmin(op, name)
}

// SetDefault marca la impresora como destino por omisión del sistema.
func (c *Client) SetDefault(name string) error {
	return c.sendAdmin(ipp.OperationCupsSetDefault, name)
}

func (c *Client) sendAdmin(op int16, printer string) (err error) {
	defer func() { err = classifyNil(recovered(recover(), err)) }()

	req := ipp.NewRequest(op, 1)
	req.OperationAttributes[ipp.AttributePrinterURI] = printerURI(printer)
	req.OperationAttributes[ipp.AttributeRequestingUserName] = c.user

	_, err = c.adapter.SendRequest(c.adapter.GetHttpUri("admin", nil), req, nil)
	return err
}

func printerURI(name string) string {
	return "ipp://localhost/printers/" + name
}

func jobURI(id int) string {
	return "ipp://localhost/jobs/" + strconv.Itoa(id)
}

func classifyNil(err error) error {
	if e := classify(err); e != nil {
		return e
	}
	return nil
}
