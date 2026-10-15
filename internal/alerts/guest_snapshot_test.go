package alerts

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestGuestSnapshotResourceTypeUsesCanonicalSystemContainer(t *testing.T) {
	containerSnapshot := guestSnapshotFromContainer(models.Container{})
	if got := containerSnapshot.resourceType(); got != "system-container" {
		t.Fatalf("expected container resourceType to be system-container, got %q", got)
	}

	vmSnapshot := guestSnapshotFromVM(models.VM{})
	if got := vmSnapshot.resourceType(); got != "vm" {
		t.Fatalf("expected VM resourceType to be vm, got %q", got)
	}
}

func TestGuestSnapshotObservationAvailability(t *testing.T) {
	for _, test := range []struct {
		name       string
		memory     models.Memory
		disk       models.Disk
		reason     string
		wantMemory bool
		wantDisk   bool
	}{
		{name: "legacy measured zero", memory: models.Memory{}, disk: models.Disk{}},
		{name: "current independent read", memory: models.Memory{Usage: 96, Observation: models.MemoryObservation{State: "current", Source: "agent"}}, disk: models.Disk{Usage: 98}},
		{name: "retained memory only", memory: models.Memory{Usage: 96, Observation: models.MemoryObservation{State: "last-known"}}, disk: models.Disk{Usage: 98}, wantMemory: true},
		{name: "unavailable numeric placeholder", memory: models.Memory{Usage: 0, Observation: models.MemoryObservation{State: "unavailable"}}, wantMemory: true},
		{name: "unknown observation version", memory: models.Memory{Usage: 96, Observation: models.MemoryObservation{State: "future-version"}}, wantMemory: true},
		{name: "explicit unavailable overrides current", memory: models.Memory{Usage: 96, UsageUnavailable: true, Observation: models.MemoryObservation{State: "current"}}, wantMemory: true},
		{name: "retained filesystem only", memory: models.Memory{Usage: 96}, disk: models.Disk{Usage: 98}, reason: "prev-vm-locked", wantDisk: true},
		{name: "failed empty filesystem inventory", memory: models.Memory{Usage: 96}, disk: models.Disk{Usage: -1}, reason: "agent-error", wantDisk: true},
		{name: "legacy unavailable aggregate", memory: models.Memory{Usage: 96}, disk: models.Disk{Usage: -1}, wantDisk: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A deferred QGA session does not by itself make independent
			// selected memory or Pulse-agent filesystems unavailable.
			vm := models.VM{Memory: test.memory, Disk: test.disk, DiskStatusReason: test.reason, GuestAgentStatus: "deferred"}
			for _, guest := range []any{vm, &vm} {
				snapshot, ok := extractGuestSnapshot(guest)
				if !ok || snapshot.MemoryUnavailable != test.wantMemory || snapshot.DiskUnavailable != test.wantDisk {
					t.Fatalf("observation flags lost: %+v", snapshot)
				}
				metrics := snapshot.metrics()
				if metrics.MemoryUnavailable != test.wantMemory || metrics.DiskUnavailable != test.wantDisk || metrics.MemUsage != test.memory.Usage || metrics.DiskUsage != test.disk.Usage {
					t.Fatal("filter projection lost availability or rewrote retained values")
				}
			}
			ct := models.Container{Memory: test.memory, Disk: test.disk}
			for _, guest := range []any{ct, &ct} {
				snapshot, _ := extractGuestSnapshot(guest)
				if snapshot.MemoryUnavailable != test.wantMemory || snapshot.DiskUnavailable != (test.disk.Usage < 0) {
					t.Fatal("LXC observation availability changed with an unrelated VM disk failure")
				}
			}
		})
	}
}

func TestGuestAlertRetainedDiskStillHonoursExplicitOverride(t *testing.T) {
	for _, override := range []ThresholdConfig{{Disabled: true}, {Disk: &HysteresisThreshold{Trigger: -1}}} {
		m := guestObservationManager(t)
		vm := guestObservationVM()
		vm.Memory.Usage = 10
		m.CheckGuest(vm, "site")
		if len(m.GetActiveAlerts()) != 2 {
			t.Fatal("expected two current filesystem incidents")
		}
		_, keySource, key := guestDiskIdentity(vm.Disks[0], 0)
		cfg := m.GetConfig()
		cfg.Overrides[guestDiskOverrideKey(vm, vm.ID, keySource)] = override
		m.UpdateConfig(cfg)
		vm.DiskStatusReason = "prev-vm-locked"
		m.CheckGuest(vm, "site")
		active := m.GetActiveAlerts()
		if len(active) != 1 || active[0].ResourceID == vm.ID+"-disk-"+key {
			t.Fatalf("explicit filesystem disablement was lost or crossed disk identity: %+v", active)
		}
	}
}

