package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseProbeAddress_Shapes(t *testing.T) {
	cases := []struct {
		in          string
		wantHost    string
		wantPort    int
		expectError bool
	}{
		{"pve01.lan", "pve01.lan", 0, false},
		{"pve01.lan:8006", "pve01.lan", 8006, false},
		{"192.168.1.10", "192.168.1.10", 0, false},
		{"192.168.1.10:443", "192.168.1.10", 443, false},
		{"https://pve01.lan:8006", "pve01.lan", 8006, false},
		{"https://pve01.lan", "pve01.lan", 0, false},
		{"   ", "", 0, true},
		{"pve01.lan:99999", "", 0, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			host, port, err := parseProbeAddress(c.in)
			if c.expectError {
				if err == nil {
					t.Fatalf("expected error for %q", c.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != c.wantHost {
				t.Fatalf("host = %q, want %q", host, c.wantHost)
			}
			if port != c.wantPort {
				t.Fatalf("port = %d, want %d", port, c.wantPort)
			}
		})
	}
}

func TestTargetsForPort_ExplicitNarrowsToMatching(t *testing.T) {
	targets := targetsForPort(8006)
	seen := map[ConnectionType]bool{}
	for _, t := range targets {
		seen[t.Type] = true
	}
	if !seen[ConnectionTypePVE] {
		t.Fatal("expected PVE target for port 8006")
	}
	if !seen[ConnectionTypePMG] {
		t.Fatal("expected PMG target for port 8006")
	}
	if seen[ConnectionTypePBS] {
		t.Fatal("did not expect PBS target for port 8006")
	}
}

func TestTargetsForPort_ZeroReturnsAll(t *testing.T) {
	if len(targetsForPort(0)) != len(defaultProbeTargets) {
		t.Fatal("zero port should return all default targets")
	}
}

// probeTestServer spins up an HTTPS server on a random port, overriding the
// target port in defaultProbeTargets so probeOne can hit it. Restores the
// original targets in cleanup. Returns the host:port string.
func probeTestServer(t *testing.T, handler http.HandlerFunc) (host string, port int, cleanup func()) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)

	u := srv.URL
	u = strings.TrimPrefix(u, "https://")
	h, p, err := net.SplitHostPort(u)
	if err != nil {
		t.Fatalf("failed to split host:port %q: %v", u, err)
	}
	portInt, err := strconv.Atoi(p)
	if err != nil {
		t.Fatalf("failed to parse port %q: %v", p, err)
	}
	return h, portInt, func() {
		srv.Close()
	}
}

func pveHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "pve-api-daemon/3.4")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, body)
	}
}

func pbsHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "proxmox-backup-api/3.2")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, body)
	}
}

func vmwareHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<namespaces>
  <namespace>urn:vim25</namespace>
