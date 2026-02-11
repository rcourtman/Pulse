package agentupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func testBinary() []byte {
	switch runtime.GOOS {
	case "darwin":
		return []byte{0xcf, 0xfa, 0xed, 0xfe, 0x01, 0x02, 0x03, 0x04}
	case "windows":
		return []byte{'M', 'Z', 0x90, 0x00, 0x01, 0x02, 0x03, 0x04}
	default:
		return []byte{0x7f, 'E', 'L', 'F', 0x01, 0x02, 0x03, 0x04}
	}
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeTempExec(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	execPath := filepath.Join(dir, "agent")
	if err := os.WriteFile(execPath, []byte("old-binary"), 0755); err != nil {
		t.Fatalf("write exec: %v", err)
	}
	return dir, execPath
}

func newUpdaterForTest(serverURL string) *Updater {
	cfg := Config{
		PulseURL:       serverURL,
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
		CheckInterval:  10 * time.Millisecond,
	}
	return New(cfg)
}

func TestRestartProcess(t *testing.T) {
	orig := execFn
	t.Cleanup(func() { execFn = orig })

	execFn = func(string, []string, []string) error { return nil }
	if err := restartProcess("/bin/true"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	execFn = func(string, []string, []string) error { return errors.New("boom") }
	if err := restartProcess("/bin/false"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDetermineArchOverrides(t *testing.T) {
	origOS, origArch, origUname := runtimeGOOS, runtimeGOARCH, unameCommand
	t.Cleanup(func() {
		runtimeGOOS = origOS
		runtimeGOARCH = origArch
		unameCommand = origUname
	})

	runtimeGOOS = "linux"
	runtimeGOARCH = "arm"
	if got := determineArch(); got != "linux-armv7" {
		t.Fatalf("expected linux-armv7, got %q", got)
	}

	runtimeGOARCH = "386"
	if got := determineArch(); got != "linux-386" {
		t.Fatalf("expected linux-386, got %q", got)
	}

	runtimeGOOS = "solaris"
	runtimeGOARCH = "amd64"
	unameCommand = func() ([]byte, error) { return []byte("aarch64"), nil }
	if got := determineArch(); got != "linux-arm64" {
		t.Fatalf("expected linux-arm64, got %q", got)
	}

	unameCommand = func() ([]byte, error) { return []byte("x86_64"), nil }
	if got := determineArch(); got != "linux-amd64" {
		t.Fatalf("expected linux-amd64, got %q", got)
	}

	unameCommand = func() ([]byte, error) { return []byte("armv7l"), nil }
	if got := determineArch(); got != "linux-armv7" {
		t.Fatalf("expected linux-armv7, got %q", got)
	}

	unameCommand = func() ([]byte, error) { return []byte("mips"), nil }
	if got := determineArch(); got != "" {
		t.Fatalf("expected empty arch for unknown uname, got %q", got)
	}

	unameCommand = func() ([]byte, error) { return nil, errors.New("fail") }
	if got := determineArch(); got != "" {
		t.Fatalf("expected empty arch on uname error, got %q", got)
	}
}

func TestUnameCommandDefault(t *testing.T) {
	if _, err := exec.LookPath("uname"); err != nil {
		t.Skip("uname not available")
	}

	orig := unameCommand
	t.Cleanup(func() { unameCommand = orig })
	unameCommand = orig

	if _, err := unameCommand(); err != nil {
		t.Fatalf("expected uname to run, got %v", err)
	}
}

func TestVerifyBinaryMagicOverrides(t *testing.T) {
	origOS := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = origOS })

	tmpDir := t.TempDir()

	runtimeGOOS = "linux"
	elfPath := filepath.Join(tmpDir, "elf")
	elfData := []byte{0x7f, 'E', 'L', 'F', 0x00}
	if err := os.WriteFile(elfPath, elfData, 0644); err != nil {
		t.Fatalf("write elf: %v", err)
	}
	if err := verifyBinaryMagic(elfPath); err != nil {
		t.Fatalf("expected ELF to validate, got %v", err)
	}

	badELFPath := filepath.Join(tmpDir, "bad-elf")
	if err := os.WriteFile(badELFPath, []byte{0x00, 0x00, 0x00, 0x00}, 0644); err != nil {
		t.Fatalf("write bad elf: %v", err)
	}
	if err := verifyBinaryMagic(badELFPath); err == nil {
		t.Fatalf("expected ELF error")
	}

	runtimeGOOS = "darwin"
	machoPath := filepath.Join(tmpDir, "macho")
	machoData := []byte{0xcf, 0xfa, 0xed, 0xfe, 0x00}
	if err := os.WriteFile(machoPath, machoData, 0644); err != nil {
		t.Fatalf("write macho: %v", err)
	}
	if err := verifyBinaryMagic(machoPath); err != nil {
		t.Fatalf("expected macho to validate, got %v", err)
	}

	badPath := filepath.Join(tmpDir, "macho-bad")
	if err := os.WriteFile(badPath, []byte{0x00, 0x00, 0x00, 0x00}, 0644); err != nil {
		t.Fatalf("write bad macho: %v", err)
	}
	if err := verifyBinaryMagic(badPath); err == nil {
		t.Fatalf("expected macho error")
	}

	runtimeGOOS = "windows"
	pePath := filepath.Join(tmpDir, "pe.exe")
	if err := os.WriteFile(pePath, []byte{'M', 'Z', 0x00, 0x00}, 0644); err != nil {
		t.Fatalf("write pe: %v", err)
	}
	if err := verifyBinaryMagic(pePath); err != nil {
		t.Fatalf("expected PE to validate, got %v", err)
	}

	badPEPath := filepath.Join(tmpDir, "bad-pe.exe")
	if err := os.WriteFile(badPEPath, []byte{0x00, 0x00, 0x00, 0x00}, 0644); err != nil {
		t.Fatalf("write bad pe: %v", err)
	}
	if err := verifyBinaryMagic(badPEPath); err == nil {
		t.Fatalf("expected PE error")
	}

	runtimeGOOS = "plan9"
	planPath := filepath.Join(tmpDir, "plan9")
	if err := os.WriteFile(planPath, []byte{0x00, 0x01, 0x02, 0x03}, 0644); err != nil {
		t.Fatalf("write plan9: %v", err)
	}
	if err := verifyBinaryMagic(planPath); err != nil {
		t.Fatalf("expected unknown OS to skip verification, got %v", err)
	}
}

func TestIsUnraidOverride(t *testing.T) {
	orig := unraidVersionPath
	t.Cleanup(func() { unraidVersionPath = orig })

	tmpDir := t.TempDir()
	unraidVersionPath = filepath.Join(tmpDir, "unraid-version")
	if isUnraid() {
		t.Fatalf("expected false when file missing")
	}
	if err := os.WriteFile(unraidVersionPath, []byte("6.12"), 0644); err != nil {
		t.Fatalf("write unraid: %v", err)
	}
	if !isUnraid() {
		t.Fatalf("expected true when file exists")
	}
}

func TestGetServerVersion(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		var sawToken bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-API-Token") == "token" && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				sawToken = true
			}
			_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.cfg.APIToken = "token"
		u.client = server.Client()

		version, err := u.getServerVersion(context.Background())
		if err != nil {
			t.Fatalf("getServerVersion error: %v", err)
		}
		if version != "1.2.3" {
			t.Fatalf("expected version 1.2.3, got %q", version)
		}
		if !sawToken {
			t.Fatalf("expected token headers to be set")
		}
	})

	t.Run("InvalidURL", func(t *testing.T) {
		u := newUpdaterForTest("http://[::1")
		if _, err := u.getServerVersion(context.Background()); err == nil {
			t.Fatalf("expected error for invalid URL")
		}
	})

	t.Run("RequestError", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		u.client = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			}),
		}
		if _, err := u.getServerVersion(context.Background()); err == nil {
			t.Fatalf("expected request error")
		}
	})

	t.Run("StatusError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.client = server.Client()
		if _, err := u.getServerVersion(context.Background()); err == nil {
			t.Fatalf("expected status error")
		}
	})

	t.Run("DecodeError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("{"))
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.client = server.Client()
		if _, err := u.getServerVersion(context.Background()); err == nil {
			t.Fatalf("expected decode error")
		}
	})
}

