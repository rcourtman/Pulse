package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestInstallScriptClient(t *testing.T, expectedMethod, expectedURL string, status int, body string, err error) *http.Client {
	t.Helper()

	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if expectedMethod != "" && req.Method != expectedMethod {
				t.Fatalf("unexpected method: %s", req.Method)
			}
			if expectedURL != "" && req.URL.String() != expectedURL {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: status,
				Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
}

func TestDownloadUnifiedInstallScript_MethodNotAllowed(t *testing.T) {
	router := &Router{}

	req := httptest.NewRequest(http.MethodPost, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScript(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestDownloadUnifiedInstallScriptPS_MethodNotAllowed(t *testing.T) {
	router := &Router{}

	req := httptest.NewRequest(http.MethodPut, "/install.ps1", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScriptPS(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestDownloadUnifiedInstallScript_ProxyFallback(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0-rc.1"
	expectedURL := "https://github.com/rcourtman/Pulse/releases/download/v6.0.0-rc.1/install.sh"
	payload := "#!/bin/bash\necho hi"
	router.installScriptClient = newTestInstallScriptClient(t, http.MethodGet, expectedURL, http.StatusOK, payload, nil)

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScript(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("X-Served-From"); got != "github-fallback" {
		t.Fatalf("unexpected X-Served-From header: %q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "text/x-shellscript" {
		t.Fatalf("unexpected Content-Type: %q", got)
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "install.sh") {
		t.Fatalf("missing Content-Disposition filename")
	}
	if strings.TrimSpace(w.Body.String()) != payload {
		t.Fatalf("unexpected response body")
	}
}

func TestDownloadUnifiedInstallScriptPS_ProxyFallback(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "6.0.0"
	expectedURL := "https://github.com/rcourtman/Pulse/releases/download/v6.0.0/install.ps1"
	payload := "Write-Host 'hi'"
	router.installScriptClient = newTestInstallScriptClient(t, http.MethodGet, expectedURL, http.StatusOK, payload, nil)

	req := httptest.NewRequest(http.MethodGet, "/install.ps1", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScriptPS(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("unexpected Content-Type: %q", got)
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "install.ps1") {
		t.Fatalf("missing Content-Disposition filename")
	}
}

func TestProxyInstallScriptFromGitHub_NonOK(t *testing.T) {
	router := &Router{
		serverVersion:       "v6.0.0-rc.1",
		installScriptClient: newTestInstallScriptClient(t, http.MethodGet, "", http.StatusNotFound, "", nil),
	}

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.proxyInstallScriptFromGitHub(w, req, "install.sh")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Install script not found") {
		t.Fatalf("expected not found message")
	}
}

func TestProxyInstallScriptFromGitHub_Error(t *testing.T) {
	router := &Router{
		serverVersion:       "v6.0.0-rc.1",
		installScriptClient: newTestInstallScriptClient(t, http.MethodGet, "", 0, "", errors.New("boom")),
	}

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.proxyInstallScriptFromGitHub(w, req, "install.sh")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Failed to fetch install script") {
		t.Fatalf("expected fetch failure message")
	}
}

func TestDownloadUnifiedInstallScript_ProxyFallbackRejectsDevelopmentBuild(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "dev"

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScript(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Install script unavailable for current server build") {
		t.Fatalf("unexpected response body: %q", w.Body.String())
	}
}

func TestDownloadUnifiedInstallScript_ProxyFallbackPreservesHEAD(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0-rc.1"
	expectedURL := "https://github.com/rcourtman/Pulse/releases/download/v6.0.0-rc.1/install.sh"
	router.installScriptClient = newTestInstallScriptClient(t, http.MethodHead, expectedURL, http.StatusOK, "", nil)

	req := httptest.NewRequest(http.MethodHead, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScript(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty HEAD response body")
	}
}

func TestDownloadUnifiedAgent_MethodNotAllowed(t *testing.T) {
	router := &Router{}

	req := httptest.NewRequest(http.MethodPost, "/download/pulse-agent", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedAgent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestDownloadUnifiedAgent_NoArchNotFound(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_BIN_DIR", tempDir)

	router := &Router{projectRoot: tempDir}
	req := httptest.NewRequest(http.MethodGet, "/download/pulse-agent", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedAgent(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Specify ?arch=linux-amd64") {
		t.Fatalf("expected guidance message")
	}
}
