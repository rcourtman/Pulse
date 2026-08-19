package ai

import (
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// dockerAlertScopeAlias mirrors the alert subsystem's docker resource IDs
// ("docker:<host>" and "docker:<host>/<container>") so an alert's resource ID
// resolves to the patrol runtime record for that host or container (#1699).
// Returns "" without a host part rather than the shared "docker:unknown"
// fallback, which would alias unrelated records together.
func dockerAlertScopeAlias(hostID, containerID string) string {
	if strings.TrimSpace(hostID) == "" {
		return ""
	}
	return alerts.DockerResourceID(hostID, containerID)
}

// dockerServiceAlertScopeAliases mirrors the alert subsystem's Swarm service
// resource IDs ("docker:<host>/service/<serviceID>") onto the owning Docker
// host record. Patrol has no service-level analysis, and a service's tasks run
// as containers on the host, so the host is the smallest unit Patrol can
// investigate for a service alert (#1699). Hosts without an ID are skipped for
// the same anti-aliasing reason as dockerAlertScopeAlias: the host-less
// "docker-service:<name>" fallback could collide across hosts.
func dockerServiceAlertScopeAliases(hostID string, services []models.DockerService) []string {
	if strings.TrimSpace(hostID) == "" || len(services) == 0 {
		return nil
	}
	aliases := make([]string, 0, len(services))
	for _, service := range services {
		aliases = append(aliases, alerts.DockerServiceResourceID(hostID, service.ID, service.Name))
	}
	return aliases
}

type patrolRuntimeResourceKind string

const (
	patrolRuntimeResourceNode         patrolRuntimeResourceKind = "node"
	patrolRuntimeResourceVM           patrolRuntimeResourceKind = "vm"
	patrolRuntimeResourceContainer    patrolRuntimeResourceKind = "container"
	patrolRuntimeResourceStorage      patrolRuntimeResourceKind = "storage"
	patrolRuntimeResourceDockerHost   patrolRuntimeResourceKind = "docker-host"
	patrolRuntimeResourceDockerItem   patrolRuntimeResourceKind = "docker-container"
	patrolRuntimeResourceHost         patrolRuntimeResourceKind = "host"
	patrolRuntimeResourceTrueNAS      patrolRuntimeResourceKind = "truenas"
	patrolRuntimeResourcePBS          patrolRuntimeResourceKind = "pbs"
	patrolRuntimeResourcePMG          patrolRuntimeResourceKind = "pmg"
	patrolRuntimeResourceK8sCluster   patrolRuntimeResourceKind = "k8s-cluster"
	patrolRuntimeResourcePhysicalDisk patrolRuntimeResourceKind = "physical-disk"
	// patrolRuntimeResourceUnified covers canonical resource kinds that do not
	// yet have a legacy StateSnapshot projection (for example availability
	// network endpoints). These resources remain exact-ID scoped and are
	// supplied to Patrol through the unified-resource provider only.
	patrolRuntimeResourceUnified patrolRuntimeResourceKind = "unified-resource"
)

type patrolRuntimeResourceRecord struct {
	kind         patrolRuntimeResourceKind
	resourceType string
	ids          []string
	aliases      []string
}

// patrolRuntimeState is the patrol subsystem's internal runtime state contract.
// It intentionally includes only the fields patrol currently consumes, so the
// subsystem can stop mirroring the full global StateSnapshot shape.
type patrolRuntimeState struct {
	readState               unifiedresources.ReadState
	unifiedResourceProvider UnifiedResourceProvider
	Nodes                   []models.Node
	VMs                     []models.VM
	Containers              []models.Container
	PhysicalDisks           []models.PhysicalDisk
	DockerHosts             []models.DockerHost
	KubernetesClusters      []models.KubernetesCluster
	Hosts                   []models.Host
	Storage                 []models.Storage
	PBSInstances            []models.PBSInstance
	PMGInstances            []models.PMGInstance
	PBSBackups              []models.PBSBackup
	PVEBackups              models.PVEBackups
	ConnectionHealth        map[string]bool
	ActiveAlerts            []models.Alert
	RecentlyResolved        []models.ResolvedAlert
}

func newPatrolRuntimeState(snapshot models.StateSnapshot) patrolRuntimeState {
	return newPatrolRuntimeStateWithProviders(snapshot, nil, nil)
}

func newPatrolRuntimeStateWithProviders(
	snapshot models.StateSnapshot,
	readState unifiedresources.ReadState,
	unifiedResourceProvider UnifiedResourceProvider,
) patrolRuntimeState {
	snapshot.NormalizeCollections()
	return patrolRuntimeState{
		readState:               readState,
		unifiedResourceProvider: unifiedResourceProvider,
		Nodes:                   snapshot.Nodes,
		VMs:                     snapshot.VMs,
		Containers:              snapshot.Containers,
		PhysicalDisks:           snapshot.PhysicalDisks,
		DockerHosts:             snapshot.DockerHosts,
		KubernetesClusters:      snapshot.KubernetesClusters,
		Hosts:                   snapshot.Hosts,
		Storage:                 snapshot.Storage,
		PBSInstances:            snapshot.PBSInstances,
		PMGInstances:            snapshot.PMGInstances,
		PBSBackups:              snapshot.PBSBackups,
		PVEBackups:              snapshot.PVEBackups,
		ConnectionHealth:        snapshot.ConnectionHealth,
		ActiveAlerts:            snapshot.ActiveAlerts,
		RecentlyResolved:        snapshot.RecentlyResolved,
	}
}

func emptyPatrolRuntimeState(
	readState unifiedresources.ReadState,
	unifiedResourceProvider UnifiedResourceProvider,
) patrolRuntimeState {
	return newPatrolRuntimeStateWithProviders(
		models.EmptyStateSnapshot(),
		readState,
		unifiedResourceProvider,
	)
}

func (s patrolRuntimeState) resourceSnapshot() models.StateSnapshot {
	snapshot := models.EmptyStateSnapshot()
	snapshot.Nodes = s.Nodes
	snapshot.VMs = s.VMs
	snapshot.Containers = s.Containers
	snapshot.PhysicalDisks = s.PhysicalDisks
	snapshot.DockerHosts = s.DockerHosts
	snapshot.KubernetesClusters = s.KubernetesClusters
	snapshot.Hosts = s.Hosts
	snapshot.Storage = s.Storage
	snapshot.PBSInstances = s.PBSInstances
	snapshot.PMGInstances = s.PMGInstances
	return snapshot
}

func (s patrolRuntimeState) withDerivedProviders() patrolRuntimeState {
	return s.withDerivedProvidersAndResources(nil)
}

func (s patrolRuntimeState) withDerivedProvidersAndResources(resources []unifiedresources.Resource) patrolRuntimeState {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(s.resourceSnapshot())
	if len(resources) > 0 {
		registry.IngestResources(resources)
	}
	s.readState = registry
	s.unifiedResourceProvider = unifiedresources.NewUnifiedAIAdapter(registry)
	return s
}

func (p *PatrolService) currentPatrolRuntimeState() patrolRuntimeState {
	if p == nil {
		return emptyPatrolRuntimeState(nil, nil)
	}
	p.mu.RLock()
	stateProvider := p.stateProvider
	readState := p.readState
	unifiedResourceProvider := p.unifiedResourceProvider
	p.mu.RUnlock()
	if stateProvider == nil {
		return emptyPatrolRuntimeState(readState, unifiedResourceProvider)
	}
	return newPatrolRuntimeStateWithProviders(stateProvider.ReadSnapshot(), readState, unifiedResourceProvider)
}

func (p *PatrolService) hasPatrolRuntimeInputs() bool {
	if p == nil {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stateProvider != nil || p.readState != nil || p.unifiedResourceProvider != nil
}

func (p *PatrolService) patrolRuntimeStateForSnapshot(snapshot models.StateSnapshot) patrolRuntimeState {
	if p == nil {
		return newPatrolRuntimeState(snapshot)
	}
	p.mu.RLock()
	readState := p.readState
	unifiedResourceProvider := p.unifiedResourceProvider
	p.mu.RUnlock()
	return newPatrolRuntimeStateWithProviders(snapshot, readState, unifiedResourceProvider)
}

func patrolVisitRuntimeResources(s patrolRuntimeState, visit func(patrolRuntimeResourceRecord) bool) {
	emittedIDs := make(map[string]struct{})
	emitTyped := func(kind patrolRuntimeResourceKind, resourceType string, ids []string, aliases ...string) bool {
		if resourceType == "" {
			resourceType = canonicalPatrolRuntimeResourceType(kind)
		}
		if !visit(patrolRuntimeResourceRecord{kind: kind, resourceType: resourceType, ids: ids, aliases: aliases}) {
			return false
		}
		for _, id := range ids {
			if token := canonicalPatrolScopeToken(id); token != "" {
				emittedIDs[token] = struct{}{}
			}
		}
		return true
	}
	emit := func(kind patrolRuntimeResourceKind, ids []string, aliases ...string) bool {
		return emitTyped(kind, "", ids, aliases...)
	}
	hostResourceKind := func(platform string) patrolRuntimeResourceKind {
		if strings.EqualFold(strings.TrimSpace(platform), "truenas") {
			return patrolRuntimeResourceTrueNAS
		}
		return patrolRuntimeResourceHost
	}

	if rs := s.readState; rs != nil {
		for _, n := range rs.Nodes() {
			if !emit(patrolRuntimeResourceNode, []string{n.ID(), n.SourceID()}, n.Name()) {
				return
			}
		}
		for _, vm := range rs.VMs() {
			if !emit(patrolRuntimeResourceVM, []string{vm.ID(), vm.SourceID()}, vm.Name()) {
				return
			}
		}
		for _, ct := range rs.Containers() {
			if !emit(patrolRuntimeResourceContainer, []string{ct.ID(), ct.SourceID()}, ct.Name()) {
				return
			}
		}
		for _, storage := range rs.StoragePools() {
			if !emit(patrolRuntimeResourceStorage, []string{storage.ID(), storage.SourceID()}, storage.Name()) {
				return
			}
		}
		for _, dh := range rs.DockerHosts() {
			aliases := append([]string{dh.Name(), dh.Hostname(), dockerAlertScopeAlias(dh.HostSourceID(), "")},
				dockerServiceAlertScopeAliases(dh.HostSourceID(), dh.Services())...)
			if !emit(patrolRuntimeResourceDockerHost, []string{dh.ID(), dh.HostSourceID()}, aliases...) {
				return
			}
		}
		for _, dc := range rs.DockerContainers() {
			if !emit(patrolRuntimeResourceDockerItem, []string{dc.ID(), dc.ContainerID()}, dc.Name(), dockerAlertScopeAlias(dc.HostSourceID(), dc.ContainerID())) {
				return
			}
		}
		for _, h := range rs.Hosts() {
			if !emit(hostResourceKind(h.Platform()), []string{h.ID()}, h.Name(), h.Hostname()) {
				return
			}
		}
		for _, pbs := range rs.PBSInstances() {
			if !emit(patrolRuntimeResourcePBS, []string{pbs.ID()}, pbs.Name()) {
				return
			}
		}
		for _, pmg := range rs.PMGInstances() {
			if !emit(patrolRuntimeResourcePMG, []string{pmg.ID()}, pmg.Name()) {
				return
			}
		}
		for _, k := range rs.K8sClusters() {
			if !emit(patrolRuntimeResourceK8sCluster, []string{k.ID()}, k.Name()) {
				return
			}
		}
	} else {
		for _, n := range s.Nodes {
			if !emit(patrolRuntimeResourceNode, []string{n.ID}, n.Name) {
				return
			}
		}
		for _, vm := range s.VMs {
			if !emit(patrolRuntimeResourceVM, []string{vm.ID}, vm.Name) {
				return
			}
		}
		for _, ct := range s.Containers {
			if !emit(patrolRuntimeResourceContainer, []string{ct.ID}, ct.Name) {
				return
			}
		}
		for _, storage := range s.Storage {
			if !emit(patrolRuntimeResourceStorage, []string{storage.ID}, storage.Name) {
				return
			}
		}
		for _, dh := range s.DockerHosts {
			aliases := append([]string{dh.DisplayName, dh.CustomDisplayName, dh.Hostname, dockerAlertScopeAlias(dh.ID, "")},
				dockerServiceAlertScopeAliases(dh.ID, dh.Services)...)
			if !emit(patrolRuntimeResourceDockerHost, []string{dh.ID}, aliases...) {
				return
			}
			for _, dc := range dh.Containers {
				if !emit(patrolRuntimeResourceDockerItem, []string{dc.ID}, dc.Name, dockerAlertScopeAlias(dh.ID, dc.ID)) {
					return
				}
			}
		}
		for _, h := range s.Hosts {
			if !emit(hostResourceKind(h.Platform), []string{h.ID}, h.DisplayName, h.Hostname) {
				return
			}
		}
		for _, pbs := range s.PBSInstances {
			if !emit(patrolRuntimeResourcePBS, []string{pbs.ID}, pbs.Name, pbs.Host) {
				return
			}
		}
		for _, pmg := range s.PMGInstances {
			if !emit(patrolRuntimeResourcePMG, []string{pmg.ID}, pmg.Name, pmg.Host) {
				return
			}
		}
		for _, k := range s.KubernetesClusters {
			if !emit(patrolRuntimeResourceK8sCluster, []string{k.ID}, k.Name, k.DisplayName, k.CustomDisplayName) {
				return
			}
		}
	}

	if s.unifiedResourceProvider != nil {
		for _, disk := range s.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypePhysicalDisk) {
			ids := []string{disk.ID}
			aliases := []string{disk.Name}
			if disk.PhysicalDisk != nil {
				if strings.TrimSpace(disk.PhysicalDisk.DevPath) != "" {
					ids = append(ids, disk.PhysicalDisk.DevPath)
				}
				aliases = append(aliases, disk.PhysicalDisk.Model)
			}
			if !emit(patrolRuntimeResourcePhysicalDisk, ids, aliases...) {
				return
			}
		}
		for _, resource := range s.unifiedResourceProvider.GetAll() {
			if _, alreadyEmitted := emittedIDs[canonicalPatrolScopeToken(resource.ID)]; alreadyEmitted {
				continue
			}
			ids, aliases := patrolUnifiedResourceIdentity(resource)
			if !emitTyped(patrolRuntimeResourceUnified, string(resource.Type), ids, aliases...) {
				return
			}
		}
		return
	}

	for _, disk := range s.PhysicalDisks {
		if !emit(patrolRuntimeResourcePhysicalDisk, []string{disk.ID, disk.DevPath}, disk.Model) {
			return
		}
	}
}

func patrolVisitRuntimeSupplementalSnapshotFields(s patrolRuntimeState, visit func(ids []string, aliases []string) bool) {
	emit := func(ids []string, aliases ...string) bool {
		return visit(ids, aliases)
	}

	for _, n := range s.Nodes {
		if !emit([]string{n.ID}, n.Name) {
			return
		}
	}
	for _, vm := range s.VMs {
		if !emit([]string{vm.ID}, vm.Name) {
			return
		}
	}
	for _, ct := range s.Containers {
		if !emit([]string{ct.ID}, ct.Name) {
			return
		}
	}
	for _, storage := range s.Storage {
		if !emit([]string{storage.ID}, storage.Name) {
			return
		}
	}
	for _, dh := range s.DockerHosts {
		if !emit([]string{dh.ID}, dh.DisplayName, dh.CustomDisplayName, dh.Hostname) {
			return
		}
		for _, dc := range dh.Containers {
			if !emit([]string{dc.ID}, dc.Name) {
				return
			}
		}
	}
	for _, h := range s.Hosts {
		if !emit([]string{h.ID}, h.DisplayName, h.Hostname) {
			return
		}
	}
	for _, pbs := range s.PBSInstances {
		if !emit([]string{pbs.ID}, pbs.Name, pbs.Host) {
			return
		}
	}
	for _, pmg := range s.PMGInstances {
		if !emit([]string{pmg.ID}, pmg.Name, pmg.Host) {
			return
		}
	}
	for _, k := range s.KubernetesClusters {
		if !emit([]string{k.ID}, k.Name, k.DisplayName, k.CustomDisplayName) {
			return
		}
	}
	for _, disk := range s.PhysicalDisks {
		if !emit([]string{disk.ID, disk.DevPath}, disk.Model) {
			return
		}
	}
}

func patrolRuntimeKnownResources(s patrolRuntimeState) map[string]bool {
	known := make(map[string]bool)
	add := func(values ...string) {
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				known[value] = true
			}
		}
	}

	patrolVisitRuntimeResources(s, func(record patrolRuntimeResourceRecord) bool {
		add(record.ids...)
		add(record.aliases...)
		return true
	})
	patrolVisitRuntimeSupplementalSnapshotFields(s, func(ids []string, aliases []string) bool {
		add(ids...)
		add(aliases...)
		return true
	})

	return known
}

