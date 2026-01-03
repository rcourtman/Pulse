package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts"
)

func TestHTTPServer_Health(t *testing.T) {
	proxy := &Proxy{}
	config := &Config{
		HTTPEnabled:   true,
		HTTPAuthToken: "secret-token",
	}
	server := NewHTTPServer(proxy, config)

	// Test valid health check
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()

	// Apply middleware stack manually or construct the handler chain
	handler := server.authMiddleware(http.HandlerFunc(server.handleHealth))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}

	// Test invalid method
	req = httptest.NewRequest(http.MethodPost, "/health", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHTTPServer_AuthMiddleware(t *testing.T) {
	proxy := &Proxy{
		audit: newAuditLogger(os.DevNull), // avoid nil panic
	}
	config := &Config{
		HTTPAuthToken: "secret",
	}
	server := NewHTTPServer(proxy, config)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := server.authMiddleware(next)

	tests := []struct {
		name       string
		authHeader string
		wantCode   int
	}{
		{"MissingHeader", "", http.StatusUnauthorized},
		{"InvalidFormat", "Basic user:pass", http.StatusUnauthorized},
		{"InvalidToken", "Bearer wrong", http.StatusUnauthorized},
		{"ValidToken", "Bearer secret", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected code %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestHTTPServer_Temperature(t *testing.T) {
	// Mock SSH execution
	origExec := execCommandFunc
	defer func() { execCommandFunc = origExec }()
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if strings.Contains(args, "ssh") {
			// Return mock sensor JSON
			return mockExecCommand(`{"coretemp-isa-0000":{"Package id 0":{"temp1_input": 50.0}}}`)
		}
		return mockExecCommand("")
	}

	// Mock keyscan to avoid trying actual network keyscan
	// But p.getTemperatureViaSSH depends on p.knownHosts being set.

	tmpDir := t.TempDir()
	km, _ := knownhosts.NewManager(filepath.Join(tmpDir, "known_hosts"), knownhosts.WithKeyscanFunc(func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
		return []byte(fmt.Sprintf("%s ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKqy\n", host)), nil
	}))
	os.WriteFile(filepath.Join(tmpDir, "id_ed25519"), []byte("priv"), 0600)

	proxy := &Proxy{
		sshKeyPath:        tmpDir,
		knownHosts:        km,
		metrics:           NewProxyMetrics("test"),
		maxSSHOutputBytes: 1024,
		nodeGate:          newNodeGate(),
		config:            &Config{}, // Init config to avoid panic in getTemperatureViaSSH if accessed?
	}
	// Init node validator
	proxy.nodeValidator, _ = newNodeValidator(&Config{}, proxy.metrics)

	config := &Config{
		HTTPAuthToken: "secret",
	}
	server := NewHTTPServer(proxy, config)

	// Test valid request
	req := httptest.NewRequest("GET", "/temps?node=valid-node", nil)
	w := httptest.NewRecorder()
	server.handleTemperature(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "50.0") {
		t.Errorf("expected temp 50.0 in response")
	}

	// Test missing node
	req = httptest.NewRequest("GET", "/temps", nil)
	w = httptest.NewRecorder()
	server.handleTemperature(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing node, got %d", w.Code)
	}

	// Test invalid node name
	req = httptest.NewRequest("GET", "/temps?node=-invalid-", nil)
	w = httptest.NewRecorder()
	server.handleTemperature(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid node name, got %d", w.Code)
	}

	// Test SSH failure
	execCommandFunc = func(name string, arg ...string) *exec.Cmd {
		args := strings.Join(arg, " ")
		if strings.Contains(args, "ssh") {
			return errorExecCommand("ssh failed")
		}
		// Also fail local fallback
		if name == "sensors" {
			return errorExecCommand("sensors failed")
		}
		return mockExecCommand("")
	}
	// Need to mock getTemperatureLocal failing too.

	req = httptest.NewRequest("GET", "/temps?node=fail-node", nil)
	w = httptest.NewRecorder()
	server.handleTemperature(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for ssh failure, got %d", w.Code)
	}
}

func TestHTTPServer_SourceIPMiddleware(t *testing.T) {
	proxy := &Proxy{
		audit: newAuditLogger(os.DevNull),
	}
	config := &Config{
		AllowedSourceSubnets: []string{"192.168.1.0/24", "10.0.0.1/32"},
	}
	server := NewHTTPServer(proxy, config)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := server.sourceIPMiddleware(next)

	tests := []struct {
		name     string
		remoteIP string
		wantCode int
	}{
		{"AllowedSubnet", "192.168.1.10:1234", http.StatusOK},
		{"AllowedSingle", "10.0.0.1:5678", http.StatusOK},
		{"DeniedIP", "1.2.3.4:1234", http.StatusForbidden},
		{"InvalidIP", "invalid-ip", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteIP
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected code %d for %s, got %d", tt.wantCode, tt.remoteIP, w.Code)
			}
		})
	}
}

func TestHTTPServer_StartValidation(t *testing.T) {
	server := NewHTTPServer(&Proxy{}, &Config{HTTPEnabled: true})
	// Missing certs
	if err := server.Start(); err == nil {
		t.Error("expected error when starting without certs")
	}

	server = NewHTTPServer(&Proxy{}, &Config{HTTPEnabled: false})
	if err := server.Start(); err != nil {
		t.Error("expected no error when HTTP disabled")
	}
}

func TestHTTPServer_RateLimiter(t *testing.T) {
	proxy := &Proxy{
		metrics: NewProxyMetrics("test"),
	}
	// proxy.rateLimiter must be initialized
	proxy.rateLimiter = newRateLimiter(proxy.metrics, nil, nil, nil)

	config := &Config{}
	server := NewHTTPServer(proxy, config)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := server.rateLimitMiddleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHashIPToUID(t *testing.T) {
	uid1 := hashIPToUID("192.168.1.1")
	uid2 := hashIPToUID("192.168.1.1")
	uid3 := hashIPToUID("10.0.0.1")

	if uid1 != uid2 {
		t.Error("expected deterministic hash")
	}
	if uid1 == uid3 {
		t.Error("expected different hash for different IPs")
	}
}

func TestHTTPServer_Stop(t *testing.T) {
	server := NewHTTPServer(&Proxy{}, &Config{})
	if err := server.Stop(context.Background()); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
	// Test nil server
	s2 := &HTTPServer{}
	if err := s2.Stop(context.Background()); err != nil {
		t.Errorf("Stop failed for nil server: %v", err)
	}
}

func TestResponseWriter(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder()}
	rw.WriteHeader(http.StatusTeapot)
	if rw.statusCode != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, rw.statusCode)
	}
}
