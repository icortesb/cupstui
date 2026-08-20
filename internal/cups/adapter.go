package cups

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	ipp "github.com/phin1x/go-ipp"
)

// socketAdapter speaks IPP to cupsd over its unix socket.
//
// It exists instead of ipp.SocketAdapter because that one builds a fresh
// http.Transport per request and never closes it: the keep-alive connection
// stays in the pool of a transport nothing references any more, so the socket
// survives until the collector gets to it. Refreshing every few seconds that
// exhausts the MaxClients of cupsd (100 by default) and stalls the whole
// printing system, not just this application.
//
// Here there is one transport, living as long as the adapter, so queries reuse
// a single connection.
type socketAdapter struct {
	socket    string
	certPaths []string
	client    *http.Client
}

// socketSearchPaths covers the usual locations of the cupsd socket.
var socketSearchPaths = []string{
	"/run/cups/cups.sock",
	"/var/run/cups/cups.sock",
	"/var/run/cupsd",
	"/private/var/run/cupsd",
}

// certSearchPaths holds the CUPS local authentication certificate. It is
// usually readable by root alone; without it, the socket peer credentials are
// enough for the local user.
var certSearchPaths = []string{"/run/cups/certs/0", "/etc/cups/certs/0"}

func newSocketAdapter(socket string) *socketAdapter {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &socketAdapter{
		socket:    socket,
		certPaths: certSearchPaths,
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return dialer.DialContext(ctx, "unix", socket)
				},
				MaxIdleConns:        2,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// close releases the persistent connection.
func (a *socketAdapter) close() {
	if tr, ok := a.client.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

// findSocket returns the first path in the list that is a socket.
func findSocket(paths []string) (string, error) {
	for _, path := range paths {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSocket != 0 {
			return path, nil
		}
	}
	return "", ipp.SocketNotFoundError
}

func (a *socketAdapter) SendRequest(url string, r *ipp.Request, additionalData io.Writer) (*ipp.Response, error) {
	return a.SendRequestContext(context.Background(), url, r, additionalData)
}

func (a *socketAdapter) SendRequestContext(ctx context.Context, url string, r *ipp.Request, additionalData io.Writer) (*ipp.Response, error) {
	payload, err := r.Encode()
	if err != nil {
		return nil, fmt.Errorf("no se pudo codificar el pedido IPP: %w", err)
	}

	size := len(payload)
	var body io.Reader = bytes.NewReader(payload)
	if r.File != nil && r.FileSize != -1 {
		size += r.FileSize
		body = io.MultiReader(bytes.NewReader(payload), r.File)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("no se pudo armar el pedido HTTP: %w", err)
	}
	req.Header.Set("Content-Length", strconv.Itoa(size))
	req.Header.Set("Content-Type", ipp.ContentTypeIPP)
	// Only root can read the local certificate. Sending the header empty makes
	// cupsd log an error for every request; without the header it authenticates
	// by unix socket credentials and leaves error_log alone.
	if cert := a.cert(); cert != "" {
		req.Header.Set("Authorization", "Local "+cert)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain the body so the connection returns to the pool instead of
		// closing and having to redial on the next refresh.
		io.Copy(io.Discard, resp.Body)
		return nil, ipp.HTTPError{Code: resp.StatusCode}
	}

	buf := new(bytes.Buffer)
	if resp.ContentLength > 0 {
		buf.Grow(int(resp.ContentLength))
	}
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return nil, fmt.Errorf("no se pudo leer la respuesta IPP: %w", err)
	}

	ippResp, err := ipp.NewResponseDecoder(buf).Decode(additionalData)
	if err != nil {
		return nil, fmt.Errorf("no se pudo decodificar la respuesta IPP: %w", err)
	}
	if err := ippResp.CheckForErrors(); err != nil {
		return nil, err
	}
	return ippResp, nil
}

// cert reads the local certificate when it is readable; otherwise it returns
// empty and CUPS authenticates by unix socket credentials.
func (a *socketAdapter) cert() string {
	for _, path := range a.certPaths {
		if b, err := os.ReadFile(path); err == nil {
			return string(bytes.TrimSpace(b))
		}
	}
	return ""
}

// GetHttpUri builds the URLs cupsd expects. The host is irrelevant because the
// transport goes over the socket, but net/http requires a valid one.
func (a *socketAdapter) GetHttpUri(namespace string, object interface{}) string {
	uri := "http://localhost"
	if namespace != "" {
		uri += "/" + namespace
	}
	if object != nil {
		uri += fmt.Sprintf("/%v", object)
	}
	return uri
}

func (a *socketAdapter) TestConnection() error {
	conn, err := net.DialTimeout("unix", a.socket, 5*time.Second)
	if err != nil {
		return err
	}
	return conn.Close()
}
