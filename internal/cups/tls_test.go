package cups

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ipp "github.com/phin1x/go-ipp"
)

// selfSigned makes a certificate for 127.0.0.1, which is what a CUPS server
// with a generated certificate looks like to a client.
func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "cupstui-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func ippOK(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	body, err := (&ipp.Response{ProtocolVersionMajor: 2, StatusCode: ipp.StatusOk, RequestId: 1}).Encode()
	if err != nil {
		t.Errorf("could not encode the response: %v", err)
		return
	}
	w.Header().Set("Content-Type", ipp.ContentTypeIPP)
	w.Write(body)
}

// startTLSCUPS serves IPP over TLS from the first byte, as Encryption Always
// expects.
func startTLSCUPS(t *testing.T) string {
	t.Helper()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ippOK(t, w)
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	return ln.Addr().String()
}

func TestAlwaysConnectsOverTLS(t *testing.T) {
	addr := startTLSCUPS(t)
	a := newRemoteAdapter(addr, "", "", Server{
		Address: addr, Encryption: EncryptAlways, AllowAnyRoot: true,
	})

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
}

func TestAnUnverifiableCertificateIsRefusedByDefault(t *testing.T) {
	// Accepting a certificate nothing can vouch for makes a man in the middle
	// look exactly like the real server.
	addr := startTLSCUPS(t)
	a := newRemoteAdapter(addr, "", "", Server{Address: addr, Encryption: EncryptAlways})

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	_, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil)
	if err == nil {
		t.Fatal("want the certificate to be refused")
	}
	if e := classify(err); e.Kind != KindUntrusted {
		t.Errorf("Kind = %v, want KindUntrusted", e.Kind)
	}
	if !strings.Contains(classify(err).Hint, "AllowAnyRoot") {
		t.Errorf("Hint = %q, want it to name the way out", classify(err).Hint)
	}
}

// startUpgradeCUPS speaks plain HTTP until a client asks to upgrade, which is
// how CUPS serves Encryption Required.
func startUpgradeCUPS(t *testing.T) (addr string, upgrades *int64) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	var count int64
	cert := selfSigned(t)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()

				reader := bufio.NewReader(conn)
				request, err := http.ReadRequest(reader)
				if err != nil {
					return
				}
				if !strings.Contains(request.Header.Get("Upgrade"), "TLS") {
					conn.Write([]byte("HTTP/1.1 426 Upgrade Required\r\nContent-Length: 0\r\n\r\n"))
					return
				}

				atomic.AddInt64(&count, 1)
				conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: TLS/1.2\r\n\r\n"))

				secure := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}})
				if err := secure.Handshake(); err != nil {
					return
				}
				defer secure.Close()

				// The reply to the OPTIONS, which RFC 2817 has the server
				// finish over the new connection.
				secure.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))

				// The reply is written by hand: an http.Server around a
				// one-shot listener closes the connection the moment its
				// second Accept fails, before the handler has answered.
				secureReq, err := http.ReadRequest(bufio.NewReader(secure))
				if err != nil {
					return
				}
				io.Copy(io.Discard, secureReq.Body)
				secureReq.Body.Close()
				body, err := (&ipp.Response{ProtocolVersionMajor: 2, StatusCode: ipp.StatusOk, RequestId: 1}).Encode()
				if err != nil {
					return
				}
				fmt.Fprintf(secure, "HTTP/1.1 200 OK\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n",
					ipp.ContentTypeIPP, len(body))
				secure.Write(body)
			}()
		}
	}()

	return ln.Addr().String(), &count
}

func TestRequiredUpgradesThePlainConnection(t *testing.T) {
	addr, upgrades := startUpgradeCUPS(t)
	a := newRemoteAdapter(addr, "", "", Server{
		Address: addr, Encryption: EncryptRequired, AllowAnyRoot: true,
	})

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if atomic.LoadInt64(upgrades) == 0 {
		t.Error("the connection was never upgraded")
	}
}

func TestNeverStaysInTheClear(t *testing.T) {
	addr, upgrades := startUpgradeCUPS(t)
	a := newRemoteAdapter(addr, "", "", Server{Address: addr, Encryption: EncryptNever})

	req := ipp.NewRequest(ipp.OperationCupsGetPrinters, 1)
	// The server refuses, which is its right; what matters is that no upgrade
	// was attempted behind the user's back.
	a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil)
	if atomic.LoadInt64(upgrades) != 0 {
		t.Error("an upgrade was attempted with encryption turned off")
	}
}

func TestRequiredCompletesTheOriginalRequestBeforeHandingOver(t *testing.T) {
	// RFC 2817 has the server finish the request that asked for the upgrade,
	// over the new connection. CUPS does exactly that, and the answer has to be
	// consumed: left in the stream, the HTTP client reads it as the reply to
	// the next request and everything after it is off by one.
	addr := startUpgradeCUPSAnsweringOptions(t)
	a := newRemoteAdapter(addr, "", "", Server{
		Address: addr, Encryption: EncryptRequired, AllowAnyRoot: true,
	})

	for i := 0; i < 3; i++ {
		req := ipp.NewRequest(ipp.OperationCupsGetPrinters, int32(i))
		if _, err := a.SendRequestContext(context.Background(), a.GetHttpUri("", nil), req, nil); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
}

// startUpgradeCUPSAnsweringOptions upgrades and then answers the OPTIONS that
// asked for the upgrade, the way CUPS does.
func startUpgradeCUPSAnsweringOptions(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	cert := selfSigned(t)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()

				if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
					return
				}
				conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: TLS/1.2\r\n\r\n"))

				secure := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}})
				if err := secure.Handshake(); err != nil {
					return
				}
				defer secure.Close()

				// The reply to the OPTIONS, over the encrypted connection.
				secure.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))

				reader := bufio.NewReader(secure)
				for {
					request, err := http.ReadRequest(reader)
					if err != nil {
						return
					}
					// The body has to be consumed, otherwise the next read
					// starts in the middle of it.
					io.Copy(io.Discard, request.Body)
					request.Body.Close()

					body, err := (&ipp.Response{ProtocolVersionMajor: 2, StatusCode: ipp.StatusOk, RequestId: 1}).Encode()
					if err != nil {
						return
					}
					fmt.Fprintf(secure, "HTTP/1.1 200 OK\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n",
						ipp.ContentTypeIPP, len(body))
					secure.Write(body)
				}
			}()
		}
	}()

	return ln.Addr().String()
}
