package discovery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInferTypeFromMetadata(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		parts []string
		want  string
	}{
		{
			name:  "detects PMG from auth header",
			parts: []string{`PMGAuth realm="Proxmox Mail Gateway"`, "pmgproxy/4.0"},
			want:  "pmg",
		},
		{
			name:  "detects PVE from realm string",
			parts: []string{`PVEAuth realm="Proxmox Virtual Environment"`, "pve-api-daemon/3.0"},
			want:  "pve",
		},
		{
			name:  "detects PBS from cookie",
			parts: []string{"PBS", "PBSCookie=abc123", `PBSAuth realm="Proxmox Backup Server"`},
			want:  "pbs",
		},
		{
			name:  "returns empty when no markers",
			parts: []string{"Custom Certificate", "Example Corp"},
			want:  "",
		},
		{
			name:  "tolerates compact strings",
			parts: []string{"ProxmoxMailGateway"},
			want:  "pmg",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := inferTypeFromMetadata(tc.parts...); got != tc.want {
				t.Fatalf("inferTypeFromMetadata(%v) = %q, want %q", tc.parts, got, tc.want)
			}
		})
	}
}

func TestInferTypeFromCertificate(t *testing.T) {
	t.Parallel()

	state := tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{
			{
				Subject: pkix.Name{
					CommonName:         "Proxmox Mail Gateway",
					Organization:       []string{"Proxmox"},
					OrganizationalUnit: []string{"PMG"},
				},
			},
		},
	}

	if got := inferTypeFromCertificate(state); got != "pmg" {
		t.Fatalf("inferTypeFromCertificate returned %q, want %q", got, "pmg")
	}

	if got := inferTypeFromCertificate(tls.ConnectionState{}); got != "" {
		t.Fatalf("expected empty result for missing certificates, got %q", got)
	}
}

func TestDetectProductFromEndpoint(t *testing.T) {
	t.Parallel()

	var requestPaths []string

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		switch {
		case strings.Contains(r.URL.Path, "statistics/mail"):
			w.Header().Set("Proxmox-Product", "Proxmox Mail Gateway")
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "api2/json/version"):
			w.Header().Set("Proxmox-Product", "Proxmox Backup Server")
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "mail/queue"):
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	scanner := &Scanner{
		timeout:    time.Second,
		httpClient: ts.Client(),
	}

	address := strings.TrimPrefix(ts.URL, "https://")
	if product := scanner.detectProductFromEndpoint(context.Background(), address, "api2/json/statistics/mail"); product != "pmg" {
		t.Fatalf("detectProductFromEndpoint returned %q, want %q", product, "pmg")
	}

	if product := scanner.detectProductFromEndpoint(context.Background(), address, "api2/json/version"); product != "pbs" {
		t.Fatalf("detectProductFromEndpoint returned %q, want %q", product, "pbs")
	}

	if product := scanner.detectProductFromEndpoint(context.Background(), address, "api2/json/unknown/path"); product != "" {
		t.Fatalf("expected empty result for unknown endpoint, got %q", product)
	}

	if len(requestPaths) == 0 {
		t.Fatalf("expected detectProductFromEndpoint to perform requests")
	}
}

func TestIsPMGServer(t *testing.T) {
	t.Parallel()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "statistics/mail") {
			w.Header().Set("Proxmox-Product", "Proxmox Mail Gateway")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	scanner := &Scanner{
		timeout:    time.Second,
		httpClient: ts.Client(),
	}

	address := strings.TrimPrefix(ts.URL, "https://")
	if !scanner.isPMGServer(context.Background(), address) {
		t.Fatalf("expected PMG detection to succeed")
	}

	tsNoMatch := httptest.NewTLSServer(http.NotFoundHandler())
	defer tsNoMatch.Close()

	scanner.httpClient = tsNoMatch.Client()
	address = strings.TrimPrefix(tsNoMatch.URL, "https://")
	if scanner.isPMGServer(context.Background(), address) {
		t.Fatalf("expected PMG detection to fail for endpoints without markers")
	}
}

