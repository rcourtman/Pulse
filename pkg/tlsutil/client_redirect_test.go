package tlsutil

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type infrastructureRedirectTransport func(*http.Request) (*http.Response, error)

func (f infrastructureRedirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// A recording transport proves refusal before a second request, independently
// of DNS, destination reachability or whether Go would strip selected headers.
func TestInfrastructureRedirectOriginPolicy(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, destination := range []struct {
			name, location string
			allowed        bool
		}{
			{"relative", "/proxy/version?view=full", true},
			{"same-origin", "https://pve.example.test/proxy/version", true},
			{"default-port-and-case", "https://PVE.example.test:443/proxy/version", true},
			{"downgrade", "http://pve.example.test/proxy/version", false},
			{"other-port", "https://pve.example.test:8006/proxy/version", false},
			{"subdomain", "https://peer.pve.example.test/proxy/version", false},
			{"other-host", "https://pbs.example.test/proxy/version", false},
			{"userinfo", "https://synthetic:password@pve.example.test/proxy/version", false},
		} {
			t.Run(fmt.Sprintf("%d/%s", status, destination.name), func(t *testing.T) {
				client := CreateHTTPClient(true, "")
				calls := 0
				client.Transport = infrastructureRedirectTransport(func(r *http.Request) (*http.Response, error) {
					calls++
					response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}")), Request: r}
					if calls == 1 {
						response.StatusCode = status
						response.Header.Set("Location", destination.location)
					}
					return response, nil
				})
				req, err := http.NewRequest("POST", "https://pve.example.test/api2/json/version", strings.NewReader("synthetic-body"))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer synthetic-token")
				req.Header.Set("Cookie", "PVEAuthCookie=synthetic-ticket")
				req.Header.Set("CSRFPreventionToken", "synthetic-csrf")
				response, err := client.Do(req)
				if response != nil {
					response.Body.Close()
				}
				if destination.allowed {
					if err != nil || calls != 2 {
						t.Fatalf("same-origin path redirect: requests=%d err=%v", calls, err)
					}
				} else if err == nil || calls != 1 {
					t.Fatalf("untrusted redirect reached transport: requests=%d err=%v", calls, err)
				}
			})
		}
	}
}

func TestInfrastructureRedirectChainAndLimit(t *testing.T) {
	for _, escape := range []bool{false, true} {
		t.Run(fmt.Sprintf("escape=%t", escape), func(t *testing.T) {
			client := CreateHTTPClient(false, "")
			calls := 0
			client.Transport = infrastructureRedirectTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				location := fmt.Sprintf("/proxy/step-%d", calls)
				if escape && calls == 2 {
					location = "http://pve.example.test/stolen"
				}
				return &http.Response{StatusCode: 307, Header: http.Header{"Location": {location}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			})
			response, err := client.Get("https://pve.example.test/api2/json/version")
			if response != nil {
				response.Body.Close()
			}
			wantCalls := 10
			if escape {
				wantCalls = 2
			}
			if err == nil || calls != wantCalls {
				t.Fatalf("chain refusal: requests=%d want=%d err=%v", calls, wantCalls, err)
			}
		})
	}
}

func TestInfrastructureRedirectRefusesConnectedOriginChanges(t *testing.T) {
	for _, mode := range []string{"ca", "pin", "explicit-opt-out"} {
		for _, destination := range []string{"plaintext", "other-tls-port"} {
			t.Run(mode+"/"+destination, func(t *testing.T) {
				var destinationRequests atomic.Int32
				sinkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					destinationRequests.Add(1)
					io.Copy(io.Discard, r.Body)
					w.Write([]byte("{}"))
				})
				sink := httptest.NewUnstartedServer(sinkHandler)
				if destination == "plaintext" {
					sink.Start()
				} else {
					sink.StartTLS()
				}
				defer sink.Close()
				source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, sink.URL+"/redirected", http.StatusTemporaryRedirect)
				}))
				defer source.Close()
				// httptest's TLS fixture certificate is shared by both servers:
				// pin/CA rejection must not accidentally stand in for origin policy.
				client := CreateHTTPClient(mode != "explicit-opt-out", "")
				transport := client.Transport.(*http.Transport)
				if mode == "pin" {
					digest := sha256.Sum256(source.Certificate().Raw)
					transport.TLSClientConfig = FingerprintVerifier(hex.EncodeToString(digest[:]))
				} else if mode == "ca" {
					roots := x509.NewCertPool()
					roots.AddCert(source.Certificate())
					transport.TLSClientConfig.RootCAs = roots
				}
				defer client.CloseIdleConnections()
				req, err := http.NewRequest("POST", source.URL+"/api2/json/access/ticket", strings.NewReader("synthetic-password"))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer synthetic-token")
				req.Header.Set("Cookie", "PVEAuthCookie=synthetic-ticket")
				req.Header.Set("CSRFPreventionToken", "synthetic-csrf")
				response, err := client.Do(req)
				if response != nil {
					response.Body.Close()
				}
				if err == nil || !strings.Contains(err.Error(), "same origin") || destinationRequests.Load() != 0 {
					t.Fatalf("changed origin reached actual destination: requests=%d err=%v", destinationRequests.Load(), err)
				}
			})
		}
	}
}
