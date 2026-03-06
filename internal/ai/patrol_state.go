package ai

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

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
	Stats                   models.Stats
	ActiveAlerts            []models.Alert
	RecentlyResolved        []models.ResolvedAlert
	LastUpdate              time.Time
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
		Stats:                   snapshot.Stats,
		ActiveAlerts:            snapshot.ActiveAlerts,
		RecentlyResolved:        snapshot.RecentlyResolved,
		LastUpdate:              snapshot.LastUpdate,
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
		Stats:              s.Stats,
		ActiveAlerts:       s.ActiveAlerts,
		RecentlyResolved:   s.RecentlyResolved,
		LastUpdate:         s.LastUpdate,
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

func patrolRuntimeKnownResources(s patrolRuntimeState) map[string]bool {
	known := make(map[string]bool)
	add := func(values ...string) {
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				known[value] = true
			}
		}
	}

	if rs := s.readState; rs != nil {
		for _, n := range rs.Nodes() {
			add(n.ID(), n.Name())
		}
		for _, vm := range rs.VMs() {
			add(vm.ID(), vm.Name())
		}
		for _, ct := range rs.Containers() {
			add(ct.ID(), ct.Name())
		}
		for _, s := range rs.StoragePools() {
			add(s.ID(), s.Name())
		}
		for _, dh := range rs.DockerHosts() {
			add(dh.ID(), dh.Name(), dh.Hostname())
		}
		for _, dc := range rs.DockerContainers() {
			add(dc.ID(), dc.Name())
		}
		for _, h := range rs.Hosts() {
			add(h.ID(), h.Name(), h.Hostname())
		}
		for _, pbs := range rs.PBSInstances() {
			add(pbs.ID(), pbs.Name())
		}
		for _, pmg := range rs.PMGInstances() {
			add(pmg.ID(), pmg.Name())
		}
		for _, k := range rs.K8sClusters() {
			add(k.ID(), k.Name())
		}
	}

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

	if rs := s.readState; rs != nil {
		for _, n := range rs.Nodes() {
			add(n.ID())
		}
		for _, vm := range rs.VMs() {
			add(vm.ID())
		}
		for _, ct := range rs.Containers() {
			add(ct.ID())
		}
		for _, storage := range rs.StoragePools() {
			add(storage.ID())
		}
		for _, dh := range rs.DockerHosts() {
			add(dh.ID())
		}
		for _, dc := range rs.DockerContainers() {
			add(dc.ID())
		}
		for _, h := range rs.Hosts() {
			add(h.ID())
		}
		for _, pbs := range rs.PBSInstances() {
			add(pbs.ID())
		}
		for _, pmg := range rs.PMGInstances() {
			add(pmg.ID())
		}
		for _, k := range rs.K8sClusters() {
			add(k.ID())
		}
	}

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

	return ids
}
