package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

// A UI-keyed datastore override must govern actual incidents, not just the
// threshold resolver. Exercise hysteresis and recurrence across SQLite reopen,
// including a same-named datastore on another PBS instance.
func TestPBSDatastoreOverrideLifecycleAcrossRestart(t *testing.T) {
	for _, legacySnapshot := range []bool{false, true} {
		name := "persisted aliases"
		if legacySnapshot {
			name = "legacy snapshot"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			start := func() *Manager {
				m := NewManagerWithDataDir(dir)
				t.Cleanup(m.Stop)
				m.EnableEventLog()
				if !m.activeStateAuthoritative.Load() {
					t.Fatal("SQLite active state is not authoritative")
				}
				m.UpdateConfig(AlertConfig{Enabled: true, ActivationState: ActivationActive,
					StorageDefault: HysteresisThreshold{Trigger: 95, Clear: 90},
					Overrides:      map[string]ThresholdConfig{"pbs-primary/backups": {Usage: &HysteresisThreshold{Trigger: 80, Clear: 70}}},
				})
				disableTestTimeThresholds(m)
				return m
			}
			storage := func(instance string, usage float64) models.Storage {
				return models.Storage{ID: instance + "-backups", AliasIDs: []string{instance + "/backups"}, Name: "backups", Instance: instance, Type: "pbs", Status: "online", Total: 1000, Used: int64(usage * 10), Free: int64(1000 - usage*10), Usage: usage}
			}
			observe := func(m *Manager, usage float64) {
				t.Helper()
				for range 5 {
					m.CheckStorage(storage("pbs-primary", usage))
					m.CheckStorage(storage("pbs-secondary", usage))
				}
				if testHasActiveAlert(t, m, canonicalMetricStateID("pbs-secondary-backups", "usage")) {
					t.Fatal("override leaked to another PBS instance")
				}
			}
			events := func(m *Manager, fired, resolved int) {
				t.Helper()
				for kind, want := range map[string]int{eventlog.TypeFired: fired, eventlog.TypeResolved: resolved} {
					if got := len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{kind}})); got != want {
						t.Fatalf("%s events = %d, want %d", kind, got, want)
					}
				}
			}
			id := canonicalMetricStateID("pbs-primary-backups", "usage")
			m := start()
			observe(m, 85)
			original := *testRequireActiveAlert(t, m, id)
			events(m, 1, 0)
			if legacySnapshot {
				m.mu.Lock()
				delete(m.activeAlerts[id].Metadata, storagePolicyAliasesKey)
				m.mu.Unlock()
				if err := m.SaveActiveAlerts(); err != nil {
					t.Fatal(err)
				}
			}
			m.Stop()

			m = start()
			observe(m, 75) // Below trigger, but not below the override's clear threshold.
			if got := testRequireActiveAlert(t, m, id); !got.StartTime.Equal(original.StartTime) {
				t.Fatal("restart replaced the firing incident")
			}
			events(m, 1, 0)
			observe(m, 65)
			if testHasActiveAlert(t, m, id) {
				t.Fatal("override recovery did not clear incident")
			}
			events(m, 1, 1)
			m.Stop()

			m = start()
			observe(m, 75)
			if testHasActiveAlert(t, m, id) {
				t.Fatal("resolved incident resurrected inside hysteresis band")
			}
			events(m, 1, 1)
			observe(m, 85)
			if got := testRequireActiveAlert(t, m, id); !got.StartTime.After(original.StartTime) {
				t.Fatal("refire reused original incident start")
			}
			events(m, 2, 1)
		})
	}
}

func TestStoragePolicyAliasesLegacyIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, instance, resource, datastore string
		want                                bool
	}{
		{"hyphenated names", "pbs-backup-east", "pbs-backup-east-daily-store", "daily-store", true},
		{"different instance", "pbs-backup-west", "pbs-backup-east-daily-store", "daily-store", false},
		{"not PBS", "backup-east", "backup-east-daily-store", "daily-store", false},
		{"missing datastore", "pbs-backup-east", "pbs-backup-east-", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := storagePolicyAliases(&Alert{Instance: tc.instance, ResourceID: tc.resource, ResourceName: tc.datastore})
			if tc.want {
				if len(got) != 1 || got[0] != tc.instance+"/"+tc.datastore {
					t.Fatalf("aliases = %v", got)
				}
			} else if len(got) != 0 {
				t.Fatalf("invented alias: %v", got)
			}
		})
	}
}