func TestGuestSnapshotUsesCanonicalProxmoxCPUPercent(t *testing.T) {
	for _, tc := range []struct {
		name string
		cpu  float64
		want float64
	}{
		{name: "zero", cpu: 0, want: 0},
		{name: "fractional-capacity", cpu: 0.0058, want: 0.58},
		{name: "full-capacity", cpu: 1, want: 100},
		{name: "already-percent", cpu: 12.5, want: 12.5},
	} {
		t.Run("vm/"+tc.name, func(t *testing.T) {
			snapshot := guestSnapshotFromVM(models.VM{CPU: tc.cpu})
			if snapshot.CPUPercent != tc.want {
				t.Fatalf("VM CPU percent = %v, want %v", snapshot.CPUPercent, tc.want)
			}
		})
		t.Run("lxc/"+tc.name, func(t *testing.T) {
			snapshot := guestSnapshotFromContainer(models.Container{CPU: tc.cpu})
			if snapshot.CPUPercent != tc.want {
				t.Fatalf("LXC CPU percent = %v, want %v", snapshot.CPUPercent, tc.want)
			}
		})
	}
}

func TestParsePulseTagsRecognizesGuestRuntimeControls(t *testing.T) {
	settings := parsePulseTags([]string{
		" PULSE-NO-ALERTS ",
		"pulse-monitor-only",
		"pulse-relaxed",
		"unrelated",
	})

	if !settings.Suppress || !settings.MonitorOnly || !settings.Relaxed {
		t.Fatalf("parsePulseTags() = %+v, want all guest runtime controls enabled", settings)
	}
}

func TestEmptyGuestSnapshot_NormalizesCollections(t *testing.T) {
	snapshot := emptyGuestSnapshot()
	if snapshot.Disks == nil || snapshot.Tags == nil {
		t.Fatalf("expected empty guest snapshot slices to be initialized, got %#v", snapshot)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal guest snapshot: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode guest snapshot: %v", err)
	}

	for _, key := range []string{"Disks", "Tags"} {
		values, ok := payload[key].([]any)
		if !ok {
			t.Fatalf("expected %s to serialize as an array, got %T (%v)", key, payload[key], payload[key])
		}
		if len(values) != 0 {
			t.Fatalf("expected %s to serialize as an empty array, got %v", key, values)
		}
	}
}

func TestExtractGuestSnapshot_UnknownTypeReturnsCanonicalEmptySnapshot(t *testing.T) {
	snapshot, ok := extractGuestSnapshot(struct{}{})
	if ok {
		t.Fatal("expected unknown guest type extraction to fail")
	}
	if snapshot.Disks == nil || snapshot.Tags == nil {
		t.Fatalf("expected canonical empty guest snapshot on unknown type, got %#v", snapshot)
	}
}

func TestGuestIORateMetricsDistinguishValidZeroFromUnknown(t *testing.T) {
	validZero := guestSnapshotFromVM(models.VM{
		IORateValidity: models.IORateValidity{
			Explicit:   true,
			DiskRead:   true,
			DiskWrite:  true,
			NetworkIn:  true,
			NetworkOut: true,
		},
	})
	diskRead, diskWrite, networkIn, networkOut := guestIORateMetrics(validZero)
	if diskRead == nil || diskRead.Value != 0 || diskWrite == nil || networkIn == nil || networkOut == nil {
		t.Fatalf("valid zero rates were not alert candidates: %+v %+v %+v %+v", diskRead, diskWrite, networkIn, networkOut)
	}

	unknown := guestSnapshotFromVM(models.VM{
		IORateValidity: models.IORateValidity{Explicit: true},
	})
	diskRead, diskWrite, networkIn, networkOut = guestIORateMetrics(unknown)
	if diskRead != nil || diskWrite != nil || networkIn != nil || networkOut != nil {
		t.Fatalf("unknown rates became alert candidates: %+v %+v %+v %+v", diskRead, diskWrite, networkIn, networkOut)
	}
}

