package models

import "testing"

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

	host, changed := state.ExpireHostTelemetry("storage-host")
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
