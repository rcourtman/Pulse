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
		t.Skipf("port 9009 unavailable: %v", err)
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

func TestFriendlyPhaseName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    string
		expected string
	}{
		{"lxc container network", "lxc_container_network", "Container network"},
		{"docker bridge network", "docker_bridge_network", "Docker bridge network"},
		{"docker container network", "docker_container_network", "Docker container network"},
		{"host local network", "host_local_network", "Local network"},
		{"inferred gateway network", "inferred_gateway_network", "Gateway network"},
		{"extra targets", "extra_targets", "Additional targets"},
		{"proxmox cluster network", "proxmox_cluster_network", "Proxmox cluster network"},
		{"unknown phase returns as-is", "some_unknown_phase", "some_unknown_phase"},
		{"empty string returns empty", "", ""},
		{"manual subnet passthrough", "manual_subnet", "manual_subnet"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result := friendlyPhaseName(tc.phase)
			if result != tc.expected {
				t.Errorf("friendlyPhaseName(%q) = %q, want %q", tc.phase, result, tc.expected)
			}
		})
	}
}

func TestDefaultProductsForPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		port     int
		expected []string
	}{
		{"port 8006 returns PVE and PMG", 8006, []string{productPVE, productPMG}},
		{"port 8007 returns PBS", 8007, []string{productPBS}},
		{"port 443 returns nil", 443, nil},
		{"port 80 returns nil", 80, nil},
		{"port 0 returns nil", 0, nil},
		{"random port returns nil", 12345, nil},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result := defaultProductsForPort(tc.port)
			if tc.expected == nil {
				if result != nil {
					t.Errorf("defaultProductsForPort(%d) = %v, want nil", tc.port, result)
				}
				return
			}
			if len(result) != len(tc.expected) {
				t.Errorf("defaultProductsForPort(%d) = %v, want %v", tc.port, result, tc.expected)
				return
			}
			for i, v := range tc.expected {
				if result[i] != v {
					t.Errorf("defaultProductsForPort(%d)[%d] = %q, want %q", tc.port, i, result[i], v)
				}
			}
		})
	}
}

func TestCloneHeader(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		result := cloneHeader(nil)
		if result != nil {
			t.Errorf("cloneHeader(nil) = %v, want nil", result)
		}
	})

	t.Run("empty header returns empty header", func(t *testing.T) {
		input := http.Header{}
		result := cloneHeader(input)
		if result == nil {
			t.Fatal("cloneHeader(empty) returned nil")
		}
		if len(result) != 0 {
			t.Errorf("cloneHeader(empty) has length %d, want 0", len(result))
		}
	})

	t.Run("clones single-value headers", func(t *testing.T) {
		input := http.Header{
			"Content-Type":  []string{"application/json"},
			"Authorization": []string{"Bearer token123"},
		}
		result := cloneHeader(input)

		if result.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", result.Get("Content-Type"), "application/json")
		}
		if result.Get("Authorization") != "Bearer token123" {
			t.Errorf("Authorization = %q, want %q", result.Get("Authorization"), "Bearer token123")
		}
	})

	t.Run("clones multi-value headers", func(t *testing.T) {
		input := http.Header{
			"Set-Cookie": []string{"cookie1=val1", "cookie2=val2", "cookie3=val3"},
		}
		result := cloneHeader(input)

		cookies := result.Values("Set-Cookie")
		if len(cookies) != 3 {
			t.Fatalf("Set-Cookie count = %d, want 3", len(cookies))
		}
		if cookies[0] != "cookie1=val1" || cookies[1] != "cookie2=val2" || cookies[2] != "cookie3=val3" {
			t.Errorf("Set-Cookie values = %v, want [cookie1=val1 cookie2=val2 cookie3=val3]", cookies)
		}
	})

	t.Run("clone is independent of original", func(t *testing.T) {
		input := http.Header{
			"X-Custom": []string{"original"},
		}
		result := cloneHeader(input)

		// Modify original
		input.Set("X-Custom", "modified")

		// Clone should be unaffected
		if result.Get("X-Custom") != "original" {
			t.Errorf("clone was affected by original modification: got %q, want %q", result.Get("X-Custom"), "original")
		}
	})

	t.Run("original is independent of clone", func(t *testing.T) {
		input := http.Header{
			"X-Custom": []string{"original"},
		}
		result := cloneHeader(input)

		// Modify clone
		result.Set("X-Custom", "modified")

		// Original should be unaffected
		if input.Get("X-Custom") != "original" {
			t.Errorf("original was affected by clone modification: got %q, want %q", input.Get("X-Custom"), "original")
		}
	})
}

