package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestStatefulAlertReFireCooldown(t *testing.T) {
	t.Run("re-fire within cooldown does not create duplicate history entry", func(t *testing.T) {
		m := newTestManager(t)

		specResourceID := "storage-1/zfs-pool:tank"
		alertID := buildCanonicalStateID(specResourceID, specResourceID+"-health")
		reasons := []storagehealth.Reason{
			{Code: "zfs_pool_state", Severity: storagehealth.RiskCritical, Summary: "ZFS pool tank is DEGRADED"},
		}

		params := canonicalHealthAssessmentAlertParams{
			SpecID:         specResourceID + "-health",
			Signal:         "zfs_pool",
			Codes:          zfsPoolAssessmentCodes,
			Reasons:        reasons,
			AlertID:        alertID,
			AlertType:      "zfs-pool-state",
			SpecResourceID: specResourceID,
			ResourceID:     specResourceID,
			ResourceName:   "tank",
			ResourceType:   unifiedresources.ResourceTypeStorage,
			Node:           "node-1",
			Instance:       "node-1",
			Metadata:       map[string]interface{}{"resourceType": "storage"},
		}

		m.syncCanonicalHealthAssessmentAlert(params)

		original := testRequireActiveAlert(t, m, alertID)
		originalStart := original.StartTime

		historyAfterFire := len(m.historyManager.GetAllHistory(1000))
		if historyAfterFire != 1 {
			t.Fatalf("expected 1 history entry after initial fire, got %d", historyAfterFire)
		}

		m.mu.Lock()
		m.clearAlertNoLock(alertID)
		m.mu.Unlock()

		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected alert to be cleared")
		}

		m.syncCanonicalHealthAssessmentAlert(params)

		reactivated := testRequireActiveAlert(t, m, alertID)

		historyAfterReFire := len(m.historyManager.GetAllHistory(1000))
		if historyAfterReFire != historyAfterFire {
			t.Errorf("expected %d history entries after re-fire (same as after initial fire), got %d", historyAfterFire, historyAfterReFire)
		}

		if !reactivated.StartTime.Equal(originalStart) {
			t.Errorf("expected reactivated alert to preserve original StartTime %v, got %v", originalStart, reactivated.StartTime)
		}
	})

	t.Run("re-fire after cooldown expiry creates new history entry", func(t *testing.T) {
		m := newTestManager(t)

		specResourceID := "storage-2/zfs-pool:data"
		alertID := buildCanonicalStateID(specResourceID, specResourceID+"-health")
		reasons := []storagehealth.Reason{
			{Code: "zfs_pool_state", Severity: storagehealth.RiskCritical, Summary: "ZFS pool data is FAULTED"},
		}

		params := canonicalHealthAssessmentAlertParams{
			SpecID:         specResourceID + "-health",
			Signal:         "zfs_pool",
			Codes:          zfsPoolAssessmentCodes,
			Reasons:        reasons,
			AlertID:        alertID,
			AlertType:      "zfs-pool-state",
			SpecResourceID: specResourceID,
			ResourceID:     specResourceID,
			ResourceName:   "data",
			ResourceType:   unifiedresources.ResourceTypeStorage,
			Node:           "node-2",
			Instance:       "node-2",
			Metadata:       map[string]interface{}{"resourceType": "storage"},
		}

		m.syncCanonicalHealthAssessmentAlert(params)

		historyAfterFire := len(m.historyManager.GetAllHistory(1000))
		if historyAfterFire != 1 {
			t.Fatalf("expected 1 history entry after initial fire, got %d", historyAfterFire)
		}

		m.mu.Lock()
		m.clearAlertNoLock(alertID)
		m.mu.Unlock()

		m.resolvedMutex.Lock()
		if resolved, ok := m.recentlyResolved[alertID]; ok && resolved != nil {
			resolved.ResolvedTime = time.Now().Add(-10 * time.Minute)
		}
		m.resolvedMutex.Unlock()

		m.syncCanonicalHealthAssessmentAlert(params)

		testRequireActiveAlert(t, m, alertID)

		historyAfterReFire := len(m.historyManager.GetAllHistory(1000))
		if historyAfterReFire != 2 {
			t.Errorf("expected 2 history entries after re-fire past cooldown, got %d", historyAfterReFire)
		}
	})
}
