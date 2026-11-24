package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownloadAgentHelperRejectsUnsupportedArchitecture(t *testing.T) {
	router := &Router{}
	request := httptest.NewRequest(http.MethodGet, "/download/pulse-agent-helper?arch=darwin-arm64", nil)
	response := httptest.NewRecorder()

	router.handleDownloadAgentHelper(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestDownloadAgentHelperServesSignedLocalLinuxBinary(t *testing.T) {
	router, root := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0"
	content := []byte("linux helper")
	path := filepath.Join(root, "bin", "pulse-agent-helper-linux-amd64")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".sig", []byte("helper-signature"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".sshsig", []byte("helper-ssh-signature"), 0o644); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/download/pulse-agent-helper?arch=linux-amd64", nil)
	response := httptest.NewRecorder()
	router.handleDownloadAgentHelper(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	wantChecksum := sha256.Sum256(content)
	if got := response.Header().Get(checksumHeaderName); got != hex.EncodeToString(wantChecksum[:]) {
		t.Fatalf("checksum = %q", got)
	}
	if got := response.Header().Get(signatureHeaderName); got != "helper-signature" {
		t.Fatalf("signature = %q", got)
	}
	if got := response.Header().Get(sshSignatureHeaderName); got != encodedTestSSHSignature("helper-ssh-signature") {
		t.Fatalf("SSH signature = %q", got)
	}
}

func TestDownloadAgentHelperMissingDevBinaryReturnsExactBuildCommand(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "dev"
	request := httptest.NewRequest(http.MethodGet, "/download/pulse-agent-helper?arch=linux-armv7", nil)
	response := httptest.NewRecorder()

	router.handleDownloadAgentHelper(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	want := "CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o bin/pulse-agent-helper-linux-armv7 ./cmd/pulse-agent-helper"
	if !strings.Contains(response.Body.String(), want) {
		t.Fatalf("body missing build command: %s", response.Body.String())
	}
}

func TestDownloadAgentHelperProxiesPublishedSignedAsset(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0-rc.1"
	assetURL := "https://github.com/rcourtman/Pulse/releases/download/v6.0.0-rc.1/pulse-agent-helper-linux-amd64"
	router.installScriptClient = newTestInstallScriptClientSequence(t, []expectedHTTPExchange{
		{Method: http.MethodGet, URL: assetURL, Status: http.StatusOK, Body: "helper"},
		{Method: http.MethodGet, URL: assetURL + ".sig", Status: http.StatusOK, Body: "helper-signature"},
		{Method: http.MethodGet, URL: assetURL + ".sshsig", Status: http.StatusOK, Body: "helper-ssh-signature"},
	})
	request := httptest.NewRequest(http.MethodGet, "/download/pulse-agent-helper?arch=linux-amd64", nil)
	response := httptest.NewRecorder()

	router.handleDownloadAgentHelper(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("X-Served-From"); got != "github-proxy" {
		t.Fatalf("served from = %q", got)
	}
	if got := response.Header().Get(signatureHeaderName); got != "helper-signature" {
		t.Fatalf("signature = %q", got)
	}
}

func TestDownloadAgentRunnerRejectsUnsupportedArchitecture(t *testing.T) {
	router := &Router{}
	request := httptest.NewRequest(http.MethodGet, "/download/pulse-agent-runner?arch=darwin-arm64", nil)
	response := httptest.NewRecorder()
	router.handleDownloadAgentRunner(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestDownloadAgentRunnerServesSignedLocalLinuxBinary(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(binDir, "pulse-agent-runner-linux-amd64")
	content := []byte("runner")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".sig", []byte("detached"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".sshsig", []byte("ssh-signature"), 0o644); err != nil {
		t.Fatal(err)
	}
	router := &Router{projectRoot: root, serverVersion: "6.5.0"}
	request := httptest.NewRequest(http.MethodGet, "/download/pulse-agent-runner?arch=linux-amd64", nil)
	response := httptest.NewRecorder()
	router.handleDownloadAgentRunner(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get(checksumHeaderName); got == "" {
		t.Fatal("missing checksum header")
	}
	if got := response.Header().Get(signatureHeaderName); got != "detached" {
		t.Fatalf("signature = %q", got)
	}
	if got := response.Header().Get(sshSignatureHeaderName); got == "" {
		t.Fatal("missing SSH signature header")
	}
	if !bytes.Equal(response.Body.Bytes(), content) {
		t.Fatalf("body = %q", response.Body.Bytes())
	}
}

func TestDownloadAgentRunnerMissingDevBinaryReturnsExactBuildCommand(t *testing.T) {
	router := &Router{projectRoot: t.TempDir(), serverVersion: "dev"}
	request := httptest.NewRequest(http.MethodGet, "/download/pulse-agent-runner?arch=linux-armv7", nil)
	response := httptest.NewRecorder()
	router.handleDownloadAgentRunner(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	want := "CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o bin/pulse-agent-runner-linux-armv7 ./cmd/pulse-agent-runner"
	if !strings.Contains(response.Body.String(), want) {
		t.Fatalf("response missing build command %q: %s", want, response.Body.String())
	}
}

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

const proxiedHelperAssetBase = "https://github.com/rcourtman/Pulse/releases/download/v6.0.0-rc.1/"

// newBlockingReleaseAssetClient stands in for GitHub. Binary fetches block
// until their gate is closed; signature sidecars are answered at once.
func newBlockingReleaseAssetClient(body string, gates map[string]chan struct{}, binaryFetches *atomic.Int32) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		content := "signature"
		if gate, ok := gates[req.URL.String()]; ok {
			binaryFetches.Add(1)
			<-gate
			content = body
		} else if !strings.HasSuffix(req.URL.Path, ".sig") && !strings.HasSuffix(req.URL.Path, ".sshsig") {
			return nil, fmt.Errorf("unexpected request: %s", req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(content)),
		}, nil
	})}
}

func waitForProxiedReleaseAssets(t *testing.T, router *Router, done func(assets map[string]*proxiedReleaseAsset) bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		router.proxiedReleaseAssets.mu.Lock()
		ok := done(router.proxiedReleaseAssets.assets)
		router.proxiedReleaseAssets.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("proxied release assets did not reach the expected state")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestProxiedReleaseAssetIsSharedByConcurrentDownloads(t *testing.T) {
	const downloads = 12
	assetURL := proxiedHelperAssetBase + "pulse-agent-helper-linux-amd64"
	gates := map[string]chan struct{}{assetURL: make(chan struct{})}
	var binaryFetches atomic.Int32

	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0-rc.1"
	router.installScriptClient = newBlockingReleaseAssetClient("helper", gates, &binaryFetches)

	responses := make([]*httptest.ResponseRecorder, downloads)
	var wg sync.WaitGroup
	for i := range responses {
		responses[i] = httptest.NewRecorder()
		wg.Add(1)
		go func(response *httptest.ResponseRecorder) {
			defer wg.Done()
			router.handleDownloadAgentHelper(response, httptest.NewRequest(http.MethodGet, "/download/pulse-agent-helper?arch=linux-amd64", nil))
		}(responses[i])
	}
	waitForProxiedReleaseAssets(t, router, func(assets map[string]*proxiedReleaseAsset) bool {
		return assets[assetURL] != nil && assets[assetURL].refs == downloads
	})
	close(gates[assetURL])
	wg.Wait()

	if got := binaryFetches.Load(); got != 1 {
		t.Fatalf("binary fetched %d times for %d concurrent downloads, want 1", got, downloads)
	}
	for i, response := range responses {
		if response.Code != http.StatusOK || response.Body.String() != "helper" {
			t.Fatalf("download %d: status = %d, body = %q", i, response.Code, response.Body.String())
		}
		if response.Header().Get(checksumHeaderName) == "" {
			t.Fatalf("download %d: missing checksum header", i)
		}
	}
	waitForProxiedReleaseAssets(t, router, func(assets map[string]*proxiedReleaseAsset) bool { return len(assets) == 0 })
}

func TestProxiedReleaseAssetsAreBoundedAcrossDistinctAssets(t *testing.T) {
	previousWait := proxiedReleaseAssetSlotWait
	t.Cleanup(func() { proxiedReleaseAssetSlotWait = previousWait })

	helperAMD64 := proxiedHelperAssetBase + "pulse-agent-helper-linux-amd64"
	runnerAMD64 := proxiedHelperAssetBase + "pulse-agent-runner-linux-amd64"
	helperARM64 := proxiedHelperAssetBase + "pulse-agent-helper-linux-arm64"
	gates := map[string]chan struct{}{
		helperAMD64: make(chan struct{}),
		runnerAMD64: make(chan struct{}),
		helperARM64: make(chan struct{}),
	}
	close(gates[helperARM64])
	var binaryFetches atomic.Int32

	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0-rc.1"
	router.installScriptClient = newBlockingReleaseAssetClient("binary", gates, &binaryFetches)

	held := []*httptest.ResponseRecorder{httptest.NewRecorder(), httptest.NewRecorder()}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		router.handleDownloadAgentHelper(held[0], httptest.NewRequest(http.MethodGet, "/download/pulse-agent-helper?arch=linux-amd64", nil))
	}()
	go func() {
		defer wg.Done()
		router.handleDownloadAgentRunner(held[1], httptest.NewRequest(http.MethodGet, "/download/pulse-agent-runner?arch=linux-amd64", nil))
	}()
	waitForProxiedReleaseAssets(t, router, func(assets map[string]*proxiedReleaseAsset) bool {
		return len(assets) == maxProxiedReleaseAssets
	})

	// A third asset is refused while both copies are in use...
	proxiedReleaseAssetSlotWait = 20 * time.Millisecond
	refused := httptest.NewRecorder()
	router.handleDownloadAgentHelper(refused, httptest.NewRequest(http.MethodGet, "/download/pulse-agent-helper?arch=linux-arm64", nil))
	if refused.Code != http.StatusServiceUnavailable || refused.Header().Get("Retry-After") == "" {
		t.Fatalf("third asset: status = %d, Retry-After = %q", refused.Code, refused.Header().Get("Retry-After"))
	}
	if got := binaryFetches.Load(); got != maxProxiedReleaseAssets {
		t.Fatalf("binary fetches = %d, want %d", got, maxProxiedReleaseAssets)
	}

	// ...and served as soon as one of them is released.
	proxiedReleaseAssetSlotWait = time.Minute
	waited := httptest.NewRecorder()
	wg.Add(1)
	go func() {
		defer wg.Done()
		router.handleDownloadAgentHelper(waited, httptest.NewRequest(http.MethodGet, "/download/pulse-agent-helper?arch=linux-arm64", nil))
	}()
	close(gates[helperAMD64])
	close(gates[runnerAMD64])
	wg.Wait()

	for i, response := range append(held, waited) {
		if response.Code != http.StatusOK || response.Body.String() != "binary" {
			t.Fatalf("download %d: status = %d, body = %q", i, response.Code, response.Body.String())
		}
	}
}

func TestProxiedReleaseAssetIsReleasedWhenClientNeverReads(t *testing.T) {
	previousTimeout := proxiedReleaseAssetWriteTimeout
	proxiedReleaseAssetWriteTimeout = 100 * time.Millisecond
	t.Cleanup(func() { proxiedReleaseAssetWriteTimeout = previousTimeout })

	assetURL := proxiedHelperAssetBase + "pulse-agent-helper-linux-amd64"
	gates := map[string]chan struct{}{assetURL: make(chan struct{})}
	close(gates[assetURL])
	var binaryFetches atomic.Int32

	router, _ := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0-rc.1"
	// Larger than the loopback socket buffers, so the write cannot complete unread.
	router.installScriptClient = newBlockingReleaseAssetClient(strings.Repeat("x", 32<<20), gates, &binaryFetches)

	var serving atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		serving.Add(1)
		defer serving.Add(-1)
		router.handleDownloadAgentHelper(w, req)
	}))
	defer server.Close()

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET /download/pulse-agent-helper?arch=linux-amd64 HTTP/1.1\r\nHost: pulse\r\n\r\n"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for binaryFetches.Load() == 0 || serving.Load() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("response to a client that never reads was not abandoned")
		}
		time.Sleep(10 * time.Millisecond)
	}
	waitForProxiedReleaseAssets(t, router, func(assets map[string]*proxiedReleaseAsset) bool { return len(assets) == 0 })
}