func patrolRuntimeResourceIDs(s patrolRuntimeState) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	patrolVisitRuntimeResources(s, func(record patrolRuntimeResourceRecord) bool {
		for _, id := range record.ids {
			add(id)
		}
		return true
	})
	patrolVisitRuntimeSupplementalSnapshotFields(s, func(ids []string, _ []string) bool {
		for _, id := range ids {
			add(id)
		}
		return true
	})

	return ids
}

func patrolRuntimeSortedResourceIDs(s patrolRuntimeState) []string {
	ids := patrolRuntimeResourceIDs(s)
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)
	return ids
}

func canonicalPatrolScopeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func canonicalPatrolRuntimeResourceType(kind patrolRuntimeResourceKind) string {
	switch kind {
	case patrolRuntimeResourceContainer:
		return "system-container"
	case patrolRuntimeResourceDockerItem:
		return "app-container"
	case patrolRuntimeResourcePhysicalDisk:
		return "physical-disk"
	case patrolRuntimeResourceHost:
		return "agent"
	case patrolRuntimeResourceTrueNAS:
		return "truenas"
	case patrolRuntimeResourceUnified:
		return ""
	default:
		return string(kind)
	}
}

// resolvePatrolScopeState expands canonical unified IDs, source IDs, and
// unique known aliases onto the complete identity set for the matching Patrol
// runtime record. It never fuzzy-matches and refuses ambiguous aliases.
func resolvePatrolScopeState(state patrolRuntimeState, scope PatrolScope) (PatrolScope, PatrolScopeResolution) {
	// Recompute private type evidence from the current runtime snapshot on every
	// resolution pass; callers may synchronously pre-resolve and the accepted run
	// resolves again to close collection races.
	scope.resolvedResourceTypes = nil
	resolution := PatrolScopeResolution{}
	for _, requested := range scope.ResourceIDs {
		if requested = strings.TrimSpace(requested); requested != "" {
			resolution.RequestedResourceIDs = append(resolution.RequestedResourceIDs, requested)
		}
	}
	if len(resolution.RequestedResourceIDs) == 0 {
		filtered := filterPatrolStateByScopeState(state, scope)
		resolution.EffectiveResourceIDs = patrolRuntimeSortedResourceIDs(filtered)
		return scope, resolution
	}

	type identityRecord struct {
		ids          []string
		aliases      []string
		resourceType string
		idTokens     map[string]bool
		aliasTokens  map[string]bool
	}
	records := make([]identityRecord, 0)
	patrolVisitRuntimeResources(state, func(record patrolRuntimeResourceRecord) bool {
		candidate := identityRecord{
			ids: append([]string(nil), record.ids...), aliases: append([]string(nil), record.aliases...),
			resourceType: record.resourceType,
			idTokens:     make(map[string]bool), aliasTokens: make(map[string]bool),
		}
		for _, value := range record.ids {
			if token := canonicalPatrolScopeToken(value); token != "" {
				candidate.idTokens[token] = true
			}
		}
		for _, value := range record.aliases {
			if token := canonicalPatrolScopeToken(value); token != "" {
				candidate.aliasTokens[token] = true
			}
		}
		if len(candidate.idTokens)+len(candidate.aliasTokens) > 0 {
			records = append(records, candidate)
		}
		return true
	})

	expanded := make([]string, 0, len(scope.ResourceIDs))
	seenExpanded := make(map[string]bool)
	seenResolved := make(map[string]bool)
	seenResolvedTypes := make(map[string]bool)
	addExpanded := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !seenExpanded[value] {
			seenExpanded[value] = true
			expanded = append(expanded, value)
		}
	}
	addResolved := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !seenResolved[value] {
			seenResolved[value] = true
			resolution.ResolvedResourceIDs = append(resolution.ResolvedResourceIDs, value)
		}
	}
	addResolvedType := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !seenResolvedTypes[value] {
			seenResolvedTypes[value] = true
			scope.resolvedResourceTypes = append(scope.resolvedResourceTypes, value)
		}
	}
	for _, requested := range resolution.RequestedResourceIDs {
		token := canonicalPatrolScopeToken(requested)
		matches := make([]identityRecord, 0, 1)
		for _, record := range records {
			if record.idTokens[token] {
				matches = append(matches, record)
			}
		}
		// A canonical or source ID is authoritative. Only fall back to known
		// display aliases when no exact identity field matched.
		if len(matches) == 0 {
			for _, record := range records {
				if record.aliasTokens[token] {
					matches = append(matches, record)
				}
			}
		}
		switch len(matches) {
		case 0:
			resolution.UnmatchedResourceIDs = append(resolution.UnmatchedResourceIDs, requested)
		case 1:
			for _, id := range matches[0].ids {
				addExpanded(id)
				addResolved(id)
			}
			addResolvedType(matches[0].resourceType)
		default:
			resolution.AmbiguousResourceIDs = append(resolution.AmbiguousResourceIDs, requested)
		}
	}
	scope.ResourceIDs = expanded
	scope.resolvedIdentityOnly = len(expanded) > 0
	if len(expanded) > 0 {
		filtered := filterPatrolStateByScopeState(state, scope)
		resolution.EffectiveResourceIDs = patrolRuntimeSortedResourceIDs(filtered)
	}
	sort.Strings(resolution.ResolvedResourceIDs)
	sort.Strings(scope.resolvedResourceTypes)
	sort.Strings(resolution.EffectiveResourceIDs)
	sort.Strings(resolution.UnmatchedResourceIDs)
	sort.Strings(resolution.AmbiguousResourceIDs)
	return scope, resolution
}