// A filesystem alert's resource ID ("<guest>-disk-<key>") names nothing the
// resource registry can open, so it carries the guest it belongs to under
// guestResourceId: the ID the guest's own alerts are raised under. Both the
// canonical-spec path (VMs, containers) and the legacy checkMetric path (a
// guest of unknown kind) stamp it.
func TestGuestFilesystemAlertsNameOwningGuest(t *testing.T) {
	vm := guestObservationVM()
	legacy := guestSnapshotFromVM(vm)
	legacy.Kind = guestKindUnknown
	for name, guest := range map[string]any{"canonical": vm, "legacy": legacy} {
		t.Run(name, func(t *testing.T) {
			// Only the canonical guest raises a guest-level (memory) alert
			// beside its filesystems; it ties the stamp to that alert's ID.
			wantGuestLevel := name == "canonical"
			m := guestObservationManager(t)
			m.CheckGuest(guest, "site")
			var filesystems int
			var guestLevel bool
			for _, alert := range m.GetActiveAlerts() {
				if !strings.HasPrefix(alert.ResourceID, vm.ID+"-disk-") {
					guestLevel = guestLevel || alert.ResourceID == vm.ID
					continue
				}
				filesystems++
				if got := alert.Metadata["guestResourceId"]; got != vm.ID {
					t.Fatalf("filesystem alert %s guestResourceId = %v, want %q", alert.ResourceID, got, vm.ID)
				}
			}
			if filesystems != 2 || guestLevel != wantGuestLevel {
				t.Fatalf("expected two filesystem alerts (guest-level alert under %q: %v): %+v", vm.ID, wantGuestLevel, m.GetActiveAlerts())
			}
		})
	}
}

func snapshotPartialFixture(t *testing.T) (*Manager, models.GuestSnapshot, map[string]GuestLookup) {
	t.Helper()
	m := newTestManager(t)
	m.ClearActiveAlerts()
	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.SnapshotDefaults = SnapshotAlertConfig{Enabled: true, WarningDays: 7, CriticalDays: 14, WarningSizeGiB: 50, CriticalSizeGiB: 100}
	m.UpdateConfig(cfg)
	snapshot := models.GuestSnapshot{ID: "inst-node-100-weekly", Name: "weekly", Instance: "inst", Node: "node", VMID: 100, Type: "qemu", Time: time.Now().Add(-10 * 24 * time.Hour), SizeBytes: 1 << 30}
	guests := map[string]GuestLookup{"inst:node:100": {ResourceID: "inst:node:100", Name: "database", Instance: "inst", Node: "node", VMID: 100, Type: "qemu"}}
	return m, snapshot, guests
}

func requireSnapshotPartialAlert(t *testing.T, m *Manager, level AlertLevel) *Alert {
	t.Helper()
	active := m.GetActiveAlerts()
	if len(active) != 1 || active[0].Level != level {
		t.Fatalf("expected one %s snapshot condition, got %+v", level, active)
	}
	return active[0]
}

// No native snapshot, guest command, backup, clock change or database fault is
// induced. These are ordinary detector inputs, including the mapper's epoch.
func TestSnapshotPartialEvidenceSizeIndependentOfTime(t *testing.T) {
	for _, guestType := range []string{"qemu", "lxc"} {
		for _, clock := range []string{"missing", "epoch", "negative", "future"} {
			t.Run(guestType+"/"+clock, func(t *testing.T) {
				m, snapshot, guests := snapshotPartialFixture(t)
				snapshot.Type = guestType
				switch clock {
				case "missing":
					snapshot.Time = time.Time{}
				case "epoch":
					snapshot.Time = time.Unix(0, 0)
				case "negative":
					snapshot.Time = time.Unix(-1, 0)
				case "future":
					snapshot.Time = time.Now().Add(24 * time.Hour)
				}
				snapshot.SizeBytes = 120 << 30
				m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
				alert := requireSnapshotPartialAlert(t, m, AlertLevelCritical)
				if alert.Value != 120 || alert.Threshold != 100 || alert.Metadata["primaryMetric"] != "size" || !strings.Contains(alert.Message, "120.0 GiB") {
					t.Fatalf("valid size breach was not evaluated: %+v", alert)
				}
				for _, key := range []string{"snapshotAgeDays", "snapshotAgeHours", "snapshotCreatedAt", "snapshotAgeObservedAt", "thresholdDays"} {
					if _, exists := alert.Metadata[key]; exists {
						t.Errorf("unavailable age fabricated %s=%v", key, alert.Metadata[key])
					}
				}
				if !strings.Contains(alert.Message, "age was not re-evaluated") || (clock == "future" && !strings.Contains(alert.Message, "clocks")) {
					t.Fatalf("missing fixed age explanation: %s", alert.Message)
				}
				snapshot.SizeBytes = 1 << 30
				m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
				if len(m.GetActiveAlerts()) != 0 {
					t.Fatal("fresh size recovery did not clear size-only condition")
				}
			})
		}
	}
}

