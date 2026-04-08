package monitoring

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

func TestVMwarePollerSupplementalInventoryReadyAtUsesPersistedActiveConnections(t *testing.T) {
	mtp, persistence := newTestTenantPersistence(t)

	connection := config.NewVMwareVCenterInstance()
	connection.ID = "vc-ready"
	connection.Host = "vc-ready.lab.local"
	connection.Username = "administrator@vsphere.local"
	connection.Password = "secret"
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("SaveVMwareConfig() error = %v", err)
	}

	poller := NewVMwarePoller(mtp, time.Second)

	if readyAt, settled := poller.SupplementalInventoryReadyAt(nil, "default"); settled || !readyAt.IsZero() {
		t.Fatalf("SupplementalInventoryReadyAt() before any attempt = (%v, %t), want (zero, false)", readyAt, settled)
	}

	attemptedAt := time.Now().UTC()
	poller.RecordConnectionTestSuccess("default", connection.ID, &vmware.InventorySummary{}, attemptedAt)

	readyAt, settled := poller.SupplementalInventoryReadyAt(nil, "default")
	if !settled {
		t.Fatal("expected readiness to settle after the first recorded attempt")
	}
	if !readyAt.Equal(attemptedAt) {
		t.Fatalf("SupplementalInventoryReadyAt() = %v, want %v", readyAt, attemptedAt)
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

func TestVMwarePollerPublishesDegradedObservedSummaryOnPartialSuccess(t *testing.T) {
	poller := NewVMwarePoller(nil, time.Minute)
	connection := config.NewVMwareVCenterInstance()
	connection.ID = "vc-partial"
	connection.Host = "vc.partial.local"
	connection.Username = "administrator@vsphere.local"
	connection.Password = "secret"

	collectedAt := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	poller.newProvider = func(instance config.VMwareVCenterInstance) (vmwarePollerProvider, error) {
		return vmware.NewProvider(vmware.InventorySnapshot{
			ConnectionID: instance.ID,
			CollectedAt:  collectedAt,
			VIRelease:    "8.0.3",
			Hosts:        []vmware.InventoryHost{{Host: "host-101", Name: "esxi-01"}},
			EnrichmentIssues: []vmware.InventoryEnrichmentIssue{
				{Stage: "signals", EntityType: "host", EntityID: "host-101", Category: "permission", Message: "VMware permissions are insufficient for HostSystem overall status"},
				{Stage: "signals", EntityType: "vm", EntityID: "vm-201", Category: "permission", Message: "VMware permissions are insufficient for HostSystem overall status"},
				{Stage: "topology", EntityType: "vm", EntityID: "vm-201", Category: "unavailable", Message: "VMware vm guest identity is temporarily unavailable"},
			},
		}), nil
	}
	poller.providersByOrg["default"] = map[string]vmwarePollerProvider{}
	provider, err := poller.newProvider(connection)
	if err != nil {
		t.Fatalf("newProvider() error = %v", err)
	}
	poller.providersByOrg["default"][connection.ID] = provider
	poller.configsByOrg["default"] = map[string]config.VMwareVCenterInstance{connection.ID: connection}

	poller.pollAll(context.Background())

	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil || summary.Poll.LastError != nil {
		t.Fatalf("expected successful degraded poll summary, got %+v", summary.Poll)
	}
	if summary.Observed == nil || !summary.Observed.Degraded || summary.Observed.IssueCount != 3 {
		t.Fatalf("expected degraded observed summary, got %+v", summary.Observed)
	}
	if len(summary.Observed.Issues) != 2 {
		t.Fatalf("expected aggregated observed issues, got %+v", summary.Observed.Issues)
	}
	if summary.Observed.Issues[0].Occurrences != 2 || summary.Observed.Issues[0].Category != "permission" {
		t.Fatalf("expected aggregated permission issue first, got %+v", summary.Observed.Issues[0])
	}
}

type sequencedVMwarePollerProvider struct {
	snapshots []*vmware.InventorySnapshot
	index     int
	current   *vmware.InventorySnapshot
}

func (p *sequencedVMwarePollerProvider) Refresh(context.Context) error {
	if len(p.snapshots) == 0 {
		p.current = nil
		return nil
	}
	if p.index >= len(p.snapshots) {
		p.current = p.snapshots[len(p.snapshots)-1]
		return nil
	}
	p.current = p.snapshots[p.index]
	p.index++
	return nil
}

func (p *sequencedVMwarePollerProvider) Records() []unifiedresources.IngestRecord { return nil }
func (p *sequencedVMwarePollerProvider) ActivityChanges() []unifiedresources.ResourceChange {
	return nil
}
func (p *sequencedVMwarePollerProvider) Snapshot() *vmware.InventorySnapshot { return p.current }
func (p *sequencedVMwarePollerProvider) Close()                              {}

func TestVMwarePollerLogsDegradedTransitionsOnlyWhenStateChanges(t *testing.T) {
	logs := captureVMwarePollerLogs(t)
	poller := NewVMwarePoller(nil, time.Minute)
	connection := config.NewVMwareVCenterInstance()
	connection.ID = "vc-log"
	connection.Host = "vc.log.local"
	connection.Username = "administrator@vsphere.local"
	connection.Password = "secret"

	issueA := vmware.InventoryEnrichmentIssue{
		Stage:    "signals",
		Category: "permission",
		Message:  "VMware permissions are insufficient for HostSystem overall status",
	}
	issueB := vmware.InventoryEnrichmentIssue{
		Stage:    "topology",
		Category: "unavailable",
		Message:  "VMware vm guest identity is temporarily unavailable",
	}
	provider := &sequencedVMwarePollerProvider{
		snapshots: []*vmware.InventorySnapshot{
			{
				ConnectionID:     connection.ID,
				CollectedAt:      time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC),
				Hosts:            []vmware.InventoryHost{{Host: "host-101", Name: "esxi-01"}},
				EnrichmentIssues: []vmware.InventoryEnrichmentIssue{issueA, issueA},
			},
			{
				ConnectionID:     connection.ID,
				CollectedAt:      time.Date(2026, 3, 31, 11, 1, 0, 0, time.UTC),
				Hosts:            []vmware.InventoryHost{{Host: "host-101", Name: "esxi-01"}},
				EnrichmentIssues: []vmware.InventoryEnrichmentIssue{issueA, issueA},
			},
			{
				ConnectionID:     connection.ID,
				CollectedAt:      time.Date(2026, 3, 31, 11, 2, 0, 0, time.UTC),
				Hosts:            []vmware.InventoryHost{{Host: "host-101", Name: "esxi-01"}},
				EnrichmentIssues: []vmware.InventoryEnrichmentIssue{issueB},
			},
			{
				ConnectionID: connection.ID,
				CollectedAt:  time.Date(2026, 3, 31, 11, 3, 0, 0, time.UTC),
				Hosts:        []vmware.InventoryHost{{Host: "host-101", Name: "esxi-01"}},
			},
		},
	}

	entry := vmwarePollerProviderEntry{
		orgID:        "default",
		connectionID: connection.ID,
		provider:     provider,
	}
	for i := 0; i < 4; i++ {
		poller.pollConnection(context.Background(), entry)
	}

	output := logs.String()
	if got := strings.Count(output, `"action":"refresh_partial"`); got != 1 {
		t.Fatalf("expected one refresh_partial warning, got %d logs: %s", got, output)
	}
	if got := strings.Count(output, `"action":"refresh_partial_changed"`); got != 1 {
		t.Fatalf("expected one refresh_partial_changed warning, got %d logs: %s", got, output)
	}
	if got := strings.Count(output, `"action":"refresh_recovered"`); got != 1 {
		t.Fatalf("expected one refresh_recovered info log, got %d logs: %s", got, output)
	}

	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Observed == nil || summary.Observed.Degraded {
		t.Fatalf("expected recovered observed summary after final healthy poll, got %+v", summary.Observed)
	}
}

func captureVMwarePollerLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	t.Cleanup(func() {
		log.Logger = origLogger
	})

	return &buf
}
