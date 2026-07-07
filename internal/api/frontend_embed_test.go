package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func resetDevProxy() {
	devProxyOnce = sync.Once{}
	devProxy = nil
	devProxyErr = nil
}

func TestGetFrontendDevProxy(t *testing.T) {
	resetDevProxy()
	t.Setenv("FRONTEND_DEV_SERVER", "http://localhost:1234")

	proxy, err := getFrontendDevProxy()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy == nil {
		t.Fatalf("expected proxy to be initialized")
	}

	resetDevProxy()
	t.Setenv("FRONTEND_DEV_SERVER", "://bad-url")
	if _, err := getFrontendDevProxy(); err == nil {
		t.Fatalf("expected error for invalid URL")
	}
}

func TestGetFrontendFS_Override(t *testing.T) {
	tmp := t.TempDir()
	indexPath := tmp + "/index.html"
	if err := os.WriteFile(indexPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	t.Setenv("PULSE_FRONTEND_DIR", tmp)
	fs, err := getFrontendFS()
	if err != nil {
		t.Fatalf("getFrontendFS: %v", err)
	}

	f, err := fs.Open("index.html")
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestServeFrontendHandler_StaticAndSPA(t *testing.T) {
	resetDevProxy()
	t.Setenv("FRONTEND_DEV_SERVER", "")
	tmp := t.TempDir()
	if err := os.MkdirAll(tmp+"/assets", 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(tmp+"/index.html", []byte("<html>ok</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(tmp+"/logo.svg", []byte("<svg></svg>"), 0o644); err != nil {
		t.Fatalf("write svg: %v", err)
	}
	if err := os.WriteFile(tmp+"/assets/index-abc123.css", []byte("body{}"), 0o644); err != nil {
		t.Fatalf("write css: %v", err)
	}
	t.Setenv("PULSE_FRONTEND_DIR", tmp)

	handler := serveFrontendHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("root status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content type, got %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc == "" {
		t.Fatalf("expected cache control headers")
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/index-abc123.css", nil)
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("hashed css status = %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("expected immutable cache control, got %q", cc)
	}

	req = httptest.NewRequest(http.MethodGet, "/logo.svg", nil)
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("svg status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Fatalf("expected svg content type, got %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatalf("expected no-cache headers, got %q", cc)
	}

	req = httptest.NewRequest(http.MethodGet, "/app/settings", nil)
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("spa route status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content type for spa, got %q", ct)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("api route status = %d", rec.Code)
	}
}

func TestServeIndexWithNonceAddsNonceToGeneratedInlineTags(t *testing.T) {
	content := []byte(`<html><head>` +
		`<script type="importmap">{"integrity":{}}</script>` +
		`<script src="/assets/index.js"></script>` +
		`<style>body{color:#111}</style>` +
		`<script nonce="__CSP_NONCE__">window.__theme="dark"</script>` +
		`</head><body></body></html>`)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), cspNonceKey{}, "test-nonce"))
	rec := httptest.NewRecorder()

	serveIndexWithNonce(rec, req, content)

	body := rec.Body.String()
	if strings.Contains(body, "__CSP_NONCE__") {
		t.Fatalf("expected nonce placeholder to be replaced, got body %q", body)
	}
	if !strings.Contains(body, `<script type="importmap" nonce="test-nonce">`) {
		t.Fatalf("expected import map script to receive nonce, got body %q", body)
	}
	if !strings.Contains(body, `<style nonce="test-nonce">body{color:#111}</style>`) {
		t.Fatalf("expected inline style to receive nonce, got body %q", body)
	}
	if !strings.Contains(body, `<script src="/assets/index.js"></script>`) {
		t.Fatalf("expected external script to remain without nonce, got body %q", body)
	}
	if got := strings.Count(body, `nonce="test-nonce"`); got != 3 {
		t.Fatalf("nonce count = %d, want 3 in body %q", got, body)
	}
}