func TestSnapshotPartialEvidenceRetainsAgeAndAcceptsSizeChanges(t *testing.T) {
	m, snapshot, guests := snapshotPartialFixture(t)
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	before := requireSnapshotPartialAlert(t, m, AlertLevelWarning)
	m.mu.Lock()
	m.activeAlerts[before.CanonicalState].Acknowledged = true
	m.activeAlerts[before.CanonicalState].AckUser = "fixture-operator"
	m.mu.Unlock()

	snapshot.Time = time.Now().Add(24 * time.Hour)
	snapshot.SizeBytes = 120 << 30
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	mixed := requireSnapshotPartialAlert(t, m, AlertLevelCritical)
	if mixed.Metadata["primaryMetric"] != "size" || mixed.Metadata["snapshotAgeHeld"] != true || mixed.Metadata["snapshotSizeHeld"] != false || !strings.Contains(mixed.Message, "last known 10.0 days") {
		t.Fatalf("current size did not coexist with held age: %+v", mixed)
	}
	if mixed.Metadata["snapshotAgeDays"] != before.Metadata["snapshotAgeDays"] || mixed.Metadata["snapshotAgeObservedAt"] != before.Metadata["snapshotAgeObservedAt"] {
		t.Fatal("held age grew or its observation time was renewed")
	}
	if mixed.ID != before.ID || !mixed.StartTime.Equal(before.StartTime) || !mixed.Acknowledged || mixed.AckUser != "fixture-operator" {
		t.Fatal("partial read lost occurrence or acknowledgement")
	}

	snapshot.Time = time.Time{}
	snapshot.SizeBytes = 1 << 30
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	held := requireSnapshotPartialAlert(t, m, AlertLevelWarning)
	if held.Metadata["primaryMetric"] != "age" || !held.LastSeen.Equal(before.LastSeen) || held.Metadata["snapshotAgeDays"] != before.Metadata["snapshotAgeDays"] {
		t.Fatal("size recovery silently cleared or renewed the independent held age condition")
	}
	for range 2 {
		m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
		repeated := requireSnapshotPartialAlert(t, m, AlertLevelWarning)
		if !repeated.LastSeen.Equal(before.LastSeen) || !repeated.StartTime.Equal(before.StartTime) || repeated.Value != before.Value {
			t.Fatal("repeated missing age renewed or replaced the occurrence")
		}
	}
	snapshot.Time = time.Now().Add(-time.Hour)
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("complete current age and size recovery did not clear the condition")
	}
}

func TestSnapshotPartialEvidenceRetainsMissingSizeWithoutRenewingIt(t *testing.T) {
	m, snapshot, guests := snapshotPartialFixture(t)
	snapshot.Time = time.Now().Add(-time.Hour)
	snapshot.SizeBytes = 120 << 30
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	before := requireSnapshotPartialAlert(t, m, AlertLevelCritical)
	for _, size := range []int64{0, -1, 0} {
		snapshot.SizeBytes = size
		m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
		held := requireSnapshotPartialAlert(t, m, AlertLevelCritical)
		if held.Metadata["snapshotSizeHeld"] != true || held.Metadata["snapshotSizeObservedAt"] != before.Metadata["snapshotSizeObservedAt"] || !held.LastSeen.Equal(before.LastSeen) || !strings.Contains(held.Message, "last known 120.0 GiB") {
			t.Fatal("missing optional size cleared or renewed its previous breach")
		}
	}
	snapshot.SizeBytes = 1 << 30
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("positive complete size did not establish recovery")
	}
}