func TestCopyMetadata(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns empty map", func(t *testing.T) {
		result := copyMetadata(nil)
		if result == nil {
			t.Fatal("copyMetadata(nil) returned nil, want empty map")
		}
		if len(result) != 0 {
			t.Errorf("copyMetadata(nil) has length %d, want 0", len(result))
		}
	})

	t.Run("empty map returns empty map", func(t *testing.T) {
		input := map[string]string{}
		result := copyMetadata(input)
		if result == nil {
			t.Fatal("copyMetadata(empty) returned nil")
		}
		if len(result) != 0 {
			t.Errorf("copyMetadata(empty) has length %d, want 0", len(result))
		}
	})

	t.Run("copies all entries", func(t *testing.T) {
		input := map[string]string{
			"key1":        "value1",
			"key2":        "value2",
			"environment": "docker_bridge",
		}
		result := copyMetadata(input)

		if len(result) != 3 {
			t.Fatalf("copyMetadata returned %d entries, want 3", len(result))
		}
		if result["key1"] != "value1" {
			t.Errorf("key1 = %q, want %q", result["key1"], "value1")
		}
		if result["key2"] != "value2" {
			t.Errorf("key2 = %q, want %q", result["key2"], "value2")
		}
		if result["environment"] != "docker_bridge" {
			t.Errorf("environment = %q, want %q", result["environment"], "docker_bridge")
		}
	})

	t.Run("clone is independent of original", func(t *testing.T) {
		input := map[string]string{"key": "original"}
		result := copyMetadata(input)

		// Modify original
		input["key"] = "modified"

		// Clone should be unaffected
		if result["key"] != "original" {
			t.Errorf("clone was affected by original modification: got %q, want %q", result["key"], "original")
		}
	})

	t.Run("original is independent of clone", func(t *testing.T) {
		input := map[string]string{"key": "original"}
		result := copyMetadata(input)

		// Modify clone
		result["key"] = "modified"

		// Original should be unaffected
		if input["key"] != "original" {
			t.Errorf("original was affected by clone modification: got %q, want %q", input["key"], "original")
		}
	})
}

