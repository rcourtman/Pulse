package ai

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	proxmoxGuestLifecycleFindingSource   = "proxmox-guest-lifecycle"
	proxmoxGuestStoppedFindingKey        = "proxmox-guest-stopped"
	proxmoxGuestStoppedFindingIDPrefix   = "proxmox:guest:stopped:"
	proxmoxGuestLifecycleResolveRunning  = "proxmox_guest_lifecycle:running"
	proxmoxGuestLifecycleResolveRemoved  = "proxmox_guest_lifecycle:resource_removed"
	proxmoxVMLifecycleCapabilityHandler  = "proxmox.vm.lifecycle"
	proxmoxLXCLifecycleCapabilityHandler = "proxmox.ct.lifecycle"
)

type proxmoxGuestLifecycleSnapshot struct {
	id           string
	name         string
	resourceType unifiedresources.ResourceType
	kind         string
	status       unifiedresources.ResourceStatus
	node         string
	instance     string
	vmid         int
	template     bool
	lock         string
	observedAt   time.Time
	sourceFresh  bool
	capabilities []unifiedresources.ResourceCapability
}

type proxmoxGuestLifecycleWatcher struct {
	mu    sync.Mutex
	prior map[string]proxmoxGuestLifecycleSnapshot
}

func newProxmoxGuestLifecycleWatcher() *proxmoxGuestLifecycleWatcher {
	return &proxmoxGuestLifecycleWatcher{prior: make(map[string]proxmoxGuestLifecycleSnapshot)}
}

// DetectProxmoxGuestLifecycleFindings is the pure detector used by Patrol and
// the end-to-end action lifecycle proof. A stopped guest is actionable only
// when it was freshly observed running before a newer, fresh stopped
// observation and the stopped resource advertises the exact governed start
// capability for its guest kind.
func DetectProxmoxGuestLifecycleFindings(before, after unifiedresources.ReadState, now time.Time) []*Finding {
	previous := proxmoxGuestLifecycleSnapshots(before)
	current := proxmoxGuestLifecycleSnapshots(after)
	ids := sortedProxmoxGuestLifecycleIDs(current)
	findings := make([]*Finding, 0)
	for _, id := range ids {
		afterSnapshot := current[id]
		beforeSnapshot, ok := previous[id]
		if !ok || !proxmoxGuestStoppedTransition(beforeSnapshot, afterSnapshot) {
			continue
		}
		findings = append(findings, buildProxmoxGuestStoppedFinding(afterSnapshot, now.UTC()))
	}
	return findings
}

