package models

import "time"

// AgentReportLease defines a family's grace window for missing reports.
// A report exactly at the deadline is still current.
type AgentReportLease struct {
	MinimumWindow   time.Duration
	MaximumWindow   time.Duration
	GraceMultiplier int
}

// Window applies the reporting grace multiplier and bounds. A missing
// interval uses the minimum window as the default report interval.
func (l AgentReportLease) Window(intervalSeconds int) time.Duration {
	if intervalSeconds <= 0 {
		intervalSeconds = int(l.MinimumWindow / time.Second)
	}
	window := time.Duration(intervalSeconds) * time.Second * time.Duration(l.GraceMultiplier)
	return min(max(window, l.MinimumWindow), l.MaximumWindow)
}

func (l AgentReportLease) current(now, lastSeen time.Time, intervalSeconds int) bool {
	return !lastSeen.IsZero() && now.Sub(lastSeen) <= l.Window(intervalSeconds)
}

// DockerAgentLiveness carries connectivity evidence for one Docker host.
type DockerAgentLiveness struct {
	ID      string
	Expired *DockerHost // detached alert input; nil while the report is current
}

// DockerAgentLiveness reads reporting state without copying the inventory of a
// current host. An expired host still supplies the original alert input.
func (s *State) DockerAgentLiveness(now time.Time, lease AgentReportLease) []DockerAgentLiveness {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]DockerAgentLiveness, len(s.DockerHosts))
	for i := range s.DockerHosts {
		host := &s.DockerHosts[i]
		result[i].ID = host.ID
		if !lease.current(now, host.LastSeen, host.IntervalSeconds) {
			expired := cloneDockerHost(*host)
			result[i].Expired = &expired
		}
	}
	return result
}

// KubernetesAgentLiveness carries connectivity evidence for one cluster.
type KubernetesAgentLiveness struct {
	ID      string
	Current bool
}

// KubernetesAgentLiveness checks only cluster reports; no workload data is
// used to tell whether a cluster's agent has gone quiet.
func (s *State) KubernetesAgentLiveness(now time.Time, lease AgentReportLease) []KubernetesAgentLiveness {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]KubernetesAgentLiveness, len(s.KubernetesClusters))
	for i := range s.KubernetesClusters {
		cluster := &s.KubernetesClusters[i]
		result[i] = KubernetesAgentLiveness{
			ID:      cluster.ID,
			Current: lease.current(now, cluster.LastSeen, cluster.IntervalSeconds),
		}
	}
	return result
}

// HostAgentLiveness carries connectivity evidence for one host agent.
type HostAgentLiveness struct {
	ID, Hostname, Status string
	LastSeen             time.Time
	IntervalSeconds      int
	Expired              *Host // detached fallback if the lease cannot be expired in state
}

// HostAgentLiveness reads the connectivity state of current hosts without
// copying their telemetry. Keep detached evidence for the existing expiry
// and alert paths only for a host whose report is no longer current.
func (s *State) HostAgentLiveness(now time.Time, lease AgentReportLease) []HostAgentLiveness {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]HostAgentLiveness, len(s.Hosts))
	for i := range s.Hosts {
		host := &s.Hosts[i]
		result[i] = HostAgentLiveness{
			ID: host.ID, Hostname: host.Hostname, Status: host.Status,
			LastSeen: host.LastSeen, IntervalSeconds: host.IntervalSeconds,
		}
		if !lease.current(now, host.LastSeen, host.IntervalSeconds) {
			expired := cloneHost(*host)
			result[i].Expired = &expired
		}
	}
	return result
}
