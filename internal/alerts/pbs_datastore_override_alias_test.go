package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Overrides written by the thresholds UI key PBS datastores by the canonical
// "<instance-id>/<name>" resource ID, while CheckStorage evaluates the legacy
// "<instance-id>-<name>" storage model. The alias carried by the conversion
// must bridge the two so a UI-written override actually applies (#1591).
func TestResolveStorageThresholds_HonorsCanonicalPBSDatastoreAlias(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	m.UpdateConfig(AlertConfig{
		Enabled:        true,
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides: map[string]ThresholdConfig{
			"pbs-pbs-docker/main": {Usage: &HysteresisThreshold{Trigger: 95, Clear: 90}},
		},
	})

	storage := models.Storage{
		ID:       "pbs-pbs-docker-main",
		AliasIDs: []string{"pbs-pbs-docker/main"},
		Name:     "main",
		Type:     "pbs",
		Instance: "pbs-pbs-docker",
	}

	m.mu.RLock()
	thresholds := m.resolveStorageThresholdsNoLock(storage)
	m.mu.RUnlock()

	if thresholds.Usage == nil || thresholds.Usage.Trigger != 95 {
		t.Fatalf("expected canonical-keyed override to apply via alias, got %+v", thresholds.Usage)
	}
}
