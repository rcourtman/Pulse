package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSSHConfig_AllowedDirectives(t *testing.T) {
	// Setup temporary home directory
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	handler := &SystemSettingsHandler{} // No dependencies needed for this handler

	validConfig := `
Host bastion
	Hostname 1.2.3.4
	User pulse
	IdentityFile ~/.ssh/id_rsa
	StrictHostKeyChecking no

Host internal
	ProxyJump bastion
	User internal
`
	req := httptest.NewRequest(http.MethodPost, "/api/config/ssh", strings.NewReader(validConfig))
	w := httptest.NewRecorder()

	handler.HandleSSHConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify file was written
	configPath := filepath.Join(tempHome, ".ssh", "config")
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, validConfig, string(content))
}

func TestHandleSSHConfig_ForbiddenDirectives(t *testing.T) {
	handler := &SystemSettingsHandler{}

	forbiddenConfig := `
Host bad
	PermitRootLogin yes
`
	req := httptest.NewRequest(http.MethodPost, "/api/config/ssh", strings.NewReader(forbiddenConfig))
	w := httptest.NewRecorder()

	handler.HandleSSHConfig(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "forbidden directive")
}

func TestHandleSSHConfig_Empty(t *testing.T) {
	handler := &SystemSettingsHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/config/ssh", strings.NewReader(""))
	w := httptest.NewRecorder()

	handler.HandleSSHConfig(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Empty SSH config")
}

func TestHandleSSHConfig_TooLarge(t *testing.T) {
	handler := &SystemSettingsHandler{}

	// 32KB + 1 byte
	largeConfig := string(make([]byte, 32*1024+1))
	req := httptest.NewRequest(http.MethodPost, "/api/config/ssh", strings.NewReader(largeConfig))
	w := httptest.NewRecorder()

	handler.HandleSSHConfig(w, req)

	// Expect 413 Payload Too Large OR 400 Bad Request depending on how MaxBytesReader behaves
	// The implementation checks for MaxBytesError and returns 413
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestHandleSSHConfig_WrongMethod(t *testing.T) {
	handler := &SystemSettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/config/ssh", nil)
	w := httptest.NewRecorder()
	handler.HandleSSHConfig(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestSetMonitor(t *testing.T) {
	handler := &SystemSettingsHandler{}
	monitor := &mockMonitor{} // mockMonitor is defined in system_settings_handlers_test.go (same package)

	handler.SetMonitor(monitor)
	assert.Equal(t, monitor, handler.legacyMonitor)
}

func TestSetConfig(t *testing.T) {
	handler := &SystemSettingsHandler{}
	cfg := &config.Config{AuthUser: "test"}

	handler.SetConfig(cfg)
	assert.Equal(t, cfg, handler.config)
}