func TestEnsurePolicyDefaults(t *testing.T) {
	t.Parallel()

	defaults := envdetect.DefaultScanPolicy()

	t.Run("zero policy gets defaults for zero/negative fields", func(t *testing.T) {
		input := envdetect.ScanPolicy{}
		result := ensurePolicyDefaults(input)

		if result.MaxConcurrent != defaults.MaxConcurrent {
			t.Errorf("MaxConcurrent = %d, want %d", result.MaxConcurrent, defaults.MaxConcurrent)
		}
		if result.DialTimeout != defaults.DialTimeout {
			t.Errorf("DialTimeout = %v, want %v", result.DialTimeout, defaults.DialTimeout)
		}
		if result.HTTPTimeout != defaults.HTTPTimeout {
			t.Errorf("HTTPTimeout = %v, want %v", result.HTTPTimeout, defaults.HTTPTimeout)
		}
		// MaxHostsPerScan = 0 is preserved (means unlimited), only < 0 gets default
		if result.MaxHostsPerScan != 0 {
			t.Errorf("MaxHostsPerScan = %d, want 0 (zero is preserved as unlimited)", result.MaxHostsPerScan)
		}
	})

	t.Run("negative MaxConcurrent gets default", func(t *testing.T) {
		input := envdetect.ScanPolicy{MaxConcurrent: -1}
		result := ensurePolicyDefaults(input)
		if result.MaxConcurrent != defaults.MaxConcurrent {
			t.Errorf("MaxConcurrent = %d, want %d", result.MaxConcurrent, defaults.MaxConcurrent)
		}
	})

	t.Run("positive MaxConcurrent preserved", func(t *testing.T) {
		input := envdetect.ScanPolicy{MaxConcurrent: 100}
		result := ensurePolicyDefaults(input)
		if result.MaxConcurrent != 100 {
			t.Errorf("MaxConcurrent = %d, want 100", result.MaxConcurrent)
		}
	})

	t.Run("negative DialTimeout gets default", func(t *testing.T) {
		input := envdetect.ScanPolicy{DialTimeout: -time.Second}
		result := ensurePolicyDefaults(input)
		if result.DialTimeout != defaults.DialTimeout {
			t.Errorf("DialTimeout = %v, want %v", result.DialTimeout, defaults.DialTimeout)
		}
	})

	t.Run("positive DialTimeout preserved", func(t *testing.T) {
		input := envdetect.ScanPolicy{DialTimeout: 5 * time.Second}
		result := ensurePolicyDefaults(input)
		if result.DialTimeout != 5*time.Second {
			t.Errorf("DialTimeout = %v, want 5s", result.DialTimeout)
		}
	})

	t.Run("negative HTTPTimeout gets default", func(t *testing.T) {
		input := envdetect.ScanPolicy{HTTPTimeout: -time.Second}
		result := ensurePolicyDefaults(input)
		if result.HTTPTimeout != defaults.HTTPTimeout {
			t.Errorf("HTTPTimeout = %v, want %v", result.HTTPTimeout, defaults.HTTPTimeout)
		}
	})

	t.Run("positive HTTPTimeout preserved", func(t *testing.T) {
		input := envdetect.ScanPolicy{HTTPTimeout: 10 * time.Second}
		result := ensurePolicyDefaults(input)
		if result.HTTPTimeout != 10*time.Second {
			t.Errorf("HTTPTimeout = %v, want 10s", result.HTTPTimeout)
		}
	})

	t.Run("zero MaxHostsPerScan preserved (unlimited)", func(t *testing.T) {
		input := envdetect.ScanPolicy{MaxHostsPerScan: 0}
		result := ensurePolicyDefaults(input)
		// MaxHostsPerScan = 0 means unlimited, should be preserved (not replaced with default)
		if result.MaxHostsPerScan != 0 {
			t.Errorf("MaxHostsPerScan = %d, want 0 (zero should be preserved as unlimited)", result.MaxHostsPerScan)
		}
	})

	t.Run("negative MaxHostsPerScan gets default", func(t *testing.T) {
		input := envdetect.ScanPolicy{MaxHostsPerScan: -1}
		result := ensurePolicyDefaults(input)
		if result.MaxHostsPerScan != defaults.MaxHostsPerScan {
			t.Errorf("MaxHostsPerScan = %d, want %d", result.MaxHostsPerScan, defaults.MaxHostsPerScan)
		}
	})

	t.Run("positive MaxHostsPerScan preserved", func(t *testing.T) {
		input := envdetect.ScanPolicy{MaxHostsPerScan: 500}
		result := ensurePolicyDefaults(input)
		if result.MaxHostsPerScan != 500 {
			t.Errorf("MaxHostsPerScan = %d, want 500", result.MaxHostsPerScan)
		}
	})

	t.Run("boolean fields preserved", func(t *testing.T) {
		input := envdetect.ScanPolicy{
			EnableReverseDNS: true,
			ScanGateways:     false,
		}
		result := ensurePolicyDefaults(input)
		if result.EnableReverseDNS != true {
			t.Errorf("EnableReverseDNS = %v, want true", result.EnableReverseDNS)
		}
		if result.ScanGateways != false {
			t.Errorf("ScanGateways = %v, want false", result.ScanGateways)
		}
	})

	t.Run("all custom values preserved", func(t *testing.T) {
		input := envdetect.ScanPolicy{
			MaxConcurrent:    25,
			DialTimeout:      3 * time.Second,
			HTTPTimeout:      5 * time.Second,
			MaxHostsPerScan:  256,
			EnableReverseDNS: false,
			ScanGateways:     true,
		}
		result := ensurePolicyDefaults(input)

		if result.MaxConcurrent != 25 {
			t.Errorf("MaxConcurrent = %d, want 25", result.MaxConcurrent)
		}
		if result.DialTimeout != 3*time.Second {
			t.Errorf("DialTimeout = %v, want 3s", result.DialTimeout)
		}
		if result.HTTPTimeout != 5*time.Second {
			t.Errorf("HTTPTimeout = %v, want 5s", result.HTTPTimeout)
		}
		if result.MaxHostsPerScan != 256 {
			t.Errorf("MaxHostsPerScan = %d, want 256", result.MaxHostsPerScan)
		}
		if result.EnableReverseDNS != false {
			t.Errorf("EnableReverseDNS = %v, want false", result.EnableReverseDNS)
		}
		if result.ScanGateways != true {
			t.Errorf("ScanGateways = %v, want true", result.ScanGateways)
		}
	})
}

