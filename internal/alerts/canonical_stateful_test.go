package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
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

func TestRAIDSpareCanonicalActivationAndRecovery(t *testing.T) {
	m := newEventLogManager(t)
	cfg := m.GetConfig()
	cfg.TimeThresholds = map[string]int{}
	m.UpdateConfig(cfg)
	host := models.Host{ID: "raid-host", Hostname: "linux-raid", Status: "online", RAID: []models.HostRAIDArray{{
		Device: "/dev/md1", Level: "raid5", State: "clean", TotalDevices: 5, ActiveDevices: 4, WorkingDevices: 5, SpareDevices: 1,
	}}}
	id := buildCanonicalStateID("agent:raid-host/raid:md1", "agent:raid-host/raid:md1-health")
	m.CheckHost(host)
	if testHasActiveAlert(t, m, id) || len(queryAlertEvents(t, m, eventlog.Filter{})) != 0 {
		t.Fatal("legacy healthy spare report activated an incident")
	}

	// An unreplaced required member is critical, even when the attached spare
	// makes the old total/active arithmetic look like a healthy spare tuple.
	host.RAID[0].RequiredDevices = 4
	host.RAID[0].TotalDevices = 4
	host.RAID[0].ActiveDevices = 3
	host.RAID[0].WorkingDevices = 4
	m.CheckHost(host)
	active := testRequireActiveAlert(t, m, id)
	if active.Level != AlertLevelCritical || active.Metadata["raidRequiredDevices"] != 4 {
		t.Fatalf("deficit alert lost required count or severity: %+v", active)
	}
	for _, operation := range []string{"check", "resync", "recovery"} {
		host.RAID[0].Operation = operation
		host.RAID[0].RebuildPercent = 25
		m.CheckHost(host)
		if got := testRequireActiveAlert(t, m, id); got.Level != AlertLevelCritical || !got.StartTime.Equal(active.StartTime) {
			t.Fatalf("maintenance/recovery masked or reopened required-member deficit: %+v", got)
		}
	}
	m.HandleHostTelemetryExpired(host)
	if got := testRequireActiveAlert(t, m, id); got.Level != AlertLevelCritical {
		t.Fatal("transient operation expiry removed a static member deficit")
	}

	host.RAID[0].TotalDevices = 5
	host.RAID[0].ActiveDevices = 4
	host.RAID[0].WorkingDevices = 5
	host.RAID[0].Operation = ""
	host.RAID[0].RebuildPercent = 0
	m.CheckHost(host)
	m.CheckHost(host)
	if testHasActiveAlert(t, m, id) {
		t.Fatal("full member recovery plus spare did not resolve the incident")
	}
	for kind, want := range map[string]int{eventlog.TypeFired: 1, eventlog.TypeResolved: 1} {
		if got := len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{kind}})); got != want {
			t.Fatalf("canonical %s events=%d, want %d", kind, got, want)
		}
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

// The shared assessment path also owns ZFS health, not only custom sensors.
func TestHealthAssessmentEscalationDelivery(t *testing.T) {
	m := newTestManager(t)
	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.FlappingEnabled = false
	cfg.Schedule.MaxAlertsHour = 2
	m.UpdateConfig(cfg)
	var delivered []AlertLevel
	m.SetAlertCallback(func(a *Alert) { delivered = append(delivered, a.Level) })
	resourceID := "storage-1/zfs-pool:tank"
	params := canonicalHealthAssessmentAlertParams{
		SpecID: resourceID + "-health", Signal: "zfs_pool", Codes: zfsPoolAssessmentCodes,
		AlertID:   buildCanonicalStateID(resourceID, resourceID+"-health"),
		AlertType: "zfs-pool-state", SpecResourceID: resourceID, ResourceID: resourceID,
		ResourceName: "tank", ResourceType: unifiedresources.ResourceTypeStorage,
	}
	observe := func(severity storagehealth.RiskLevel) {
		t.Helper()
		params.Reasons = []storagehealth.Reason{{Code: "zfs_pool_state", Severity: severity, Summary: "pool health"}}
		if _, ok := m.syncCanonicalHealthAssessmentAlert(params); !ok {
			t.Fatal("assessment rejected")
		}
	}
	observe(storagehealth.RiskWarning)
	observe(storagehealth.RiskWarning)
	observe(storagehealth.RiskCritical)
	observe(storagehealth.RiskCritical)
	observe(storagehealth.RiskWarning)
	if len(delivered) != 2 || delivered[0] != AlertLevelWarning || delivered[1] != AlertLevelCritical {
		t.Fatalf("initial/escalated deliveries = %v", delivered)
	}
	// The two admitted notifications exhaust the configured hourly budget;
	// oscillating back to critical must not bypass it.
	observe(storagehealth.RiskCritical)
	if len(delivered) != 2 {
		t.Fatalf("rate limit bypassed: %v", delivered)
	}
	if a := testRequireActiveAlert(t, m, params.AlertID); a.Level != AlertLevelCritical {
		t.Fatalf("rate-limited incident failed to update: %v", a.Level)
	}
}

