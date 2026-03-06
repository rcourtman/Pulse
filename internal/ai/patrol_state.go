package ai

import (
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type patrolRuntimeResourceKind string

const (
	patrolRuntimeResourceNode         patrolRuntimeResourceKind = "node"
	patrolRuntimeResourceVM           patrolRuntimeResourceKind = "vm"
	patrolRuntimeResourceContainer    patrolRuntimeResourceKind = "container"
	patrolRuntimeResourceStorage      patrolRuntimeResourceKind = "storage"
	patrolRuntimeResourceDockerHost   patrolRuntimeResourceKind = "docker-host"
	patrolRuntimeResourceDockerItem   patrolRuntimeResourceKind = "docker-container"
	patrolRuntimeResourceHost         patrolRuntimeResourceKind = "host"
	patrolRuntimeResourcePBS          patrolRuntimeResourceKind = "pbs"
	patrolRuntimeResourcePMG          patrolRuntimeResourceKind = "pmg"
	patrolRuntimeResourceK8sCluster   patrolRuntimeResourceKind = "k8s-cluster"
	patrolRuntimeResourcePhysicalDisk patrolRuntimeResourceKind = "physical-disk"
)

type patrolRuntimeResourceRecord struct {
	kind    patrolRuntimeResourceKind
	ids     []string
	aliases []string
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

func (s patrolRuntimeState) snapshot() models.StateSnapshot {
	return models.StateSnapshot{
		Nodes:              s.Nodes,
		VMs:                s.VMs,
		Containers:         s.Containers,
		PhysicalDisks:      s.PhysicalDisks,
		DockerHosts:        s.DockerHosts,
		KubernetesClusters: s.KubernetesClusters,
		Hosts:              s.Hosts,
		Storage:            s.Storage,
		PBSInstances:       s.PBSInstances,
		PMGInstances:       s.PMGInstances,
		PBSBackups:         s.PBSBackups,
		PVEBackups:         s.PVEBackups,
		ConnectionHealth:   s.ConnectionHealth,
		ActiveAlerts:       s.ActiveAlerts,
		RecentlyResolved:   s.RecentlyResolved,
	}
}

func (s patrolRuntimeState) withDerivedProviders() patrolRuntimeState {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(s.snapshot())
	s.readState = registry
	s.unifiedResourceProvider = unifiedresources.NewUnifiedAIAdapter(registry)
	return s
}

func (p *PatrolService) currentPatrolRuntimeState() patrolRuntimeState {
	if p == nil {
		return patrolRuntimeState{}
	}
	p.mu.RLock()
	stateProvider := p.stateProvider
	readState := p.readState
	unifiedResourceProvider := p.unifiedResourceProvider
	p.mu.RUnlock()
	if stateProvider == nil {
		return patrolRuntimeState{
			readState:               readState,
			unifiedResourceProvider: unifiedResourceProvider,
		}
	}
	return newPatrolRuntimeStateWithProviders(stateProvider.ReadSnapshot(), readState, unifiedResourceProvider)
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
	emit := func(kind patrolRuntimeResourceKind, ids []string, aliases ...string) bool {
		return visit(patrolRuntimeResourceRecord{kind: kind, ids: ids, aliases: aliases})
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
			if !emit(patrolRuntimeResourceDockerHost, []string{dh.ID(), dh.HostSourceID()}, dh.Name(), dh.Hostname()) {
				return
			}
		}
		for _, dc := range rs.DockerContainers() {
			if !emit(patrolRuntimeResourceDockerItem, []string{dc.ID(), dc.ContainerID()}, dc.Name()) {
				return
			}
		}
		for _, h := range rs.Hosts() {
			if !emit(patrolRuntimeResourceHost, []string{h.ID()}, h.Name(), h.Hostname()) {
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
			if !emit(patrolRuntimeResourceDockerHost, []string{dh.ID}, dh.DisplayName, dh.CustomDisplayName, dh.Hostname) {
				return
			}
			for _, dc := range dh.Containers {
				if !emit(patrolRuntimeResourceDockerItem, []string{dc.ID}, dc.Name) {
					return
				}
			}
		}
		for _, h := range s.Hosts {
			if !emit(patrolRuntimeResourceHost, []string{h.ID}, h.DisplayName, h.Hostname) {
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
		return
	}

	for _, disk := range s.PhysicalDisks {
		if !emit(patrolRuntimeResourcePhysicalDisk, []string{disk.ID, disk.DevPath}, disk.Model) {
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
	for _, n := range s.Nodes {
		add(n.ID, n.Name)
	}
	for _, vm := range s.VMs {
		add(vm.ID, vm.Name)
	}
	for _, ct := range s.Containers {
		add(ct.ID, ct.Name)
	}
	for _, storage := range s.Storage {
		add(storage.ID, storage.Name)
	}
	for _, dh := range s.DockerHosts {
		add(dh.ID, dh.DisplayName, dh.CustomDisplayName, dh.Hostname)
		for _, dc := range dh.Containers {
			add(dc.ID, dc.Name)
		}
	}
	for _, h := range s.Hosts {
		add(h.ID, h.DisplayName, h.Hostname)
	}
	for _, pbs := range s.PBSInstances {
		add(pbs.ID, pbs.Name, pbs.Host)
	}
	for _, pmg := range s.PMGInstances {
		add(pmg.ID, pmg.Name, pmg.Host)
	}
	for _, k := range s.KubernetesClusters {
		add(k.ID, k.Name, k.DisplayName, k.CustomDisplayName)
	}
	for _, disk := range s.PhysicalDisks {
		add(disk.ID, disk.DevPath, disk.Model)
	}

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
	for _, n := range s.Nodes {
		add(n.ID)
	}
	for _, vm := range s.VMs {
		add(vm.ID)
	}
	for _, ct := range s.Containers {
		add(ct.ID)
	}
	for _, storage := range s.Storage {
		add(storage.ID)
	}
	for _, dh := range s.DockerHosts {
		add(dh.ID)
		for _, dc := range dh.Containers {
			add(dc.ID)
		}
	}
	for _, h := range s.Hosts {
		add(h.ID)
	}
	for _, pbs := range s.PBSInstances {
		add(pbs.ID)
	}
	for _, pmg := range s.PMGInstances {
		add(pmg.ID)
	}
	for _, k := range s.KubernetesClusters {
		add(k.ID)
	}
	for _, disk := range s.PhysicalDisks {
		add(disk.ID)
		add(disk.DevPath)
	}

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

func patrolRuntimeStorageResourceCount(s patrolRuntimeState) int {
	count := 0
	patrolVisitRuntimeResources(s, func(record patrolRuntimeResourceRecord) bool {
		if record.kind == patrolRuntimeResourceStorage || record.kind == patrolRuntimeResourcePhysicalDisk {
			count++
		}
		return true
	})
	return count
}

type patrolRuntimeResourceCounts struct {
	nodes      int
	guests     int
	docker     int
	storage    int
	hosts      int
	pbs        int
	pmg        int
	kubernetes int
}

func (c patrolRuntimeResourceCounts) total() int {
	return c.nodes + c.guests + c.docker + c.storage + c.hosts + c.pbs + c.pmg + c.kubernetes
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
		case patrolRuntimeResourcePBS:
			counts.pbs++
		case patrolRuntimeResourcePMG:
			counts.pmg++
		case patrolRuntimeResourceK8sCluster:
			counts.kubernetes++
		}
		return true
	})
	return counts
}