func TestCheckAndUpdateBranches(t *testing.T) {
	t.Run("Disabled", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		u.cfg.Disabled = true
		u.performUpdateFn = func(context.Context) error {
			t.Fatalf("should not update when disabled")
			return nil
		}
		u.CheckAndUpdate(context.Background())
	})

	t.Run("DevCurrent", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		u.cfg.CurrentVersion = "dev"
		u.performUpdateFn = func(context.Context) error {
			t.Fatalf("should not update in dev mode")
			return nil
		}
		u.CheckAndUpdate(context.Background())
	})

	t.Run("NoPulseURL", func(t *testing.T) {
		u := newUpdaterForTest("")
		u.performUpdateFn = func(context.Context) error {
			t.Fatalf("should not update without URL")
			return nil
		}
		u.CheckAndUpdate(context.Background())
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.client = server.Client()
		u.performUpdateFn = func(context.Context) error {
			t.Fatalf("should not update on server error")
			return nil
		}
		u.CheckAndUpdate(context.Background())
	})

	t.Run("ServerDev", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"version":"dev"}`))
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.client = server.Client()
		u.performUpdateFn = func(context.Context) error {
			t.Fatalf("should not update when server dev")
			return nil
		}
		u.CheckAndUpdate(context.Background())
	})

	t.Run("UpToDate", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.client = server.Client()
		u.performUpdateFn = func(context.Context) error {
			t.Fatalf("should not update when up to date")
			return nil
		}
		u.CheckAndUpdate(context.Background())
	})

	t.Run("ServerOlder", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"version":"0.9.0"}`))
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.client = server.Client()
		u.performUpdateFn = func(context.Context) error {
			t.Fatalf("should not downgrade")
			return nil
		}
		u.CheckAndUpdate(context.Background())
	})

	t.Run("ServerNewer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"version":"1.1.0"}`))
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.client = server.Client()
		var called int32
		u.performUpdateFn = func(context.Context) error {
			atomic.AddInt32(&called, 1)
			return nil
		}
		u.CheckAndUpdate(context.Background())
		if called != 1 {
			t.Fatalf("expected update to be called")
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"version":"1.1.0"}`))
		}))
		t.Cleanup(server.Close)

		u := newUpdaterForTest(server.URL)
		u.client = server.Client()
		var called int32
		u.performUpdateFn = func(context.Context) error {
			atomic.AddInt32(&called, 1)
			return errors.New("fail")
		}
		u.CheckAndUpdate(context.Background())
		if called != 1 {
			t.Fatalf("expected update to be called")
		}
	})
}

func TestRunLoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":"1.1.0"}`))
	}))
	t.Cleanup(server.Close)

	u := newUpdaterForTest(server.URL)
	u.client = server.Client()
	u.initialDelay = 0
	u.newTicker = func(d time.Duration) *time.Ticker {
		return time.NewTicker(5 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called int32
	u.performUpdateFn = func(context.Context) error {
		if atomic.AddInt32(&called, 1) >= 2 {
			cancel()
		}
		return nil
	}

	done := make(chan struct{})
	go func() {
		u.RunLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("RunLoop did not exit")
	}

	if atomic.LoadInt32(&called) < 1 {
		t.Fatalf("expected RunLoop to invoke update")
	}
}

func TestRunLoopEarlyExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	u := newUpdaterForTest("http://example")
	u.cfg.Disabled = true

	done := make(chan struct{})
	go func() {
		u.RunLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected RunLoop to exit quickly")
	}

	u2 := newUpdaterForTest("http://example")
	u2.cfg.CurrentVersion = "dev"

	done2 := make(chan struct{})
	go func() {
		u2.RunLoop(context.Background())
		close(done2)
	}()

	select {
	case <-done2:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected RunLoop to exit for dev")
	}
}

func TestRunLoopInitialCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	u := newUpdaterForTest("http://example")
	u.initialDelay = 5 * time.Second

	done := make(chan struct{})
	go func() {
		u.RunLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected RunLoop to exit on cancel")
	}
}

func TestGetUpdatedFromVersion(t *testing.T) {
	origExec := osExecutableFn
	origEval := evalSymlinksFn
	t.Cleanup(func() {
		osExecutableFn = origExec
		evalSymlinksFn = origEval
	})

	tmpDir := t.TempDir()
	execPath := filepath.Join(tmpDir, "agent")
	infoPath := filepath.Join(tmpDir, ".pulse-update-info")
	if err := os.WriteFile(infoPath, []byte("1.0.0"), 0644); err != nil {
		t.Fatalf("write update info: %v", err)
	}

	osExecutableFn = func() (string, error) { return execPath, nil }
	evalSymlinksFn = func(string) (string, error) { return execPath, nil }

	version := GetUpdatedFromVersion()
	if version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %q", version)
	}
	if _, err := os.Stat(infoPath); err == nil {
		t.Fatalf("expected update info file to be removed")
	}

	missingPath := filepath.Join(tmpDir, "agent-missing")
	osExecutableFn = func() (string, error) { return missingPath, nil }
	evalSymlinksFn = func(string) (string, error) { return "", errors.New("fail") }
	if GetUpdatedFromVersion() != "" {
		t.Fatalf("expected empty version on missing file")
	}

	osExecutableFn = func() (string, error) { return "", errors.New("fail") }
	if GetUpdatedFromVersion() != "" {
		t.Fatalf("expected empty version on error")
	}
}

