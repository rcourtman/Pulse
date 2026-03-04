package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadSSOConfig_CloneAndNilHandling(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	cp.crypto = nil

	settings := &SSOConfig{
		Providers: []SSOProvider{
			{
				ID:      "oidc-1",
				Name:    "OIDC Provider",
				Type:    SSOProviderTypeOIDC,
				Enabled: true,
				OIDC: &OIDCProviderConfig{
					IssuerURL:    "https://issuer.example.com",
					ClientID:     "pulse-client",
					EnvOverrides: map[string]bool{"clientId": true},
				},
			},
		},
		DefaultProviderID:      "oidc-1",
		AllowMultipleProviders: true,
	}

	require.NoError(t, cp.SaveSSOConfig(settings))
	require.NotNil(t, settings.Providers[0].OIDC.EnvOverrides)
	assert.True(t, settings.Providers[0].OIDC.EnvOverrides["clientId"])

	loaded, err := cp.LoadSSOConfig()
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Len(t, loaded.Providers, 1)
	require.NotNil(t, loaded.Providers[0].OIDC)
	assert.Nil(t, loaded.Providers[0].OIDC.EnvOverrides)

	require.NoError(t, cp.SaveSSOConfig(nil))
	loadedDefault, err := cp.LoadSSOConfig()
	require.NoError(t, err)
	require.NotNil(t, loadedDefault)
	assert.Empty(t, loadedDefault.Providers)
	assert.True(t, loadedDefault.AllowMultipleProviders)
}

func TestSaveSSOConfig_ErrorPaths(t *testing.T) {
	t.Run("mkdir error", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())
		cp.SetFileSystem(&mockFSError{
			FileSystem: defaultFileSystem{},
			mkdirError: errors.New("mkdir error"),
		})

		err := cp.SaveSSOConfig(NewSSOConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mkdir error")
	})

	t.Run("write error", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())
		cp.SetFileSystem(&mockFSError{
			FileSystem: defaultFileSystem{},
			writeError: errors.New("write error"),
		})

		err := cp.SaveSSOConfig(NewSSOConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write error")
	})
}

func TestLoadSSOConfig_DoesNotMigrateLegacyOIDC(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	cp.crypto = nil

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "oidc.enc"), []byte(`{"enabled":true}`), 0600))

	cfg, err := cp.LoadSSOConfig()
	require.NoError(t, err)
	assert.Nil(t, cfg)
	assert.NoFileExists(t, filepath.Join(tempDir, "sso.enc"))
}

func TestLoadSSOConfig_FallbackAndErrors(t *testing.T) {
	t.Run("missing sso returns nil config", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())
		cp.crypto = nil

		cfg, err := cp.LoadSSOConfig()
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("legacy oidc present still returns nil config", func(t *testing.T) {
		tempDir := t.TempDir()
		cp := NewConfigPersistence(tempDir)
		cp.crypto = nil

		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "oidc.enc"), []byte(`{"enabled":true}`), 0600))

		cfg, err := cp.LoadSSOConfig()
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("invalid sso json returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		cp := NewConfigPersistence(tempDir)
		cp.crypto = nil

		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "sso.enc"), []byte("{not-json"), 0600))

		cfg, err := cp.LoadSSOConfig()
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("decrypt failure returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		cp := NewConfigPersistence(tempDir)

		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "sso.enc"), []byte("garbage"), 0600))

		cfg, err := cp.LoadSSOConfig()
		require.Error(t, err)
		assert.Nil(t, cfg)
	})
}
