package tempproxy

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient("https://example.com/api", "test-token")

	if client.baseURL != "https://example.com/api" {
		t.Errorf("baseURL = %q, want https://example.com/api", client.baseURL)
	}
	if client.authToken != "test-token" {
		t.Errorf("authToken = %q, want test-token", client.authToken)
	}
	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
	if client.timeout != defaultTimeout {
		t.Errorf("timeout = %v, want %v", client.timeout, defaultTimeout)
	}
}

func TestNewHTTPClient_TrimsTrailingSlash(t *testing.T) {
	client := NewHTTPClient("https://example.com/api/", "token")

	if client.baseURL != "https://example.com/api" {
		t.Errorf("baseURL = %q, want trailing slash removed", client.baseURL)
	}
}

func TestHTTPClient_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		token    string
		expected bool
	}{
		{
			name:     "both configured",
			baseURL:  "https://example.com",
			token:    "token",
			expected: true,
		},
		{
			name:     "empty baseURL",
			baseURL:  "",
			token:    "token",
			expected: false,
		},
		{
			name:     "empty token",
			baseURL:  "https://example.com",
			token:    "",
			expected: false,
		},
		{
			name:     "both empty",
			baseURL:  "",
			token:    "",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHTTPClient(tc.baseURL, tc.token)
			if client.IsAvailable() != tc.expected {
				t.Errorf("IsAvailable() = %v, want %v", client.IsAvailable(), tc.expected)
			}
		})
	}
}

