package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUnifiedAgentRouter(t *testing.T) (*Router, string) {
	tempDir := t.TempDir()

	// Create required directories
	err := os.MkdirAll(filepath.Join(tempDir, "scripts"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tempDir, "bin"), 0755)
	require.NoError(t, err)

	router := &Router{
		projectRoot:   tempDir,
		checksumCache: make(map[string]checksumCacheEntry),
	}

	return router, tempDir
}

func TestDownloadInstallScript_Local(t *testing.T) {
	router, tempDir := setupUnifiedAgentRouter(t)

	// Create dummy script
	scriptContent := "#!/bin/bash\necho 'installing'"
	scriptPath := filepath.Join(tempDir, "scripts", "install.sh")
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/install/install.sh", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScript(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, scriptContent, w.Body.String())
	assert.Equal(t, "text/x-shellscript", w.Header().Get("Content-Type"))
}

func TestDownloadInstallScriptPS_Local(t *testing.T) {
	router, tempDir := setupUnifiedAgentRouter(t)

	// Create dummy script
	scriptContent := "Write-Host 'installing'"
	scriptPath := filepath.Join(tempDir, "scripts", "install.ps1")
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/install/install.ps1", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedInstallScriptPS(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, scriptContent, w.Body.String())
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
}

func TestDownloadUnifiedAgent_Local_Generic(t *testing.T) {
	router, tempDir := setupUnifiedAgentRouter(t)

	// Create dummy binary in project root / bin
	binContent := "ELF binary content"
	binPath := filepath.Join(tempDir, "bin", "pulse-agent")
	err := os.WriteFile(binPath, []byte(binContent), 0755)
	require.NoError(t, err)

	// Since cachedSHA256 might not be initialized or working without real file usage pattern,
	// checking if our manual Router setup handles it.
	// cachedSHA256 needs 'checksumCache' map initialized which we did in setupUnifiedAgentRouter.

	req := httptest.NewRequest(http.MethodGet, "/api/install/agent", nil)
	w := httptest.NewRecorder()

	// Handle calls r.cachedSHA256 which reads the file
	router.handleDownloadUnifiedAgent(w, req)

	// We expect success if cachedSHA256 works
	if w.Code == http.StatusInternalServerError {
		// If cachedSHA256 fails (maybe because it's not exported or implemented elsewhere
		// and depends on something I missed), we will fail here.
		// cachedSHA256 is called in unified_agent.go but defined presumably in router.go or router_utils.go (unexported).
		// I initialized checksumCache so it should work.
		t.Logf("Handler returned 500: %s", w.Body.String())
	}

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, binContent, w.Body.String())

	// Verify Checksum Header
	hash := sha256.Sum256([]byte(binContent))
	expectedChecksum := hex.EncodeToString(hash[:])
	assert.Equal(t, expectedChecksum, w.Header().Get("X-Checksum-Sha256"))
}

func TestDownloadUnifiedAgent_Local_SpecificArch(t *testing.T) {
	router, tempDir := setupUnifiedAgentRouter(t)

	// Create dummy binary for linux-amd64
	binContent := "ELF AMD64 content"
	binPath := filepath.Join(tempDir, "bin", "pulse-agent-linux-amd64")
	err := os.WriteFile(binPath, []byte(binContent), 0755)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/install/agent?arch=amd64", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedAgent(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, binContent, w.Body.String())
}

func TestDownloadUnifiedAgent_Redirect(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)

	// Ensure NO local files exist (temp dir is empty of binaries)

	req := httptest.NewRequest(http.MethodGet, "/api/install/agent?arch=linux-amd64", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedAgent(w, req)

	// Since local binary is missing, it should redirect to GitHub
	require.Equal(t, http.StatusTemporaryRedirect, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, "github.com")
	assert.Contains(t, location, "pulse-agent-linux-amd64")
	assert.Equal(t, "github-redirect", w.Header().Get("X-Served-From"))
}

func TestDownloadUnifiedAgent_Redirect_Windows(t *testing.T) {
	router, _ := setupUnifiedAgentRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/install/agent?arch=windows-amd64", nil)
	w := httptest.NewRecorder()

	router.handleDownloadUnifiedAgent(w, req)

	require.Equal(t, http.StatusTemporaryRedirect, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, "pulse-agent-windows-amd64.exe")
}

func TestNormalizeUnifiedAgentArch(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"amd64", "linux-amd64"},
		{"x86_64", "linux-amd64"},
		{"linux-amd64", "linux-amd64"},
		{"arm64", "linux-arm64"},
		{"aarch64", "linux-arm64"},
		{"windows-amd64", "windows-amd64"},
		{"darwin-arm64", "darwin-arm64"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeUnifiedAgentArch(tt.input))
		})
	}
}
