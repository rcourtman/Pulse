package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

func TestVMwarePollerPollsConfiguredConnections(t *testing.T) {
	previous := vmware.IsFeatureEnabled()
	vmware.SetFeatureEnabled(true)
	t.Cleanup(func() { vmware.SetFeatureEnabled(previous) })

	mtp, persistence := newTestTenantPersistence(t)
	connection := config.NewVMwareVCenterInstance()
	connection.ID = "vc-1"
	connection.Name = "Lab VC"
	connection.Host = "vc.lab.local"
	connection.Username = "administrator@vsphere.local"
	connection.Password = "super-secret"

	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("SaveVMwareConfig() error = %v", err)
	}

	poller := NewVMwarePoller(mtp, 20*time.Millisecond)
	poller.newProvider = func(instance config.VMwareVCenterInstance) (vmwarePollerProvider, error) {
		return vmware.NewProvider(vmware.InventorySnapshot{
			ConnectionID:   instance.ID,
			ConnectionName: instance.Name,
			VCenterHost:    instance.Host,
			CollectedAt:    time.Now().UTC(),
			Hosts: []vmware.InventoryHost{{
				Host:            "host-101",
				Name:            "esxi-01.lab.local",
				ConnectionState: "CONNECTED",
				PowerState:      "POWERED_ON",
				HostUUID:        "uuid-host-1",
				RecentTasks: []vmware.InventoryTask{{
					Task:      "task-11",
					Name:      "Reconnect host",
					State:     "running",
					StartedAt: time.Now().UTC().Add(-time.Minute),
				}},
			}},
			VMs: []vmware.InventoryVM{{
				VM:            "vm-201",
				Name:          "app-01",
				PowerState:    "POWERED_ON",
				CPUCount:      2,
				MemorySizeMiB: 4096,
				RecentEvents: []vmware.InventoryEvent{{
					Event:     "event-201",
					Type:      "VmMessageEvent",
					Message:   "Snapshot completed successfully",
					User:      "vpxuser",
					CreatedAt: time.Now().UTC().Add(-2 * time.Minute),
				}},
			}},
			Datastores: []vmware.InventoryDatastore{{
				Datastore: "datastore-11",
				Name:      "nvme-primary",
				Type:      "VMFS",
				FreeSpace: 50,
				Capacity:  100,
			}},
		}), nil
	}
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)

	waitForCondition(t, 2*time.Second, func() bool {
		return len(poller.GetCurrentRecordsForOrg("default")) == 3
	}, "expected VMware poller to ingest projected records")

	records := poller.GetCurrentRecordsForOrg("default")
	if len(records) != 3 {
		t.Fatalf("expected 3 VMware records, got %d", len(records))
	}
	gotSourceIDs := make([]string, 0, len(records))
	for _, record := range records {
		gotSourceIDs = append(gotSourceIDs, record.SourceID)
	}
	wantSourceIDs := []string{
		"vc-1:host:host-101",
		"vc-1:vm:vm-201",
		"vc-1:datastore:datastore-11",
	}
	for i, want := range wantSourceIDs {
		if gotSourceIDs[i] != want {
			t.Fatalf("record %d sourceID = %q, want %q", i, gotSourceIDs[i], want)
		}
	}

	ownedSources := poller.SnapshotOwnedSourcesForOrg("default")
	if len(ownedSources) != 1 || ownedSources[0] != unifiedresources.SourceVMware {
		t.Fatalf("owned sources = %#v, want [%q]", ownedSources, unifiedresources.SourceVMware)
	}

	changes := poller.GetCurrentChangesForOrg("default")
	if len(changes) != 2 {
		t.Fatalf("expected 2 VMware activity changes, got %d", len(changes))
	}
	if changes[0].Kind != unifiedresources.ChangeActivity {
		t.Fatalf("latest VMware change kind = %q, want %q", changes[0].Kind, unifiedresources.ChangeActivity)
	}
	if changes[0].SourceAdapter != unifiedresources.AdapterVMware {
		t.Fatalf("latest VMware change source adapter = %q, want %q", changes[0].SourceAdapter, unifiedresources.AdapterVMware)
	}
	if supplemental := poller.SupplementalChanges(nil, "default"); len(supplemental) != 2 {
		t.Fatalf("expected SupplementalChanges to mirror cached VMware changes, got %d", len(supplemental))
	}
}

func TestVMwarePollerFeatureFlagGate(t *testing.T) {
	previous := vmware.IsFeatureEnabled()
	vmware.SetFeatureEnabled(false)
	t.Cleanup(func() { vmware.SetFeatureEnabled(previous) })

	mtp, persistence := newTestTenantPersistence(t)
	connection := config.NewVMwareVCenterInstance()
	connection.ID = "vc-flag-off"
	connection.Host = "vc.disabled.local"
	connection.Username = "user"
	connection.Password = "secret"

	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("SaveVMwareConfig() error = %v", err)
	}

	poller := NewVMwarePoller(mtp, 20*time.Millisecond)
	initialStopped := poller.stopped
	poller.Start(context.Background())

	if poller.cancel != nil {
		t.Fatal("expected Start() to be a no-op with feature flag disabled")
	}
	if poller.stopped != initialStopped {
		t.Fatal("expected stopped channel to remain unchanged when Start() is gated")
	}
	if records := poller.GetCurrentRecordsForOrg("default"); len(records) != 0 {
		t.Fatalf("expected no VMware records when feature flag is disabled, got %d", len(records))
	}
}
