package cups

import (
	"errors"
	"os"
	"syscall"

	ipp "github.com/phin1x/go-ipp"
)

// Kind clasifica el fallo para que la UI muestre algo accionable en vez del
// error crudo de la librería.
type Kind int

const (
	KindUnknown Kind = iota
	KindDaemonDown
	KindForbidden
	KindNotFound
)

// Error es el error que expone este paquete: conserva el original y le suma
// una clasificación y una pista para el usuario.
type Error struct {
	Kind Kind
	Hint string
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Hint
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// Códigos de estado IPP relevantes (RFC 8011 §14.1.4).
const (
	ippForbidden        int16 = 0x0401
	ippNotAuthenticated int16 = 0x0402
	ippNotAuthorized    int16 = 0x0403
	ippNotFound         int16 = 0x0406
)

func classify(err error) *Error {
	if err == nil {
		return nil
	}

	if isDaemonDown(err) {
		return &Error{
			Kind: KindDaemonDown,
			Hint: "CUPS no responde — probá: systemctl start cups",
			Err:  err,
		}
	}

	var httpErr ipp.HTTPError
	if errors.As(err, &httpErr) && (httpErr.Code == 401 || httpErr.Code == 403) {
		return &Error{Kind: KindForbidden, Hint: permissionHint, Err: err}
	}

	var ippErr ipp.IPPError
	if errors.As(err, &ippErr) {
		switch ippErr.Status {
		case ippForbidden, ippNotAuthenticated, ippNotAuthorized:
			return &Error{Kind: KindForbidden, Hint: permissionHint, Err: err}
		case ippNotFound:
			return &Error{Kind: KindNotFound, Hint: "la impresora o el trabajo ya no existe", Err: err}
		}
	}

	if ipp.IsNotExistsError(err) {
		return &Error{Kind: KindNotFound, Hint: "la impresora o el trabajo ya no existe", Err: err}
	}

	return &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
}

const permissionHint = "sin permisos para esta operación (hace falta estar en el grupo del SystemGroup de CUPS, normalmente wheel)"

func isDaemonDown(err error) bool {
	if errors.Is(err, ipp.SocketNotFoundError) {
		return true
	}
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ENOENT) ||
		errors.Is(err, os.ErrPermission) ||
		errors.Is(err, os.ErrNotExist)
}
