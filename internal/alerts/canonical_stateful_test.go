package alerts

import (
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestCanonicalLifecycleCopiesValidatedAlertCorrelation(t *testing.T) {
	m := newTestManager(t)
	spec, err := buildCanonicalConnectivitySpec(
		"node-1",
		"Node 1",
		unifiedresources.ResourceType("node"),
		AlertLevelCritical,
		1,
		false,
	)
	if err != nil {
		t.Fatalf("build connectivity spec: %v", err)
	}
	correlation := NewSharedSystemAlertCorrelation(
		"pve:delly",
		AlertCorrelationRoleSupporting,
		"proxmox-node-membership",
	)

	_, ok := m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec:         spec,
		Evidence:     alertspecs.AlertEvidence{ObservedAt: time.Now(), Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: false}},
		AlertID:      canonicalConnectivityStateID("node-1"),
		AlertType:    "connectivity",
		ResourceID:   "node-1",
		ResourceName: "Node 1",
		Correlation:  correlation,
	})
	if !ok {
		t.Fatal("expected canonical lifecycle evaluation")
	}

	alert := testRequireActiveAlert(t, m, canonicalConnectivityStateID("node-1"))
	if alert.Correlation == nil || alert.Correlation.Key != "pve:delly" {
		t.Fatalf("canonical alert missing correlation: %+v", alert.Correlation)
	}
	correlation.Key = "pve:changed"
	if alert.Correlation.Key != "pve:delly" {
		t.Fatal("canonical alert retained the caller's mutable correlation pointer")
	}
}

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
		// Age the core's resolved ledger in step with the manager's records.
		m.mu.Lock()
		m.core.ShiftResolved(-10 * time.Minute)
		m.mu.Unlock()

		m.syncCanonicalHealthAssessmentAlert(params)

		testRequireActiveAlert(t, m, alertID)

		historyAfterReFire := len(m.historyManager.GetAllHistory(1000))
		if historyAfterReFire != 2 {
			t.Errorf("expected 2 history entries after re-fire past cooldown, got %d", historyAfterReFire)
		}
	})
}
