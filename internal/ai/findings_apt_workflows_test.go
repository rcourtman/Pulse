package ai

import (
	"strings"
	"testing"
	"time"

	ur "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func aptWorkflowTestHost(now time.Time) *ur.HostView {
	digestA := "sha256:" + strings.Repeat("a", 64)
	digestB := "sha256:" + strings.Repeat("b", 64)
	resource := &ur.Resource{
		ID: "agent:host-1", Type: ur.ResourceTypeAgent, Name: "host-1", Status: ur.StatusOnline,
		Capabilities: []ur.ResourceCapability{
			{Name: "install_os_updates", InternalHandler: "host.package_updates"},
			{Name: "clean_package_cache", InternalHandler: "host.storage_cleanup"},
		},
		Agent: &ur.AgentData{
			AgentID: "host-1", CommandsEnabled: true, OperationReceiptVersion: 1,
			PackageUpdates: &ur.AgentPackageUpdateMeta{Supported: true, Manager: "apt", InventoryHash: digestA, PendingCount: 3, CheckedAt: now.Add(-time.Minute), ObservedAt: now},
			StorageCleanup: &ur.AgentStorageCleanupMeta{Supported: true, Provider: "apt-package-cache", Fingerprint: digestB, ReclaimableBytes: 128 * 1024 * 1024, CheckedAt: now.Add(-time.Minute), ObservedAt: now},
			Disks:          []ur.DiskInfo{{Mountpoint: "/", Usage: 95}},
		},
	}
	view := ur.NewHostView(resource)
	return &view
}

func TestAPTWorkflowWatcherEmitsDeterministicCapabilityBoundFindings(t *testing.T) {
	now := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	host := aptWorkflowTestHost(now)
	readState := &mockReadState{hosts: []*ur.HostView{host}}
	detected := DetectAPTWorkflowFindings(readState, now)
	if len(detected) != 2 || detected[0].ID != "apt:host-update:agent:host-1" || detected[1].ID != "apt:cache-cleanup:agent:host-1" {
		t.Fatalf("canonical detector findings=%#v", detected)
	}
	emit, resolve := newAPTWorkflowWatcher().Observe(patrolRuntimeState{readState: readState}, nil, now)
	if len(emit) != 2 || len(resolve) != 0 {
		t.Fatalf("emit=%d resolve=%d", len(emit), len(resolve))
	}
	if emit[0].ID != "apt:host-update:agent:host-1" || emit[1].ID != "apt:cache-cleanup:agent:host-1" {
		t.Fatalf("unexpected stable ids: %q %q", emit[0].ID, emit[1].ID)
	}
	for _, finding := range emit {
		lower := strings.ToLower(finding.Evidence + " " + finding.Description)
		for _, forbidden := range []string{"apt-get", "/var/cache", "--no-remove", "dpkg"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("finding exposed command/path %q: %s", forbidden, lower)
			}
		}
	}
}

func TestAPTWorkflowWatcherLegacyReceiptProtocolWithFreshTelemetryEmitsNothing(t *testing.T) {
	now := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	resource := hostResourceForAPTTest(aptWorkflowTestHost(now), now)
	resource.Agent.OperationReceiptVersion = 0
	resource.Capabilities = nil
	view := ur.NewHostView(resource)
	emit, resolve := newAPTWorkflowWatcher().Observe(patrolRuntimeState{readState: &mockReadState{hosts: []*ur.HostView{&view}}}, nil, now)
	if len(emit) != 0 || len(resolve) != 0 {
		t.Fatalf("emit=%d resolve=%d", len(emit), len(resolve))
	}
}

func TestAPTWorkflowWatcherReplayAndFutureTimesRemainUnresolved(t *testing.T) {
	now := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	active := []*Finding{{ID: "apt:host-update:agent:host-1", ResourceID: "agent:host-1", Source: aptWorkflowFindingSource}}
	tests := []struct {
		name    string
		checked time.Time
		seen    time.Time
	}{
		{name: "old agent observation retransmitted with fresh server receipt", checked: now.Add(-ur.HostPackageUpdateFreshness - time.Minute), seen: now},
		{name: "future agent observation", checked: now.Add(ur.HostAPTTelemetryMaxClockSkew + time.Second), seen: now},
		{name: "future server receipt", checked: now, seen: now.Add(ur.HostAPTTelemetryMaxClockSkew + time.Second)},
		{name: "server receipt predates agent observation beyond skew", checked: now, seen: now.Add(-ur.HostAPTTelemetryMaxClockSkew - time.Second)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host := aptWorkflowTestHost(now)
			host.PackageUpdates().CheckedAt = tc.checked // copy accessor proves callers cannot mutate authority
			resource := hostResourceForAPTTest(host, now)
			resource.Capabilities = resource.Capabilities[:1]
			resource.Agent.StorageCleanup = nil
			resource.Agent.PackageUpdates.CheckedAt, resource.Agent.PackageUpdates.ObservedAt = tc.checked, tc.seen
			view := ur.NewHostView(resource)
			emit, resolve := newAPTWorkflowWatcher().Observe(patrolRuntimeState{readState: &mockReadState{hosts: []*ur.HostView{&view}}}, active, now)
			if len(emit) != 0 || len(resolve) != 0 {
				t.Fatalf("stale/skewed telemetry emitted=%d resolved=%d", len(emit), len(resolve))
			}
		})
	}
}

