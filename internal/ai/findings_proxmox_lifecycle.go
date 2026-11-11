package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const (
	proxmoxGuestLifecycleFindingSource   = "proxmox-guest-lifecycle"
	proxmoxGuestStoppedFindingKey        = "proxmox-guest-stopped"
	proxmoxGuestStoppedFindingIDPrefix   = "proxmox:guest:stopped:"
	proxmoxGuestLifecycleResolveRunning  = "proxmox_guest_lifecycle:running"
	proxmoxGuestLifecycleResolveRemoved  = "proxmox_guest_lifecycle:resource_removed"
	proxmoxVMLifecycleCapabilityHandler  = "proxmox.vm.lifecycle"
	proxmoxLXCLifecycleCapabilityHandler = "proxmox.ct.lifecycle"
	proxmoxGuestActionAuditLimit         = 100
)

type proxmoxGuestActionAuditLookup func(resourceID string, since time.Time, limit int) ([]unifiedresources.ActionAuditRecord, error)

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
	mu                 sync.Mutex
	prior              map[string]proxmoxGuestLifecycleSnapshot
	lookupActionAudits proxmoxGuestActionAuditLookup
	// readTasks reads a node's task log for a guest. Nil leaves the cause of
	// every stop unknown.
	readTasks ProxmoxGuestTaskReader
	// stops holds what Pulse worked out about each guest it saw stop, for as
	// long as the guest stays stopped.
	stops map[string]proxmoxGuestStopRecord
	// stopEpisodes numbers each guest's current stop, so a task log read that
	// comes back late cannot write to a stop that is over or to a later one.
	stopEpisodes map[string]proxmoxGuestStopEpisode
	stopSeq      uint64
}

// proxmoxGuestStopEpisode is one stop of one guest: its number and when Pulse
// first saw the guest stopped.
type proxmoxGuestStopEpisode struct {
	seq       uint64
	stoppedAt time.Time
}

func newProxmoxGuestLifecycleWatcher(lookupActionAudits ...proxmoxGuestActionAuditLookup) *proxmoxGuestLifecycleWatcher {
	watcher := &proxmoxGuestLifecycleWatcher{prior: make(map[string]proxmoxGuestLifecycleSnapshot)}
	if len(lookupActionAudits) > 0 {
		watcher.lookupActionAudits = lookupActionAudits[0]
	}
	return watcher
}