func TestHTTPClient_GetTemperature_Success(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/temps") {
			t.Errorf("Path = %q, want /temps", r.URL.Path)
		}
		if r.URL.Query().Get("node") != "node1" {
			t.Errorf("node query param = %q, want node1", r.URL.Query().Get("node"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q, want application/json", r.Header.Get("Accept"))
		}

		// Return success response
		resp := struct {
			Node        string `json:"node"`
			Temperature string `json:"temperature"`
		}{
			Node:        "node1",
			Temperature: `{"cpu": 45.0}`,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token")
	temp, err := client.GetTemperature("node1")

	if err != nil {
		t.Fatalf("GetTemperature() error = %v", err)
	}
	if temp != `{"cpu": 45.0}` {
		t.Errorf("GetTemperature() = %q, want {\"cpu\": 45.0}", temp)
	}
}

func TestHTTPClient_GetTemperature_NotConfigured(t *testing.T) {
	client := NewHTTPClient("", "")
	_, err := client.GetTemperature("node1")

	if err == nil {
		t.Fatal("Expected error for unconfigured client")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
	if proxyErr.Retryable {
		t.Error("Should not be retryable")
	}
}

func TestHTTPClient_GetTemperature_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "bad-token")
	_, err := client.GetTemperature("node1")

	if err == nil {
		t.Fatal("Expected error for 401 response")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeAuth {
		t.Errorf("Type = %v, want ErrorTypeAuth", proxyErr.Type)
	}
	if proxyErr.Retryable {
		t.Error("Auth errors should not be retryable")
	}
}

func TestHTTPClient_GetTemperature_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")
	_, err := client.GetTemperature("node1")

	if err == nil {
		t.Fatal("Expected error for 403 response")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeAuth {
		t.Errorf("Type = %v, want ErrorTypeAuth", proxyErr.Type)
	}
	if proxyErr.Message != "node not allowed by proxy" {
		t.Errorf("Message = %q, want 'node not allowed by proxy'", proxyErr.Message)
	}
}

func TestHTTPClient_GetTemperature_RateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")
	_, err := client.GetTemperature("node1")

	if err == nil {
		t.Fatal("Expected error for 429 response")
	}

	// Should have retried
	if attempts < 2 {
		t.Errorf("Expected retries, got %d attempts", attempts)
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
}

func TestHTTPClient_GetTemperature_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")
	_, err := client.GetTemperature("node1")

	if err == nil {
		t.Fatal("Expected error for 500 response")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
	// 5xx errors should be retryable
	if !proxyErr.Retryable {
		t.Error("5xx errors should be retryable")
	}
}

func TestHTTPClient_GetTemperature_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")
	_, err := client.GetTemperature("node1")

	if err == nil {
		t.Fatal("Expected error for 400 response")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	// 4xx errors (except 401, 403, 429) should not be retryable
	if proxyErr.Retryable {
		t.Error("4xx errors should not be retryable")
	}
}

func TestHTTPClient_GetTemperature_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")
	_, err := client.GetTemperature("node1")

	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
	if !strings.Contains(proxyErr.Message, "parse response JSON") {
		t.Errorf("Message = %q, should mention JSON parsing", proxyErr.Message)
	}
}

func TestHTTPClient_GetTemperature_URLEncoding(t *testing.T) {
	receivedNode := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedNode = r.URL.Query().Get("node")
		resp := struct {
			Node        string `json:"node"`
			Temperature string `json:"temperature"`
		}{
			Node:        receivedNode,
			Temperature: "{}",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")

	// Test with special characters that need URL encoding
	nodeWithSpaces := "node with spaces"
	_, err := client.GetTemperature(nodeWithSpaces)

	if err != nil {
		t.Fatalf("GetTemperature() error = %v", err)
	}
	if receivedNode != nodeWithSpaces {
		t.Errorf("Received node = %q, want %q", receivedNode, nodeWithSpaces)
	}
}

func TestHTTPClient_GetTemperature_RetryOnTransportError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Force connection close to simulate transport error
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
		}
		// Success on third attempt
		resp := struct {
			Node        string `json:"node"`
			Temperature string `json:"temperature"`
		}{
			Node:        "node1",
			Temperature: "{}",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")
	_, _ = client.GetTemperature("node1")

	// Should have made multiple attempts
	if attempts < 2 {
		t.Errorf("Expected retries, got %d attempts", attempts)
	}
}

func TestHTTPClient_GetTemperature_RequestBuildError(t *testing.T) {
	client := NewHTTPClient("http://[::1", "token")
	_, err := client.GetTemperature("node1")

	if err == nil {
		t.Fatal("Expected error for request creation")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
	if !strings.Contains(proxyErr.Message, "failed to create HTTP request") {
		t.Errorf("Message = %q, want request creation failure", proxyErr.Message)
	}
}

func TestHTTPClient_GetTemperature_ReadBodyError(t *testing.T) {
	attempts := 0
	client := NewHTTPClient("https://example.com", "token")
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       errReadCloser{},
				Header:     make(http.Header),
			}, nil
		}
		body := `{"node":"node1","temperature":"{}"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	_, err := client.GetTemperature("node1")
	if err != nil {
		t.Fatalf("Expected success after retry, got %v", err)
	}
	if attempts < 2 {
		t.Fatalf("Expected retry after read error, got %d attempts", attempts)
	}
}

func TestHTTPClient_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("Path = %q, want /health", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization header missing or incorrect")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "test-token")
	err := client.HealthCheck()

	if err != nil {
		t.Errorf("HealthCheck() error = %v", err)
	}
}

func TestHTTPClient_HealthCheck_NotConfigured(t *testing.T) {
	client := NewHTTPClient("", "")
	err := client.HealthCheck()

	if err == nil {
		t.Fatal("Expected error for unconfigured client")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
}

func TestHTTPClient_HealthCheck_RequestBuildError(t *testing.T) {
	client := NewHTTPClient("http://[::1", "token")
	err := client.HealthCheck()

	if err == nil {
		t.Fatal("Expected error for request creation")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
	if !strings.Contains(proxyErr.Message, "failed to create HTTP request") {
		t.Errorf("Message = %q, want request creation failure", proxyErr.Message)
	}
}

func TestHTTPClient_HealthCheck_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")
	err := client.HealthCheck()

	if err == nil {
		t.Fatal("Expected error for 503 response")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
	// 5xx errors should be retryable
	if !proxyErr.Retryable {
		t.Error("5xx errors should be retryable")
	}
}

func TestHTTPClient_HealthCheck_ConnectionRefused(t *testing.T) {
	// Use a URL that will refuse connection
	client := NewHTTPClient("http://127.0.0.1:1", "token")
	client.httpClient.Timeout = 100 * time.Millisecond

	err := client.HealthCheck()

	if err == nil {
		t.Fatal("Expected error for refused connection")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	if proxyErr.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", proxyErr.Type)
	}
	if !proxyErr.Retryable {
		t.Error("Connection errors should be retryable")
	}
}

func TestHTTPClient_HealthCheck_LongBody(t *testing.T) {
	// Server returns a very long body that should be limited
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		// Write more than 1024 bytes
		for i := 0; i < 200; i++ {
			w.Write([]byte("error message "))
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "token")
	err := client.HealthCheck()

	if err == nil {
		t.Fatal("Expected error")
	}

	proxyErr, ok := err.(*ProxyError)
	if !ok {
		t.Fatalf("Expected *ProxyError, got %T", err)
	}
	// Body should be limited to 1024 bytes
	if len(proxyErr.Message) > 1100 { // Some overhead for HTTP status prefix
		t.Errorf("Message too long: %d bytes", len(proxyErr.Message))
	}
}

func TestHTTPClient_Fields(t *testing.T) {
	client := &HTTPClient{
		baseURL:   "https://test.example.com",
		authToken: "secret",
		timeout:   60 * time.Second,
	}

	if client.baseURL != "https://test.example.com" {
		t.Errorf("baseURL = %q, want https://test.example.com", client.baseURL)
	}
	if client.authToken != "secret" {
		t.Errorf("authToken = %q, want secret", client.authToken)
	}
	if client.timeout != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", client.timeout)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errReadCloser struct{}

func (errReadCloser) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

func (errReadCloser) Close() error {
	return nil
}
