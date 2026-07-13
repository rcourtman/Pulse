package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestPatrolRunWiresProxmoxGuestLifecycleWatcherBeforeProviderGate(t *testing.T) {
	now := time.Now().UTC()
	patrol := NewPatrolService(nil, nil)
	patrol.SetReadState(proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now),
	))
	patrol.runPatrol(context.Background())

	patrol.SetReadState(proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second)),
	))
	patrol.runPatrol(context.Background())

	finding := patrol.GetFindings().Get("proxmox:guest:stopped:vm:160")
	if finding == nil || finding.ResolvedAt != nil || finding.Source != proxmoxGuestLifecycleFindingSource {
		t.Fatalf("production Patrol finding=%#v", finding)
	}
}

func TestDetectProxmoxGuestLifecycleFindingsEmitsVMAndLXCStoppedTransitions(t *testing.T) {
	now := time.Now().UTC()
	before := proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now),
		proxmoxLifecycleTestResource("system-container:101", unifiedresources.ResourceTypeSystemContainer, unifiedresources.StatusOnline, now),
	)
	after := proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second)),
		proxmoxLifecycleTestResource("system-container:101", unifiedresources.ResourceTypeSystemContainer, unifiedresources.StatusOffline, now.Add(10*time.Second)),
	)

	findings := DetectProxmoxGuestLifecycleFindings(before, after, now.Add(10*time.Second))
	if len(findings) != 2 {
		t.Fatalf("findings=%#v, want VM and LXC stopped transitions", findings)
	}
	if findings[0].ID != "proxmox:guest:stopped:system-container:101" || findings[1].ID != "proxmox:guest:stopped:vm:160" {
		t.Fatalf("finding IDs=%q/%q", findings[0].ID, findings[1].ID)
	}
	for _, finding := range findings {
		if finding.Key != proxmoxGuestStoppedFindingKey || finding.Source != proxmoxGuestLifecycleFindingSource || finding.Severity != FindingSeverityWarning || finding.Category != FindingCategoryReliability {
			t.Fatalf("finding=%#v", finding)
		}
		if len(finding.Evidence) == 0 || len(finding.Evidence) > 384 {
			t.Fatalf("unbounded evidence=%q", finding.Evidence)
		}
		for _, forbidden := range []string{"qm ", "pct ", "command", "token", "secret"} {
			if strings.Contains(strings.ToLower(finding.Evidence), forbidden) {
				t.Fatalf("finding evidence exposes execution detail %q: %s", forbidden, finding.Evidence)
			}
		}
	}
}

func TestProxmoxGuestLifecycleWatcherSeedsWithoutFlaggingExistingStoppedGuests(t *testing.T) {
	now := time.Now().UTC()
	watcher := newProxmoxGuestLifecycleWatcher()
	emit, resolve := watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now),
	)}, nil, now)
	if len(emit) != 0 || len(resolve) != 0 {
		t.Fatalf("first stopped observation must seed only: emit=%#v resolve=%#v", emit, resolve)
	}
}

func TestProxmoxGuestLifecycleWatcherFailsClosedOnFreshnessCapabilityAndGuestGuards(t *testing.T) {
	now := time.Now().UTC()
	resourceIDs := []string{"vm:stale", "vm:same-clock", "vm:no-capability", "vm:template", "vm:locked"}
	running := make([]unifiedresources.Resource, 0, len(resourceIDs))
	for _, id := range resourceIDs {
		running = append(running, proxmoxLifecycleTestResource(id, unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now))
	}
	watcher := newProxmoxGuestLifecycleWatcher()
	watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(running...)}, nil, now)

	stale := proxmoxLifecycleTestResource("vm:stale", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second))
	stale.SourceStatus[unifiedresources.SourceProxmox] = unifiedresources.SourceStatus{Status: "stale", LastSeen: now.Add(10 * time.Second)}
	sameClock := proxmoxLifecycleTestResource("vm:same-clock", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now)
	noCapability := proxmoxLifecycleTestResource("vm:no-capability", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second))
	noCapability.Capabilities = nil
	template := proxmoxLifecycleTestResource("vm:template", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second))
	template.Proxmox.Template = true
	locked := proxmoxLifecycleTestResource("vm:locked", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second))
	locked.Proxmox.Lock = "backup"

	emit, resolve := watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(stale, sameClock, noCapability, template, locked)}, nil, now.Add(10*time.Second))
	if len(emit) != 0 || len(resolve) != 0 {
		t.Fatalf("guarded observations must remain unknown: emit=%#v resolve=%#v", emit, resolve)
	}
}

func TestProxmoxGuestLifecycleWatcherReconcilesOnlyFreshRunningOrRemoval(t *testing.T) {
	now := time.Now().UTC()
	watcher := newProxmoxGuestLifecycleWatcher()
	watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now),
	)}, nil, now)
	emit, _ := watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second)),
	)}, nil, now.Add(10*time.Second))
	if len(emit) != 1 {
		t.Fatalf("emit=%#v, want transition finding", emit)
	}
	active := []*Finding{emit[0]}

	staleRunning := proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now.Add(20*time.Second))
	staleRunning.SourceStatus[unifiedresources.SourceProxmox] = unifiedresources.SourceStatus{Status: "stale", LastSeen: now.Add(20 * time.Second)}
	_, resolve := watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(staleRunning)}, active, now.Add(20*time.Second))
	if len(resolve) != 0 {
		t.Fatalf("stale running state must not resolve active finding: %#v", resolve)
	}

	_, resolve = watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now.Add(30*time.Second)),
	)}, active, now.Add(30*time.Second))
	if len(resolve) != 1 || resolve[0].DedupKey != emit[0].ID || resolve[0].Reason != proxmoxGuestLifecycleResolveRunning {
		t.Fatalf("fresh running resolution=%#v", resolve)
	}

	watcher = newProxmoxGuestLifecycleWatcher()
	watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now),
	)}, active, now)
	_, resolve = watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState()}, active, now.Add(time.Second))
	if len(resolve) != 1 || resolve[0].Reason != proxmoxGuestLifecycleResolveRemoved {
		t.Fatalf("resource removal resolution=%#v", resolve)
	}
}

func proxmoxLifecycleReadState(resources ...unifiedresources.Resource) unifiedresources.ReadState {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources(resources)
	return registry
}

func proxmoxLifecycleTestResource(id string, resourceType unifiedresources.ResourceType, status unifiedresources.ResourceStatus, observedAt time.Time) unifiedresources.Resource {
	handler := proxmoxVMLifecycleCapabilityHandler
	platform := "qemu"
	vmid := 160
	if resourceType == unifiedresources.ResourceTypeSystemContainer {
		handler = proxmoxLXCLifecycleCapabilityHandler
		platform = "lxc"
		vmid = 101
	}
	capabilityName := "shutdown"
	if status == unifiedresources.StatusOffline {
		capabilityName = "start"
	}
	return unifiedresources.Resource{
		ID: id, Type: resourceType, Technology: platform, Name: "guest-" + id, Status: status, LastSeen: observedAt, UpdatedAt: observedAt,
		Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
		SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
			unifiedresources.SourceProxmox: {Status: "online", LastSeen: observedAt},
		},
		Proxmox: &unifiedresources.ProxmoxData{SourceID: "homelab:delly:" + id, NodeName: "delly", Instance: "homelab", VMID: vmid},
		Capabilities: []unifiedresources.ResourceCapability{{
			Name: capabilityName, Type: unifiedresources.CapabilityTypeCommon, MinimumApprovalLevel: unifiedresources.ApprovalAdmin, Platform: platform, InternalHandler: handler,
		}},
	}
}
