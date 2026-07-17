package securityutil

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRestrictedOutboundHTTPClient_BlocksMetadataServiceEvenViaProxy(t *testing.T) {
	client := NewRestrictedOutboundHTTPClient(0, RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
	})

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://169.254.169.254/api/version", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error for metadata service address, got nil")
	}
	if !strings.Contains(err.Error(), "metadata service address is not allowed") {
		t.Fatalf("expected 'metadata service address is not allowed', got %v", err)
	}
}

// A host like "localhost" can resolve to ::1 ahead of 127.0.0.1 while the
// target service (e.g. Ollama's default bind) listens only on 127.0.0.1. The
// restricted dialer must fall through to later permitted IPs instead of
// failing on the first refused address family.
func TestRestrictedOutboundHTTPClient_TriesAllPermittedResolvedIPs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	serverHost, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse test server address: %v", err)
	}
	if serverHost != "127.0.0.1" {
		t.Skipf("test server bound to %s, need an IPv4 loopback listener", serverHost)
	}

	client := NewRestrictedOutboundHTTPClient(0, RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
		ResolveIPAddrs: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.ParseIP("::1")},
				{IP: net.ParseIP("127.0.0.1")},
			}, nil
		},
	})

	resp, err := client.Get("http://ipv6-first.test:" + port + "/")
	if err != nil {
		t.Fatalf("expected fallback to the second permitted IP to succeed, got %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", string(body))
	}
}

func TestRestrictedOutboundHTTPClient_BlocksLinkLocalEvenViaProxy(t *testing.T) {
	client := NewRestrictedOutboundHTTPClient(0, RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
	})

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://169.254.10.20/test", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error for link-local address, got nil")
	}
	if !strings.Contains(err.Error(), "link-local addresses are not allowed") {
		t.Fatalf("expected 'link-local addresses are not allowed', got %v", err)
	}
}
