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

// socketAdapter habla IPP con cupsd por su socket unix.
//
// Existe en vez de usar ipp.SocketAdapter porque aquel arma un http.Transport
// nuevo en cada request y nunca lo cierra: la conexión keep-alive queda en el
// pool de un transport que ya nadie referencia, así que el socket sobrevive
// hasta que el recolector lo junte. Con un refresco cada pocos segundos eso
// agota el MaxClients de cupsd (100 por omisión) y deja colgado a todo el
// sistema de impresión, no solo a esta aplicación.
//
// Acá el transport es uno solo y vive lo que vive el adapter, así que las
// consultas reusan una única conexión.
type socketAdapter struct {
	socket    string
	certPaths []string
	client    *http.Client
}

// socketSearchPaths cubre las ubicaciones habituales del socket de cupsd.
var socketSearchPaths = []string{
	"/run/cups/cups.sock",
	"/var/run/cups/cups.sock",
	"/var/run/cupsd",
	"/private/var/run/cupsd",
}

// certSearchPaths es el certificado de autenticación local de CUPS. Suele ser
// legible solo por root; sin él la autenticación por credenciales del socket
// alcanza para el usuario local.
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

// close libera la conexión persistente.
func (a *socketAdapter) close() {
	if tr, ok := a.client.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

// findSocket devuelve el primer path de la lista que sea un socket.
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
	req.Header.Set("Authorization", "Local "+a.cert())

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Se drena el cuerpo para que la conexión vuelva al pool en lugar de
		// cerrarse y tener que redialar en el próximo refresco.
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

// cert lee el certificado local si es accesible; si no lo es, se devuelve
// vacío y CUPS autentica por las credenciales del socket unix.
func (a *socketAdapter) cert() string {
	for _, path := range a.certPaths {
		if b, err := os.ReadFile(path); err == nil {
			return string(bytes.TrimSpace(b))
		}
	}
	return ""
}

// GetHttpUri arma las URLs que espera cupsd. El host es irrelevante porque el
// transporte va por el socket, pero net/http necesita uno válido.
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
