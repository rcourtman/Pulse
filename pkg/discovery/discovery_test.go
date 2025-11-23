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

	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery/envdetect"
)

func newTestScanner(client *http.Client) *Scanner {
	policy := envdetect.DefaultScanPolicy()
	policy.DialTimeout = time.Second
	profile := &envdetect.EnvironmentProfile{
		Policy:   policy,
		Metadata: map[string]string{},
	}

	return &Scanner{
		policy:     policy,
		profile:    profile,
		httpClient: client,
	}
}

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

	scanner := newTestScanner(ts.Client())

	address := strings.TrimPrefix(ts.URL, "https://")
	finding := scanner.ProbeAPIEndpoint(context.Background(), address, "api2/json/statistics/mail")
	if finding.ProductGuess != ProductPMG {
		t.Fatalf("ProbeAPIEndpoint returned %q, want %q", finding.ProductGuess, ProductPMG)
	}

	versionFinding := scanner.ProbeAPIEndpoint(context.Background(), address, "api2/json/version")
	if versionFinding.ProductGuess != ProductPBS {
		t.Fatalf("ProbeAPIEndpoint returned %q, want %q", versionFinding.ProductGuess, ProductPBS)
	}

	unknownFinding := scanner.ProbeAPIEndpoint(context.Background(), address, "api2/json/unknown/path")
	if unknownFinding.ProductGuess != "" || unknownFinding.Status != http.StatusNotFound {
		t.Fatalf("expected empty result for unknown endpoint, got %+v", unknownFinding)
	}

	if len(requestPaths) == 0 {
		t.Fatalf("expected ProbeAPIEndpoint to perform requests")
	}
}

func TestIsPMGServer(t *testing.T) {
	t.Parallel()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Proxmox-Product", "Proxmox Mail Gateway")
		w.Header().Set("WWW-Authenticate", `PMGAuth realm="Proxmox Mail Gateway"`)
		if strings.Contains(r.URL.Path, "statistics/mail") ||
			strings.Contains(r.URL.Path, "api2/json/version") {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	scanner := newTestScanner(ts.Client())

	host, portStr, err := net.SplitHostPort(ts.Listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("strconv.Atoi: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	probe := scanner.ProbeProxmoxService(ctx, host, port)
	if probe == nil || !probe.Positive || probe.PrimaryProduct != ProductPMG {
		t.Fatalf("expected PMG detection to succeed, got %+v", probe)
	}

	tsNoMatch := httptest.NewTLSServer(http.NotFoundHandler())
	defer tsNoMatch.Close()

	scanner.httpClient = tsNoMatch.Client()
	host, portStr, err = net.SplitHostPort(tsNoMatch.Listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, err = strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("strconv.Atoi: %v", err)
	}
	probe = scanner.ProbeProxmoxService(ctx, host, port)
	if probe != nil && probe.Positive {
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

	scanner := newTestScanner(ts.Client())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	probe := scanner.ProbeProxmoxService(ctx, host, port)
	if probe == nil || !probe.Positive {
		t.Fatalf("ProbeProxmoxService returned nil")
	}

	if probe.PrimaryProduct != ProductPBS {
		t.Fatalf("expected product pbs, got %q", probe.PrimaryProduct)
	}

	if probe.Version != "2.4.1" {
		t.Fatalf("expected version 2.4.1, got %q", probe.Version)
	}

	if probe.Release != "1" {
		t.Fatalf("expected release 1, got %q", probe.Release)
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

	scanner := newTestScanner(&http.Client{
		Timeout:   500 * time.Millisecond,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	probe := scanner.ProbeProxmoxService(ctx, "127.0.0.1", 9008)
	if probe == nil || !probe.Positive {
		t.Fatalf("expected server discovery despite unauthorized response: %+v", probe)
	}

	if probe.PrimaryProduct != ProductPVE {
		t.Fatalf("expected product pve, got %q", probe.PrimaryProduct)
	}

	if probe.Version != "Unknown" {
		t.Fatalf("expected version Unknown, got %q", probe.Version)
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

	scanner := newTestScanner(&http.Client{
		Timeout:   500 * time.Millisecond,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	})
	scanner.policy.MaxConcurrent = 4
	scanner.policy.DialTimeout = 200 * time.Millisecond
	scanner.policy.HTTPTimeout = 500 * time.Millisecond
	if scanner.profile != nil {
		scanner.profile.Policy = scanner.policy
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var mu sync.Mutex
	var callbacks []DiscoveredServer

	// Add a manual check for the TCP-only port.
	if probe := scanner.ProbeProxmoxService(ctx, "127.0.0.1", 9009); probe != nil && probe.Positive {
		t.Fatalf("expected ProbeProxmoxService to ignore TCP-only host, got %+v", probe)
	}

	result, err := scanner.DiscoverServersWithCallback(ctx, subnet, func(server DiscoveredServer, phase string) {
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

func TestDiscoverServersCancelledContext(t *testing.T) {
	t.Parallel()

	scanner := NewScanner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := scanner.DiscoverServersWithCallback(ctx, "127.0.0.1/32", nil)
	if err == nil {
		t.Fatalf("expected context error, got nil")
	}
	if result == nil {
		t.Fatalf("expected result object even on cancellation")
	}
	if len(result.Servers) != 0 {
		t.Fatalf("expected no servers on cancelled context")
	}
}

func TestPBSDiscoveryWithUnauthorizedVersion(t *testing.T) {
	// Handler that simulates PBS requiring auth for version endpoint
	// and NOT providing any helpful headers initially to test the port+auth heuristic
	pbsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/version" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.NotFound(w, r)
	})

	server := startTLSServerOn(t, "127.0.0.1:8007", pbsHandler)
	_ = server

	scanner := newTestScanner(&http.Client{
		Timeout:   500 * time.Millisecond,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Probe port 8007
	probe := scanner.ProbeProxmoxService(ctx, "127.0.0.1", 8007)

	if probe == nil {
		t.Fatal("ProbeProxmoxService returned nil")
	}

	if !probe.Positive {
		t.Errorf("Expected positive identification for PBS on port 8007 with 401 version response, got negative. Score: %f", probe.PrimaryScore)
	}

	if probe.PrimaryProduct != ProductPBS {
		t.Errorf("Expected product %q, got %q", ProductPBS, probe.PrimaryProduct)
	}
}
