package server

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
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

func TestBoundedGzipWriterPoolLimitsIdleCompressionState(t *testing.T) {
	const capacity = 2
	pool := newBoundedGzipWriterPool(capacity)
	writers := make([]*gzip.Writer, 0, capacity+3)
	for range capacity + 3 {
		writer := pool.get()
		// Force allocation of the writer's BestSpeed working state. The pool's
		// contract is about bounding these initialized writers, not just shells.
		if _, err := writer.Write([]byte(strings.Repeat("state-payload", 1024))); err != nil {
			t.Fatalf("prime gzip writer: %v", err)
		}
		writers = append(writers, writer)
	}

	for _, writer := range writers {
		if err := writer.Close(); err != nil {
			t.Fatalf("close gzip writer: %v", err)
		}
		pool.put(writer)
	}

	if got := len(pool.idle); got != capacity {
		t.Fatalf("idle gzip writers = %d, want bounded capacity %d", got, capacity)
	}
}

func TestBoundedGzipWriterPoolSupportsNoIdleRetention(t *testing.T) {
	pool := newBoundedGzipWriterPool(0)
	writer := pool.get()
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	pool.put(writer)
	if got := len(pool.idle); got != 0 {
		t.Fatalf("idle gzip writers = %d, want 0", got)
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

func TestAcceptsGzipNegotiatesAllHeaderValuesAndStrictQvalues(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   bool
	}{
		{name: "zero with three fractional digits", values: []string{"gzip;q=0.000"}, want: false},
		{name: "case insensitive zero", values: []string{"gzip; Q = 0"}, want: false},
		{name: "positive fractional quality", values: []string{"br, gzip; q=0.001"}, want: true},
		{name: "wildcard permits gzip", values: []string{"identity;q=0, *;q=1"}, want: true},
		{name: "explicit gzip exclusion beats wildcard", values: []string{"*;q=1, gzip;q=0"}, want: false},
		{name: "second header line permits gzip", values: []string{"identity;q=0", "gzip"}, want: true},
		{name: "identity only", values: []string{"identity"}, want: false},
		{name: "invalid precision", values: []string{"gzip;q=0.0000"}, want: false},
		{name: "invalid value above one", values: []string{"gzip;q=1.001"}, want: false},
		{name: "missing quality value", values: []string{"gzip;q"}, want: false},
		{name: "duplicate quality parameter", values: []string{"gzip;q=0;q=1"}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
			req.Header["Accept-Encoding"] = append([]string(nil), tc.values...)
			if got := acceptsGzip(req); got != tc.want {
				t.Fatalf("acceptsGzip() = %v, want %v for %q", got, tc.want, tc.values)
			}
		})
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

func TestWithGzipSkipsTokenListWebSocketUpgrades(t *testing.T) {
	var sawWrappedWriter bool
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawWrappedWriter = w.(*gzipResponseWriter)
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/agent/ws", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Connection", "keep-alive, Upgrade")
	req.Header.Set("Upgrade", "websocket, h2c")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if sawWrappedWriter {
		t.Fatalf("token-list websocket upgrade received the gzip wrapper; hijacking would fail")
	}
}

func TestWithGzipTokenListWebSocketHandshakeThroughAPIErrorHandler(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	handler := withGzip(api.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("websocket upgrade: %v", err)
			return
		}
		_ = conn.Close()
	})))
	server := httptest.NewServer(handler)
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatalf("dial test server: %v", err)
	}
	defer conn.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/agent/ws", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, err := fmt.Fprintf(conn,
		"GET /api/agent/ws HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive, Upgrade\r\nUpgrade: websocket, h2c\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: MDEyMzQ1Njc4OWFiY2RlZg==\r\nAccept-Encoding: gzip\r\n\r\n",
		host,
	); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		t.Fatalf("read handshake: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101", response.StatusCode)
	}
}

func TestWithGzipExplicitEmpty200HasValidGzipBody(t *testing.T) {
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if decoded := gunzipBody(t, rec.Body); decoded != "" {
		t.Fatalf("decoded empty response = %q, want empty", decoded)
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

func TestWithGzipFlushBeforeFirstWriteKeepsEncodingCoherent(t *testing.T) {
	payload := strings.Repeat(`{"tick":1}`, 512)
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte(payload))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/stream-ish", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// The flush commits the headers, so whatever the headers said must match
	// the body encoding. Compressing here is fine only because the decision
	// ran before the flush.
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip decided at flush time", got)
	}
	if decoded := gunzipBody(t, rec.Body); decoded != payload {
		t.Fatalf("decompressed body does not match payload")
	}
}

func TestWithGzipFlushOnEventStreamStaysIdentity(t *testing.T) {
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("data: {\"tick\":1}\n\n"))
		w.(http.Flusher).Flush()
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/ai/chat/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for flushed SSE", got)
	}
	if !strings.HasPrefix(rec.Body.String(), "data:") {
		t.Fatalf("SSE body altered: %q", rec.Body.String())
	}
}

func TestWithGzipPreservesStatusAndEmptyBodies(t *testing.T) {
	for _, status := range []int{
		http.StatusNoContent,
		http.StatusResetContent,
		http.StatusNotModified,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
			}))

			req := httptest.NewRequest(http.MethodDelete, "/api/thing", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != status {
				t.Fatalf("status = %d, want %d", rec.Code, status)
			}
			if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q, want empty for %d", got, status)
			}
			if rec.Body.Len() != 0 {
				t.Fatalf("%d grew a %d-byte body", status, rec.Body.Len())
			}
		})
	}
}

func TestWithGzipPreservesPartialContentRepresentation(t *testing.T) {
	payload := strings.Repeat("x", 1024)
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Range", "bytes 0-1023/2048")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(payload))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/export?segment=first", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want identity for Content-Range semantics", got)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 0-1023/2048" {
		t.Fatalf("Content-Range = %q, want original byte range", got)
	}
	if rec.Body.String() != payload {
		t.Fatalf("partial response body was transformed")
	}
}

func TestWithGzipDefersDecisionPastInformationalResponse(t *testing.T) {
	payload := strings.Repeat(`{"status":"ready"}`, 256)
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	server := httptest.NewServer(handler)
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Accept-Encoding", "gzip")
	var informationalStatuses []int
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{
		Got1xxResponse: func(code int, _ textproto.MIMEHeader) error {
			informationalStatuses = append(informationalStatuses, code)
			return nil
		},
	}))
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer response.Body.Close()

	if len(informationalStatuses) != 1 || informationalStatuses[0] != http.StatusEarlyHints {
		t.Fatalf("informational statuses = %v, want [103]", informationalStatuses)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if got := response.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip on final response", got)
	}
	if decoded := gunzipBody(t, response.Body); decoded != payload {
		t.Fatalf("decompressed body does not match payload")
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
