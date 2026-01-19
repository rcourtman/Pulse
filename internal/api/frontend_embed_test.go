package api

import (
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

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestGetFrontendFSOverride(t *testing.T) {
	resetDevProxy()
	dir := t.TempDir()
	writeFile(t, dir, "index.html", "<html>ok</html>")
	t.Setenv("PULSE_FRONTEND_DIR", dir)

	fsys, err := getFrontendFS()
	if err != nil {
		t.Fatalf("getFrontendFS error: %v", err)
	}

	f, err := fsys.Open("index.html")
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if string(data) != "<html>ok</html>" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestServeFrontendHandler(t *testing.T) {
	resetDevProxy()
	dir := t.TempDir()
	writeFile(t, dir, "index.html", "<html>index</html>")
	writeFile(t, dir, "app-123.js", "console.log('ok');")
	t.Setenv("PULSE_FRONTEND_DIR", dir)
	t.Setenv("FRONTEND_DEV_SERVER", "")

	handler := serveFrontendHandler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "index") {
		t.Fatalf("unexpected root response: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Fatal("expected cache headers for index")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/app-123.js", nil)
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected asset response: %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("expected immutable cache header, got %s", rec.Header().Get("Cache-Control"))
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/missing", nil)
	handler(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "index") {
		t.Fatalf("expected SPA fallback, got %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	handler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for api path, got %d", rec.Code)
	}
}