func TestClonePhase(t *testing.T) {
	t.Parallel()

	t.Run("nil subnets returns empty subnets", func(t *testing.T) {
		input := envdetect.SubnetPhase{
			Name:       "test_phase",
			Subnets:    nil,
			Confidence: 0.8,
			Priority:   1,
		}
		result := clonePhase(input)

		if result.Name != "test_phase" {
			t.Errorf("Name = %q, want %q", result.Name, "test_phase")
		}
		if result.Confidence != 0.8 {
			t.Errorf("Confidence = %v, want 0.8", result.Confidence)
		}
		if result.Priority != 1 {
			t.Errorf("Priority = %d, want 1", result.Priority)
		}
		if result.Subnets != nil {
			t.Errorf("Subnets = %v, want nil", result.Subnets)
		}
	})

	t.Run("empty subnets cloned correctly", func(t *testing.T) {
		input := envdetect.SubnetPhase{
			Name:    "empty_phase",
			Subnets: []net.IPNet{},
		}
		result := clonePhase(input)

		if result.Subnets == nil {
			t.Error("Subnets should not be nil for empty input slice")
		}
		if len(result.Subnets) != 0 {
			t.Errorf("Subnets length = %d, want 0", len(result.Subnets))
		}
	})

	t.Run("subnets are deep copied", func(t *testing.T) {
		_, subnet1, _ := net.ParseCIDR("192.168.1.0/24")
		_, subnet2, _ := net.ParseCIDR("10.0.0.0/8")
		input := envdetect.SubnetPhase{
			Name:       "multi_subnet",
			Subnets:    []net.IPNet{*subnet1, *subnet2},
			Confidence: 0.9,
			Priority:   2,
		}
		result := clonePhase(input)

		if len(result.Subnets) != 2 {
			t.Fatalf("Subnets length = %d, want 2", len(result.Subnets))
		}
		if result.Subnets[0].String() != "192.168.1.0/24" {
			t.Errorf("Subnets[0] = %v, want 192.168.1.0/24", result.Subnets[0].String())
		}
		if result.Subnets[1].String() != "10.0.0.0/8" {
			t.Errorf("Subnets[1] = %v, want 10.0.0.0/8", result.Subnets[1].String())
		}
	})

	t.Run("modifications to clone do not affect original", func(t *testing.T) {
		_, subnet1, _ := net.ParseCIDR("172.16.0.0/16")
		input := envdetect.SubnetPhase{
			Name:       "original",
			Subnets:    []net.IPNet{*subnet1},
			Confidence: 0.7,
		}
		result := clonePhase(input)

		// Modify clone
		result.Name = "modified"
		result.Confidence = 0.1
		_, newSubnet, _ := net.ParseCIDR("192.168.0.0/16")
		result.Subnets[0] = *newSubnet

		// Original should be unchanged
		if input.Name != "original" {
			t.Errorf("original Name was modified: got %q", input.Name)
		}
		if input.Confidence != 0.7 {
			t.Errorf("original Confidence was modified: got %v", input.Confidence)
		}
		if input.Subnets[0].String() != "172.16.0.0/16" {
			t.Errorf("original Subnets[0] was modified: got %v", input.Subnets[0].String())
		}
	})
}

