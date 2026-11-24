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

func TestHandleSSHConfig_RejectsUnsafeValues(t *testing.T) {
	handler := &SystemSettingsHandler{}

	tests := []struct {
		name   string
		config string
	}{
		{"command substitution in jump user", "Host *\n\tProxyJump $(touch${IFS}/tmp/pwned)@jump.invalid\n"},
		{"backticks in jump host", "Host *\n\tProxyJump root@`id`\n"},
		{"option as jump host", "Host *\n\tProxyJump -oProxyCommand\n"},
		{"option as jump user", "Host *\n\tProxyJump -A@jump\n"},
		{"unsafe later hop", "Host *\n\tProxyJump jump1,root@jump2;id\n"},
		{"hash inside a word is not a comment", "Host *\n\tProxyJump jump#$(id)@jump.invalid\n"},
		{"hash inside user is not a comment", "Host *\n\tUser root#$(id)\n"},
		{"quote in hostname", "Host node\n\tHostname node';id;'\n"},
		{"quoted hostname", "Host node\n\tHostname \"node';id;'\"\n"},
		{"hostname token", "Host node\n\tHostname %h\n"},
		{"command substitution in user", "Host *\n\tUser $(id)\n"},
		{"command substitution in host pattern", "Host $(id)\n"},
		{"command substitution in identity file", "Host *\n\tIdentityFile ~/.ssh/$(id)\n"},
		{"unknown strict host key checking value", "Host *\n\tStrictHostKeyChecking $(id)\n"},
		{"extra argument", "Host *\n\tUser root extra\n"},
		{"missing value", "Host *\n\tProxyJump\n"},
		{"missing value before comment", "Host *\n\tProxyJump # jump\n"},
		{"equals separator", "Host *\n\tProxyJump = jump\n"},
		{"carriage return inside value", "Host *\n\tUser root\rProxyCommand id\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempHome := t.TempDir()
			t.Setenv("HOME", tempHome)

			req := httptest.NewRequest(http.MethodPost, "/api/config/ssh", strings.NewReader(tt.config))
			w := httptest.NewRecorder()

			handler.HandleSSHConfig(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "validation_error")
			assert.NoFileExists(t, filepath.Join(tempHome, ".ssh", "config"))
		})
	}
}

func TestHandleSSHConfig_AllowsCommentsAndJumpLists(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	handler := &SystemSettingsHandler{}

	validConfig := "# Written by the Pulse setup script\r\n" +
		"Host pve1 10.0.0.* !10.0.0.1 fd00::*\r\n" +
		"    HostName fd00::10 # via the management network\r\n" +
		"    ProxyJump root@bastion.example.com:2222,jump_2\r\n" +
		"    IdentityFile /opt/pulse/.ssh/id_ed25519_sensors\r\n" +
		"    StrictHostKeyChecking accept-new\r\n"
	req := httptest.NewRequest(http.MethodPost, "/api/config/ssh", strings.NewReader(validConfig))
	w := httptest.NewRecorder()

	handler.HandleSSHConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	content, err := os.ReadFile(filepath.Join(tempHome, ".ssh", "config"))
	require.NoError(t, err)
	assert.Equal(t, validConfig, string(content))
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

func TestHandleSSHConfig_HardensExistingPermissions(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	sshDir := filepath.Join(tempHome, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0o755))
	configPath := filepath.Join(sshDir, "config")
	require.NoError(t, os.WriteFile(configPath, []byte("Host old\n\tHostname old.example\n"), 0o644))
	require.NoError(t, os.Chmod(sshDir, 0o755))
	require.NoError(t, os.Chmod(configPath, 0o644))

	handler := &SystemSettingsHandler{}
	validConfig := "Host bastion\n\tHostname 1.2.3.4\n\tUser pulse\n"
	req := httptest.NewRequest(http.MethodPost, "/api/config/ssh", strings.NewReader(validConfig))
	w := httptest.NewRecorder()

	handler.HandleSSHConfig(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	sshDirInfo, err := os.Stat(sshDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), sshDirInfo.Mode().Perm())

	configInfo, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), configInfo.Mode().Perm())
}

func TestSetMonitor(t *testing.T) {
	handler := &SystemSettingsHandler{}
	monitor := &mockMonitor{} // mockMonitor is defined in system_settings_handlers_test.go (same package)

	handler.SetMonitor(monitor)
	assert.Equal(t, monitor, handler.defaultMonitor)
}

func TestSetConfig(t *testing.T) {
	handler := &SystemSettingsHandler{}
	cfg := &config.Config{AuthUser: "test"}

	handler.SetConfig(cfg)
	assert.Equal(t, cfg, handler.config)
}
