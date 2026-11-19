package config

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/stretchr/testify/require"
)

func TestTrueNASStoredScopeRoundTrip(t *testing.T) {
	for bits := 0; bits < 8; bits++ {
		t.Run(fmt.Sprintf("%03b", bits), func(t *testing.T) {
			dir := t.TempDir()
			p := NewConfigPersistence(dir)
			instance := NewTrueNASInstance()
			instance.Host = "nas.example.invalid"
			instance.Username = "pulse-monitor"
			instance.APIKey = "fixture-only-key"
			instance.Fingerprint = "saved-fingerprint"
			instance.OutboundPolicy = securityutil.ConnectionOutboundTenantPublic
			instance.MonitorDatasets = bits&1 != 0
			instance.MonitorPools = bits&2 != 0
			instance.MonitorReplication = bits&4 != 0
			require.NoError(t, p.SaveTrueNASConfig([]TrueNASInstance{instance}))
			raw, err := os.ReadFile(p.trueNASFile)
			require.NoError(t, err)
			require.NotContains(t, string(raw), instance.APIKey)

			reopened, err := NewConfigPersistence(dir).LoadTrueNASConfig()
			require.NoError(t, err)
			require.Equal(t, []TrueNASInstance{instance}, reopened)
		})
	}
}

func TestTrueNASStoredScopeDefaultsOnlyUnmarkedHistoricalZeros(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
		want   [3]bool
	}{
		{"absent", "", [3]bool{true, true, true}},
		{"serialized historical zeros", `,"monitorDatasets":false,"monitorPools":false,"monitorReplication":false`, [3]bool{true, true, true}},
		{"historical partial on", `,"monitorPools":true`, [3]bool{false, true, false}},
		{"marked all off", `,"collectionScopeConfigured":true,"monitorDatasets":false,"monitorPools":false,"monitorReplication":false`, [3]bool{}},
	} {
		for _, encrypted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/encrypted=%t", tc.name, encrypted), func(t *testing.T) {
				dir := t.TempDir()
				p := NewConfigPersistence(dir)
				raw := []byte(`[{"id":"nas","host":"nas.example.invalid","username":"pulse-monitor","apiKey":"fixture-only-key","useHttps":true,"enabled":true` + tc.fields + `}]`)
				if encrypted {
					var err error
					raw, err = p.crypto.Encrypt(raw)
					require.NoError(t, err)
				}
				require.NoError(t, os.WriteFile(p.trueNASFile, raw, 0o600))
				for i := 0; i < 2; i++ {
					// Also exercise plaintext migration and reopening.
					rows, err := NewConfigPersistence(dir).LoadTrueNASConfig()
					require.NoError(t, err)
					require.Len(t, rows, 1)
					row := rows[0]
					require.Equal(t, tc.want, [3]bool{row.MonitorDatasets, row.MonitorPools, row.MonitorReplication})
					require.Equal(t, "fixture-only-key", row.APIKey)
					require.True(t, row.Enabled)
					require.True(t, row.UseHTTPS)
				}
				raw, err := os.ReadFile(p.trueNASFile)
				require.NoError(t, err)
				_, err = p.crypto.Decrypt(raw)
				require.NoError(t, err)
			})
		}
	}
}

func TestTrueNASStoredScopeSurvivesHistoricalReadSave(t *testing.T) {
	p := NewConfigPersistence(t.TempDir())
	require.NoError(t, os.WriteFile(p.trueNASFile, []byte(`[{"id":"legacy","enabled":true}]`), 0o600))
	// Historical generic read/save did not normalize surviving connections.
	// An unrelated edit could therefore write absent fields as false.
	legacy, err := loadSlice[TrueNASInstance](p, p.trueNASFile, true)
	require.NoError(t, err)
	require.False(t, legacy[0].MonitorDatasets)
	require.NoError(t, saveJSON(p, p.trueNASFile, legacy, true))
	rows, err := p.LoadTrueNASConfig()
	require.NoError(t, err)
	require.Equal(t, [3]bool{true, true, true}, [3]bool{rows[0].MonitorDatasets, rows[0].MonitorPools, rows[0].MonitorReplication})

	// The next explicit save (not mere field presence) distinguishes all off.
	rows[0].MonitorDatasets, rows[0].MonitorPools, rows[0].MonitorReplication = false, false, false
	require.NoError(t, p.SaveTrueNASConfig(rows))
	off, err := p.LoadTrueNASConfig()
	require.NoError(t, err)
	require.Equal(t, rows, off)
	data, err := json.Marshal(off[0])
	require.NoError(t, err)
	require.NotContains(t, string(data), "collectionScopeConfigured")
}