func TestCloneProfile(t *testing.T) {
	t.Parallel()

	t.Run("nil profile returns default profile", func(t *testing.T) {
		result := cloneProfile(nil)

		if result == nil {
			t.Fatal("cloneProfile(nil) returned nil")
		}
		if result.Type != envdetect.Unknown {
			t.Errorf("Type = %v, want Unknown", result.Type)
		}
		if result.Confidence != 0.3 {
			t.Errorf("Confidence = %v, want 0.3", result.Confidence)
		}
		if len(result.Warnings) != 1 || result.Warnings[0] != "Environment profile unavailable; using defaults" {
			t.Errorf("Warnings = %v, want single default warning", result.Warnings)
		}
		if result.Metadata == nil {
			t.Error("Metadata should not be nil")
		}
	})

	t.Run("basic fields are copied", func(t *testing.T) {
		input := &envdetect.EnvironmentProfile{
			Type:       envdetect.LXCPrivileged,
			Confidence: 0.95,
		}
		result := cloneProfile(input)

		if result.Type != envdetect.LXCPrivileged {
			t.Errorf("Type = %v, want LXCPrivileged", result.Type)
		}
		if result.Confidence != 0.95 {
			t.Errorf("Confidence = %v, want 0.95", result.Confidence)
		}
	})

	t.Run("metadata is deep copied", func(t *testing.T) {
		input := &envdetect.EnvironmentProfile{
			Type: envdetect.DockerBridge,
			Metadata: map[string]string{
				"gateway": "192.168.1.1",
				"network": "bridge",
			},
		}
		result := cloneProfile(input)

		if result.Metadata["gateway"] != "192.168.1.1" {
			t.Errorf("Metadata[gateway] = %q, want 192.168.1.1", result.Metadata["gateway"])
		}

		// Modify clone
		result.Metadata["gateway"] = "10.0.0.1"
		result.Metadata["new_key"] = "new_value"

		// Original should be unchanged
		if input.Metadata["gateway"] != "192.168.1.1" {
			t.Errorf("original Metadata[gateway] was modified: got %q", input.Metadata["gateway"])
		}
		if _, exists := input.Metadata["new_key"]; exists {
			t.Error("original Metadata has new_key which should not exist")
		}
	})

	t.Run("warnings are deep copied", func(t *testing.T) {
		input := &envdetect.EnvironmentProfile{
			Type:     envdetect.Unknown,
			Warnings: []string{"warning1", "warning2"},
		}
		result := cloneProfile(input)

		if len(result.Warnings) != 2 {
			t.Fatalf("Warnings length = %d, want 2", len(result.Warnings))
		}

		// Modify clone
		result.Warnings[0] = "modified"
		result.Warnings = append(result.Warnings, "warning3")

		// Original should be unchanged
		if input.Warnings[0] != "warning1" {
			t.Errorf("original Warnings[0] was modified: got %q", input.Warnings[0])
		}
		if len(input.Warnings) != 2 {
			t.Errorf("original Warnings length changed: got %d", len(input.Warnings))
		}
	})

	t.Run("extra targets are deep copied", func(t *testing.T) {
		input := &envdetect.EnvironmentProfile{
			Type:         envdetect.Native,
			ExtraTargets: []net.IP{net.ParseIP("192.168.1.100"), net.ParseIP("10.0.0.50")},
		}
		result := cloneProfile(input)

		if len(result.ExtraTargets) != 2 {
			t.Fatalf("ExtraTargets length = %d, want 2", len(result.ExtraTargets))
		}
		if result.ExtraTargets[0].String() != "192.168.1.100" {
			t.Errorf("ExtraTargets[0] = %v, want 192.168.1.100", result.ExtraTargets[0])
		}

		// Modify clone
		result.ExtraTargets[0] = net.ParseIP("1.2.3.4")

		// Original should be unchanged
		if input.ExtraTargets[0].String() != "192.168.1.100" {
			t.Errorf("original ExtraTargets[0] was modified: got %v", input.ExtraTargets[0])
		}
	})

	t.Run("phases are deep copied", func(t *testing.T) {
		_, subnet, _ := net.ParseCIDR("172.17.0.0/16")
		input := &envdetect.EnvironmentProfile{
			Type: envdetect.DockerBridge,
			Phases: []envdetect.SubnetPhase{
				{
					Name:       "docker_bridge",
					Subnets:    []net.IPNet{*subnet},
					Confidence: 0.85,
					Priority:   1,
				},
			},
		}
		result := cloneProfile(input)

		if len(result.Phases) != 1 {
			t.Fatalf("Phases length = %d, want 1", len(result.Phases))
		}
		if result.Phases[0].Name != "docker_bridge" {
			t.Errorf("Phases[0].Name = %q, want docker_bridge", result.Phases[0].Name)
		}

		// Modify clone
		result.Phases[0].Name = "modified_phase"
		_, newSubnet, _ := net.ParseCIDR("10.0.0.0/8")
		result.Phases[0].Subnets[0] = *newSubnet

		// Original should be unchanged
		if input.Phases[0].Name != "docker_bridge" {
			t.Errorf("original Phases[0].Name was modified: got %q", input.Phases[0].Name)
		}
		if input.Phases[0].Subnets[0].String() != "172.17.0.0/16" {
			t.Errorf("original Phases[0].Subnets[0] was modified: got %v", input.Phases[0].Subnets[0].String())
		}
	})

	t.Run("result is independent pointer", func(t *testing.T) {
		input := &envdetect.EnvironmentProfile{
			Type:       envdetect.Native,
			Confidence: 0.9,
		}
		result := cloneProfile(input)

		if result == input {
			t.Error("cloneProfile should return a new pointer, not the same one")
		}

		// Modify clone's scalar field
		result.Confidence = 0.1

		// Original should be unchanged
		if input.Confidence != 0.9 {
			t.Errorf("original Confidence was modified: got %v", input.Confidence)
		}
	})
}
