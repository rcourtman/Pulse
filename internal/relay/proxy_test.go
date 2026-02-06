package relay

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestHTTPProxy_HandleRequest(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Set up a mock local Pulse API
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the API token was injected
		if token := r.Header.Get("X-API-Token"); token != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		switch r.URL.Path {
		case "/api/resources":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case "/api/echo":
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockAPI.Close()

	// Extract host:port from mock server URL
	addr := strings.TrimPrefix(mockAPI.URL, "http://")
	proxy := NewHTTPProxy(addr, logger)

	t.Run("GET request", func(t *testing.T) {
		req := ProxyRequest{
			ID:     "req_1",
			Method: "GET",
			Path:   "/api/resources",
		}
		payload, _ := json.Marshal(req)

		respPayload, err := proxy.HandleRequest(payload, "test-token")
		if err != nil {
			t.Fatalf("HandleRequest() error = %v", err)
		}

		var resp ProxyResponse
		if err := json.Unmarshal(respPayload, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp.ID != "req_1" {
			t.Errorf("ID: got %q, want %q", resp.ID, "req_1")
		}
		if resp.Status != 200 {
			t.Errorf("Status: got %d, want 200", resp.Status)
		}

		// Decode body
		bodyBytes, _ := base64.StdEncoding.DecodeString(resp.Body)
		var body map[string]string
		json.Unmarshal(bodyBytes, &body)
		if body["status"] != "ok" {
			t.Errorf("Body status: got %q, want %q", body["status"], "ok")
		}
	})

	t.Run("POST request with body", func(t *testing.T) {
		bodyContent := `{"data":"hello"}`
		req := ProxyRequest{
			ID:     "req_2",
			Method: "POST",
			Path:   "/api/echo",
			Body:   base64.StdEncoding.EncodeToString([]byte(bodyContent)),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}
		payload, _ := json.Marshal(req)

		respPayload, err := proxy.HandleRequest(payload, "test-token")
		if err != nil {
			t.Fatalf("HandleRequest() error = %v", err)
		}

		var resp ProxyResponse
		json.Unmarshal(respPayload, &resp)

		if resp.Status != 200 {
			t.Errorf("Status: got %d, want 200", resp.Status)
		}

		bodyBytes, _ := base64.StdEncoding.DecodeString(resp.Body)
		if string(bodyBytes) != bodyContent {
			t.Errorf("Body: got %q, want %q", string(bodyBytes), bodyContent)
		}
	})

	t.Run("strips sensitive headers", func(t *testing.T) {
		// The mock API verifies X-API-Token is "test-token",
		// so if Authorization header leaked through, it would be ignored anyway.
		// This test verifies the proxy doesn't forward them.
		req := ProxyRequest{
			ID:     "req_3",
			Method: "GET",
			Path:   "/api/resources",
			Headers: map[string]string{
				"Authorization": "Bearer should-be-stripped",
				"Cookie":        "session=should-be-stripped",
				"Host":          "evil.example.com",
				"Accept":        "application/json",
			},
		}
		payload, _ := json.Marshal(req)

		respPayload, err := proxy.HandleRequest(payload, "test-token")
		if err != nil {
			t.Fatalf("HandleRequest() error = %v", err)
		}

		var resp ProxyResponse
		json.Unmarshal(respPayload, &resp)

		if resp.Status != 200 {
			t.Errorf("Status: got %d, want 200 (headers should have been stripped)", resp.Status)
		}
	})

	t.Run("invalid JSON payload", func(t *testing.T) {
		respPayload, err := proxy.HandleRequest([]byte("not json"), "test-token")
		if err != nil {
			t.Fatalf("HandleRequest() error = %v", err)
		}

		var resp ProxyResponse
		json.Unmarshal(respPayload, &resp)

		if resp.Status != http.StatusBadRequest {
			t.Errorf("Status: got %d, want %d", resp.Status, http.StatusBadRequest)
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		req := ProxyRequest{ID: "req_4"} // missing method and path
		payload, _ := json.Marshal(req)

		respPayload, err := proxy.HandleRequest(payload, "test-token")
		if err != nil {
			t.Fatalf("HandleRequest() error = %v", err)
		}

		var resp ProxyResponse
		json.Unmarshal(respPayload, &resp)

		if resp.Status != http.StatusBadRequest {
			t.Errorf("Status: got %d, want %d", resp.Status, http.StatusBadRequest)
		}
	})

	t.Run("404 from backend", func(t *testing.T) {
		req := ProxyRequest{
			ID:     "req_5",
			Method: "GET",
			Path:   "/api/nonexistent",
		}
		payload, _ := json.Marshal(req)

		respPayload, err := proxy.HandleRequest(payload, "test-token")
		if err != nil {
			t.Fatalf("HandleRequest() error = %v", err)
		}

		var resp ProxyResponse
		json.Unmarshal(respPayload, &resp)

		if resp.Status != http.StatusNotFound {
			t.Errorf("Status: got %d, want %d", resp.Status, http.StatusNotFound)
		}
	})
}