func (w *proxmoxGuestLifecycleWatcher) Observe(state patrolRuntimeState, active []*Finding, now time.Time) (emit []*Finding, resolve []resolveSentinel) {
	if w == nil {
		return nil, nil
	}
	if state.readState == nil {
		state = state.withDerivedProviders()
	}
	if state.readState == nil {
		return nil, nil
	}

	current := proxmoxGuestLifecycleSnapshots(state.readState)
	activeByResource := make(map[string]*Finding)
	for _, finding := range active {
		if finding == nil || finding.Source != proxmoxGuestLifecycleFindingSource {
			continue
		}
		activeByResource[strings.TrimSpace(finding.ResourceID)] = finding
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.prior == nil {
		w.prior = make(map[string]proxmoxGuestLifecycleSnapshot)
	}

	resolved := make(map[string]struct{})
	for _, id := range sortedProxmoxGuestLifecycleIDs(current) {
		snapshot := current[id]
		activeFinding := activeByResource[id]
		if snapshot.authoritative() {
			switch {
			case activeFinding != nil && snapshot.status == unifiedresources.StatusOnline:
				resolve = append(resolve, resolveSentinel{DedupKey: activeFinding.ID, Reason: proxmoxGuestLifecycleResolveRunning})
				resolved[activeFinding.ID] = struct{}{}
			case activeFinding != nil && snapshot.startable():
				emit = append(emit, buildProxmoxGuestStoppedFinding(snapshot, now.UTC()))
			case activeFinding == nil:
				if previous, ok := w.prior[id]; ok && proxmoxGuestStoppedTransition(previous, snapshot) {
					emit = append(emit, buildProxmoxGuestStoppedFinding(snapshot, now.UTC()))
				}
			}
			w.prior[id] = snapshot
		}
	}

	for id := range w.prior {
		if _, present := current[id]; !present {
			delete(w.prior, id)
		}
	}
	for _, finding := range activeByResource {
		if _, present := current[strings.TrimSpace(finding.ResourceID)]; present {
			continue
		}
		if _, alreadyResolved := resolved[finding.ID]; alreadyResolved {
			continue
		}
		resolve = append(resolve, resolveSentinel{DedupKey: finding.ID, Reason: proxmoxGuestLifecycleResolveRemoved})
	}
	return emit, resolve
}

func proxmoxGuestLifecycleSnapshots(readState unifiedresources.ReadState) map[string]proxmoxGuestLifecycleSnapshot {
	snapshots := make(map[string]proxmoxGuestLifecycleSnapshot)
	if readState == nil {
		return snapshots
	}
	for _, vm := range readState.VMs() {
		if vm == nil || strings.TrimSpace(vm.ID()) == "" {
			continue
		}
		freshness, ok := vm.SourceStatus(unifiedresources.SourceProxmox)
		snapshots[strings.TrimSpace(vm.ID())] = proxmoxGuestLifecycleSnapshot{
			id: strings.TrimSpace(vm.ID()), name: strings.TrimSpace(vm.Name()), resourceType: unifiedresources.ResourceTypeVM,
			kind: "VM", status: vm.Status(), node: vm.Node(), instance: vm.Instance(), vmid: vm.VMID(), template: vm.Template(), lock: strings.TrimSpace(vm.Lock()),
			observedAt: freshness.LastSeen.UTC(), sourceFresh: ok && strings.EqualFold(strings.TrimSpace(freshness.Status), "online") && !freshness.LastSeen.IsZero(),
			capabilities: vm.Capabilities(),
		}
	}
	for _, container := range readState.Containers() {
		if container == nil || strings.TrimSpace(container.ID()) == "" {
			continue
		}
		freshness, ok := container.SourceStatus(unifiedresources.SourceProxmox)
		snapshots[strings.TrimSpace(container.ID())] = proxmoxGuestLifecycleSnapshot{
			id: strings.TrimSpace(container.ID()), name: strings.TrimSpace(container.Name()), resourceType: unifiedresources.ResourceTypeSystemContainer,
			kind: "LXC", status: container.Status(), node: container.Node(), instance: container.Instance(), vmid: container.VMID(), template: container.Template(), lock: strings.TrimSpace(container.Lock()),
			observedAt: freshness.LastSeen.UTC(), sourceFresh: ok && strings.EqualFold(strings.TrimSpace(freshness.Status), "online") && !freshness.LastSeen.IsZero(),
			capabilities: container.Capabilities(),
		}
	}
	return snapshots
}

func sortedProxmoxGuestLifecycleIDs(snapshots map[string]proxmoxGuestLifecycleSnapshot) []string {
	ids := make([]string, 0, len(snapshots))
	for id := range snapshots {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s proxmoxGuestLifecycleSnapshot) authoritative() bool {
	if !s.sourceFresh || s.observedAt.IsZero() || s.id == "" || s.vmid <= 0 || s.template || s.lock != "" {
		return false
	}
	return s.status == unifiedresources.StatusOnline || s.status == unifiedresources.StatusOffline
}

func (s proxmoxGuestLifecycleSnapshot) startable() bool {
	if !s.authoritative() || s.status != unifiedresources.StatusOffline {
		return false
	}
	wantHandler := proxmoxVMLifecycleCapabilityHandler
	if s.resourceType == unifiedresources.ResourceTypeSystemContainer {
		wantHandler = proxmoxLXCLifecycleCapabilityHandler
	}
	for _, capability := range s.capabilities {
		if strings.TrimSpace(capability.Name) == "start" && strings.TrimSpace(capability.InternalHandler) == wantHandler {
			return true
		}
	}
	return false
}

func proxmoxGuestStoppedTransition(before, after proxmoxGuestLifecycleSnapshot) bool {
	return before.authoritative() && before.status == unifiedresources.StatusOnline &&
		after.startable() && after.observedAt.After(before.observedAt)
}

func buildProxmoxGuestStoppedFinding(snapshot proxmoxGuestLifecycleSnapshot, now time.Time) *Finding {
	name := snapshot.name
	if name == "" {
		name = snapshot.id
	}
	id := proxmoxGuestStoppedFindingIDPrefix + snapshot.id
	return &Finding{
		ID: id, Key: proxmoxGuestStoppedFindingKey, Severity: FindingSeverityWarning, Category: FindingCategoryReliability,
		ResourceID: snapshot.id, ResourceName: name, ResourceType: string(snapshot.resourceType), Node: snapshot.node,
		Title:          fmt.Sprintf("Proxmox %s %s stopped", snapshot.kind, name),
		Description:    fmt.Sprintf("Pulse observed %s transition from running to stopped across fresh Proxmox inventory updates.", name),
		Impact:         "Services provided by this guest may be unavailable until it is started or the stop is confirmed as intentional.",
		Recommendation: "Review the guest and approve the governed Start action if this stop was not intentional.",
		Evidence:       fmt.Sprintf("kind=%s status=stopped instance=%s node=%s vmid=%d observed_at=%s", strings.ToLower(snapshot.kind), snapshot.instance, snapshot.node, snapshot.vmid, snapshot.observedAt.Format(time.RFC3339)),
		Source:         proxmoxGuestLifecycleFindingSource, DetectedAt: now, LastSeenAt: now,
	}
}