func TestAPTWorkflowWatcherCapabilityLossDoesNotResolveActiveFinding(t *testing.T) {
	now := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	resource := hostResourceForAPTTest(aptWorkflowTestHost(now), now)
	resource.Agent.PackageUpdates.PendingCount = 0
	resource.Capabilities = []ur.ResourceCapability{{Name: "replacement", InternalHandler: "host.package_updates"}}
	view := ur.NewHostView(resource)
	active := []*Finding{{ID: "apt:host-update:agent:host-1", ResourceID: "agent:host-1", Source: aptWorkflowFindingSource}}
	emit, resolve := newAPTWorkflowWatcher().Observe(patrolRuntimeState{readState: &mockReadState{hosts: []*ur.HostView{&view}}}, active, now)
	if len(emit) != 0 || len(resolve) != 0 {
		t.Fatalf("capability loss emitted=%d resolved=%d", len(emit), len(resolve))
	}
	cleared, handled, err := verifyAPTWorkflowFinding(patrolRuntimeState{readState: &mockReadState{hosts: []*ur.HostView{&view}}}, aptHostUpdateFindingKey, resource.ID, now)
	if cleared || !handled || err == nil {
		t.Fatalf("verification cleared=%t handled=%t err=%v", cleared, handled, err)
	}
}

func TestAPTWorkflowWatcherFreshClearReconcilesAndRemovalIsExplicit(t *testing.T) {
	now := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	resource := hostResourceForAPTTest(aptWorkflowTestHost(now), now)
	resource.Agent.PackageUpdates.PendingCount = 0
	view := ur.NewHostView(resource)
	active := []*Finding{{ID: "apt:host-update:agent:host-1", ResourceID: "agent:host-1", Source: aptWorkflowFindingSource}}
	_, resolve := newAPTWorkflowWatcher().Observe(patrolRuntimeState{readState: &mockReadState{hosts: []*ur.HostView{&view}}}, active, now)
	if len(resolve) != 1 || resolve[0].Reason != aptWorkflowResolveCleared {
		t.Fatalf("fresh clear resolve=%+v", resolve)
	}
	_, removed := newAPTWorkflowWatcher().Observe(patrolRuntimeState{readState: &mockReadState{}}, active, now)
	if len(removed) != 1 || removed[0].Reason != aptWorkflowResolveRemoved {
		t.Fatalf("removed resolve=%+v", removed)
	}
}

func hostResourceForAPTTest(_ *ur.HostView, now time.Time) *ur.Resource {
	digestA := "sha256:" + strings.Repeat("a", 64)
	digestB := "sha256:" + strings.Repeat("b", 64)
	return &ur.Resource{
		ID: "agent:host-1", Type: ur.ResourceTypeAgent, Name: "host-1", Status: ur.StatusOnline,
		Capabilities: []ur.ResourceCapability{{Name: "install_os_updates", InternalHandler: "host.package_updates"}, {Name: "clean_package_cache", InternalHandler: "host.storage_cleanup"}},
		Agent: &ur.AgentData{AgentID: "host-1", CommandsEnabled: true, OperationReceiptVersion: 1,
			PackageUpdates: &ur.AgentPackageUpdateMeta{Supported: true, Manager: "apt", InventoryHash: digestA, PendingCount: 3, CheckedAt: now.Add(-time.Minute), ObservedAt: now},
			StorageCleanup: &ur.AgentStorageCleanupMeta{Supported: true, Provider: "apt-package-cache", Fingerprint: digestB, ReclaimableBytes: 128 * 1024 * 1024, CheckedAt: now.Add(-time.Minute), ObservedAt: now},
			Disks:          []ur.DiskInfo{{Mountpoint: "/", Usage: 95}}},
	}
}
