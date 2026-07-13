package ai

import (
	"context"
	"errors"
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

func TestProxmoxGuestLifecycleWatcherSuppressesGovernedStopWithinObservationWindow(t *testing.T) {
	now := time.Now().UTC()
	running := proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now)
	stopped := proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second))
	audit := proxmoxLifecycleGovernedStopAudit("vm:160", "shutdown", now.Add(3*time.Second), now.Add(8*time.Second))
	lookupCalls := 0
	watcher := newProxmoxGuestLifecycleWatcher(func(resourceID string, since time.Time, limit int) ([]unifiedresources.ActionAuditRecord, error) {
		lookupCalls++
		if resourceID != "vm:160" || !since.IsZero() || limit != proxmoxGuestActionAuditLimit {
			t.Fatalf("lookup resource=%q since=%s limit=%d", resourceID, since, limit)
		}
		return []unifiedresources.ActionAuditRecord{audit}, nil
	})
	watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(running)}, nil, now)
	emit, resolve := watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(stopped)}, nil, now.Add(11*time.Second))
	if lookupCalls != 1 || len(emit) != 0 || len(resolve) != 0 {
		t.Fatalf("governed stop must suppress exactly once: calls=%d emit=%#v resolve=%#v", lookupCalls, emit, resolve)
	}

	emit, _ = watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(stopped)}, nil, now.Add(12*time.Second))
	if lookupCalls != 1 || len(emit) != 0 {
		t.Fatalf("suppressed transition must still advance baseline: calls=%d emit=%#v", lookupCalls, emit)
	}
}

func TestProxmoxGuestLifecycleWatcherEmitsWhenGovernedStopAuditIsNotCausal(t *testing.T) {
	now := time.Now().UTC()
	before := proxmoxGuestLifecycleSnapshot{id: "vm:160", observedAt: now}
	after := proxmoxGuestLifecycleSnapshot{id: "vm:160", observedAt: now.Add(10 * time.Second)}
	valid := proxmoxLifecycleGovernedStopAudit("vm:160", "stop", now.Add(3*time.Second), now.Add(8*time.Second))

	tests := []struct {
		name   string
		mutate func(*unifiedresources.ActionAuditRecord)
	}{
		{name: "different resource", mutate: func(a *unifiedresources.ActionAuditRecord) { a.Request.ResourceID = "vm:999" }},
		{name: "restart capability", mutate: func(a *unifiedresources.ActionAuditRecord) { a.Request.CapabilityName = "restart" }},
		{name: "failed audit state", mutate: func(a *unifiedresources.ActionAuditRecord) { a.State = unifiedresources.ActionStateFailed }},
		{name: "legacy result only", mutate: func(a *unifiedresources.ActionAuditRecord) { a.Result.ActionResultV2 = nil }},
		{name: "failed execution", mutate: func(a *unifiedresources.ActionAuditRecord) { a.Result.Success = false }},
		{name: "stale approval", mutate: func(a *unifiedresources.ActionAuditRecord) { a.Approvals[0].Timestamp = now.Add(-time.Second) }},
		{name: "approval after stop", mutate: func(a *unifiedresources.ActionAuditRecord) { a.Approvals[0].Timestamp = now.Add(11 * time.Second) }},
		{name: "policy authorization", mutate: func(a *unifiedresources.ActionAuditRecord) {
			a.Approvals[0].PolicyLease = &unifiedresources.ActionPolicyAuthorizationLease{}
		}},
		{name: "completion before approval", mutate: func(a *unifiedresources.ActionAuditRecord) { a.UpdatedAt = now.Add(2 * time.Second) }},
		{name: "future completion", mutate: func(a *unifiedresources.ActionAuditRecord) { a.UpdatedAt = now.Add(12 * time.Second) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			audit := valid
			audit.Approvals = append([]unifiedresources.ActionApprovalRecord(nil), valid.Approvals...)
			result := *valid.Result
			audit.Result = &result
			test.mutate(&audit)
			if proxmoxGuestAuditExplainsStoppedTransition(audit, before, after, now.Add(11*time.Second)) {
				t.Fatalf("non-causal audit explained transition: %#v", audit)
			}
		})
	}
}

func TestProxmoxGuestLifecycleWatcherFailsOpenWhenActionAuditLookupFails(t *testing.T) {
	now := time.Now().UTC()
	watcher := newProxmoxGuestLifecycleWatcher(func(string, time.Time, int) ([]unifiedresources.ActionAuditRecord, error) {
		return nil, errors.New("audit store unavailable")
	})
	watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOnline, now),
	)}, nil, now)
	emit, _ := watcher.Observe(patrolRuntimeState{readState: proxmoxLifecycleReadState(
		proxmoxLifecycleTestResource("vm:160", unifiedresources.ResourceTypeVM, unifiedresources.StatusOffline, now.Add(10*time.Second)),
	)}, nil, now.Add(11*time.Second))
	if len(emit) != 1 {
		t.Fatalf("audit lookup failure must preserve warning: %#v", emit)
	}
}

func TestNewPatrolServiceWiresProxmoxLifecycleToOrgScopedActionStore(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	service := &Service{orgID: "org-1", resourceExportStore: store, resourceExportStoreOrgID: "org-1"}
	patrol := NewPatrolService(service, nil)
	if patrol.proxmoxGuestLifecycleWatcher == nil || patrol.proxmoxGuestLifecycleWatcher.lookupActionAudits == nil {
		t.Fatal("production Proxmox lifecycle watcher has no action-audit lookup")
	}
	audits, err := patrol.proxmoxGuestLifecycleWatcher.lookupActionAudits("vm:160", time.Time{}, 10)
	if err != nil || len(audits) != 0 {
		t.Fatalf("org-scoped action lookup audits=%#v err=%v", audits, err)
	}
	service.resourceExportStoreOrgID = "org-2"
	audits, err = patrol.proxmoxGuestLifecycleWatcher.lookupActionAudits("vm:160", time.Time{}, 10)
	if err != nil || audits != nil {
		t.Fatalf("cross-org action store must fail closed: audits=%#v err=%v", audits, err)
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

func proxmoxLifecycleGovernedStopAudit(resourceID, capability string, approvedAt, completedAt time.Time) unifiedresources.ActionAuditRecord {
	resultV2 := unifiedresources.ActionResultV2{
		Version:      unifiedresources.ActionResultV2Version,
		Execution:    unifiedresources.ActionExecutionTruth{Status: unifiedresources.ActionExecutionSucceeded},
		Verification: unifiedresources.ActionVerificationTruth{Status: unifiedresources.ActionVerificationNotAttempted, EvidenceClass: unifiedresources.ActionEvidenceNone},
		Compensation: unifiedresources.ActionCompensationTruth{Support: unifiedresources.ActionCompensationUnavailable, Status: unifiedresources.ActionCompensationNotAvailable},
	}
	return unifiedresources.ActionAuditRecord{
		ID: "action-stop-1", CreatedAt: approvedAt.Add(-time.Minute), UpdatedAt: completedAt, State: unifiedresources.ActionStateCompleted,
		Request:   unifiedresources.ActionRequest{ResourceID: resourceID, CapabilityName: capability},
		Approvals: []unifiedresources.ActionApprovalRecord{{Timestamp: approvedAt, Outcome: unifiedresources.OutcomeApproved}},
		Result:    &unifiedresources.ExecutionResult{Success: true, ActionResultV2: &resultV2},
	}
}
