package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/stretchr/testify/require"
)

// A SMART pass does not erase a canonical warning. Both query entry points must
// preserve its evidence, including the distinction between absent and zero.
func TestPhysicalDiskEvidencePreservesCanonicalRisk(t *testing.T) {
	zero, mediaErrors := int64(0), int64(12)
	used := 37
	risk := &unifiedresources.PhysicalDiskRisk{
		Level: storagehealth.RiskWarning,
		Reasons: []unifiedresources.PhysicalDiskRiskReason{{
			Code: "media_errors", Severity: storagehealth.RiskWarning,
			Summary: "12 media errors reported",
		}},
	}
	observed := time.Date(2026, 9, 5, 21, 0, 0, 0, time.UTC)
	resources := []unifiedresources.Resource{
		{ID: "disk-warning", Name: "Warning disk", Type: unifiedresources.ResourceTypePhysicalDisk,
			Status: unifiedresources.StatusWarning, ParentName: "node-one", LastSeen: observed,
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
				Health: "PASSED", Wearout: 63, Risk: risk,
				SMART: &unifiedresources.SMARTMeta{MediaErrors: &mediaErrors, UDMACRCErrors: &zero, PercentageUsed: &used},
			}},
		{ID: "disk-unknown", Name: "Unknown disk", Type: unifiedresources.ResourceTypePhysicalDisk,
			Status: unifiedresources.StatusUnknown, ParentName: "node-one", LastSeen: observed,
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{Health: "UNKNOWN", Wearout: -1}},
		{ID: "disk-stale", Name: "Stale disk", Type: unifiedresources.ResourceTypePhysicalDisk,
			Status: unifiedresources.StatusWarning, ParentName: "node-one", LastSeen: observed,
			SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
				unifiedresources.SourceProxmox: {Status: "stale", LastSeen: observed},
			},
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{Health: "PASSED", Wearout: 63}},
	}
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:           &mockStateProvider{state: models.StateSnapshot{}},
		UnifiedResourceProvider: &stubUnifiedResourceProvider{resources: resources},
		ControlLevel:            ControlLevelReadOnly,
	})
	for _, tool := range []struct {
		name string
		args map[string]interface{}
		key  string
	}{
		{"pulse_metrics", map[string]interface{}{"type": "disks"}, "disks"},
		{"pulse_query", map[string]interface{}{"action": "list", "type": "physical-disks"}, "physical_disks"},
	} {
		t.Run(tool.name, func(t *testing.T) {
			result, err := executor.ExecuteTool(context.Background(), tool.name, tool.args)
			require.NoError(t, err)
			require.False(t, result.IsError, "%+v", result.Content)
			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &payload))
			var disks []PhysicalDiskSummary
			require.NoError(t, json.Unmarshal(payload[tool.key], &disks))
			require.Len(t, disks, 3)
			byID := map[string]PhysicalDiskSummary{}
			for _, disk := range disks {
				byID[disk.ID] = disk
			}
			warning := byID["disk-warning"]
			require.Equal(t, "PASSED", warning.Health)
			require.Equal(t, unifiedresources.StatusWarning, warning.Status)
			require.Equal(t, risk, warning.Risk)
			require.Equal(t, resources[0].PhysicalDisk.SMART, warning.SMART)
			require.NotNil(t, warning.LifeRemainingPercent)
			require.Equal(t, 63, *warning.LifeRemainingPercent)
			require.Equal(t, observed, warning.LastChecked)
			require.Equal(t, "node-one", warning.Node)
			unknown := byID["disk-unknown"]
			require.Equal(t, unifiedresources.StatusUnknown, unknown.Status)
			require.Nil(t, unknown.SMART)
			require.Nil(t, unknown.Risk)
			require.Nil(t, unknown.LifeRemainingPercent)
			stale := byID["disk-stale"]
			require.Equal(t, "PASSED", stale.Health)
			require.Equal(t, unifiedresources.StatusWarning, stale.Status)
			require.Nil(t, stale.Risk)
			require.Equal(t, resources[2].SourceStatus, stale.SourceStatus)
			encoded, err := json.Marshal(warning.SMART)
			require.NoError(t, err)
			require.Contains(t, string(encoded), `"udmaCrcErrors":0`)
			require.NotContains(t, string(encoded), `"pendingSectors"`)
		})
	}
	for _, action := range []string{"get", "health"} {
		t.Run(action, func(t *testing.T) {
			result, err := executor.ExecuteTool(context.Background(), "pulse_query", map[string]interface{}{
				"action": action, "resource_type": "physical-disk", "resource_id": "disk-warning",
			})
			require.NoError(t, err)
			require.False(t, result.IsError, "%+v", result.Content)
			var disk PhysicalDiskSummary
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &disk))
			require.Equal(t, risk, disk.Risk)
			require.Equal(t, resources[0].PhysicalDisk.SMART, disk.SMART)
			require.Equal(t, 63, *disk.LifeRemainingPercent)
			require.Contains(t, result.Content[0].Text, `"life_remaining_percent":63`)
			require.NotContains(t, result.Content[0].Text, `"wearout"`)
		})
	}

}

