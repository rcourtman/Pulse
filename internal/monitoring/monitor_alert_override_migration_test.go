package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestSyncUnifiedResourceAlertsPersistsCanonicalOverrideSuccession(t *testing.T) {
	const (
		oldID = "agent-535886018cb53055"
		newID = "agent-b9ed6d0e20e94eaf"
	)

	dataDir := t.TempDir()
	persistence := config.NewConfigPersistence(dataDir)
	manager := alerts.NewManagerWithDataDir(dataDir)
	t.Cleanup(manager.Stop)

	alertConfig := manager.GetConfig()
	alertConfig.Enabled = false
	alertConfig.Overrides = map[string]alerts.ThresholdConfig{
		oldID: {
			Memory: &alerts.HysteresisThreshold{Trigger: 95, Clear: 90},
		},
	}
	manager.UpdateConfig(alertConfig)
	if err := persistence.SaveAlertConfig(manager.GetConfig()); err != nil {
		t.Fatalf("seed legacy alert config: %v", err)
	}

	monitor := &Monitor{
		alertManager:  manager,
		configPersist: persistence,
		state:         models.NewState(),
	}
	monitor.syncUnifiedResourceAlertsToState([]unifiedresources.Resource{{
		ID:                     newID,
		Type:                   unifiedresources.ResourceTypeAgent,
		SupersededCanonicalIDs: []string{oldID},
	}})

	inMemory := manager.GetConfig()
	if _, exists := inMemory.Overrides[oldID]; exists {
		t.Fatalf("in-memory override remained under superseded identity %s", oldID)
	}
	if override := inMemory.Overrides[newID]; override.Memory == nil || override.Memory.Trigger != 95 {
		t.Fatalf("in-memory override missing under canonical identity %s: %+v", newID, override)
	}

	reloaded, err := config.NewConfigPersistence(dataDir).LoadAlertConfig()
	if err != nil {
		t.Fatalf("reload migrated alert config: %v", err)
	}
	if _, exists := reloaded.Overrides[oldID]; exists {
		t.Fatalf("persisted override remained under superseded identity %s", oldID)
	}
	if override := reloaded.Overrides[newID]; override.Memory == nil || override.Memory.Trigger != 95 {
		t.Fatalf("reloaded override missing under canonical identity %s: %+v", newID, override)
	}
}
