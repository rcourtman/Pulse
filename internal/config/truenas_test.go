package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTrueNASNewInstanceDefaultsAndUniqueID(t *testing.T) {
	first := NewTrueNASInstance()
	second := NewTrueNASInstance()

	require.NotEqual(t, first.ID, second.ID)
	_, err := uuid.Parse(first.ID)
	require.NoError(t, err)
	_, err = uuid.Parse(second.ID)
	require.NoError(t, err)

	require.True(t, first.UseHTTPS)
	require.True(t, first.Enabled)
	require.Equal(t, 60, first.PollIntervalSecs)
}

func TestTrueNASValidate(t *testing.T) {
	tests := []struct {
		name      string
		instance  *TrueNASInstance
		wantError string
	}{
		{
			name:      "nil instance",
			instance:  nil,
			wantError: "truenas instance is required",
		},
		{
			name: "missing host",
			instance: &TrueNASInstance{
				APIKey: "key",
			},
			wantError: "truenas host is required",
		},
		{
			name: "missing credentials",
			instance: &TrueNASInstance{
				Host: "nas.local",
			},
			wantError: "truenas credentials are required",
		},
		{
			name: "api key auth",
			instance: &TrueNASInstance{
				Host:   "nas.local",
				APIKey: "key",
			},
		},
		{
			name: "username password auth",
			instance: &TrueNASInstance{
				Host:     "nas.local",
				Username: "admin",
				Password: "secret",
			},
		},
		{
			name: "username without password",
			instance: &TrueNASInstance{
				Host:     "nas.local",
				Username: "admin",
			},
			wantError: "truenas credentials are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.instance.Validate()
			if tt.wantError == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestTrueNASRedacted(t *testing.T) {
	instance := TrueNASInstance{
		ID:       "id-1",
		Name:     "Home NAS",
		Host:     "nas.local",
		APIKey:   "api-secret",
		Username: "admin",
		Password: "password-secret",
		Enabled:  true,
	}

	redacted := instance.Redacted()

	require.Equal(t, trueNASSensitiveMask, redacted.APIKey)
	require.Equal(t, trueNASSensitiveMask, redacted.Password)
	require.Equal(t, instance.Username, redacted.Username)
	require.Equal(t, instance.Host, redacted.Host)
	require.Equal(t, "api-secret", instance.APIKey)
	require.Equal(t, "password-secret", instance.Password)
}

func TestTrueNASPersistenceRoundTripEncrypted(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	instance := NewTrueNASInstance()
	instance.Name = "Main NAS"
	instance.Host = "nas.local"
	instance.Port = 443
	instance.APIKey = "api-key-secret"
	instance.Fingerprint = "abcd"
	instance.InsecureSkipVerify = true
	instances := []TrueNASInstance{instance}

	require.NoError(t, cp.SaveTrueNASConfig(instances))

	filePath := filepath.Join(tempDir, "truenas.enc")
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	plain, err := json.MarshalIndent(instances, "", "  ")
	require.NoError(t, err)
	require.NotEqual(t, plain, raw)
	require.False(t, strings.Contains(string(raw), "api-key-secret"))
	require.False(t, strings.Contains(string(raw), "nas.local"))

	loaded, err := cp.LoadTrueNASConfig()
	require.NoError(t, err)
	require.Equal(t, instances, loaded)
}

func TestTrueNASLoadConfigMissingFileReturnsEmptySlice(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())

	loaded, err := cp.LoadTrueNASConfig()
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Empty(t, loaded)
}
