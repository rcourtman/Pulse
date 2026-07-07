package config

import (
	"bytes"
	"encoding/json"
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

	t.Run("plaintext sso rewrites encrypted storage on load", func(t *testing.T) {
		tempDir := t.TempDir()
		cp := NewConfigPersistence(tempDir)

		plaintext := &SSOConfig{
			Providers: []SSOProvider{
				{
					ID:      "oidc-1",
					Name:    "OIDC Provider",
					Type:    SSOProviderTypeOIDC,
					Enabled: true,
					OIDC: &OIDCProviderConfig{
						IssuerURL: "https://issuer.example.com",
						ClientID:  "pulse-client",
					},
				},
			},
		}
		raw, err := json.Marshal(plaintext)
		require.NoError(t, err)

		filePath := filepath.Join(tempDir, "sso.enc")
		require.NoError(t, os.WriteFile(filePath, raw, 0o600))

		cfg, err := cp.LoadSSOConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Len(t, cfg.Providers, 1)

		rewritten, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.False(t, bytes.Equal(rewritten, raw))
	})
}

func TestLoadSSOConfig_MigratesLegacyOIDC(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	cp.crypto = nil

	legacy := OIDCConfig{
		Enabled:       true,
		IssuerURL:     "https://issuer.example.com",
		ClientID:      "pulse-client",
		ClientSecret:  "client-secret",
		RedirectURL:   "https://pulse.example.com/api/oidc/callback",
		Scopes:        []string{"openid", "email"},
		GroupsClaim:   "groups",
		AllowedGroups: []string{"admins"},
		GroupRoleMappings: map[string]string{
			"admins": "admin",
		},
	}
	raw, err := json.Marshal(legacy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "oidc.enc"), raw, 0600))

	cfg, err := cp.LoadSSOConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	provider := cfg.GetProvider(LegacyOIDCProviderID)
	require.NotNil(t, provider)
	require.NotNil(t, provider.OIDC)
	assert.Equal(t, "Single Sign-On", provider.DisplayName)
	assert.Equal(t, SSOProviderTypeOIDC, provider.Type)
	assert.True(t, provider.Enabled)
	assert.Equal(t, "https://issuer.example.com", provider.OIDC.IssuerURL)
	assert.Equal(t, "pulse-client", provider.OIDC.ClientID)
	assert.Equal(t, "client-secret", provider.OIDC.ClientSecret)
	assert.Equal(t, "https://pulse.example.com/api/oidc/callback", provider.OIDC.RedirectURL)
	assert.Equal(t, []string{"openid", "email"}, provider.OIDC.Scopes)
	assert.Equal(t, "groups", provider.GroupsClaim)
	assert.Equal(t, []string{"admins"}, provider.AllowedGroups)
	assert.Equal(t, map[string]string{"admins": "admin"}, provider.GroupRoleMappings)
	assert.FileExists(t, filepath.Join(tempDir, "sso.enc"))
}

func TestLoadSSOConfig_FallbackAndErrors(t *testing.T) {
	t.Run("missing sso returns nil config", func(t *testing.T) {
		cp := NewConfigPersistence(t.TempDir())
		cp.crypto = nil

		cfg, err := cp.LoadSSOConfig()
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("disabled legacy oidc returns nil config", func(t *testing.T) {
		tempDir := t.TempDir()
		cp := NewConfigPersistence(tempDir)
		cp.crypto = nil

		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "oidc.enc"), []byte(`{"enabled":false,"issuerUrl":"https://issuer.example.com","clientId":"pulse-client"}`), 0600))

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
