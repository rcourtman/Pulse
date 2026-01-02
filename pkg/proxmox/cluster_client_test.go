package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClusterClientHandlesRateLimitWithoutMarkingUnhealthy(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			current := atomic.AddInt32(&requestCount, 1)
			if current == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"error":"rate limited"}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online","cpu":0,"maxcpu":1,"mem":0,"maxmem":1,"disk":0,"maxdisk":1,"uptime":1,"level":"normal"}]}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{}}`)
		}
	}))
	defer server.Close()

	cfg := ClientConfig{
		Host:       server.URL,
		TokenName:  "pulse@pve!token",
		TokenValue: "sometokenvalue",
		VerifySSL:  false,
		Timeout:    2 * time.Second,
	}

	cc := NewClusterClient("test-cluster", cfg, []string{server.URL}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	nodes, err := cc.GetNodes(ctx)
	if err != nil {
		t.Fatalf("expected GetNodes to succeed after retry, got error: %v", err)
	}

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after retry, got %d", len(nodes))
	}

	health := cc.GetHealthStatus()
	if healthy, ok := health[server.URL]; !ok || !healthy {
		t.Fatalf("expected endpoint %s to remain healthy, got health map: %+v", server.URL, health)
	}

	if atomic.LoadInt32(&requestCount) < 2 {
		t.Fatalf("expected at least 2 requests to backend, got %d", requestCount)
	}
}