// A single optimistic endurance reading must not close a still-worn disk's
// occurrence. Otherwise the next physical-disk poll fires and notifies again,
// as reported in #2112 after the host-agent SMART merge correction shipped.
func TestCheckDiskHealthWearoutFlappingDoesNotRepeatNotifications(t *testing.T) {
	m := newEventLogManager(t)
	m.SetAlertCallback(func(*Alert) {})
	disk := proxmox.Disk{
		DevPath: "/dev/sda", Model: "KINGSTON SA400", Serial: "stable-serial",
		Type: "ssd", Health: "PASSED", Wearout: 0,
	}
	check := func(wearout int) {
		disk.Wearout = wearout
		m.CheckDiskHealth("pve", "rocket", disk)
	}

	check(0)
	active := m.GetActiveAlerts()
	if len(active) != 1 || active[0].Type != "disk-wearout" {
		t.Fatalf("spent SSD did not open its wearout alert: %+v", active)
	}
	initialID := active[0].ID
	for range 12 {
		check(100) // intermittent healthy-looking reading
		if held := m.GetActiveAlerts(); len(held) != 1 || held[0].Value != 0 || held[0].Message != active[0].Message {
			t.Fatalf("unconfirmed recovery rewrote the worn-disk warning: %+v", held)
		}
		check(0) // unchanged worn drive on the next poll
	}
	active = m.GetActiveAlerts()
	if len(active) != 1 || active[0].ID != initialID || active[0].Value != 0 {
		t.Fatalf("flapping reading changed the worn disk occurrence: %+v", active)
	}
	if got := len(m.historyManager.GetAllHistory(100)); got != 1 {
		t.Fatalf("flapping produced %d history entries, want one", got)
	}
	if got := len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeNotificationDispatched}})); got != 1 {
		t.Fatalf("flapping produced %d dispatches, want one", got)
	}
	if got := len(m.GetRecentlyResolved()); got != 0 {
		t.Fatalf("flapping produced %d resolved occurrences, want none", got)
	}
	check(-1)
	if got := len(m.GetActiveAlerts()); got != 1 {
		t.Fatalf("unreported wearout incorrectly proved recovery: %d active", got)
	}

	// A genuinely sustained high reading still resolves; this is a recovery
	// confirmation, not a permanent wearout-alert latch.
	check(100)
	check(100)
	if got := len(m.GetActiveAlerts()); got != 1 {
		t.Fatalf("alert recovered before the third healthy reading: %d", got)
	}
	check(100)
	if got := len(m.GetActiveAlerts()); got != 0 {
		t.Fatalf("alert did not recover after three healthy readings: %d", got)
	}
	if got := len(m.GetRecentlyResolved()); got != 1 {
		t.Fatalf("sustained recovery produced %d resolved occurrences, want one", got)
	}
	check(0)
	if got := len(m.GetActiveAlerts()); got != 1 {
		t.Fatalf("new low-life reading after confirmed recovery did not alert: %d", got)
	}
	if got := len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeNotificationDispatched}})); got != 2 {
		t.Fatalf("genuine recurrence produced %d total dispatches, want two", got)
	}
}

