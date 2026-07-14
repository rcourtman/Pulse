package relay

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func decodeProxyResponseBody(t *testing.T, resp ProxyResponse) string {
	t.Helper()
	if resp.Body == "" {
		return ""
	}
	body, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil {
		t.Fatalf("failed to decode proxy response body: %v", err)
	}
	return string(body)
}

// TestLocalHandlerDispatchWithTLSListener pins the regression that broke
// Remote Access sync on HTTPS-enabled instances: the main listener serves
// TLS, so the old loopback dial to http://localAddr got a bare
// "Client sent an HTTP request to an HTTPS server" 400 for every proxied
// request. With in-process dispatch the listener's scheme is irrelevant.
func TestLocalHandlerDispatchWithTLSListener(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"uri":%q}`, r.URL.RequestURI())
	})
	tlsListener := httptest.NewTLSServer(handler)
	defer tlsListener.Close()
	localAddr := strings.TrimPrefix(tlsListener.URL, "https://")

	// Old behavior (no handler): plaintext dial against the TLS listener
	// yields the bare 400 the mobile app surfaced as "alerts: HTTP 400".
	dialProxy := NewHTTPProxy(localAddr, zerolog.Nop())
	payload, _ := json.Marshal(ProxyRequest{ID: "r1", Method: "GET", Path: "/api/actions?view=pending&limit=100"})
	respBytes, err := dialProxy.HandleRequest(payload, "token")
	if err != nil {
		t.Fatalf("HandleRequest returned error: %v", err)
	}
	if resp := decodeProxyResponse(t, respBytes); resp.Status != http.StatusBadRequest {
		t.Fatalf("expected plaintext dial against TLS listener to fail with 400, got %d", resp.Status)
	}

	// New behavior: in-process dispatch reaches the same handler regardless
	// of the listener's scheme.
	handlerProxy := NewHTTPProxyWithLocalHandler(localAddr, handler, zerolog.Nop())
	for _, path := range []string{
		"/api/ai/patrol/findings?limit=20",
		"/api/actions?view=pending&limit=100",
	} {
		payload, _ := json.Marshal(ProxyRequest{ID: "r2", Method: "GET", Path: path})
		respBytes, err := handlerProxy.HandleRequest(payload, "token")
		if err != nil {
			t.Fatalf("HandleRequest(%s) returned error: %v", path, err)
		}
		resp := decodeProxyResponse(t, respBytes)
		if resp.Status != http.StatusOK {
			t.Fatalf("expected 200 for %s via local handler dispatch, got %d (%s)", path, resp.Status, decodeProxyResponseBody(t, resp))
		}
		if got, want := decodeProxyResponseBody(t, resp), fmt.Sprintf(`{"uri":%q}`, path); got != want {
			t.Fatalf("expected handler to observe %q, got %s", path, got)
		}
	}
}

func TestLocalHandlerDispatchForwardsRequestDetails(t *testing.T) {
	var seen struct {
		method     string
		requestURI string
		remoteAddr string
		apiToken   string
		body       string
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.method = r.Method
		seen.requestURI = r.RequestURI
		seen.remoteAddr = r.RemoteAddr
		seen.apiToken = r.Header.Get("X-API-Token")
		bodyBytes := make([]byte, 64)
		n, _ := r.Body.Read(bodyBytes)
		seen.body = string(bodyBytes[:n])
		w.WriteHeader(http.StatusCreated)
	})
	proxy := NewHTTPProxyWithLocalHandler("127.0.0.1:1", handler, zerolog.Nop())

	payload, _ := json.Marshal(ProxyRequest{
		ID:     "r1",
		Method: "POST",
		Path:   "/api/ai/patrol/acknowledge",
		Body:   base64.StdEncoding.EncodeToString([]byte(`{"finding_id":"f1"}`)),
	})
	respBytes, err := proxy.HandleRequest(payload, "secret-token")
	if err != nil {
		t.Fatalf("HandleRequest returned error: %v", err)
	}
	if resp := decodeProxyResponse(t, respBytes); resp.Status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Status)
	}
	if seen.method != http.MethodPost {
		t.Errorf("expected POST, got %q", seen.method)
	}
	if seen.requestURI != "/api/ai/patrol/acknowledge" {
		t.Errorf("unexpected RequestURI %q", seen.requestURI)
	}
	if seen.remoteAddr != "127.0.0.1:0" {
		t.Errorf("expected loopback RemoteAddr, got %q", seen.remoteAddr)
	}
	if seen.apiToken != "secret-token" {
		t.Errorf("expected injected API token, got %q", seen.apiToken)
	}
	if seen.body != `{"finding_id":"f1"}` {
		t.Errorf("unexpected body %q", seen.body)
	}
}

func TestLocalHandlerDispatchStreamsSSE(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("response writer does not implement http.Flusher")
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "data: chunk-%d\n\n", i)
			flusher.Flush()
		}
	})
	proxy := NewHTTPProxyWithLocalHandler("127.0.0.1:1", handler, zerolog.Nop())

	payload, _ := json.Marshal(ProxyRequest{ID: "s1", Method: "GET", Path: "/api/ai/chat/stream"})
	frames := make([][]byte, 0, 8)
	done := make(chan error, 1)
	go func() {
		done <- proxy.HandleStreamRequest(t.Context(), payload, "token", func(frame []byte) {
			frames = append(frames, frame)
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("HandleStreamRequest returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for stream to complete")
	}

	if len(frames) < 3 {
		t.Fatalf("expected header, data, and done frames, got %d", len(frames))
	}
	head := decodeProxyResponse(t, frames[0])
	if head.Status != http.StatusOK || !head.Stream || head.Headers["Content-Type"] != "text/event-stream" {
		t.Fatalf("unexpected stream header frame: %+v", head)
	}
	var streamed strings.Builder
	sawDone := false
	for _, frame := range frames[1:] {
		resp := decodeProxyResponse(t, frame)
		streamed.WriteString(decodeProxyResponseBody(t, resp))
		if resp.StreamDone {
			sawDone = true
		}
	}
	if !sawDone {
		t.Error("expected a stream_done frame")
	}
	for i := 0; i < 3; i++ {
		if want := fmt.Sprintf("data: chunk-%d", i); !strings.Contains(streamed.String(), want) {
			t.Errorf("streamed output missing %q: %q", want, streamed.String())
		}
	}
}

func TestLocalHandlerDispatchRecoversFromHandlerPanic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	proxy := NewHTTPProxyWithLocalHandler("127.0.0.1:1", handler, zerolog.Nop())

	payload, _ := json.Marshal(ProxyRequest{ID: "p1", Method: "GET", Path: "/api/health"})
	respBytes, err := proxy.HandleRequest(payload, "token")
	if err != nil {
		t.Fatalf("HandleRequest returned error: %v", err)
	}
	resp := decodeProxyResponse(t, respBytes)
	if resp.Status != http.StatusBadGateway && resp.Status != http.StatusInternalServerError {
		t.Fatalf("expected 502 or 500 after handler panic, got %d", resp.Status)
	}
}
