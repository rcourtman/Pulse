package monitoring

import (
	"context"
	"errors"
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

	waitForCondition(t, 2*time.Second, func() bool {
		summary, ok := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
		return ok && summary.Poll != nil && summary.Poll.LastSuccessAt != nil && summary.Observed != nil
	}, "expected VMware poller to publish connection summary")

	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.IntervalSeconds != 60 {
		t.Fatalf("expected VMware poll interval 60, got %+v", summary.Poll)
	}
	if summary.Observed == nil || summary.Observed.Hosts != 1 || summary.Observed.VMs != 1 || summary.Observed.Datastores != 1 {
		t.Fatalf("unexpected observed summary: %+v", summary.Observed)
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

func TestVMwarePollerConnectionSummariesCaptureFailuresWithoutClearingObservedSummary(t *testing.T) {
	poller := NewVMwarePoller(nil, time.Minute)
	connection := config.NewVMwareVCenterInstance()
	connection.ID = "vc-1"
	connection.Host = "vc.lab.local"
	connection.Username = "administrator@vsphere.local"
	connection.Password = "secret"

	successAt := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
	failureAt := successAt.Add(5 * time.Minute)
	poller.RecordConnectionTestSuccess("default", connection.ID, &vmware.InventorySummary{
		Hosts:      2,
		VMs:        14,
		Datastores: 3,
		VIRelease:  "8.0.3",
	}, successAt)
	poller.RecordConnectionTestFailure("default", connection.ID, &vmware.ConnectionError{
		Category: "permission",
		Message:  "VMware permissions are insufficient for vmware performance metrics",
	}, failureAt)

	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastError == nil {
		t.Fatalf("expected poll failure summary, got %+v", summary.Poll)
	}
	if summary.Poll.ConsecutiveFailures != 1 || summary.Poll.LastError.Category != "permission" {
		t.Fatalf("unexpected poll failure details: %+v", summary.Poll)
	}
	if summary.Observed == nil || summary.Observed.VMs != 14 || summary.Observed.VIRelease != "8.0.3" {
		t.Fatalf("expected observed summary to be preserved after failure, got %+v", summary.Observed)
	}
}

type failingVMwarePollerProvider struct {
	err error
}

func (p *failingVMwarePollerProvider) Refresh(context.Context) error            { return p.err }
func (p *failingVMwarePollerProvider) Records() []unifiedresources.IngestRecord { return nil }
func (p *failingVMwarePollerProvider) ActivityChanges() []unifiedresources.ResourceChange {
	return nil
}
func (p *failingVMwarePollerProvider) Snapshot() *vmware.InventorySnapshot { return nil }
func (p *failingVMwarePollerProvider) Close()                              {}

func TestVMwarePollerLiveFailureKeepsCachedRecords(t *testing.T) {
	previous := vmware.IsFeatureEnabled()
	vmware.SetFeatureEnabled(true)
	t.Cleanup(func() { vmware.SetFeatureEnabled(previous) })

	mtp, persistence := newTestTenantPersistence(t)
	connection := config.NewVMwareVCenterInstance()
	connection.ID = "vc-live-fail"
	connection.Host = "vc.fail.local"
	connection.Username = "administrator@vsphere.local"
	connection.Password = "secret"
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("SaveVMwareConfig() error = %v", err)
	}

	providerCalls := 0
	poller := NewVMwarePoller(mtp, 20*time.Millisecond)
	poller.newProvider = func(instance config.VMwareVCenterInstance) (vmwarePollerProvider, error) {
		providerCalls++
		if providerCalls == 1 {
			return vmware.NewProvider(vmware.InventorySnapshot{
				ConnectionID: instance.ID,
				CollectedAt:  time.Now().UTC(),
				VIRelease:    "8.0.3",
				Hosts:        []vmware.InventoryHost{{Host: "host-1", Name: "esxi-01"}},
			}), nil
		}
		return &failingVMwarePollerProvider{
			err: errors.New("refresh vmware inventory: VMware permissions are insufficient for vmware performance metrics"),
		}, nil
	}
	poller.syncConnections()
	poller.pollAll(context.Background())

	initialRecords := poller.GetCurrentRecordsForOrg("default")
	if len(initialRecords) != 1 {
		t.Fatalf("expected initial cached VMware records, got %d", len(initialRecords))
	}

	poller.providersByOrg["default"][connection.ID] = &failingVMwarePollerProvider{
		err: &vmware.ConnectionError{Category: "permission", Message: "VMware permissions are insufficient for vmware performance metrics"},
	}
	poller.pollAll(context.Background())

	recordsAfterFailure := poller.GetCurrentRecordsForOrg("default")
	if len(recordsAfterFailure) != 1 {
		t.Fatalf("expected cached VMware records to survive live failure, got %d", len(recordsAfterFailure))
	}
	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastError == nil || summary.Poll.LastError.Category != "permission" {
		t.Fatalf("expected permission poll error after live failure, got %+v", summary.Poll)
	}
}