</namespaces>`)
	}
}

func truenasHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `"TRUENAS-SCALE-24.04"`)
	}
}

// testProbeClient mirrors probeHTTPClient but shorter timeouts for tests.
func testProbeClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:       (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
		Timeout: 1 * time.Second,
	}
}

func withSingleTarget(t *testing.T, tgt probeTarget) func() {
	t.Helper()
	orig := defaultProbeTargets
	defaultProbeTargets = []probeTarget{tgt}
	return func() { defaultProbeTargets = orig }
}

func TestProbeOne_DetectsPVE(t *testing.T) {
	host, port, done := probeTestServer(t, pveHandler(`<html><title>delly - Proxmox Virtual Environment</title></html>`))
	defer done()

	tgt := probeTarget{
		Type:       ConnectionTypePVE,
		Scheme:     "https",
		Port:       port,
		Path:       "/",
		identifyFn: defaultProbeTargets[0].identifyFn,
	}
	cand, ok := probeOne(context.Background(), host, tgt, testProbeClient())
	if !ok {
		t.Fatal("expected PVE detection")
	}
	if cand.Type != ConnectionTypePVE {
		t.Fatalf("type = %q, want pve", cand.Type)
	}
	if cand.Hints["product"] != "Proxmox VE" {
		t.Fatalf("product hint not set: %+v", cand.Hints)
	}
}

func TestProbeOne_DetectsPBS(t *testing.T) {
	host, port, done := probeTestServer(t, pbsHandler(`<html><title>pbs-docker - Proxmox Backup Server</title></html>`))
	defer done()

	// Find the PBS identifyFn from defaults.
	var pbsFn func(*http.Response, []byte) (bool, map[string]string)
	for _, tgt := range defaultProbeTargets {
		if tgt.Type == ConnectionTypePBS {
			pbsFn = tgt.identifyFn
		}
	}
	if pbsFn == nil {
		t.Fatal("pbs target not in defaults")
	}
	tgt := probeTarget{Type: ConnectionTypePBS, Scheme: "https", Port: port, Path: "/", identifyFn: pbsFn}
	if _, ok := probeOne(context.Background(), host, tgt, testProbeClient()); !ok {
		t.Fatal("expected PBS detection")
	}
}

func TestProbeOne_DetectsPMGByTitle(t *testing.T) {
	// Simulate a PMG root that omits the Server banner but still advertises the
	// product via HTML title.
	host, port, done := probeTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><title>mail01 - Proxmox Mail Gateway</title></html>`)
	})
	defer done()

	var pmgFn func(*http.Response, []byte) (bool, map[string]string)
	for _, tgt := range defaultProbeTargets {
		if tgt.Type == ConnectionTypePMG {
			pmgFn = tgt.identifyFn
		}
	}
	tgt := probeTarget{Type: ConnectionTypePMG, Scheme: "https", Port: port, Path: "/", identifyFn: pmgFn}
	if _, ok := probeOne(context.Background(), host, tgt, testProbeClient()); !ok {
		t.Fatal("expected PMG detection via HTML title")
	}
}

func TestProbeOne_DetectsVMware(t *testing.T) {
	host, port, done := probeTestServer(t, vmwareHandler())
	defer done()

	var vmFn func(*http.Response, []byte) (bool, map[string]string)
	for _, tgt := range defaultProbeTargets {
		if tgt.Type == ConnectionTypeVMware {
			vmFn = tgt.identifyFn
		}
	}
	tgt := probeTarget{Type: ConnectionTypeVMware, Scheme: "https", Port: port, Path: "/sdk/vimServiceVersions.xml", identifyFn: vmFn}
	if _, ok := probeOne(context.Background(), host, tgt, testProbeClient()); !ok {
		t.Fatal("expected VMware detection")
	}
}

func TestProbeOne_DetectsTrueNAS(t *testing.T) {
	host, port, done := probeTestServer(t, truenasHandler())
	defer done()

	var tnFn func(*http.Response, []byte) (bool, map[string]string)
	for _, tgt := range defaultProbeTargets {
		if tgt.Type == ConnectionTypeTrueNAS {
			tnFn = tgt.identifyFn
		}
	}
	tgt := probeTarget{Type: ConnectionTypeTrueNAS, Scheme: "https", Port: port, Path: "/api/v2.0/system/product_name", identifyFn: tnFn}
	if _, ok := probeOne(context.Background(), host, tgt, testProbeClient()); !ok {
		t.Fatal("expected TrueNAS detection")
	}
}

func TestProbeOne_RejectsNonMatchingServer(t *testing.T) {
	host, port, done := probeTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.24")
		fmt.Fprint(w, `<html><body>hello</body></html>`)
	})
	defer done()

	for _, tgt := range defaultProbeTargets {
		t := probeTarget{Type: tgt.Type, Scheme: "https", Port: port, Path: tgt.Path, identifyFn: tgt.identifyFn}
		if _, ok := probeOne(context.Background(), host, t, testProbeClient()); ok {
			return
		}
	}
	// None matched — expected outcome.
}

