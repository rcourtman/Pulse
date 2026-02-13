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

func decodeProxyResponse(t *testing.T, data []byte) ProxyResponse {
	t.Helper()
	var resp ProxyResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

func decodeProxyErrorMessage(t *testing.T, resp ProxyResponse) string {
	t.Helper()
	bodyBytes, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	return payload["error"]
}

func TestNewHTTPProxy_ConfiguresClients(t *testing.T) {
	proxy := NewHTTPProxy("127.0.0.1:7655", zerolog.Nop())

	if proxy.localAddr != "127.0.0.1:7655" {
		t.Errorf("localAddr: got %q, want %q", proxy.localAddr, "127.0.0.1:7655")
	}
	if proxy.client == nil {
		t.Fatal("client is nil")
	}
	if proxy.streamClient == nil {
		t.Fatal("streamClient is nil")
	}
	if proxy.client.Timeout != proxyRequestTimeout {
		t.Errorf("client timeout: got %v, want %v", proxy.client.Timeout, proxyRequestTimeout)
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := proxy.client.CheckRedirect(req, nil); err != http.ErrUseLastResponse {
		t.Errorf("client redirect policy: got %v, want %v", err, http.ErrUseLastResponse)
	}
	if err := proxy.streamClient.CheckRedirect(req, nil); err != http.ErrUseLastResponse {
		t.Errorf("stream client redirect policy: got %v, want %v", err, http.ErrUseLastResponse)
	}
}

func TestHTTPProxy_HandleRequest_ErrorBranches(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("rejects invalid base64 body", func(t *testing.T) {
		proxy := NewHTTPProxy("127.0.0.1:1", logger)
		req := ProxyRequest{ID: "req_bad_b64", Method: http.MethodPost, Path: "/api/echo", Body: "%%%not-base64%%%"}
		payload, _ := json.Marshal(req)

		resp := decodeProxyResponse(t, mustHandleRequest(t, proxy, payload))
		if resp.Status != http.StatusBadRequest {
			t.Errorf("status: got %d, want %d", resp.Status, http.StatusBadRequest)
		}
		if got := decodeProxyErrorMessage(t, resp); got != "invalid base64 body" {
			t.Errorf("error message: got %q, want %q", got, "invalid base64 body")
		}
	})

	t.Run("rejects oversized request body", func(t *testing.T) {
		proxy := NewHTTPProxy("127.0.0.1:1", logger)
		req := ProxyRequest{
			ID:     "req_oversize",
			Method: http.MethodPost,
			Path:   "/api/echo",
			Body:   base64.StdEncoding.EncodeToString(make([]byte, maxProxyBodySize+1)),
		}
		payload, _ := json.Marshal(req)

		resp := decodeProxyResponse(t, mustHandleRequest(t, proxy, payload))
		if resp.Status != http.StatusRequestEntityTooLarge {
			t.Errorf("status: got %d, want %d", resp.Status, http.StatusRequestEntityTooLarge)
		}
		if got := decodeProxyErrorMessage(t, resp); !strings.Contains(got, "request body exceeds") {
			t.Errorf("error message: got %q, expected request-body size error", got)
		}
	})

	t.Run("rejects oversized response body", func(t *testing.T) {
		mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(make([]byte, maxProxyBodySize+1))
		}))
		defer mockAPI.Close()

		proxy := NewHTTPProxy(strings.TrimPrefix(mockAPI.URL, "http://"), logger)
		req := ProxyRequest{ID: "resp_oversize", Method: http.MethodGet, Path: "/api/large"}
		payload, _ := json.Marshal(req)

		resp := decodeProxyResponse(t, mustHandleRequest(t, proxy, payload))
		if resp.Status != http.StatusRequestEntityTooLarge {
			t.Errorf("status: got %d, want %d", resp.Status, http.StatusRequestEntityTooLarge)
		}
		if got := decodeProxyErrorMessage(t, resp); !strings.Contains(got, "response body exceeds") {
			t.Errorf("error message: got %q, expected response-body size error", got)
		}
	})

	t.Run("normalizes paths without leading slash", func(t *testing.T) {
		mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.URL.Path)
		}))
		defer mockAPI.Close()

		proxy := NewHTTPProxy(strings.TrimPrefix(mockAPI.URL, "http://"), logger)
		req := ProxyRequest{ID: "req_path", Method: http.MethodGet, Path: "api/normalized"}
		payload, _ := json.Marshal(req)

		resp := decodeProxyResponse(t, mustHandleRequest(t, proxy, payload))
		if resp.Status != http.StatusOK {
			t.Fatalf("status: got %d, want %d", resp.Status, http.StatusOK)
		}
		body, err := base64.StdEncoding.DecodeString(resp.Body)
		if err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got, want := string(body), "/api/normalized"; got != want {
			t.Errorf("proxied path: got %q, want %q", got, want)
		}
	})
}

func mustHandleRequest(t *testing.T, proxy *HTTPProxy, payload []byte) []byte {
	t.Helper()
	resp, err := proxy.HandleRequest(payload, "token")
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}
	return resp
}
