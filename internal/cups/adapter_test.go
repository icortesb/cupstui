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

// fakeCUPS is a pretend cupsd on a unix socket that counts how many separate
// connections were opened to it.
type fakeCUPS struct {
	socket      string
	connections *int64
	headers     *atomic.Value // http.Header of the last request
}

func (f *fakeCUPS) lastHeader(name string) string {
	h, _ := f.headers.Load().(http.Header)
	return h.Get(name)
}

func startFakeCUPS(t *testing.T) *fakeCUPS {
	t.Helper()

	// The socket path has a hard limit of 108 bytes, so a short name inside
	// the TempDir is used.
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
	// The regression behind this test: go-ipp builds a new http.Transport per
	// request and never closes it, so every refresh leaves a socket hanging
	// until the GC collects it. Refreshing every 3s that fills the MaxClients
	// of cupsd (100) and leaves the printing system serving nobody.
	fake := startFakeCUPS(t)
	a := newSocketAdapter(fake.socket)

	for i := 0; i < 20; i++ {
		req := ipp.NewRequest(ipp.OperationCupsGetPrinters, int32(i))
		if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	if got := atomic.LoadInt64(fake.connections); got != 1 {
		t.Errorf("the adapter opened %d connections for 20 requests, want 1", got)
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
			t.Errorf("GetHttpUri(%q, %v) = %q, want %q", c.namespace, c.object, got, c.want)
		}
	}
}

func TestSocketAdapterReportsMissingSocket(t *testing.T) {
	a := newSocketAdapter(filepath.Join(t.TempDir(), "no-existe.sock"))
	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)

	_, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil)
	if err == nil {
		t.Fatal("want an error when the socket does not exist")
	}
	if classify(err).Kind != KindDaemonDown {
		t.Errorf("the error should classify as KindDaemonDown, got %v", err)
	}
}

func TestFindSocketPrefersAnExistingSocket(t *testing.T) {
	fake := startFakeCUPS(t)
	got, err := findSocket([]string{"/no/existe", fake.socket})
	if err != nil {
		t.Fatalf("findSocket: %v", err)
	}
	if got != fake.socket {
		t.Errorf("findSocket = %q, want %q", got, fake.socket)
	}
}

func TestFindSocketFailsWhenNoneExists(t *testing.T) {
	_, err := findSocket([]string{"/no/existe", "/tampoco"})
	if classify(err).Kind != KindDaemonDown {
		t.Errorf("want KindDaemonDown, got %v", err)
	}
}

func TestSocketAdapterOmitsAuthorizationWhenThereIsNoCertificate(t *testing.T) {
	// Only root can read the CUPS local certificate. Sending
	// "Authorization: Local" with an empty value makes cupsd write
	// "Local authentication certificate not found." to its error_log for every
	// request, dozens of lines a minute with the automatic refresh. The request
	// authenticates by unix socket credentials regardless.
	fake := startFakeCUPS(t)
	a := newSocketAdapter(fake.socket)
	a.certPaths = []string{filepath.Join(t.TempDir(), "no-existe")}

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	if got := fake.lastHeader("Authorization"); got != "" {
		t.Errorf("Authorization = %q was sent, none was expected", got)
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
		t.Errorf("Authorization = %q, want \"Local abc123\"", got)
	}
}

func startFakeHTTPCUPS(t *testing.T, requireAuth bool) (*fakeCUPS, string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
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
			if requireAuth {
				user, pass, ok := r.BasicAuth()
				if !ok || user != "ana" || pass != "secret" {
					w.Header().Set("WWW-Authenticate", `Basic realm="CUPS"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}
			body, err := (&ipp.Response{ProtocolVersionMajor: 2, StatusCode: ipp.StatusOk, RequestId: 1}).Encode()
			if err != nil {
				t.Errorf("could not encode the response: %v", err)
			}
			w.Header().Set("Content-Type", ipp.ContentTypeIPP)
			w.Write(body)
		}),
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	return &fakeCUPS{connections: &conns, headers: &headers}, ln.Addr().String()
}

func TestRemoteAdapterTalksOverTCP(t *testing.T) {
	fake, addr := startFakeHTTPCUPS(t, false)
	a := newRemoteAdapter(addr, "", "")

	for i := 0; i < 10; i++ {
		req := ipp.NewRequest(ipp.OperationCupsGetPrinters, int32(i))
		if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	// The same reuse the local transport gets: a remote server has the same
	// MaxClients to exhaust.
	if got := atomic.LoadInt64(fake.connections); got != 1 {
		t.Errorf("the adapter opened %d connections for 10 requests, want 1", got)
	}
}

func TestRemoteAdapterBuildsURIsAgainstTheServer(t *testing.T) {
	a := newRemoteAdapter("print.example.org:631", "", "")
	if got := a.GetHttpUri("admin", nil); got != "http://print.example.org:631/admin" {
		t.Errorf("GetHttpUri = %q", got)
	}
}

func TestRemoteAdapterReportsAnUnauthenticatedServerAsForbidden(t *testing.T) {
	_, addr := startFakeHTTPCUPS(t, true)
	a := newRemoteAdapter(addr, "", "")

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	_, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil)
	if classify(err).Kind != KindForbidden {
		t.Errorf("want KindForbidden, got %v", err)
	}
}

func TestRemoteAdapterSendsCredentialsWhenItHasThem(t *testing.T) {
	_, addr := startFakeHTTPCUPS(t, true)
	a := newRemoteAdapter(addr, "ana", "secret")

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
}

func TestRemoteAdapterAcceptsCredentialsAfterTheFact(t *testing.T) {
	// The password is asked for once the server refuses, so the adapter has to
	// take it without being rebuilt and losing its connection pool.
	_, addr := startFakeHTTPCUPS(t, true)
	a := newRemoteAdapter(addr, "ana", "")

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err == nil {
		t.Fatal("want the first attempt to be refused")
	}

	a.setPassword("secret")
	req = ipp.NewRequest(ipp.OperationCupsGetPrinters, 2)
	if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
		t.Errorf("after the password, SendRequest failed: %v", err)
	}
}
