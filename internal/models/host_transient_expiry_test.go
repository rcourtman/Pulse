package models

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

func TestExpireHostTelemetryClearsTransientStorageOperationsOnly(t *testing.T) {
	state := NewState()
	state.UpsertHost(Host{
		ID:     "storage-host",
		Status: "online",
		RAID: []HostRAIDArray{{
			Device:         "/dev/md127",
			State:          "degraded",
			FailedDevices:  1,
			Operation:      "recovery",
			RebuildPercent: 45,
			RebuildSpeed:   "90M/sec",
		}},
		Unraid: &HostUnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   "check",
			SyncProgress: 72,
			NumDisabled:  1,
		},
	})

	host, changed := state.ExpireHostTelemetry("storage-host", time.Time{})
	if !changed {
		t.Fatal("expected host telemetry to change")
	}
	if host.Status != "offline" {
		t.Fatalf("status = %q, want offline", host.Status)
	}
	if host.Unraid == nil || host.Unraid.SyncAction != "" || host.Unraid.SyncProgress != 0 {
		t.Fatalf("Unraid transient state not cleared: %+v", host.Unraid)
	}
	if host.Unraid.NumDisabled != 1 || host.Unraid.ArrayState != "STARTED" {
		t.Fatalf("Unraid last-known topology was lost: %+v", host.Unraid)
	}
	if len(host.RAID) != 1 ||
		host.RAID[0].Operation != "" ||
		host.RAID[0].RebuildPercent != 0 ||
		host.RAID[0].RebuildSpeed != "" {
		t.Fatalf("RAID transient state not cleared: %+v", host.RAID)
	}
	if host.RAID[0].FailedDevices != 1 || host.RAID[0].State != "degraded" {
		t.Fatalf("RAID last-known health was lost: %+v", host.RAID[0])
	}
}

func TestExpireHostTelemetryMarksSMARTReadingsNoLongerCollected(t *testing.T) {
	state := NewState()
	judged := time.Now().Add(-time.Hour)
	state.UpsertHost(Host{
		ID:       "agent",
		Status:   "online",
		LastSeen: judged,
		Sensors: HostSensorSummary{SMART: []HostDiskSMART{
			{
				Device: "sda", Serial: "CURRENT", Temperature: 41, Health: "PASSED",
				IO: &DiskIO{Device: "sda", ReadBytes: 1000},
				Collection: &diskinventory.CollectionStatus{
					Serial:      diskinventory.Available("smartctl"),
					Temperature: diskinventory.Available("smartctl"),
					IO:          diskinventory.Available("kernel_diskstats"),
				},
			},
			// Agents before collection provenance report only what they collected.
			{Device: "sdb", Serial: "LEGACY", Temperature: 38, IO: &DiskIO{Device: "sdb"}},
			{
				Device: "sdc", Serial: "ASLEEP", Standby: true,
				Collection: &diskinventory.CollectionStatus{
					Temperature: diskinventory.Unavailable("smartctl", "disk is in standby"),
					IO:          diskinventory.Unsupported("controller", "per-member counters unavailable"),
				},
			},
		}},
	})

	host, changed := state.ExpireHostTelemetry("agent", judged)
	if !changed {
		t.Fatal("expected host telemetry to change")
	}
	stopped := diskinventory.Unavailable("smartctl", "host agent stopped reporting")
	current := host.Sensors.SMART[0]
	if current.Temperature != 41 || current.IO == nil || current.IO.ReadBytes != 1000 {
		t.Fatalf("last-known SMART readings were dropped: %+v", current)
	}
	if current.Collection.Temperature != stopped ||
		current.Collection.IO != diskinventory.Unavailable("kernel_diskstats", "host agent stopped reporting") {
		t.Fatalf("expired readings still claim to be collected: %+v", current.Collection)
	}
	if current.Collection.Serial != diskinventory.Available("smartctl") {
		t.Fatalf("disk identity evidence must stay last-known: %+v", current.Collection.Serial)
	}
	legacy := host.Sensors.SMART[1]
	if legacy.Temperature != 38 || legacy.Collection == nil ||
		legacy.Collection.Temperature.State != diskinventory.FieldUnavailable ||
		legacy.Collection.IO.State != diskinventory.FieldUnavailable {
		t.Fatalf("provenance-less readings not expired: %+v", legacy)
	}
	asleep := host.Sensors.SMART[2].Collection
	if asleep.Temperature != diskinventory.Unavailable("smartctl", "disk is in standby") ||
		asleep.IO.State != diskinventory.FieldUnsupported {
		t.Fatalf("readings that were already not collected must keep their state: %+v", asleep)
	}

	if _, changed := state.ExpireHostTelemetry("agent", judged); changed {
		t.Fatal("re-evaluating an expired host must not report a change")
	}
	stored, _ := state.GetHost("agent")
	if stored.Sensors.SMART[0].Collection.Temperature != stopped {
		t.Fatalf("expiry was not kept in state: %+v", stored.Sensors.SMART[0].Collection)
	}
}

// Expiry is judged on a host snapshot. A report accepted before the expiry
// takes the state lock must not be expired on the strength of that judgement.
func TestExpireHostTelemetryLeavesHostThatReportedSinceJudgement(t *testing.T) {
	state := NewState()
	judged := time.Now().Add(-time.Hour)
	fresh := Host{
		ID: "agent", Status: "online", LastSeen: time.Now(),
		Sensors: HostSensorSummary{SMART: []HostDiskSMART{{
			Device: "sda", Temperature: 41,
			Collection: &diskinventory.CollectionStatus{Temperature: diskinventory.Available("smartctl")},
		}}},
	}
	state.UpsertHost(fresh)

	if host, changed := state.ExpireHostTelemetry("agent", judged); changed || host.ID != "" {
		t.Fatalf("a host that reported after the stale judgement was expired: %+v changed=%v", host, changed)
	}
	stored, _ := state.GetHost("agent")
	if stored.Status != "online" || stored.Sensors.SMART[0].Collection.Temperature.State != diskinventory.FieldAvailable {
		t.Fatalf("fresh report was altered: %+v", stored)
	}
}