func TestCheckServerRetrievesVersion(t *testing.T) {
	t.Parallel()

	const responseVersion = `{"data":{"version":"2.4.1","release":"1"}}`

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/version" {
			w.Header().Set("Content-Type", "application/json")
			http.SetCookie(w, &http.Cookie{Name: "PBSCookie", Value: "abc"})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(responseVersion))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	host, portStr, err := net.SplitHostPort(ts.Listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("strconv.Atoi: %v", err)
	}

	scanner := &Scanner{
		timeout:    time.Second,
		httpClient: ts.Client(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server := scanner.checkServer(ctx, host, port, "pbs")
	if server == nil {
		t.Fatalf("checkServer returned nil")
	}

	if server.Type != "pbs" {
		t.Fatalf("expected type pbs, got %q", server.Type)
	}

	if server.Version != "2.4.1" {
		t.Fatalf("expected version 2.4.1, got %q", server.Version)
	}

	if server.Release != "1" {
		t.Fatalf("expected release 1, got %q", server.Release)
	}
}

func startTLSServerOn(t *testing.T, addr string, handler http.Handler) *httptest.Server {
	t.Helper()

	srv := httptest.NewUnstartedServer(handler)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Skipf("port %s unavailable: %v", addr, err)
	}
	srv.Listener = ln
	srv.StartTLS()
	t.Cleanup(func() { srv.Close() })
	return srv
}

func TestCheckServerHandlesUnauthorized(t *testing.T) {
	t.Parallel()

	unauthorizedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", "PVEAuth realm=\"Proxmox Virtual Environment\"")
		w.WriteHeader(http.StatusUnauthorized)
	})

	srv := startTLSServerOn(t, "127.0.0.1:9008", unauthorizedHandler)
	_ = srv

	scanner := &Scanner{
		timeout: time.Second,
		httpClient: &http.Client{
			Timeout:   500 * time.Millisecond,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server := scanner.checkServer(ctx, "127.0.0.1", 9008, "pve")
	if server == nil {
		t.Fatalf("expected server discovery despite unauthorized response")
	}

	if server.Type != "pve" {
		t.Fatalf("expected type pve, got %q", server.Type)
	}

	if server.Version != "Unknown" {
		t.Fatalf("expected version Unknown, got %q", server.Version)
	}
}

func TestDiscoverServersWithCallback(t *testing.T) {
	t.Parallel()

	const subnet = "127.0.0.0/29"

	noTLSListener, err := net.Listen("tcp", "127.0.0.1:9009")
	if err != nil {
		t.Fatalf("failed to listen on 9009: %v", err)
	}
	go func() {
		for {
			conn, err := noTLSListener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	t.Cleanup(func() { noTLSListener.Close() })

	pveHandler := http.NewServeMux()
	pveHandler.HandleFunc("/api2/json/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Proxmox-Product", "Proxmox Virtual Environment")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{
				"version": "8.1",
				"release": "1",
			},
		})
	})

	pbsHandler := http.NewServeMux()
	pbsHandler.HandleFunc("/api2/json/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Proxmox-Product", "Proxmox Backup Server")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{
				"version": "2.4.1",
				"release": "2",
			},
		})
	})

	pveServer := startTLSServerOn(t, "127.0.0.1:8006", pveHandler)
	_ = pveServer
	pbsServer := startTLSServerOn(t, "127.0.0.1:8007", pbsHandler)
	_ = pbsServer

	scanner := NewScanner()
	scanner.concurrent = 4
	scanner.timeout = 200 * time.Millisecond
	scanner.httpClient = &http.Client{
		Timeout:   500 * time.Millisecond,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var mu sync.Mutex
	var callbacks []DiscoveredServer

	// Add a manual check for the TCP-only port.
	// We call the unexported helper directly inside the package to ensure it does not panic.
	if server := scanner.checkServer(ctx, "127.0.0.1", 9009, "pve"); server == nil {
		// keep discovery results unaffected but verify we survive the TCP-only host
		t.Fatalf("expected checkServer to handle TCP-only host without panic")
	}

	result, err := scanner.DiscoverServersWithCallback(ctx, subnet, func(server DiscoveredServer) {
		mu.Lock()
		callbacks = append(callbacks, server)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("DiscoverServersWithCallback returned error: %v", err)
	}

	if len(result.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d: %+v", len(result.Servers), result.Servers)
	}

	seen := make(map[string]DiscoveredServer, len(result.Servers))
	for _, server := range result.Servers {
		seen[server.Type] = server
	}

	pve, ok := seen["pve"]
	if !ok {
		t.Fatalf("expected to discover pve server")
	}
	if pve.Version != "8.1" {
		t.Fatalf("expected pve version 8.1, got %q", pve.Version)
	}

	pbs, ok := seen["pbs"]
	if !ok {
		t.Fatalf("expected to discover pbs server")
	}
	if pbs.Version != "2.4.1" {
		t.Fatalf("expected pbs version 2.4.1, got %q", pbs.Version)
	}

	mu.Lock()
	callbackCount := len(callbacks)
	mu.Unlock()
	if callbackCount < 2 {
		t.Fatalf("expected callbacks for both servers, got %d", callbackCount)
	}
}