func TestPerformUpdateWrapper(t *testing.T) {
	t.Run("ExecPathError", func(t *testing.T) {
		origExec := osExecutableFn
		t.Cleanup(func() { osExecutableFn = origExec })
		osExecutableFn = func() (string, error) { return "", errors.New("fail") }

		u := newUpdaterForTest("http://example")
		if err := u.performUpdate(context.Background()); err == nil {
			t.Fatalf("expected exec path error")
		}
	})

	t.Run("Success", func(t *testing.T) {
		data := testBinary()
		check := checksum(data)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Checksum-Sha256", check)
			_, _ = w.Write(data)
		}))
		t.Cleanup(server.Close)

		_, execPath := writeTempExec(t)
		u := newUpdaterForTest(server.URL)
		u.client = server.Client()

		origExec := osExecutableFn
		origRestart := restartProcessFn
		t.Cleanup(func() {
			osExecutableFn = origExec
			restartProcessFn = origRestart
		})
		osExecutableFn = func() (string, error) { return execPath, nil }
		restartProcessFn = func(string) error { return nil }

		if err := u.performUpdate(context.Background()); err != nil {
			t.Fatalf("expected update success, got %v", err)
		}
	})
}

func TestPerformUpdateInvalidRequest(t *testing.T) {
	u := newUpdaterForTest("http://[::1")
	if err := u.performUpdateWithExecPath(context.Background(), ""); err == nil {
		t.Fatalf("expected error for invalid URL")
	}
}

func TestPerformUpdateDownloadErrorAndHeaders(t *testing.T) {
	_, execPath := writeTempExec(t)
	u := newUpdaterForTest("http://example")
	u.cfg.APIToken = "token"

	var sawToken bool
	u.client = &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("X-API-Token") == "token" && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				sawToken = true
			}
			return nil, errors.New("download fail")
		}),
	}

	if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
		t.Fatalf("expected download error")
	}
	if !sawToken {
		t.Fatalf("expected auth headers to be set")
	}
}

func TestPerformUpdateStatusFallbackAndSuccess(t *testing.T) {
	data := testBinary()
	check := checksum(data)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "arch=") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("X-Checksum-Sha256", check)
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)

	_, execPath := writeTempExec(t)

	u := newUpdaterForTest(server.URL)
	u.client = server.Client()

	origRestart := restartProcessFn
	t.Cleanup(func() { restartProcessFn = origRestart })
	restartProcessFn = func(string) error { return nil }

	if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
		t.Fatalf("expected update success, got %v", err)
	}

	updated, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read updated exec: %v", err)
	}
	if !bytes.HasPrefix(updated, testBinary()[:4]) {
		t.Fatalf("expected updated binary content")
	}
}