// Replays obsolete PVE backup identities and timestamps. The live
// PBS inventory is deliberately evaluated after the production restore timer,
// matching a restart whose first successful backup check is still pending.
func TestRestoredBackupRequiresFreshEvaluation(t *testing.T) {
	for _, sqlite := range []bool{false, true} {
		name := "recovery-mirror"
		if sqlite {
			name = "sqlite-authority"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			now := time.Now().UTC()
			lastSeen := now.Add(-4 * 24 * time.Hour)
			var fixtures []*Alert
			for i, subject := range []string{"res-vm-legacy-1", "res-vm-legacy-2"} {
				resourceID := "backup-subject:" + subject
				specID := resourceID + "-backup-age"
				backupTime := now.Add(-94*24*time.Hour + time.Duration(i)*15*time.Minute)
				fixtures = append(fixtures, &Alert{
					ID: resourceID + "::" + specID, Type: "backup-age", Level: AlertLevelCritical,
					ResourceID: resourceID, ResourceName: fmt.Sprintf("backup-server-%d backup", i+1),
					CanonicalSpecID: specID, CanonicalKind: "posture-threshold",
					CanonicalState: resourceID + "::" + specID,
					Message:        "Backup via proxmox-pve is 90.0 days old", Value: 90, Threshold: 84,
					StartTime: backupTime.Add(84 * 24 * time.Hour), LastSeen: lastSeen,
					Metadata: map[string]interface{}{
						"source": "PVE", "providers": []string{"proxmox-pve"},
						"lastBackupTime": backupTime, "ageDays": float64(90), "thresholdDays": float64(84),
						"guestVmid": 0, "guestInstance": "", "guestNode": "", "guestType": "",
						"guestName": fmt.Sprintf("backup-server-%d", i+1), "orphaned": false,
						"resourceType": "vm", "rollupId": "res:" + subject[4:],
						"canonicalAlertKind": "posture-threshold", "canonicalSpecID": specID,
					},
				})
			}
			writeActiveRecoveryFixture(t, dir, fixtures)
			seed := NewManagerWithDataDir(dir)
			if sqlite {
				seed.EnableEventLog()
				if !seed.activeStateAuthoritative.Load() {
					t.Fatal("failed to establish SQLite authority")
				}
			}
			if err := seed.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			seed.Stop()

			m := NewManagerWithDataDir(dir)
			t.Cleanup(m.Stop)
			if sqlite {
				m.EnableEventLog()
			}
			m.UpdateConfig(AlertConfig{Enabled: true, ActivationState: ActivationActive,
				TimeThresholds: map[string]int{},
				BackupDefaults: BackupAlertConfig{Enabled: true, WarningDays: 70, CriticalDays: 84}})
			delivered := make(chan *Alert, 8)
			m.SetAlertCallback(func(a *Alert) { delivered <- a.Clone() })
			for _, a := range fixtures {
				if !testHasActiveAlert(t, m, a.ID) {
					t.Fatalf("the historical alert was not restored: %s", a.ID)
				}
			}

			// Exercise the real ten-second startup timer without changing source.
			select {
			case a := <-delivered:
				t.Errorf("restored stale backup warning dispatched before any fresh backup evaluation: id=%s lastSeen=%s lastBackupTime=%v",
					a.ID, a.LastSeen.Format(time.RFC3339), a.Metadata["lastBackupTime"])
			case <-time.After(12 * time.Second):
			}

			// The current PBS rollups use different resource identities.
			var rollups []recovery.ProtectionRollup
			guests := make(map[string]GuestLookup)
			byVMID := make(map[string][]GuestLookup)
			for i, resourceID := range []string{"vm-current-1", "vm-current-2"} {
				vmid := 101 + i
				node := fmt.Sprintf("pve%02d", i+1)
				guest := GuestLookup{Name: fmt.Sprintf("backup-server-%d", i+1), Instance: "pve01", Node: node,
					VMID: vmid, Type: "qemu", ResourceID: resourceID}
				guests[BuildGuestKey("pve01", node, vmid)] = guest
				byVMID[fmt.Sprint(vmid)] = []GuestLookup{guest}
				success := now.Add(-2*24*time.Hour + time.Duration(i)*15*time.Minute)
				rollups = append(rollups, recovery.ProtectionRollup{RollupID: "res:" + resourceID,
					SubjectRef: &recovery.ExternalRef{Type: "proxmox-vm", Namespace: "pve01", Name: guest.Name,
						ID: fmt.Sprintf("pve01:%s:%d", node, vmid), Class: node},
					LastSuccessAt: &success, LastOutcome: recovery.OutcomeSuccess,
					Providers: []recovery.Provider{recovery.ProviderProxmoxPBS}})
			}
			m.CheckBackups(rollups, guests, byVMID)
			for _, a := range fixtures {
				if testHasActiveAlert(t, m, a.ID) {
					t.Errorf("fresh current PBS inventory did not clear historical warning: %s", a.ID)
				}
			}
			select {
			case a := <-delivered:
				t.Fatalf("cleared historical backup emitted a notification: %s", a.ID)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func startupBackupConfig() AlertConfig {
	return AlertConfig{Enabled: true, ActivationState: ActivationActive,
		TimeThresholds: map[string]int{},
		BackupDefaults: BackupAlertConfig{Enabled: true, WarningDays: 70, CriticalDays: 84}}
}

// Seed through normal backup evaluation so revalidation must retain the same
// canonical identity, acknowledgement, occurrence, and notification receipt.
func restartOverdueBackup(t *testing.T, sqlite bool, edit func(*Alert)) (*Manager, []recovery.ProtectionRollup, *Alert) {
	t.Helper()
	dir := t.TempDir()
	success := time.Now().Add(-94 * 24 * time.Hour)
	rollups := []recovery.ProtectionRollup{{RollupID: "res:vm-startup-test",
		SubjectRef:    &recovery.ExternalRef{Type: "proxmox-vm", Namespace: "test", ID: "test:node:101"},
		LastSuccessAt: &success, LastOutcome: recovery.OutcomeSuccess,
		Providers: []recovery.Provider{recovery.ProviderProxmoxPBS}}}
	seed := NewManagerWithDataDir(dir)
	if sqlite {
		seed.EnableEventLog()
	}
	seed.UpdateConfig(startupBackupConfig())
	seed.CheckBackups(rollups, nil, nil)
	active := seed.GetActiveAlerts()
	if len(active) != 1 || active[0].Level != AlertLevelCritical {
		seed.Stop()
		t.Fatalf("expected one overdue backup incident, got %+v", active)
	}
	if edit != nil {
		seed.mu.Lock()
		current, _ := seed.getActiveAlertNoLock(active[0].ID)
		edit(current)
		seed.setActiveAlertNoLock(current.ID, current)
		seed.mu.Unlock()
	}
	if err := seed.SaveActiveAlerts(); err != nil {
		seed.Stop()
		t.Fatal(err)
	}
	seed.Stop()
	m := NewManagerWithDataDir(dir)
	t.Cleanup(m.Stop)
	if sqlite {
		m.EnableEventLog()
		if !m.activeStateAuthoritative.Load() {
			t.Fatal("failed to restore SQLite active authority")
		}
	}
	m.UpdateConfig(startupBackupConfig())
	restored := m.GetActiveAlerts()
	if len(restored) != 1 {
		t.Fatalf("expected retained overdue backup incident, got %+v", restored)
	}
	return m, rollups, restored[0].Clone()
}

func TestRestoredBackupRevalidationDeliversOverdueOccurrence(t *testing.T) {
	for _, sqlite := range []bool{false, true} {
		t.Run(fmt.Sprintf("sqlite=%t", sqlite), func(t *testing.T) {
			m, rollups, restored := restartOverdueBackup(t, sqlite, nil)
			delivered := make(chan *Alert, 4)
			m.SetAlertCallback(func(a *Alert) { delivered <- a })
			if !m.ShouldSuppressNotification(restored) {
				t.Fatal("restored snapshot can notify before a backup evaluation")
			}
			m.CheckBackups(rollups, nil, nil)
			select {
			case a := <-delivered:
				if a.ID != restored.ID || !a.StartTime.Equal(restored.StartTime) || !a.LastSeen.After(restored.LastSeen) {
					t.Fatalf("revalidation did not preserve the occurrence with fresh evidence: %+v", a)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("genuinely overdue backup did not notify after revalidation")
			}
			m.CheckBackups(rollups, nil, nil)
			select {
			case a := <-delivered:
				t.Fatalf("unchanged overdue occurrence notified twice: %s", a.ID)
			case <-time.After(100 * time.Millisecond):
			}
			if active := m.GetActiveAlerts(); len(active) != 1 || active[0].LastNotified == nil {
				t.Fatalf("revalidated notification receipt was not retained: %+v", active)
			}
		})
	}
}

func TestRestoredBackupWithoutValidEvidenceRetainsIncidentAndEscalation(t *testing.T) {
	m, rollups, restored := restartOverdueBackup(t, true, nil)
	delivered := make(chan *Alert, 4)
	m.SetAlertCallback(func(a *Alert) { delivered <- a })
	m.SetEscalateCallback(func(a *Alert, _ int) { delivered <- a })
	cfg := startupBackupConfig()
	cfg.Schedule.Escalation = EscalationConfig{Enabled: true, Levels: []EscalationLevel{{After: 5}}}
	m.UpdateConfig(cfg)
	spec, err := buildCanonicalPostureThresholdSpec(restored.CanonicalSpecID, restored.ResourceID,
		restored.ResourceName, "vm", "backup-age-days", 70, 84, "", 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, valid := m.evaluateCanonicalStatefulAlert(canonicalStatefulAlertParams{
		Spec: spec, AlertID: restored.ID, AlertType: "backup-age",
		Evidence: alertspecs.AlertEvidence{ObservedAt: time.Now()},
	}); valid {
		t.Fatal("invalid backup evidence was accepted")
	}
	m.mu.Lock()
	if m.dispatchAlert(restored, false) {
		m.mu.Unlock()
		t.Fatal("restored snapshot dispatched without valid evidence")
	}
	m.mu.Unlock()
	m.checkEscalations()
	if _, _, eligible := m.PrepareEscalationNotification(restored, 1); eligible {
		t.Fatal("restored backup snapshot bypassed revalidation through escalation")
	}
	diagnosis, found := m.DiagnoseAlertDelivery(restored.ID)
	if !found || diagnosis.Reason != alertDeliveryReasonBackupRevalidation {
		t.Fatalf("delivery diagnosis did not explain the startup hold: %+v", diagnosis)
	}
	active := m.GetActiveAlerts()
	if len(active) != 1 || !active[0].StartTime.Equal(restored.StartTime) || active[0].LastEscalation != 0 || active[0].LastNotified != nil {
		t.Fatalf("missing evidence mutated or resolved the retained incident: %+v", active)
	}
	select {
	case <-delivered:
		t.Fatal("missing evidence sent a firing or escalation notification")
	case <-time.After(100 * time.Millisecond):
	}
	// Once collection succeeds, neither the overdue notification nor the
	// previously held escalation level may be permanently lost.
	m.CheckBackups(rollups, nil, nil)
	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("overdue notification was lost after collection succeeded")
	}
	m.checkEscalations()
	if _, _, eligible := m.PrepareEscalationNotification(restored, 1); !eligible {
		t.Fatal("freshly revalidated backup cannot escalate")
	}
}

func TestRestoredBackupRevalidationRespectsAcknowledgementAndCooldown(t *testing.T) {
	for _, policy := range []string{"acknowledged", "cooldown", "first-notification-only", "snoozed"} {
		t.Run(policy, func(t *testing.T) {
			m, rollups, restored := restartOverdueBackup(t, false, func(a *Alert) {
				if policy == "acknowledged" {
					a.Acknowledged = true
					a.AckUser = "operator"
				} else if policy != "snoozed" {
					now := time.Now()
					a.LastNotified = &now
				}
			})
			if policy == "snoozed" {
				if err := m.SnoozeAlert(restored.ID, "operator", time.Now().Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			cfg := startupBackupConfig()
			if policy == "cooldown" {
				cfg.Schedule.Cooldown = 60
			}
			m.UpdateConfig(cfg)
			delivered := make(chan *Alert, 2)
			m.SetAlertCallback(func(a *Alert) { delivered <- a })
			m.CheckBackups(rollups, nil, nil)
			select {
			case a := <-delivered:
				t.Fatalf("revalidation bypassed %s: %+v", policy, a)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func TestRestoredBackupRevalidationPreservesQuietHoursReplay(t *testing.T) {
	m, rollups, restored := restartOverdueBackup(t, false, nil)
	cfg := startupBackupConfig()
	cfg.Schedule.QuietHours = QuietHours{Enabled: true, Start: "00:00", End: "23:59", Timezone: "UTC",
		Days: map[string]bool{"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
			"friday": true, "saturday": true, "sunday": true},
		Suppress: QuietHoursSuppression{Storage: true}}
	m.UpdateConfig(cfg)
	if !m.ShouldSuppressNotification(restored) || hasQuietHoursNotificationReplay(restored) {
		t.Fatal("startup hold queued stale backup evidence for quiet-hours replay")
	}
	// Fresh evidence downgrades this retained occurrence to a warning, which
	// normal quiet-hours policy defers. Critical backup-age alerts bypass it.
	warningBackupTime := time.Now().Add(-75 * 24 * time.Hour)
	rollups[0].LastSuccessAt = &warningBackupTime
	delivered := make(chan *Alert, 2)
	m.SetAlertCallback(func(a *Alert) { delivered <- a })
	m.CheckBackups(rollups, nil, nil)
	select {
	case a := <-delivered:
		if a.Level != AlertLevelWarning || !hasQuietHoursNotificationReplay(a) {
			t.Fatal("revalidated overdue backup lost normal quiet-hours replay metadata")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("revalidated backup did not enter normal quiet-hours delivery")
	}
}
