package api

import (
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

type expectedHTTPExchange struct {
	Method string
	URL    string
	Status int
	Body   string
	Err    error
}

func newTestInstallScriptClientSequence(t *testing.T, exchanges []expectedHTTPExchange) *http.Client {
	t.Helper()

	var index int
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if index >= len(exchanges) {
				t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			}
			exchange := exchanges[index]
			index++
			if exchange.Method != "" && req.Method != exchange.Method {
				t.Fatalf("unexpected method at request %d: %s", index, req.Method)
			}
			if exchange.URL != "" && req.URL.String() != exchange.URL {
				t.Fatalf("unexpected URL at request %d: %s", index, req.URL.String())
			}
			if exchange.Err != nil {
				return nil, exchange.Err
			}
			return &http.Response{
				StatusCode: exchange.Status,
				Status:     fmt.Sprintf("%d %s", exchange.Status, http.StatusText(exchange.Status)),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(exchange.Body)),
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

func TestDownloadUnifiedInstallScript_NoLocalScriptFailsClosed(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0-rc.6"
	// No agent installer is bundled. The endpoint must fail closed and must never
	// proxy the GitHub install.sh release asset (the SERVER installer). Any outbound
	// HTTP call from this endpoint is a regression of the issue #1470 root fix.
	router.installScriptClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Errorf("install-script endpoint must not make outbound calls; got %s %s", req.Method, req.URL)
		return nil, fmt.Errorf("unexpected outbound call")
	})}

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScript(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if got := w.Header().Get("X-Served-From"); got != "" {
		t.Fatalf("endpoint must not proxy; X-Served-From = %q", got)
	}
}

func TestDownloadUnifiedInstallScriptPS_NoLocalScriptFailsClosed(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0"
	router.installScriptClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Errorf("install-script endpoint must not make outbound calls; got %s %s", req.Method, req.URL)
		return nil, fmt.Errorf("unexpected outbound call")
	})}

	req := httptest.NewRequest(http.MethodGet, "/install.ps1", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScriptPS(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDownloadUnifiedInstallScript_NoLocalScriptFailsClosedForDevBuild(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "dev"

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScript(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDownloadUnifiedInstallScript_NoLocalScriptFailsClosedHEAD(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0-rc.6"

	req := httptest.NewRequest(http.MethodHead, "/install.sh", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScript(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
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