func TestSnapshotPartialEvidenceReplacementDoesNotInheritSize(t *testing.T) {
	m, snapshot, guests := snapshotPartialFixture(t)
	snapshot.SizeBytes = 120 << 30
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	requireSnapshotPartialAlert(t, m, AlertLevelCritical)
	snapshot.Time = time.Now().Add(-time.Hour) // Same name, newly-created snapshot.
	snapshot.SizeBytes = 0
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("a replacement snapshot inherited the retired snapshot's size")
	}
}

func TestSnapshotPartialEvidenceRestoredLegacyObservation(t *testing.T) {
	m, snapshot, guests := snapshotPartialFixture(t)
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	before := requireSnapshotPartialAlert(t, m, AlertLevelWarning)
	delete(before.Metadata, "snapshotAgeObservedAt")
	delete(before.Metadata, "snapshotSizeObservedAt")
	wire, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var restored Alert
	if err := json.Unmarshal(wire, &restored); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.activeAlerts[before.CanonicalState] = &restored
	m.mu.Unlock()
	snapshot.Time, snapshot.SizeBytes = time.Time{}, 0
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	held := requireSnapshotPartialAlert(t, m, AlertLevelWarning)
	if held.Metadata["snapshotAgeHeld"] != true || !held.LastSeen.Equal(before.LastSeen) || !strings.Contains(held.Message, "last known") {
		t.Fatal("legacy JSON metadata did not retain its original observation")
	}
}

func TestSnapshotPartialEvidencePolicyAndDeletionRemainAuthoritative(t *testing.T) {
	for _, action := range []string{"global-disable", "guest-disable", "snapshot-disable", "tag", "delete", "threshold-change"} {
		t.Run(action, func(t *testing.T) {
			m, snapshot, guests := snapshotPartialFixture(t)
			m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
			requireSnapshotPartialAlert(t, m, AlertLevelWarning)
			snapshot.Time, snapshot.SizeBytes = time.Time{}, 0
			cfg := m.GetConfig()
			switch action {
			case "global-disable":
				cfg.SnapshotDefaults.Enabled = false
			case "guest-disable":
				cfg.Overrides = map[string]ThresholdConfig{"inst:node:100": {Disabled: true}}
			case "snapshot-disable":
				cfg.Overrides = map[string]ThresholdConfig{"inst:node:100": {Snapshot: &SnapshotAlertConfig{Enabled: false}}}
			case "tag":
				guest := guests["inst:node:100"]
				guest.Tags = []string{"pulse-no-alerts"}
				guests["inst:node:100"] = guest
			case "threshold-change":
				cfg.SnapshotDefaults.WarningDays, cfg.SnapshotDefaults.CriticalDays = 30, 60
			}
			m.UpdateConfig(cfg)
			rows := []models.GuestSnapshot{snapshot}
			if action == "delete" {
				rows = nil
			}
			m.CheckSnapshotsForInstance("inst", rows, guests)
			if len(m.GetActiveAlerts()) != 0 {
				t.Fatal("held evidence overruled deliberate policy or authoritative deletion")
			}
		})
	}
}

func TestSnapshotPartialEvidenceScopeDoesNotTransferHeldMetrics(t *testing.T) {
	m, snapshot, guests := snapshotPartialFixture(t)
	m.CheckSnapshotsForInstance("inst", []models.GuestSnapshot{snapshot}, guests)
	original := requireSnapshotPartialAlert(t, m, AlertLevelWarning)
	peer := snapshot
	// Even a colliding legacy alias cannot lend evidence across connections.
	peer.Instance, peer.VMID = "other", 101
	peer.Time, peer.SizeBytes = time.Time{}, 0
	m.CheckSnapshotsForInstance("other", []models.GuestSnapshot{peer}, nil)
	remaining := requireSnapshotPartialAlert(t, m, AlertLevelWarning)
	if remaining.ID != original.ID {
		t.Fatal("another connection/name inherited held metrics or cleared the original")
	}
}
