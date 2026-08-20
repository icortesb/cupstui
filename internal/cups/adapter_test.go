package cups

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	ipp "github.com/phin1x/go-ipp"
)

// fakeCUPS es un cupsd de mentira sobre un socket unix que cuenta cuántas
// conexiones TCP/unix distintas le abrieron.
type fakeCUPS struct {
	socket      string
	connections *int64
	headers     *atomic.Value // http.Header del último pedido
}

func (f *fakeCUPS) lastHeader(name string) string {
	h, _ := f.headers.Load().(http.Header)
	return h.Get(name)
}

func startFakeCUPS(t *testing.T) *fakeCUPS {
	t.Helper()

	// El path del socket tiene un límite duro de 108 bytes, así que se usa un
	// nombre corto dentro del TempDir.
	sock := filepath.Join(t.TempDir(), "c.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("no se pudo abrir el socket falso: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	var conns int64
	var headers atomic.Value
	headers.Store(http.Header{})
	srv := &http.Server{
		ConnState: func(_ net.Conn, state http.ConnState) {
			if state == http.StateNew {
				atomic.AddInt64(&conns, 1)
			}
		},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers.Store(r.Header.Clone())
			resp := &ipp.Response{
				ProtocolVersionMajor: 2,
				StatusCode:           ipp.StatusOk,
				RequestId:            1,
			}
			body, err := resp.Encode()
			if err != nil {
				t.Errorf("no se pudo codificar la respuesta: %v", err)
			}
			w.Header().Set("Content-Type", ipp.ContentTypeIPP)
			w.Write(body)
		}),
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	return &fakeCUPS{socket: sock, connections: &conns, headers: &headers}
}

func TestSocketAdapterReusesOneConnection(t *testing.T) {
	// La regresión que motiva este test: go-ipp arma un http.Transport nuevo
	// por request y no lo cierra, así que cada refresco deja un socket colgado
	// hasta que el GC lo junte. Con refresco cada 3s eso llena el MaxClients
	// de cupsd (100) y deja al sistema de impresión sin atender a nadie.
	fake := startFakeCUPS(t)
	a := newSocketAdapter(fake.socket)

	for i := 0; i < 20; i++ {
		req := ipp.NewRequest(ipp.OperationCupsGetPrinters, int32(i))
		if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	if got := atomic.LoadInt64(fake.connections); got != 1 {
		t.Errorf("el adapter abrió %d conexiones para 20 requests, quiero 1", got)
	}
}

func TestSocketAdapterBuildsCUPSURIs(t *testing.T) {
	a := newSocketAdapter("/run/cups/cups.sock")
	cases := []struct {
		namespace string
		object    interface{}
		want      string
	}{
		{"", nil, "http://localhost"},
		{"admin", nil, "http://localhost/admin"},
		{"jobs", 42, "http://localhost/jobs/42"},
	}
	for _, c := range cases {
		if got := a.GetHttpUri(c.namespace, c.object); got != c.want {
			t.Errorf("GetHttpUri(%q, %v) = %q, quiero %q", c.namespace, c.object, got, c.want)
		}
	}
}

func TestSocketAdapterReportsMissingSocket(t *testing.T) {
	a := newSocketAdapter(filepath.Join(t.TempDir(), "no-existe.sock"))
	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)

	_, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil)
	if err == nil {
		t.Fatal("quiero un error cuando el socket no existe")
	}
	if classify(err).Kind != KindDaemonDown {
		t.Errorf("el error debería clasificarse como KindDaemonDown, tengo %v", err)
	}
}

func TestFindSocketPrefersAnExistingSocket(t *testing.T) {
	fake := startFakeCUPS(t)
	got, err := findSocket([]string{"/no/existe", fake.socket})
	if err != nil {
		t.Fatalf("findSocket: %v", err)
	}
	if got != fake.socket {
		t.Errorf("findSocket = %q, quiero %q", got, fake.socket)
	}
}

func TestFindSocketFailsWhenNoneExists(t *testing.T) {
	_, err := findSocket([]string{"/no/existe", "/tampoco"})
	if classify(err).Kind != KindDaemonDown {
		t.Errorf("quiero KindDaemonDown, tengo %v", err)
	}
}

func TestSocketAdapterOmitsAuthorizationWhenThereIsNoCertificate(t *testing.T) {
	// El certificado local de CUPS solo lo puede leer root. Mandar
	// "Authorization: Local" con el valor vacío hace que cupsd escriba
	// "Local authentication certificate not found." en su error_log por cada
	// pedido, o sea decenas de líneas por minuto con el refresco automático.
	// El pedido igual se autentica por las credenciales del socket unix.
	fake := startFakeCUPS(t)
	a := newSocketAdapter(fake.socket)
	a.certPaths = []string{filepath.Join(t.TempDir(), "no-existe")}

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	if got := fake.lastHeader("Authorization"); got != "" {
		t.Errorf("se mandó Authorization = %q, no se esperaba ninguno", got)
	}
}

func TestSocketAdapterSendsTheCertificateWhenItCanReadIt(t *testing.T) {
	fake := startFakeCUPS(t)
	cert := filepath.Join(t.TempDir(), "0")
	if err := os.WriteFile(cert, []byte("abc123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newSocketAdapter(fake.socket)
	a.certPaths = []string{cert}

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	if got := fake.lastHeader("Authorization"); got != "Local abc123" {
		t.Errorf("Authorization = %q, quiero \"Local abc123\"", got)
	}
}
