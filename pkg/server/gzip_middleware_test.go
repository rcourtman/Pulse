package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func gunzipBody(t *testing.T, body io.Reader) string {
	t.Helper()
	reader, err := gzip.NewReader(body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	return string(decoded)
}

func TestWithGzipCompressesJSONForAcceptingClients(t *testing.T) {
	payload := `{"resources":[` + strings.Repeat(`{"id":"vm-1","cpu":0.5},`, 200) + `{"id":"vm-last"}]}`
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Accept-Encoding", got)
	}
	if rec.Body.Len() >= len(payload) {
		t.Fatalf("compressed body (%d bytes) not smaller than payload (%d bytes)", rec.Body.Len(), len(payload))
	}
	if decoded := gunzipBody(t, rec.Body); decoded != payload {
		t.Fatalf("decompressed body does not match payload")
	}
}

func TestWithGzipPassesThroughWithoutAcceptEncoding(t *testing.T) {
	payload := strings.Repeat("plain body ", 500)
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Accept-Encoding on the identity variant too", got)
	}
	if rec.Body.String() != payload {
		t.Fatalf("body was modified without client opt-in")
	}
}

func TestWithGzipRespectsZeroQualityOptOut(t *testing.T) {
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat("x", 4096)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Header.Set("Accept-Encoding", "gzip;q=0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for q=0 opt-out", got)
	}
}

func TestWithGzipSkipsEventStreams(t *testing.T) {
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"tick\":1}\n\n"))
		if _, ok := w.(http.Flusher); !ok {
			t.Errorf("wrapped writer does not implement http.Flusher")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/ai/chat/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for SSE", got)
	}
	if !strings.HasPrefix(rec.Body.String(), "data:") {
		t.Fatalf("SSE body altered: %q", rec.Body.String())
	}
}

func TestWithGzipSkipsWebSocketUpgrades(t *testing.T) {
	var sawWrappedWriter bool
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawWrappedWriter = w.(*gzipResponseWriter)
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if sawWrappedWriter {
		t.Fatalf("websocket upgrade request received the gzip wrapper; hijacking would fail")
	}
}

func TestWithGzipSkipsSmallDeclaredBodies(t *testing.T) {
	payload := `{"status":"ok"}`
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "15")
		_, _ = w.Write([]byte(payload))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for tiny declared body", got)
	}
	if rec.Body.String() != payload {
		t.Fatalf("small body altered: %q", rec.Body.String())
	}
}

func TestWithGzipSkipsNonCompressibleContentTypes(t *testing.T) {
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(strings.Repeat("binary", 1024)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/logo.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for image/png", got)
	}
}

func TestWithGzipPreservesStatusAndEmptyBodies(t *testing.T) {
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/thing", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for 204", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 grew a %d-byte body", rec.Body.Len())
	}
}

func TestWithGzipDropsStaleContentLengthWhenCompressing(t *testing.T) {
	payload := strings.Repeat(`{"k":"v"}`, 1024)
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write([]byte(payload))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Fatalf("stale Content-Length %q survived compression", got)
	}
	if decoded := gunzipBody(t, rec.Body); decoded != payload {
		t.Fatalf("decompressed body does not match payload")
	}
}
