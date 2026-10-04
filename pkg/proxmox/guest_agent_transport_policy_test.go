package proxmox

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuestAgentCommandClientPreservesPolicyWithoutMutation(t *testing.T) {
	var proxyCalls, dialCalls, verificationCalls int
	originalError := errors.New("fixture policy")
	base := &http.Transport{
		Proxy:           func(*http.Request) (*url.URL, error) { proxyCalls++; return nil, originalError },
		DialContext:     func(context.Context, string, string) (net.Conn, error) { dialCalls++; return nil, originalError },
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, ServerName: "fixture.invalid", NextProtos: []string{"h2", "http/1.1"}, VerifyPeerCertificate: func([][]byte, [][]*x509.Certificate) error { verificationCalls++; return originalError }},
		MaxConnsPerHost: 7, ResponseHeaderTimeout: 2 * time.Second,
		ForceAttemptHTTP2: true,
	}
	original := &http.Client{Transport: base, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return originalError }}
	command, err := guestAgentCommandClient(original)
	if err != nil {
		t.Fatal(err)
	}
	transport := command.Transport.(*http.Transport)
	if transport == base || transport.TLSClientConfig == base.TLSClientConfig || command == original {
		t.Fatal("command policy aliases mutable original configuration")
	}
	if command.Timeout != original.Timeout || transport.MaxConnsPerHost != 7 || transport.ResponseHeaderTimeout != 2*time.Second {
		t.Fatal("command lost request bounds")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS13 || transport.TLSClientConfig.ServerName != "fixture.invalid" {
		t.Fatal("command lost TLS policy")
	}
	transport.Proxy(nil)
	transport.DialContext(context.Background(), "tcp", "fixture.invalid")
	if !errors.Is(transport.TLSClientConfig.VerifyPeerCertificate(nil, nil), originalError) || proxyCalls != 1 || dialCalls != 1 || verificationCalls != 1 {
		t.Fatal("command replaced configured proxy, dial or verification policy")
	}
	if !transport.DisableKeepAlives || transport.ForceAttemptHTTP2 || !transport.Protocols.HTTP1() || transport.Protocols.HTTP2() || transport.Protocols.UnencryptedHTTP2() || transport.TLSNextProto != nil {
		t.Fatal("command transport admits connection/stream replay")
	}
	if strings.Join(transport.TLSClientConfig.NextProtos, ",") != "http/1.1" {
		t.Fatal("command still advertises an alternate stream protocol")
	}
	if !errors.Is(command.CheckRedirect(nil, nil), http.ErrUseLastResponse) {
		t.Fatal("command follows redirects")
	}
	if base.DisableKeepAlives || !base.ForceAttemptHTTP2 || base.Protocols != nil || strings.Join(base.TLSClientConfig.NextProtos, ",") != "h2,http/1.1" || !errors.Is(original.CheckRedirect(nil, nil), originalError) {
		t.Fatal("command changed ordinary transport/redirect policy")
	}
}

func TestGuestAgentCommandClientRejectsUnverifiableTransports(t *testing.T) {
	var calls atomic.Int32
	for name, client := range map[string]*http.Client{
		"missing client":      nil,
		"opaque transport":    {Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("must not execute") })},
		"typed nil transport": {Transport: (*http.Transport)(nil)},
		"TLS dial":            {Transport: &http.Transport{DialTLS: func(string, string) (net.Conn, error) { calls.Add(1); return nil, errors.New("must not execute") }}},
		"TLS context dial": {Transport: &http.Transport{DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
			calls.Add(1)
			return nil, errors.New("must not execute")
		}}},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Client{httpClient: client}
			if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-transport-unverified" {
				t.Fatalf("unverified transport admitted: %v", err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("transport policy failure dispatched work")
	}
	if command, err := guestAgentCommandClient(&http.Client{}); err != nil || command == nil {
		t.Fatalf("standard default transport not admitted: %v", err)
	}
}

func TestGuestAgentCommandUsesTLSHTTP1AndSeparateConnections(t *testing.T) {
	for _, mode := range []string{"trusted CA", "pinned fingerprint"} {
		t.Run(mode, func(t *testing.T) {
			var configs, commands atomic.Int32
			var mu sync.Mutex
			peers := make(map[string]struct{})
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					configs.Add(1)
					if r.ProtoMajor != 2 {
						t.Error("ordinary API stopped using configured HTTP/2")
					}
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				commands.Add(1)
				if r.TLS == nil || r.ProtoMajor != 1 || !r.Close {
					t.Error("guest command did not use single-use TLS HTTP/1")
				}
				if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
					t.Error("guest command lost configured authentication")
				}
				mu.Lock()
				peers[r.RemoteAddr] = struct{}{}
				mu.Unlock()
				backupAgentPayload(w, r)
			}))
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()
			cfg := ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", VerifySSL: true, Timeout: time.Second}
			if mode == "pinned fingerprint" {
				cfg.Fingerprint = fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))
			}
			client, err := NewClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			transport := client.httpClient.Transport.(*http.Transport)
			if mode == "trusted CA" {
				roots := x509.NewCertPool()
				roots.AddCert(server.Certificate())
				transport.TLSClientConfig.RootCAs = roots
			}
			transport.ForceAttemptHTTP2 = true
			transport.Protocols = new(http.Protocols)
			transport.Protocols.SetHTTP1(true)
			transport.Protocols.SetHTTP2(true)
			for name, read := range backupAgentReads() {
				t.Run(name, func(t *testing.T) {
					if err := read(context.Background(), client, 105); err != nil {
						t.Fatal(err)
					}
				})
			}
			if configs.Load() != 12 || commands.Load() != 6 || len(peers) != 6 {
				t.Fatalf("config/command/connection counts = %d/%d/%d, want 12/6/6", configs.Load(), commands.Load(), len(peers))
			}
			if transport.DisableKeepAlives || !transport.Protocols.HTTP2() {
				t.Fatal("command mutated shared ordinary transport")
			}
		})
	}
}

func TestGuestAgentCommandTransportStillRejectsUntrustedTLS(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); backupAgentPayload(w, r) }))
	defer server.Close()
	for _, fingerprint := range []string{"", strings.Repeat("00", 32)} {
		client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", VerifySSL: true, Fingerprint: fingerprint, Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		command, err := guestAgentCommandClient(client.httpClient)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/agent/info", nil)
		if err != nil {
			t.Fatal(err)
		}
		if response, err := command.Do(req); err == nil {
			response.Body.Close()
			t.Fatal("command transport accepted untrusted peer")
		}
		command.CloseIdleConnections()
	}
	if requests.Load() != 0 {
		t.Fatal("untrusted TLS reached a guest endpoint")
	}
}
