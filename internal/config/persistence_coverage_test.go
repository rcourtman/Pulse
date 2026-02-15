package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFSError struct {
	FileSystem
	writeError  error
	renameError error
	readError   error
	mkdirError  error
}

func (m *mockFSError) WriteFile(name string, data []byte, perm os.FileMode) error {
	if m.writeError != nil {
		return m.writeError
	}
	return m.FileSystem.WriteFile(name, data, perm)
}

func (m *mockFSError) Rename(oldpath, newpath string) error {
	if m.renameError != nil {
		return m.renameError
	}
	return m.FileSystem.Rename(oldpath, newpath)
}

func (m *mockFSError) ReadFile(name string) ([]byte, error) {
	if m.readError != nil {
		return nil, m.readError
	}
	return m.FileSystem.ReadFile(name)
}

func (m *mockFSError) MkdirAll(path string, perm os.FileMode) error {
	if m.mkdirError != nil {
		return m.mkdirError
	}
	return m.FileSystem.MkdirAll(path, perm)
}

func TestWriteConfigFileLocked_Errors(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}}
	cp.SetFileSystem(mfs)

	data := []byte("{}")
	path := filepath.Join(tempDir, "test.json")

	// 1. WriteFile error
	mfs.writeError = errors.New("write error")
	err := cp.writeConfigFileLocked(path, data, 0600)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write error")

	// 2. Rename error
	mfs.writeError = nil
	mfs.renameError = errors.New("rename error")
	err = cp.writeConfigFileLocked(path, data, 0600)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rename error")
}

func TestSaveSystemSettings_EnvUpdateFailure(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// Create a .env file that is actually a directory to cause updateEnvFile to fail
	envPath := filepath.Join(tempDir, ".env")
	require.NoError(t, os.Mkdir(envPath, 0755))

	settings := SystemSettings{
		Theme: "dark",
	}

	// Should NOT return error even if .env update fails (logs warning)
	err := cp.SaveSystemSettings(settings)
	assert.NoError(t, err)

	// Verify system.json was still saved
	assert.FileExists(t, filepath.Join(tempDir, "system.json"))
}

func TestNewConfigPersistence_DataDirEnv(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cp := NewConfigPersistence("")
	assert.Equal(t, tempDir, cp.configDir)
}

func TestConfigPersistence_IsEncryptionEnabled(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	assert.True(t, cp.IsEncryptionEnabled())
}

func TestSaveAlertConfig_WriteError(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("write error")}
	cp.SetFileSystem(mfs)

	err := cp.SaveAlertConfig(alerts.AlertConfig{})
	assert.Error(t, err)
}

func TestSaveOIDCConfig_WriteError(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("write error")}
	cp.SetFileSystem(mfs)

	err := cp.SaveOIDCConfig(OIDCConfig{})
	assert.Error(t, err)
}

func TestSaveEmailConfig_WriteError(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("write error")}
	cp.SetFileSystem(mfs)

	err := cp.SaveEmailConfig(notifications.EmailConfig{})
	assert.Error(t, err)
}

func TestLoadAlertConfig_MockReadError(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("read error")}
	cp.SetFileSystem(mfs)

	_, err := cp.LoadAlertConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}

func TestLoadEmailConfig_Errors(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// 1. Read Error
	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("read error")}
	cp.SetFileSystem(mfs)
	_, err := cp.LoadEmailConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")

	// 2. Decrypt Error (garbage data with crypto enabled)
	cp.SetFileSystem(defaultFileSystem{})
	_ = os.WriteFile(filepath.Join(tempDir, "email.enc"), []byte("garbage"), 0600)
	// crypto is enabled by NewConfigPersistence
	_, err = cp.LoadEmailConfig()
	assert.Error(t, err)
	// Decrypt error message depends on crypto implementation, but it should error

	// 3. Unmarshal Error (garbage data with crypto disabled)
	cp.crypto = nil
	_, err = cp.LoadEmailConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestLoadAppriseConfig_Errors(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// 1. Read Error
	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("read error")}
	cp.SetFileSystem(mfs)
	_, err := cp.LoadAppriseConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")

	// 2. Decrypt Error
	cp.SetFileSystem(defaultFileSystem{})
	_ = os.WriteFile(filepath.Join(tempDir, "apprise.enc"), []byte("garbage"), 0600)
	_, err = cp.LoadAppriseConfig()
	assert.Error(t, err)

	// 3. Unmarshal Error
	cp.crypto = nil
	_, err = cp.LoadAppriseConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

type mockFSWriteSpecific struct {
	FileSystem
	failPattern string
}

func (m *mockFSWriteSpecific) WriteFile(name string, data []byte, perm os.FileMode) error {
	if strings.Contains(name, m.failPattern) {
		return os.ErrPermission
	}
	return m.FileSystem.WriteFile(name, data, perm)
}
