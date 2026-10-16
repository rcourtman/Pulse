// Package proxyfixture supplies listener-free HTTP/CONNECT/TLS evidence to
// transport tests. It is imported only by tests, never by a product runtime.
package proxyfixture

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const Address = "fixture-proxy.invalid:3128"
const fixtureEnv = "PULSE_INFRASTRUCTURE_PROXY_FIXTURE"

// Isolated reexecutes just this test with deterministic proxy settings, since
// net/http caches the environment. No listener, real proxy or network is used.
func Isolated(t *testing.T, testName string) bool {
	t.Helper()
	if os.Getenv(fixtureEnv) == testName {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+testName+"$", "-test.v")
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		switch strings.ToUpper(key) {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "REQUEST_METHOD", fixtureEnv:
			continue
		}
		cmd.Env = append(cmd.Env, value)
	}
	proxy := "http://fixture-user:fixture-password@" + Address
	cmd.Env = append(cmd.Env, fixtureEnv+"="+testName, "HTTP_PROXY="+proxy, "HTTPS_PROXY="+proxy, "NO_PROXY=excluded.example")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated proxy fixture: %v\n%s", err, output)
	}
	t.Logf("isolated proxy fixture:\n%s", output)
	return false
}

type Observation struct {
	Address, ConnectAuthority, ServerName string
	Connect                               bool
	ConnectHeaders                        http.Header
	Method, URI, Host                     string
	Headers                               http.Header
	Body                                  []byte
	Err                                   error
}

type Reply func(*http.Request, []byte) (int, string)

// Pipe replaces only the production Transport's dialer. The Transport still
// selects direct versus proxy routing and performs real HTTP and TLS. Each
// response closes its owned pipe so fixtures cannot hide pooled requests.
func Pipe(t *testing.T, transport *http.Transport, certificate *tls.Certificate, reply Reply) func() Observation {
	t.Helper()
	observations := make(chan Observation, 16)
	transport.DialContext = func(_ context.Context, _, address string) (net.Conn, error) {
		client, server := net.Pipe()
		deadline := time.Now().Add(3 * time.Second)
		if err := client.SetDeadline(deadline); err != nil {
			t.Error(err)
		}
		if err := server.SetDeadline(deadline); err != nil {
			t.Error(err)
		}
		go func() {
			got := Observation{Address: address}
			defer func() { _ = server.Close(); observations <- got }()
			var wire net.Conn = server
			reader := bufio.NewReader(wire)
			if certificate != nil {
				if address == Address {
					request, err := http.ReadRequest(reader)
					if err != nil {
						got.Err = err
						return
					}
					got.Connect = request.Method == http.MethodConnect
					got.ConnectAuthority, got.ConnectHeaders = request.Host, request.Header.Clone()
					_ = request.Body.Close()
					if !got.Connect {
						got.Err = fmt.Errorf("expected CONNECT")
						return
					}
					if _, err := io.WriteString(wire, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
						got.Err = err
						return
					}
				}
				secure := tls.Server(wire, &tls.Config{Certificates: []tls.Certificate{*certificate}, MinVersion: tls.VersionTLS12})
				if err := secure.Handshake(); err != nil {
					got.Err = err
					return
				}
				got.ServerName = secure.ConnectionState().ServerName
				wire, reader = secure, bufio.NewReader(secure)
			}
			request, err := http.ReadRequest(reader)
			if err != nil {
				got.Err = err
				return
			}
			got.Method, got.URI, got.Host, got.Headers = request.Method, request.RequestURI, request.Host, request.Header.Clone()
			got.Body, got.Err = io.ReadAll(request.Body)
			_ = request.Body.Close()
			if got.Err != nil {
				return
			}
			status, body := reply(request, got.Body)
			_, got.Err = fmt.Fprintf(wire, "HTTP/1.1 %d %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", status, http.StatusText(status), len(body), body)
		}()
		return client, nil
	}
	t.Cleanup(transport.CloseIdleConnections)
	return func() Observation {
		t.Helper()
		select {
		case got := <-observations:
			return got
		case <-time.After(4 * time.Second):
			t.Fatal("missing pipe observation")
			return Observation{}
		}
	}
}

func Certificate(t *testing.T, hostname string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Infrastructure proxy fixture"},
		DNSNames: []string{hostname}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, roots
}
