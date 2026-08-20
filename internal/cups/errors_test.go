package cups

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"

	ipp "github.com/phin1x/go-ipp"
)

func TestClassifyRecognisesDaemonDown(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"socket ausente", ipp.SocketNotFoundError},
		{"conexión rechazada", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}},
		{"socket envuelto", fmt.Errorf("unable to connect: %w", ipp.SocketNotFoundError)},
		{"permiso denegado sobre el socket", &net.OpError{Op: "dial", Err: os.ErrPermission}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := classify(c.err)
			if e.Kind != KindDaemonDown {
				t.Fatalf("Kind = %v, quiero KindDaemonDown", e.Kind)
			}
			if e.Hint == "" {
				t.Error("quiero una pista accionable para el usuario")
			}
		})
	}
}

func TestClassifyRecognisesPermissionDenied(t *testing.T) {
	for _, code := range []int{401, 403} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			if got := classify(ipp.HTTPError{Code: code}).Kind; got != KindForbidden {
				t.Errorf("Kind = %v, quiero KindForbidden", got)
			}
		})
	}
	// El equivalente a nivel IPP: not-authorized / forbidden.
	for _, st := range []int16{0x0401, 0x0403} {
		if got := classify(ipp.IPPError{Status: st, Message: "no"}).Kind; got != KindForbidden {
			t.Errorf("status %#x: Kind = %v, quiero KindForbidden", st, got)
		}
	}
}

func TestClassifyRecognisesNotFound(t *testing.T) {
	if got := classify(ipp.IPPError{Status: 0x0406, Message: "not found"}).Kind; got != KindNotFound {
		t.Errorf("Kind = %v, quiero KindNotFound", got)
	}
	if got := classify(errors.New("The printer or class does not exist.")).Kind; got != KindNotFound {
		t.Errorf("Kind = %v, quiero KindNotFound", got)
	}
}

func TestClassifyFallsBackToUnknownKeepingTheMessage(t *testing.T) {
	e := classify(errors.New("boom raro del servidor"))
	if e.Kind != KindUnknown {
		t.Errorf("Kind = %v, quiero KindUnknown", e.Kind)
	}
	if e.Error() == "" {
		t.Error("quiero conservar el mensaje crudo")
	}
	if !errors.Is(e, e.Err) {
		t.Error("quiero poder desenvolver el error original")
	}
}

func TestClassifyReturnsNilForNilError(t *testing.T) {
	if e := classify(nil); e != nil {
		t.Errorf("classify(nil) = %v, quiero nil", e)
	}
}
