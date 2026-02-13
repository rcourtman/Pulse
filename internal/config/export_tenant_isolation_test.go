package config_test

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestExportConfig_UsesPersistenceScopedGuestMetadata(t *testing.T) {
	const passphrase = "tenant-export-passphrase"

	tenantConfigDir := t.TempDir()
	foreignDataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", foreignDataDir)

	cp := config.NewConfigPersistence(tenantConfigDir)
	require.NoError(t, cp.EnsureConfigDir())
	require.NoError(t, cp.GetGuestMetadataStore().Set("tenant:node:101", &config.GuestMetadata{
		CustomURL: "https://tenant.local",
	}))

	foreignStore := config.NewGuestMetadataStore(foreignDataDir, nil)
	require.NoError(t, foreignStore.Set("foreign:node:999", &config.GuestMetadata{
		CustomURL: "https://foreign.local",
	}))

	exported, err := cp.ExportConfig(passphrase)
	require.NoError(t, err)

	decoded := mustDecodeExport(t, exported, passphrase)
	require.Contains(t, decoded.GuestMetadata, "tenant:node:101")
	require.NotContains(t, decoded.GuestMetadata, "foreign:node:999")
}

func TestImportConfig_UsesPersistenceScopedGuestMetadata(t *testing.T) {
	const passphrase = "tenant-import-passphrase"

	sourceConfigDir := t.TempDir()
	targetConfigDir := t.TempDir()
	foreignDataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", foreignDataDir)

	source := config.NewConfigPersistence(sourceConfigDir)
	require.NoError(t, source.EnsureConfigDir())
	require.NoError(t, source.GetGuestMetadataStore().Set("tenant:node:202", &config.GuestMetadata{
		CustomURL: "https://source.local",
	}))

	exported, err := source.ExportConfig(passphrase)
	require.NoError(t, err)

	foreignStore := config.NewGuestMetadataStore(foreignDataDir, nil)
	require.NoError(t, foreignStore.Set("foreign:node:999", &config.GuestMetadata{
		CustomURL: "https://foreign.local",
	}))

	target := config.NewConfigPersistence(targetConfigDir)
	require.NoError(t, target.EnsureConfigDir())
	require.NoError(t, target.ImportConfig(exported, passphrase))

	targetStore := config.NewGuestMetadataStore(targetConfigDir, nil)
	require.NotNil(t, targetStore.Get("tenant:node:202"))

	foreignAfter := config.NewGuestMetadataStore(foreignDataDir, nil)
	require.Nil(t, foreignAfter.Get("tenant:node:202"))
	require.NotNil(t, foreignAfter.Get("foreign:node:999"))
}