func TestPerformUpdateSymlinkFallback(t *testing.T) {
	data := testBinary()
	check := checksum(data)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Checksum-Sha256", check)
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)

	_, execPath := writeTempExec(t)
	u := newUpdaterForTest(server.URL)
	u.client = server.Client()

	origEval := evalSymlinksFn
	origRestart := restartProcessFn
	t.Cleanup(func() {
		evalSymlinksFn = origEval
		restartProcessFn = origRestart
	})
	evalSymlinksFn = func(string) (string, error) { return "", errors.New("fail") }
	restartProcessFn = func(string) error { return nil }

	if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
		t.Fatalf("expected update success with symlink fallback, got %v", err)
	}
}

func TestPerformUpdateErrors(t *testing.T) {
	t.Run("CreateTempError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Checksum-Sha256", checksum(testBinary()))
			_, _ = w.Write(testBinary())
		}))
		t.Cleanup(server.Close)

		_, execPath := writeTempExec(t)
		u := newUpdaterForTest(server.URL)
		u.client = server.Client()

		origCreate := createTempFn
		t.Cleanup(func() { createTempFn = origCreate })
		createTempFn = func(string, string) (*os.File, error) {
			return nil, errors.New("temp fail")
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected create temp error")
		}
	})

	t.Run("CopyError", func(t *testing.T) {
		_, execPath := writeTempExec(t)
		u := newUpdaterForTest("http://example")

		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				body := io.NopCloser(&errorReader{})
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       body,
					Header:     http.Header{"X-Checksum-Sha256": []string{checksum(testBinary())}},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected copy error")
		}
	})

	t.Run("TooLarge", func(t *testing.T) {
		_, execPath := writeTempExec(t)
		u := newUpdaterForTest("http://example")

		origMax := maxBinarySizeBytes
		t.Cleanup(func() { maxBinarySizeBytes = origMax })
		maxBinarySizeBytes = 4

		data := []byte{0x7f, 'E', 'L', 'F', 0x00, 0x01}
		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     http.Header{"X-Checksum-Sha256": []string{checksum(data)}},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected size error")
		}
	})

	t.Run("CloseError", func(t *testing.T) {
		data := testBinary()
		u := newUpdaterForTest("http://example")
		_, execPath := writeTempExec(t)

		origClose := closeFileFn
		origRestart := restartProcessFn
		t.Cleanup(func() {
			closeFileFn = origClose
			restartProcessFn = origRestart
		})
		closeFileFn = func(*os.File) error { return errors.New("close fail") }
		restartProcessFn = func(string) error { return nil }

		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     http.Header{"X-Checksum-Sha256": []string{checksum(data)}},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected close error")
		}
	})

	t.Run("InvalidBinary", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		_, execPath := writeTempExec(t)

		data := []byte{0x00, 0x00, 0x00, 0x00}
		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     http.Header{"X-Checksum-Sha256": []string{checksum(data)}},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected invalid binary error")
		}
	})

	t.Run("MissingChecksum", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		_, execPath := writeTempExec(t)

		data := testBinary()
		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     http.Header{},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected checksum missing error")
		}
	})

	t.Run("ChecksumMismatch", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		_, execPath := writeTempExec(t)

		data := testBinary()
		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     http.Header{"X-Checksum-Sha256": []string{"deadbeef"}},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected checksum mismatch error")
		}
	})

	t.Run("ChmodError", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		_, execPath := writeTempExec(t)
		data := testBinary()

		origChmod := chmodFn
		origRestart := restartProcessFn
		t.Cleanup(func() {
			chmodFn = origChmod
			restartProcessFn = origRestart
		})
		chmodFn = func(string, os.FileMode) error { return errors.New("chmod fail") }
		restartProcessFn = func(string) error { return nil }

		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     http.Header{"X-Checksum-Sha256": []string{checksum(data)}},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected chmod error")
		}
	})

	t.Run("BackupRenameError", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		_, execPath := writeTempExec(t)
		data := testBinary()

		origRename := renameFn
		origRestart := restartProcessFn
		t.Cleanup(func() {
			renameFn = origRename
			restartProcessFn = origRestart
		})
		renameFn = func(oldPath, newPath string) error {
			if strings.HasSuffix(newPath, ".backup") {
				return errors.New("backup fail")
			}
			return origRename(oldPath, newPath)
		}
		restartProcessFn = func(string) error { return nil }

		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     http.Header{"X-Checksum-Sha256": []string{checksum(data)}},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected backup rename error")
		}
	})

	t.Run("ReplaceRenameError", func(t *testing.T) {
		u := newUpdaterForTest("http://example")
		_, execPath := writeTempExec(t)
		data := testBinary()

		origRename := renameFn
		origRestart := restartProcessFn
		t.Cleanup(func() {
			renameFn = origRename
			restartProcessFn = origRestart
		})
		var calls int32
		renameFn = func(oldPath, newPath string) error {
			if atomic.AddInt32(&calls, 1) == 2 {
				return errors.New("replace fail")
			}
			return origRename(oldPath, newPath)
		}
		restartProcessFn = func(string) error { return nil }

		u.client = &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     http.Header{"X-Checksum-Sha256": []string{checksum(data)}},
				}, nil
			}),
		}

		if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil {
			t.Fatalf("expected replace rename error")
		}
	})
}