func TestProbeOne_DoesNotConfusePVEForPMG(t *testing.T) {
	host, port, done := probeTestServer(t, pveHandler(`<html><title>delly - Proxmox Virtual Environment</title></html>`))
	defer done()

	var pmgFn func(*http.Response, []byte) (bool, map[string]string)
	for _, tgt := range defaultProbeTargets {
		if tgt.Type == ConnectionTypePMG {
			pmgFn = tgt.identifyFn
		}
	}
	tgt := probeTarget{Type: ConnectionTypePMG, Scheme: "https", Port: port, Path: "/", identifyFn: pmgFn}
	if _, ok := probeOne(context.Background(), host, tgt, testProbeClient()); ok {
		t.Fatal("PMG identifyFn should not match a PVE server")
	}
}

func TestRunProbe_ReturnsSortedCandidates(t *testing.T) {
	// Spin up one PVE server; runProbe should invoke all candidates but only
	// detect PVE. Shorten the budget for the test.
	host, port, done := probeTestServer(t, pveHandler(`<html><title>delly - Proxmox Virtual Environment</title></html>`))
	defer done()

	// Override defaultProbeTargets to only the PVE target at our ephemeral port.
	var pveFn func(*http.Response, []byte) (bool, map[string]string)
	for _, tgt := range defaultProbeTargets {
		if tgt.Type == ConnectionTypePVE {
			pveFn = tgt.identifyFn
		}
	}
	restore := withSingleTarget(t, probeTarget{Type: ConnectionTypePVE, Scheme: "https", Port: port, Path: "/", identifyFn: pveFn})
	defer restore()

	results, elapsed := runProbe(context.Background(), host, port, testProbeClient())
	if len(results) != 1 || results[0].Type != ConnectionTypePVE {
		t.Fatalf("unexpected results: %+v", results)
	}
	if elapsed <= 0 {
		t.Fatal("elapsed should be positive")
	}
}

func TestRunProbe_TotalBudgetEnforced(t *testing.T) {
	// Build a dummy target that points nowhere — dial will be slow/timeout.
	restore := withSingleTarget(t, probeTarget{
		Type:   ConnectionTypePVE,
		Scheme: "https",
		Port:   1, // unlikely open
		Path:   "/",
		identifyFn: func(*http.Response, []byte) (bool, map[string]string) {
			return true, nil
		},
	})
	defer restore()

	start := time.Now()
	results, _ := runProbe(context.Background(), "127.0.0.1", 0, testProbeClient())
	elapsed := time.Since(start)
	if len(results) != 0 {
		t.Fatalf("expected no results from closed port, got %+v", results)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("probe exceeded expected timeout: %s", elapsed)
	}
}

func TestValidateProbeHostRejectsMetadataService(t *testing.T) {
	err := validateProbeHost(context.Background(), "169.254.169.254")
	if err == nil {
		t.Fatal("expected metadata service address to be rejected")
	}
	if !strings.Contains(err.Error(), "metadata service") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatedProbeEndpointRejectsMetadataService(t *testing.T) {
	_, err := validatedProbeEndpoint(context.Background(), "169.254.169.254", probeTarget{
		Type:   ConnectionTypePVE,
		Scheme: "https",
		Port:   8006,
		Path:   "/",
	})
	if err == nil {
		t.Fatal("expected metadata service endpoint to be rejected")
	}
	if !strings.Contains(err.Error(), "metadata service") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnectionsHandlersHandleProbeRejectsBlockedAddress(t *testing.T) {
	h := NewConnectionsHandlers(nil, nil, nil)
	body := strings.NewReader(`{"address":"169.254.169.254"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/connections/probe", body)
	rec := httptest.NewRecorder()

	h.HandleProbe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != "invalid_address" {
		t.Fatalf("code = %v, want invalid_address", payload["code"])
	}
	message, _ := payload["error"].(string)
	if !strings.Contains(message, "metadata service") {
		t.Fatalf("message = %q, want metadata-service guidance", message)
	}
}