// DetectProxmoxGuestLifecycleFindings is the pure detector used by the
// end-to-end action lifecycle proof. A stopped guest is actionable only
// when it was freshly observed running before a newer, fresh stopped
// observation and the stopped resource advertises the exact governed start
// capability for its guest kind. It reads no task log, so every finding it
// builds says the cause of the stop is unknown. Patrol's own check is
// proxmoxGuestLifecycleWatcher.Observe, which asks Proxmox who stopped it.
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
		cause := proxmoxGuestStopCause{unreadReason: proxmoxGuestStopReasonNoReader, since: beforeSnapshot.observedAt, until: afterSnapshot.observedAt}
		findings = append(findings, buildProxmoxGuestStoppedFinding(afterSnapshot, now.UTC(), cause))
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

	type stoppedTransition struct {
		before  proxmoxGuestLifecycleSnapshot
		after   proxmoxGuestLifecycleSnapshot
		episode uint64
	}
	transitions := make([]stoppedTransition, 0)

	w.mu.Lock()
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
				emit = append(emit, w.refreshedStoppedFindingLocked(snapshot, activeFinding, now.UTC()))
			case activeFinding == nil:
				if previous, ok := w.prior[id]; ok && proxmoxGuestStoppedTransition(previous, snapshot) {
					if w.stopEpisodes == nil {
						w.stopEpisodes = make(map[string]proxmoxGuestStopEpisode)
					}
					w.stopSeq++
					w.stopEpisodes[id] = proxmoxGuestStopEpisode{seq: w.stopSeq, stoppedAt: snapshot.observedAt}
					delete(w.stops, id)
					transitions = append(transitions, stoppedTransition{before: previous, after: snapshot, episode: w.stopSeq})
				}
			}
			w.prior[id] = snapshot
		}
		// A guest Proxmox freshly reports running has no stop left to explain.
		// This holds for a guest the check does not otherwise act on, such as
		// one carrying a Proxmox lock: a stop on record must not outlive the
		// stop. A stale Proxmox source proves nothing either way, whatever
		// another source says about the guest being reachable.
		// Nor does a running observation older than the stop, which a look
		// that overlapped a newer one can still be holding.
		if snapshot.status == unifiedresources.StatusOnline && snapshot.sourceFresh && !snapshot.observedAt.IsZero() &&
			snapshot.observedAt.After(w.stopEpisodes[id].stoppedAt) {
			w.forgetStopLocked(id)
		}
	}

	for id := range w.prior {
		if _, present := current[id]; !present {
			delete(w.prior, id)
			w.forgetStopLocked(id)
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
	w.mu.Unlock()

	// Every guest found stopped in this look shares one allowance of time for
	// its task log read.
	readCtx, cancelReads := context.WithTimeout(context.Background(), proxmoxGuestStopReadBudget)
	defer cancelReads()
	for _, transition := range transitions {
		if w.governedStopCompleted(transition.before, transition.after, now.UTC()) {
			w.rememberStop(transition.after, transition.episode, proxmoxGuestStopCause{kind: proxmoxGuestStopRequested, governed: true, task: "stop", user: "Pulse", taskAt: transition.after.observedAt})
			continue
		}
		// Asked once, when the stop is first seen. The answer travels with the
		// finding from then on.
		cause := w.readStopCause(readCtx, transition.before, transition.after)
		if !w.rememberStop(transition.after, transition.episode, cause) {
			// While the log was being read the guest ran again, went away or
			// stopped a second time. This answer is about a stop that is
			// over, and whoever saw the later state speaks for it.
			continue
		}
		if cause.kind == proxmoxGuestStopRequested {
			// A person or a job stopped it on purpose. That is not a problem
			// and nobody should be asked to approve starting it again.
			log.Debug().Str("resource_id", transition.after.id).Str("guest", transition.after.name).Int("vmid", transition.after.vmid).
				Str("task", cause.task).Str("user", cause.user).Str("task_status", cause.taskStatus).Time("task_started", cause.taskAt).
				Msg("AI Patrol: Proxmox guest was stopped through Proxmox, no finding filed")
			continue
		}
		emit = append(emit, buildProxmoxGuestStoppedFinding(transition.after, now.UTC(), cause))
	}
	return emit, resolve
}

// refreshedStoppedFindingLocked rebuilds an open stopped-guest finding for a
// guest that is still stopped. The cause was worked out when the stop was
// first seen. After a restart Pulse no longer holds it, and the open finding's
// own words are kept, because asking again later would search the wrong time.
// The caller holds w.mu.
func (w *proxmoxGuestLifecycleWatcher) refreshedStoppedFindingLocked(snapshot proxmoxGuestLifecycleSnapshot, open *Finding, now time.Time) *Finding {
	if record, ok := w.stops[snapshot.id]; ok {
		return buildProxmoxGuestStoppedFinding(snapshot, now, record.cause)
	}
	finding := buildProxmoxGuestStoppedFindingBase(snapshot, now)
	finding.Description, finding.Recommendation, finding.Evidence = open.Description, open.Recommendation, open.Evidence
	return finding
}

func (w *proxmoxGuestLifecycleWatcher) governedStopCompleted(before, after proxmoxGuestLifecycleSnapshot, now time.Time) bool {
	if w == nil || w.lookupActionAudits == nil {
		return false
	}
	audits, err := w.lookupActionAudits(after.id, time.Time{}, proxmoxGuestActionAuditLimit)
	if err != nil {
		log.Warn().Err(err).Str("resource_id", after.id).Msg("failed to inspect Proxmox guest action audits for Patrol suppression")
		return false
	}
	for _, audit := range audits {
		if proxmoxGuestAuditExplainsStoppedTransition(audit, before, after, now) {
			return true
		}
	}
	return false
}

func proxmoxGuestAuditExplainsStoppedTransition(audit unifiedresources.ActionAuditRecord, before, after proxmoxGuestLifecycleSnapshot, now time.Time) bool {
	if audit.State != unifiedresources.ActionStateCompleted ||
		unifiedresources.CanonicalResourceID(audit.Request.ResourceID) != unifiedresources.CanonicalResourceID(after.id) ||
		audit.Result == nil || !audit.Result.Success || audit.Result.ActionResultV2 == nil ||
		audit.UpdatedAt.IsZero() || audit.UpdatedAt.Before(before.observedAt) || audit.UpdatedAt.After(now) {
		return false
	}
	capability := strings.ToLower(strings.TrimSpace(audit.Request.CapabilityName))
	if capability != "shutdown" && capability != "stop" {
		return false
	}
	if unifiedresources.CanonicalActionResultV2(audit).Execution.Status != unifiedresources.ActionExecutionSucceeded {
		return false
	}
	for _, approval := range audit.Approvals {
		if approval.Outcome != unifiedresources.OutcomeApproved || approval.PolicyLease != nil || approval.Timestamp.IsZero() {
			continue
		}
		approvedAt := approval.Timestamp.UTC()
		if !approvedAt.Before(before.observedAt) && !approvedAt.After(after.observedAt) && !audit.UpdatedAt.Before(approvedAt) {
			return true
		}
	}
	return false
}

func (p *PatrolService) loadProxmoxGuestActionAudits(resourceID string, since time.Time, limit int) ([]unifiedresources.ActionAuditRecord, error) {
	if p == nil || p.aiService == nil {
		return nil, nil
	}
	p.aiService.mu.RLock()
	store := p.aiService.resourceExportStore
	orgID := strings.TrimSpace(p.aiService.orgID)
	storeOrgID := strings.TrimSpace(p.aiService.resourceExportStoreOrgID)
	p.aiService.mu.RUnlock()
	if store == nil || (storeOrgID != "" && storeOrgID != orgID) {
		return nil, nil
	}
	return store.GetActionAudits(resourceID, since, limit)
}

func proxmoxGuestLifecycleSnapshots(readState unifiedresources.ReadState) map[string]proxmoxGuestLifecycleSnapshot {
	snapshots := make(map[string]proxmoxGuestLifecycleSnapshot)
	if readState == nil {
		return snapshots
	}
	for _, vm := range readState.VMs() {
		if vm != nil {
			recordProxmoxGuestLifecycleSnapshot(snapshots, vm, unifiedresources.ResourceTypeVM, "VM")
		}
	}
	for _, container := range readState.Containers() {
		if container != nil {
			recordProxmoxGuestLifecycleSnapshot(snapshots, container, unifiedresources.ResourceTypeSystemContainer, "LXC")
		}
	}
	return snapshots
}

// proxmoxGuestView is the read surface shared by VM and LXC container views
// that guest lifecycle snapshots are built from.
type proxmoxGuestView interface {
	ID() string
	Name() string
	Status() unifiedresources.ResourceStatus
	Node() string
	Instance() string
	VMID() int
	Template() bool
	Lock() string
	Capabilities() []unifiedresources.ResourceCapability
	SourceStatus(unifiedresources.DataSource) (unifiedresources.SourceStatus, bool)
}

func recordProxmoxGuestLifecycleSnapshot(snapshots map[string]proxmoxGuestLifecycleSnapshot, guest proxmoxGuestView, resourceType unifiedresources.ResourceType, kind string) {
	id := strings.TrimSpace(guest.ID())
	if id == "" {
		return
	}
	freshness, ok := guest.SourceStatus(unifiedresources.SourceProxmox)
	snapshots[id] = proxmoxGuestLifecycleSnapshot{
		id: id, name: strings.TrimSpace(guest.Name()), resourceType: resourceType,
		kind: kind, status: guest.Status(), node: guest.Node(), instance: guest.Instance(), vmid: guest.VMID(), template: guest.Template(), lock: strings.TrimSpace(guest.Lock()),
		observedAt: freshness.LastSeen.UTC(), sourceFresh: ok && strings.EqualFold(strings.TrimSpace(freshness.Status), "online") && !freshness.LastSeen.IsZero(),
		capabilities: guest.Capabilities(),
	}
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

// buildProxmoxGuestStoppedFinding builds the finding for a guest Pulse saw
// stop, with what the node's task log said about why.
func buildProxmoxGuestStoppedFinding(snapshot proxmoxGuestLifecycleSnapshot, now time.Time, cause proxmoxGuestStopCause) *Finding {
	finding := buildProxmoxGuestStoppedFindingBase(snapshot, now)
	cause.applyTo(finding, snapshot)
	return finding
}

func buildProxmoxGuestStoppedFindingBase(snapshot proxmoxGuestLifecycleSnapshot, now time.Time) *Finding {
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
