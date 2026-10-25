package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestServeFrontendHandlerMissingAssetsDoNotBecomeSPA(t *testing.T) {
	resetDevProxy()
	t.Cleanup(resetDevProxy)
	t.Setenv("FRONTEND_DEV_SERVER", "")
	tmp := t.TempDir()
	for name, content := range map[string]string{
		"index.html":               "<html>app-shell-marker</html>",
		"docs/RECOVERY.md":         "# Recovery\nShipped recovery guidance.",
		"assets/index-abc123.js":   "window.loaded = true;",
		"assets/index-abc123.css":  "body{}",
		"docs/nested/RECOVERY.md":  "# Nested recovery\nShipped nested guidance.",
		"docs/release-notes-v6.md": "# Release notes\nShipped notes.",
	} {
		file := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PULSE_FRONTEND_DIR", tmp)
	handler := serveFrontendHandler()

	for _, target := range []string{
		"/assets/missing-chunk.js", "/assets/missing.css?v=2", "/assets/no-extension",
		"/assets", "/assets/", "/docs/NOT_SHIPPED.md", "/docs/nested/NOT_SHIPPED.md",
		"/docs/NOT_SHIPPED.MD?version=old", "/missing.js", "/missing.mjs", "/missing.css",
		"/missing.map", "/missing.json", "/missing.html", "/favicon.ico", "/missing.svg",
		"/missing.png", "/missing.jpg", "/missing.jpeg", "/missing.gif", "/missing.webp",
		"/missing.avif", "/missing.woff", "/missing.woff2", "/missing.ttf", "/missing.otf",
		"/missing.wasm", "/robots.txt", "/missing.xml", "/manifest.webmanifest",
	} {
		t.Run(target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, target, nil))
			if rec.Code != http.StatusNotFound {
				t.Errorf("missing asset status = %d, want 404", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
				t.Errorf("missing asset content type = %q", got)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("missing asset cache control = %q, want no-store", got)
			}
			if strings.Contains(rec.Body.String(), "app-shell-marker") {
				t.Error("missing asset returned the application shell")
			}
		})
	}

	for _, target := range []string{
		"/", "/app/settings", "/docs", "/docs/", "/docs/RECOVERY",
		"/docs/NOT_SHIPPED", "/docs/nested/RECOVERY", "/docs/release-notes-v6",
		"/workloads/guest.example", "/assets-overview",
	} {
		t.Run("route"+target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, target, nil))
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app-shell-marker") {
				t.Errorf("client route must still receive the shell: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}

	for _, tc := range []struct {
		target      string
		contentType string
		contains    string
		immutable   bool
	}{
		{"/docs/RECOVERY.md", "text/plain; charset=utf-8", "Shipped recovery guidance.", false},
		{"/docs/nested/RECOVERY.md", "text/plain; charset=utf-8", "Shipped nested guidance.", false},
		{"/docs/release-notes-v6.md", "text/plain; charset=utf-8", "Shipped notes.", false},
		{"/assets/index-abc123.js", "application/javascript; charset=utf-8", "window.loaded = true;", true},
		{"/assets/index-abc123.css", "text/css; charset=utf-8", "body{}", true},
	} {
		t.Run("shipped"+tc.target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodGet, tc.target, nil))
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.contains) {
				t.Errorf("shipped content changed: status=%d body=%q", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != tc.contentType {
				t.Errorf("content type = %q, want %q", got, tc.contentType)
			}
			if got := strings.Contains(rec.Header().Get("Cache-Control"), "immutable"); got != tc.immutable {
				t.Errorf("immutable cache = %t, want %t", got, tc.immutable)
			}
		})
	}
}
