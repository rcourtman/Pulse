package ai

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// patrolRuntimeState is the patrol subsystem's internal runtime state contract.
// It intentionally includes only the fields patrol currently consumes, so the
// subsystem can stop mirroring the full global StateSnapshot shape.
type patrolRuntimeState struct {
	Nodes              []models.Node
	VMs                []models.VM
	Containers         []models.Container
	PhysicalDisks      []models.PhysicalDisk
	DockerHosts        []models.DockerHost
	KubernetesClusters []models.KubernetesCluster
	Hosts              []models.Host
	Storage            []models.Storage
	PBSInstances       []models.PBSInstance
	PMGInstances       []models.PMGInstance
	PBSBackups         []models.PBSBackup
	PVEBackups         models.PVEBackups
	ConnectionHealth   map[string]bool
	Stats              models.Stats
	ActiveAlerts       []models.Alert
	RecentlyResolved   []models.ResolvedAlert
	LastUpdate         time.Time
}

func newPatrolRuntimeState(snapshot models.StateSnapshot) patrolRuntimeState {
	return patrolRuntimeState{
		Nodes:              snapshot.Nodes,
		VMs:                snapshot.VMs,
		Containers:         snapshot.Containers,
		PhysicalDisks:      snapshot.PhysicalDisks,
		DockerHosts:        snapshot.DockerHosts,
		KubernetesClusters: snapshot.KubernetesClusters,
		Hosts:              snapshot.Hosts,
		Storage:            snapshot.Storage,
		PBSInstances:       snapshot.PBSInstances,
		PMGInstances:       snapshot.PMGInstances,
		PBSBackups:         snapshot.PBSBackups,
		PVEBackups:         snapshot.PVEBackups,
		ConnectionHealth:   snapshot.ConnectionHealth,
		Stats:              snapshot.Stats,
		ActiveAlerts:       snapshot.ActiveAlerts,
		RecentlyResolved:   snapshot.RecentlyResolved,
		LastUpdate:         snapshot.LastUpdate,
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

func (p *PatrolService) currentPatrolRuntimeState() patrolRuntimeState {
	if p == nil || p.stateProvider == nil {
		return patrolRuntimeState{}
	}
	return newPatrolRuntimeState(p.stateProvider.ReadSnapshot())
}