func TestPerformUpdateUnraidPaths(t *testing.T) {
	data := testBinary()
	check := checksum(data)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Checksum-Sha256", check)
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)

	_, execPath := writeTempExec(t)
	u := newUpdaterForTest(server.URL)
	u.client = server.Client()

	origUnraid := unraidVersionPath
	origPersist := unraidPersistentPathFn
	origRead := readFileFn
	origWrite := writeFileFn
	origRename := renameFn
	origRestart := restartProcessFn
	t.Cleanup(func() {
		unraidVersionPath = origUnraid
		unraidPersistentPathFn = origPersist
		readFileFn = origRead
		writeFileFn = origWrite
		renameFn = origRename
		restartProcessFn = origRestart
	})

	tmpDir := t.TempDir()
	unraidVersionPath = filepath.Join(tmpDir, "unraid-version")
	if err := os.WriteFile(unraidVersionPath, []byte("6.12"), 0644); err != nil {
		t.Fatalf("write unraid version: %v", err)
	}

	persistDir := filepath.Join(tmpDir, "persist")
	if err := os.MkdirAll(persistDir, 0755); err != nil {
		t.Fatalf("mkdir persist: %v", err)
	}
	persistPath := filepath.Join(persistDir, "pulse-agent")
	if err := os.WriteFile(persistPath, []byte("old"), 0644); err != nil {
		t.Fatalf("write persist: %v", err)
	}
	unraidPersistentPathFn = func(string) string { return persistPath }
	restartProcessFn = func(string) error { return nil }

	t.Run("ReadError", func(t *testing.T) {
		readFileFn = func(string) ([]byte, error) { return nil, errors.New("read fail") }
		if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
			t.Fatalf("expected update success, got %v", err)
		}
	})

	t.Run("WriteError", func(t *testing.T) {
		readFileFn = func(string) ([]byte, error) { return []byte("new"), nil }
		writeFileFn = func(string, []byte, os.FileMode) error { return errors.New("write fail") }
		if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
			t.Fatalf("expected update success, got %v", err)
		}
	})

	t.Run("RenameError", func(t *testing.T) {
		readFileFn = func(string) ([]byte, error) { return []byte("new"), nil }
		writeFileFn = os.WriteFile
		renameFn = func(oldPath, newPath string) error {
			if newPath == persistPath {
				return errors.New("rename fail")
			}
			return os.Rename(oldPath, newPath)
		}
		if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
			t.Fatalf("expected update success, got %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		readFileFn = func(string) ([]byte, error) { return []byte("new"), nil }
		writeFileFn = os.WriteFile
		renameFn = os.Rename
		if err := u.performUpdateWithExecPath(context.Background(), execPath); err != nil {
			t.Fatalf("expected update success, got %v", err)
		}
	})
}

type errorReader struct {
	sent bool
}

func (e *errorReader) Read(p []byte) (int, error) {
	if e.sent {
		return 0, errors.New("read fail")
	}
	e.sent = true
	copy(p, testBinary())
	return len(testBinary()), errors.New("read fail")
}
