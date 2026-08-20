package cups

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"

	ipp "github.com/phin1x/go-ipp"
)

func TestClassifyRecognisesDaemonDown(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"socket missing", ipp.SocketNotFoundError},
		{"connection refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}},
		{"wrapped socket error", fmt.Errorf("unable to connect: %w", ipp.SocketNotFoundError)},
		{"permission denied on the socket", &net.OpError{Op: "dial", Err: os.ErrPermission}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := classify(c.err)
			if e.Kind != KindDaemonDown {
				t.Fatalf("Kind = %v, want KindDaemonDown", e.Kind)
			}
			if e.Hint == "" {
				t.Error("want an actionable hint for the user")
			}
		})
	}
}

func TestClassifyRecognisesPermissionDenied(t *testing.T) {
	for _, code := range []int{401, 403} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			if got := classify(ipp.HTTPError{Code: code}).Kind; got != KindForbidden {
				t.Errorf("Kind = %v, want KindForbidden", got)
			}
		})
	}
	// El equivalente a nivel IPP: not-authorized / forbidden.
	for _, st := range []int16{0x0401, 0x0403} {
		if got := classify(ipp.IPPError{Status: st, Message: "no"}).Kind; got != KindForbidden {
			t.Errorf("status %#x: Kind = %v, want KindForbidden", st, got)
		}
	}
}

func TestClassifyRecognisesNotFound(t *testing.T) {
	if got := classify(ipp.IPPError{Status: 0x0406, Message: "not found"}).Kind; got != KindNotFound {
		t.Errorf("Kind = %v, want KindNotFound", got)
	}
	if got := classify(errors.New("The printer or class does not exist.")).Kind; got != KindNotFound {
		t.Errorf("Kind = %v, want KindNotFound", got)
	}
}

func TestClassifyFallsBackToUnknownKeepingTheMessage(t *testing.T) {
	e := classify(errors.New("boom raro del servidor"))
	if e.Kind != KindUnknown {
		t.Errorf("Kind = %v, want KindUnknown", e.Kind)
	}
	if e.Error() == "" {
		t.Error("want the raw message kept")
	}
	if !errors.Is(e, e.Err) {
		t.Error("want to be able to unwrap the original error")
	}
}

func TestClassifyReturnsNilForNilError(t *testing.T) {
	if e := classify(nil); e != nil {
		t.Errorf("classify(nil) = %v, want nil", e)
	}
}

// asError es errors.As, con nombre propio para que los tests lean mejor.
func asError(err error, target **Error) bool {
	return errors.As(err, target)
}

func TestClassifyDistinguishesAnUnreachableServerFromAStoppedService(t *testing.T) {
	// Telling someone to run systemctl when the name simply does not resolve
	// sends them to the wrong machine.
	cases := []struct {
		name string
		err  error
	}{
		{"name does not resolve", &net.DNSError{Err: "no such host", Name: "print.example.invalid"}},
		{"no route to the host", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.EHOSTUNREACH}},
		{"the network is down", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ENETUNREACH}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := classify(c.err)
			if e.Kind != KindUnreachable {
				t.Fatalf("Kind = %v, want KindUnreachable", e.Kind)
			}
			if strings.Contains(e.Hint, "systemctl") {
				t.Errorf("Hint = %q, should not suggest starting a local service", e.Hint)
			}
			if e.Hint == "" {
				t.Error("want an actionable hint")
			}
		})
	}
}

func TestAStoppedLocalServiceStillSuggestsStartingIt(t *testing.T) {
	e := classify(ipp.SocketNotFoundError)
	if e.Kind != KindDaemonDown {
		t.Fatalf("Kind = %v, want KindDaemonDown", e.Kind)
	}
	if !strings.Contains(e.Hint, "systemctl") {
		t.Errorf("Hint = %q, want the start instruction", e.Hint)
	}
}
