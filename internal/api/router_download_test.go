package api

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTempPulseBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PULSE_BIN_DIR", dir)
	return dir
}

func TestHandleDownloadHostAgentServesWindowsExe(t *testing.T) {
	binDir := setupTempPulseBin(t)
	filePath := filepath.Join(binDir, "pulse-host-agent-windows-unit-test.exe")
	if err := os.WriteFile(filePath, []byte("exe-binary"), 0o755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/download/pulse-host-agent?platform=windows&arch=unit-test", nil)
	rr := httptest.NewRecorder()

	router := &Router{}
	router.handleDownloadHostAgent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	if got := rr.Body.String(); got != "exe-binary" {
		t.Fatalf("unexpected response body: %q", got)
	}
}

func TestHandleDownloadHostAgentServesChecksumForWindowsExe(t *testing.T) {
	const (
		arch     = "unit-sha"
		filename = "pulse-host-agent-windows-" + arch + ".exe"
	)
	binDir := setupTempPulseBin(t)
	filePath := filepath.Join(binDir, filename)

	payload := []byte("checksum-data")
	if err := os.WriteFile(filePath, payload, 0o755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/download/pulse-host-agent.sha256?platform=windows&arch=%s", arch), nil)
	rr := httptest.NewRecorder()

	router := &Router{}
	router.handleDownloadHostAgent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	expected := fmt.Sprintf("%x", sha256.Sum256(payload))
	if got := strings.TrimSpace(rr.Body.String()); got != expected {
		t.Fatalf("unexpected checksum body: got %q want %q", got, expected)
	}
}
