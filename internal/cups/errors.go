package cups

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"syscall"

	ipp "github.com/phin1x/go-ipp"
)

// Kind classifies the failure so the interface can show something actionable
// instead of the raw library error.
type Kind int

const (
	KindUnknown Kind = iota
	KindDaemonDown
	KindUnreachable
	KindUntrusted
	KindForbidden
	KindNotFound
	// KindUnauthorized is a request for credentials, not a refusal: cupsd
	// answers an administrative request this way before it has authenticated
	// anyone, and the caller is expected to try again.
	KindUnauthorized
)

// Error is what this package returns: it keeps the original and adds a
// classification and a hint for the user.
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

// Relevant IPP status codes (RFC 8011 §14.1.4).
const (
	ippForbidden        int16 = 0x0401
	ippNotAuthenticated int16 = 0x0402
	ippNotAuthorized    int16 = 0x0403
	ippNotFound         int16 = 0x0406
)

// isNotFound is cupsd answering that nothing matched the request, which it
// does both by IPP status and, for some operations, by message alone.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var ippErr ipp.IPPError
	if errors.As(err, &ippErr) && ippErr.Status == ippNotFound {
		return true
	}
	return ipp.IsNotExistsError(err)
}

func classify(err error) *Error {
	if err == nil {
		return nil
	}

	if isUntrusted(err) {
		return &Error{
			Kind: KindUntrusted,
			Hint: "the server certificate cannot be verified — add its issuer to this machine, or set AllowAnyRoot Yes in ~/.cups/client.conf to accept it unverified",
			Err:  err,
		}
	}

	if isUnreachable(err) {
		return &Error{
			Kind: KindUnreachable,
			Hint: "cannot reach the CUPS server — check CUPS_SERVER and that the host is up",
			Err:  err,
		}
	}

	if isDaemonDown(err) {
		return &Error{
			Kind: KindDaemonDown,
			Hint: "CUPS is not responding — try: systemctl start cups",
			Err:  err,
		}
	}

	var httpErr ipp.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Code {
		case 401:
			return &Error{Kind: KindUnauthorized, Hint: credentialsHint, Err: err}
		case 403:
			return &Error{Kind: KindForbidden, Hint: permissionHint, Err: err}
		}
	}

	var ippErr ipp.IPPError
	if errors.As(err, &ippErr) {
		switch ippErr.Status {
		case ippNotAuthenticated:
			return &Error{Kind: KindUnauthorized, Hint: credentialsHint, Err: err}
		case ippForbidden, ippNotAuthorized:
			return &Error{Kind: KindForbidden, Hint: permissionHint, Err: err}
		case ippNotFound:
			return &Error{Kind: KindNotFound, Hint: "the printer or job no longer exists", Err: err}
		}
	}

	if ipp.IsNotExistsError(err) {
		return &Error{Kind: KindNotFound, Hint: "the printer or job no longer exists", Err: err}
	}

	return &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
}

const permissionHint = "permission denied — this operation requires membership in the CUPS SystemGroup (usually wheel)"

const credentialsHint = "CUPS asked for credentials — press S to sign in to a remote server"

// isUntrusted is a certificate this machine cannot vouch for.
func isUntrusted(err error) bool {
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var record *tls.CertificateVerificationError

	return errors.As(err, &unknown) || errors.As(err, &hostname) ||
		errors.As(err, &invalid) || errors.As(err, &record)
}

// isUnreachable is a network failure reaching a named server, as opposed to a
// local service that is not running: the remedy is on the other machine.
func isUnreachable(err error) bool {
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return true
	}
	if errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH) {
		return true
	}

	// A refused or timed-out dial over TCP is a remote server; over a unix
	// socket it is the local service.
	var op *net.OpError
	if errors.As(err, &op) && op.Net != "" && op.Net != "unix" {
		return true
	}
	return false
}

func isDaemonDown(err error) bool {
	if errors.Is(err, ipp.SocketNotFoundError) {
		return true
	}
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ENOENT) ||
		errors.Is(err, os.ErrPermission) ||
		errors.Is(err, os.ErrNotExist)
}
