package alerts

import (
	"encoding/json"
	"testing"

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