// Normalization keeps a disk's last-known temperature when the current
// observation did not collect one (a disk in standby, a host agent past its
// reporting lease). The tools must not hand that value to the model as the
// disk's temperature: it moves to last_known_temperature with its reason.
func TestDiskToolsReportRetainedTemperatureAsLastKnown(t *testing.T) {
	standby := &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("smartctl", "disk is in standby")}
	silent := &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("smartctl", "host agent stopped reporting")}
	collected := &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")}
	resources := []unifiedresources.Resource{
		{ID: "disk-standby", Name: "Standby disk", Type: unifiedresources.ResourceTypePhysicalDisk,
			Status: unifiedresources.StatusOnline, ParentName: "node-one",
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{Health: "PASSED", Wearout: -1, Temperature: 61, Collection: standby}},
		{ID: "disk-live", Name: "Live disk", Type: unifiedresources.ResourceTypePhysicalDisk,
			Status: unifiedresources.StatusOnline, ParentName: "node-one",
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{Health: "PASSED", Wearout: -1, Temperature: 38, Collection: collected}},
	}
	diskProvider := &mockDiskHealthProvider{}
	diskProvider.On("GetHosts").Return([]*unifiedresources.HostView{
		newHostView("host-tower", "tower", "agent-tower", "tower", &unifiedresources.HostSensorMeta{
			SMART: []unifiedresources.HostSMARTMeta{
				{Device: "/dev/sda", Model: "WDC WD80EFAX", Health: "PASSED", Temperature: 58, Collection: silent},
				{Device: "/dev/sdb", Model: "WDC WD80EFAX", Health: "PASSED", Temperature: 36, Collection: collected},
			},
		}, nil, nil),
	})
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:           &mockStateProvider{state: models.StateSnapshot{}},
		UnifiedResourceProvider: &stubUnifiedResourceProvider{resources: resources},
		DiskHealthProvider:      diskProvider,
		ControlLevel:            ControlLevelReadOnly,
	})

	for _, tool := range []struct {
		name string
		args map[string]interface{}
		key  string
	}{
		{"pulse_metrics", map[string]interface{}{"type": "disks"}, "disks"},
		{"pulse_query", map[string]interface{}{"action": "list", "type": "physical-disks"}, "physical_disks"},
	} {
		t.Run(tool.name, func(t *testing.T) {
			result, err := executor.ExecuteTool(context.Background(), tool.name, tool.args)
			require.NoError(t, err)
			require.False(t, result.IsError, "%+v", result.Content)
			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &payload))
			var disks []PhysicalDiskSummary
			require.NoError(t, json.Unmarshal(payload[tool.key], &disks))
			byID := map[string]PhysicalDiskSummary{}
			for _, disk := range disks {
				byID[disk.ID] = disk
			}
			retained := byID["disk-standby"]
			require.Nil(t, retained.Temperature, "a retained reading is not the disk's temperature")
			require.NotNil(t, retained.LastKnownTemperature)
			require.Equal(t, 61, *retained.LastKnownTemperature)
			require.Equal(t, "disk is in standby", retained.LastKnownTemperatureReason)
			live := byID["disk-live"]
			require.NotNil(t, live.Temperature)
			require.Equal(t, 38, *live.Temperature)
			require.Nil(t, live.LastKnownTemperature)
			require.NotContains(t, string(payload[tool.key]), `"temperature":61`)
		})
	}

	t.Run("pulse_storage disk_health", func(t *testing.T) {
		result, err := executor.ExecuteTool(context.Background(), "pulse_storage", map[string]interface{}{"type": "disk_health"})
		require.NoError(t, err)
		require.False(t, result.IsError, "%+v", result.Content)
		var response DiskHealthResponse
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
		require.Len(t, response.Hosts, 1)
		require.Len(t, response.Hosts[0].SMART, 2)
		silentDisk, liveDisk := response.Hosts[0].SMART[0], response.Hosts[0].SMART[1]
		require.Zero(t, silentDisk.Temperature)
		require.Equal(t, 58, silentDisk.LastKnownTemperature)
		require.Equal(t, "host agent stopped reporting", silentDisk.LastKnownTemperatureReason)
		require.Equal(t, 36, liveDisk.Temperature)
		require.Zero(t, liveDisk.LastKnownTemperature)
		require.NotContains(t, result.Content[0].Text, `"temperature":58`)
	})
}

func TestDiskTemperatureFormatsRetainedReadingAsLastKnown(t *testing.T) {
	cases := []struct {
		temperature int
		collection  *diskinventory.CollectionStatus
		want        string
	}{
		{41, nil, "41C"},
		{41, &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")}, "41C"},
		{41, &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("smartctl", "disk is in standby")}, "last known 41C (disk is in standby)"},
		{41, &diskinventory.CollectionStatus{Temperature: diskinventory.Missing("smartctl", "")}, "last known 41C"},
		{0, &diskinventory.CollectionStatus{Temperature: diskinventory.Unavailable("smartctl", "disk is in standby")}, ""},
		{0, nil, ""},
	}
	for _, tc := range cases {
		split := SplitDiskTemperature(tc.temperature, tc.collection)
		require.Equal(t, tc.want, split.Format("C"), "%d %+v", tc.temperature, tc.collection)
		require.Equal(t, diskinventory.TemperatureCollected(tc.temperature, tc.collection), split.Collected > 0)
	}
}