// ResolvePatrolScope resolves a scope against the current normal Patrol
// runtime state without starting a run. API callers use it for synchronous
// validation; the run resolves again to close collection races.
func (p *PatrolService) ResolvePatrolScope(scope PatrolScope) (PatrolScope, PatrolScopeResolution) {
	if p == nil || !p.hasPatrolRuntimeInputs() {
		return scope, PatrolScopeResolution{RequestedResourceIDs: append([]string(nil), scope.ResourceIDs...), UnmatchedResourceIDs: append([]string(nil), scope.ResourceIDs...)}
	}
	return resolvePatrolScopeState(p.currentPatrolRuntimeState(), scope)
}

type patrolRuntimeResourceCounts struct {
	nodes      int
	guests     int
	docker     int
	storage    int
	hosts      int
	truenas    int
	pbs        int
	pmg        int
	kubernetes int
	unified    int
}

func (c patrolRuntimeResourceCounts) total() int {
	return c.nodes + c.guests + c.docker + c.storage + c.hosts + c.truenas + c.pbs + c.pmg + c.kubernetes + c.unified
}

func patrolRuntimeCountResources(s patrolRuntimeState) patrolRuntimeResourceCounts {
	var counts patrolRuntimeResourceCounts
	patrolVisitRuntimeResources(s, func(record patrolRuntimeResourceRecord) bool {
		switch record.kind {
		case patrolRuntimeResourceNode:
			counts.nodes++
		case patrolRuntimeResourceVM, patrolRuntimeResourceContainer:
			counts.guests++
		case patrolRuntimeResourceDockerHost:
			counts.docker++
		case patrolRuntimeResourceStorage, patrolRuntimeResourcePhysicalDisk:
			counts.storage++
		case patrolRuntimeResourceHost:
			counts.hosts++
		case patrolRuntimeResourceTrueNAS:
			counts.truenas++
		case patrolRuntimeResourcePBS:
			counts.pbs++
		case patrolRuntimeResourcePMG:
			counts.pmg++
		case patrolRuntimeResourceK8sCluster:
			counts.kubernetes++
		case patrolRuntimeResourceUnified:
			counts.unified++
		}
		return true
	})
	return counts
}
