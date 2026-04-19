package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVMwareNewInstanceDefaults(t *testing.T) {
	inst := NewVMwareVCenterInstance()
	require.NotEmpty(t, inst.ID)
	require.Equal(t, defaultVMwarePort, inst.Port)
	require.True(t, inst.Enabled)
	require.True(t, inst.MonitorVMs)
	require.True(t, inst.MonitorHosts)
	require.True(t, inst.MonitorDatastores)
}

func TestVMwareApplyDefaultsLegacyScopeMigration(t *testing.T) {
	// A record saved before Monitor* fields existed decodes with
	// zero-value false across the board. ApplyDefaults must treat that
	// as "never configured" and enable every surface so the upgrade
	// doesn't silently stop collecting.
	inst := VMwareVCenterInstance{}
	inst.ApplyDefaults()
	require.Equal(t, defaultVMwarePort, inst.Port)
	require.True(t, inst.MonitorVMs)
	require.True(t, inst.MonitorHosts)
	require.True(t, inst.MonitorDatastores)
}

func TestVMwareApplyDefaultsPreservesExplicitScope(t *testing.T) {
	inst := VMwareVCenterInstance{MonitorVMs: true, MonitorHosts: false, MonitorDatastores: false}
	inst.ApplyDefaults()
	require.True(t, inst.MonitorVMs)
	require.False(t, inst.MonitorHosts)
	require.False(t, inst.MonitorDatastores)
}

func TestVMwareRedactedPreservesMonitorFields(t *testing.T) {
	inst := VMwareVCenterInstance{
		Name: "vc", Host: "vc.lan", Password: "secret",
		MonitorVMs: true, MonitorHosts: false, MonitorDatastores: true,
	}
	redacted := inst.Redacted()
	require.Equal(t, vmwareSensitiveMask, redacted.Password)
	require.True(t, redacted.MonitorVMs)
	require.False(t, redacted.MonitorHosts)
	require.True(t, redacted.MonitorDatastores)
}