func TestClusterClientIgnoresGuestAgentTimeoutForHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"node":"test","status":"online","cpu":0,"maxcpu":1,"mem":0,"maxmem":1,"disk":0,"maxdisk":1,"uptime":1,"level":"normal"}]}`)
		case "/api2/json/nodes/test/qemu/100/agent/get-fsinfo":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"data":null,"message":"VM 100 qmp command 'guest-get-fsinfo' failed - got timeout\n"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := ClientConfig{
		Host:       server.URL,
		TokenName:  "pulse@pve!token",
		TokenValue: "sometokenvalue",
		VerifySSL:  false,
		Timeout:    2 * time.Second,
	}

	cc := NewClusterClient("test-cluster", cfg, []string{server.URL}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := cc.GetVMFSInfo(ctx, "test", 100)
	if err == nil {
		t.Fatalf("expected VM guest agent timeout error, got nil")
	}

	health := cc.GetHealthStatusWithErrors()
	endpointHealth, ok := health[server.URL]
	if !ok {
		t.Fatalf("expected health entry for endpoint %s", server.URL)
	}
	if !endpointHealth.Healthy {
		t.Fatalf("expected endpoint to remain healthy after VM-specific guest agent error, got %+v", endpointHealth)
	}
	if endpointHealth.LastError != "" {
		t.Fatalf("expected last error to remain empty for VM-specific failures, got %q", endpointHealth.LastError)
	}
}

func TestExtractStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected int
	}{
		{"empty string", "", 0},
		{"no status code", "connection refused", 0},
		{"api error 429", "api error 429: Too Many Requests", 429},
		{"status 503", "status 503: Service Unavailable", 503},
		{"API error uppercase", "API Error 401: Unauthorized", 401},
		{"status lowercase", "status 502: Bad Gateway", 502},
		{"status with text before", "request failed with status 504", 504},
		{"mixed case api error", "Api Error 403 forbidden", 403},
		{"three digit code only", "status 200 OK", 200},
		{"partial match no space", "status500 error", 0},
		{"code in url path", "/api2/json/nodes status 429", 429},
		{"multiple codes returns first match", "api error 429 then status 503", 429},
		{"invalid code format", "status abc error", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStatusCode(tt.errStr)
			if result != tt.expected {
				t.Errorf("extractStatusCode(%q) = %d, want %d", tt.errStr, result, tt.expected)
			}
		})
	}
}

func TestIsTransientRateLimitError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantTransient bool
		wantCode      int
	}{
		{"nil error", nil, false, 0},
		{"generic error", errors.New("connection refused"), false, 0},
		{"status 408 Request Timeout", errors.New("api error 408: Request Timeout"), true, 408},
		{"status 425 Too Early", errors.New("status 425: Too Early"), true, 425},
		{"status 429 Too Many Requests", errors.New("api error 429: Too Many Requests"), true, 429},
		{"status 502 Bad Gateway", errors.New("status 502: Bad Gateway"), true, 502},
		{"status 503 Service Unavailable", errors.New("api error 503: Service Unavailable"), true, 503},
		{"status 504 Gateway Timeout", errors.New("status 504"), true, 504},
		{"status 401 not transient", errors.New("api error 401: Unauthorized"), false, 401},
		{"status 403 not transient", errors.New("status 403: Forbidden"), false, 403},
		{"status 404 not transient", errors.New("api error 404: Not Found"), false, 404},
		{"status 500 not transient", errors.New("status 500: Internal Server Error"), false, 500},
		{"rate limit text no code", errors.New("rate limit exceeded"), true, 429},
		{"Rate Limit uppercase", errors.New("Rate Limit Reached"), true, 429},
		{"too many requests text", errors.New("too many requests, slow down"), true, 429},
		{"Too Many Requests uppercase", errors.New("Too Many Requests"), true, 429},
		{"rate limit with different code", errors.New("rate limit api error 503"), true, 503},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTransient, gotCode := isTransientRateLimitError(tt.err)
			if gotTransient != tt.wantTransient {
				t.Errorf("isTransientRateLimitError(%v) transient = %v, want %v", tt.err, gotTransient, tt.wantTransient)
			}
			if gotCode != tt.wantCode {
				t.Errorf("isTransientRateLimitError(%v) code = %d, want %d", tt.err, gotCode, tt.wantCode)
			}
		})
	}
}

func TestIsNotImplementedError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{"empty string", "", false},
		{"generic error", "connection refused", false},
		// Requires BOTH "not implemented" text AND a 501 code indicator
		{"not implemented without code", "not implemented", false},
		{"not implemented with status 501", "not implemented status 501", true},
		{"error 501 not implemented", "error 501: Not Implemented", true},
		// "api error 501" doesn't contain "not implemented" text, so false
		{"api error 501 no not implemented text", "api error 501: feature not available", false},
		// Has both text and code pattern
		{"api error 501 with not implemented", "api error 501: not implemented feature", true},
		{"Not Implemented uppercase", "Not Implemented status 501", true},
		{"NOT IMPLEMENTED all caps", "NOT IMPLEMENTED error 501", true},
		// Has "not implemented" but extractStatusCode returns 501 via fallback
		{"status 501 with not implemented text", "status 501 - not implemented feature", true},
		{"501 in different context no text", "VM 501 not found", false},
		{"not implemented but 500", "not implemented status 500", false},
		{"not implemented but 404", "not implemented error 404", false},
		{"space before 501 with not implemented", " 501 not implemented", true},
		// Tab character triggers extractStatusCode fallback (regex \s+ matches tab but " 501" check doesn't)
		{"tab before 501 with not implemented", "not implemented api error\t501", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotImplementedError(tt.errStr)
			if result != tt.expected {
				t.Errorf("isNotImplementedError(%q) = %v, want %v", tt.errStr, result, tt.expected)
			}
		})
	}
}

func TestIsVMSpecificError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{"empty string", "", false},
		{"generic connection error", "connection refused", false},
		{"no qemu guest agent", "no qemu guest agent", true},
		{"No QEMU guest agent uppercase", "No QEMU guest agent running", true},
		{"qemu guest agent is not running", "qemu guest agent is not running", true},
		{"QEMU guest agent is not running", "QEMU guest agent is not running", true},
		{"guest agent generic", "guest agent error", true},
		{"qmp command timeout", "qmp command 'guest-get-fsinfo' failed - got timeout", true},
		{"QMP command uppercase", "QMP Command failed", true},
		{"guest-get-fsinfo", "guest-get-fsinfo returned empty", true},
		{"guest-get-memory-blocks", "guest-get-memory-blocks failed", true},
		{"guest-get-vcpus", "error in guest-get-vcpus", true},
		{"authentication error not VM specific", "authentication failed", false},
		{"connection timeout not VM specific", "connection timeout", false},
		{"node offline not VM specific", "node offline", false},
		{"vm in error message but not agent", "VM 100 is offline", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVMSpecificError(tt.errStr)
			if result != tt.expected {
				t.Errorf("isVMSpecificError(%q) = %v, want %v", tt.errStr, result, tt.expected)
			}
		})
	}
}

func TestSanitizeEndpointError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{"empty string", "", ""},
		{"generic error unchanged", "some random error", "some random error"},
		// Context deadline exceeded
		{"context deadline basic", "context deadline exceeded", "Request timed out - Proxmox API may be slow or waiting on unreachable backend services"},
		{"context deadline storage", "Get /api2/json/nodes/delly/storage: context deadline exceeded", "Request timed out - storage API slow (check for unreachable PBS/NFS/Ceph backends)"},
		{"context deadline pbs", "pbs-backup context deadline exceeded", "Request timed out - PBS storage backend unreachable"},
		{"context deadline port 8007", "Can't connect to tower:8007 context deadline exceeded", "Request timed out - PBS storage backend unreachable"},
		// Client timeout
		{"client timeout", "Client.Timeout exceeded while awaiting headers", "Connection timed out - Proxmox API not responding in time"},
		// Connection refused
		{"connection refused", "dial tcp 192.168.0.5:8006: connect: connection refused", "Connection refused - Proxmox API not running or firewall blocking"},
		// No route to host
		{"no route to host", "dial tcp: no route to host", "Network unreachable - check network connectivity to Proxmox host"},
		// Certificate errors
		{"certificate error", "x509: certificate signed by unknown authority", "TLS certificate error - check SSL settings or add fingerprint"},
		{"cert in message", "certificate has expired", "TLS certificate error - check SSL settings or add fingerprint"},
		// Auth errors
		{"401 error", "api error 401: Unauthorized", "Authentication failed - check API token or credentials"},
		{"403 error", "status 403: Forbidden", "Authentication failed - check API token or credentials"},
		{"authentication keyword", "authentication failed: invalid token", "Authentication failed - check API token or credentials"},
		// PBS specific
		{"pbs connect error", "Can't connect to tower:8007 (Connection timed out)", "PBS storage unreachable - check Proxmox Backup Server connectivity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeEndpointError(tt.errMsg)
			if result != tt.expected {
				t.Errorf("sanitizeEndpointError(%q) = %q, want %q", tt.errMsg, result, tt.expected)
			}
		})
	}
}

func TestCalculateRateLimitBackoff(t *testing.T) {
	// Test that backoff increases with attempt number
	prev := time.Duration(0)
	for attempt := 0; attempt < 5; attempt++ {
		backoff := calculateRateLimitBackoff(attempt)

		// Base delay is 150ms per attempt
		minExpected := rateLimitBaseDelay * time.Duration(attempt+1)
		maxExpected := minExpected + rateLimitMaxJitter

		if backoff < minExpected {
			t.Errorf("calculateRateLimitBackoff(%d) = %v, expected >= %v", attempt, backoff, minExpected)
		}
		if backoff > maxExpected {
			t.Errorf("calculateRateLimitBackoff(%d) = %v, expected <= %v", attempt, backoff, maxExpected)
		}

		// Each subsequent backoff should have a higher minimum than previous
		if attempt > 0 && backoff < prev-rateLimitMaxJitter {
			t.Errorf("calculateRateLimitBackoff(%d) = %v, expected trend upward from %v", attempt, backoff, prev)
		}
		prev = backoff
	}

	// Test specific attempt values
	t.Run("attempt 0", func(t *testing.T) {
		backoff := calculateRateLimitBackoff(0)
		// attempt 0 -> base * 1 = 150ms + jitter (0-200ms)
		if backoff < 150*time.Millisecond || backoff > 350*time.Millisecond {
			t.Errorf("calculateRateLimitBackoff(0) = %v, want 150-350ms", backoff)
		}
	})

	t.Run("attempt 2", func(t *testing.T) {
		backoff := calculateRateLimitBackoff(2)
		// attempt 2 -> base * 3 = 450ms + jitter (0-200ms)
		if backoff < 450*time.Millisecond || backoff > 650*time.Millisecond {
			t.Errorf("calculateRateLimitBackoff(2) = %v, want 450-650ms", backoff)
		}
	})
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"generic error", errors.New("connection refused"), false},
		// Case-sensitive match for "authentication"
		{"authentication lowercase", errors.New("authentication failed"), true},
		// "Authentication" uppercase doesn't match (case-sensitive)
		{"Authentication uppercase does not match", errors.New("Authentication error"), false},
		{"401 status code", errors.New("status 401: Unauthorized"), true},
		{"403 status code", errors.New("api error 403: Forbidden"), true},
		{"401 in message", errors.New("got 401 response"), true},
		{"403 in message", errors.New("server returned 403"), true},
		{"404 not auth error", errors.New("status 404: Not Found"), false},
		{"500 not auth error", errors.New("api error 500"), false},
		{"rate limit not auth", errors.New("status 429: Too Many Requests"), false},
		{"token expired lowercase auth", errors.New("token expired, authentication required"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAuthError(tt.err)
			if result != tt.expected {
				t.Errorf("isAuthError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestClusterClient_GetNodes_PermanentFailure(t *testing.T) {
	// Server that always returns auth error - not retryable
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"data":null,"errors":{"username":"invalid credentials"}}`)
	}))
	defer server.Close()

	cfg := ClientConfig{
		Host:       server.URL,
		TokenName:  "invalid@pve!token",
		TokenValue: "badvalue",
		VerifySSL:  false,
		Timeout:    2 * time.Second,
	}

	cc := NewClusterClient("test-cluster", cfg, []string{server.URL}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	nodes, err := cc.GetNodes(ctx)
	if err == nil {
		t.Fatal("expected GetNodes to fail with auth error, got nil")
	}
	if nodes != nil {
		t.Errorf("expected nil nodes on error, got %v", nodes)
	}
}
