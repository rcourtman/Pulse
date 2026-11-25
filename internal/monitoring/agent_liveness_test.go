package monitoring

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAgentLivenessSweepDoesNotCloneHealthyInventories(t *testing.T) {
	empty, now := agentLivenessFixture(0)
	busy, _ := agentLivenessFixture(50)
	for _, tc := range []struct {
		name string
		run  func(*Monitor, time.Time)
	}{
		{"docker", (*Monitor).evaluateDockerAgents},
		{"kubernetes", (*Monitor).evaluateKubernetesAgents},
		{"host", (*Monitor).evaluateHostAgents},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Isolate the state sweep from alert manager background workers.
			// AllocsPerRun is process-wide; take the lowest of three fixed
			// samples so unrelated background allocations cannot decide this.
			sample := func(m *Monitor) float64 {
				run := func() { tc.run(m, now) }
				return min(testing.AllocsPerRun(25, run), testing.AllocsPerRun(25, run), testing.AllocsPerRun(25, run))
			}
			emptyAllocs := sample(empty)
			busyAllocs := sample(busy)
			t.Logf("allocations per health check: empty=%.0f populated=%.0f", emptyAllocs, busyAllocs)
			// Only the number of reporting agents, not the size of their
			// inventories, should determine the cost of a healthy sweep.
			require.LessOrEqual(t, busyAllocs, emptyAllocs+1)
		})
	}
}

func TestAgentLivenessSweepPreservesExpiryAndRecovery(t *testing.T) {
	now := time.Now()
	m := &Monitor{state: models.NewState(), alertManager: alerts.NewManagerWithDataDir(t.TempDir())}
	t.Cleanup(m.alertManager.Stop)
	// With a 15 s report interval the existing Docker/Kubernetes lease is
	// 60 s and the host lease is 90 s. Equality is still considered current.
	docker := models.DockerHost{
		ID: "docker-1", DisplayName: "Fixture Docker", Hostname: "docker-1",
		AgentID: "fixture-docker-agent", Status: "online", IntervalSeconds: 15,
		LastSeen:   now.Add(-60 * time.Second),
		Containers: []models.DockerContainer{{ID: "container-1", Image: "fixture"}},
	}
	cluster := models.KubernetesCluster{
		ID: "k8s-1", Name: "Fixture Kubernetes", Status: "online", IntervalSeconds: 15,
		LastSeen: now.Add(-60 * time.Second),
		Pods:     []models.KubernetesPod{{UID: "pod-1", Name: "fixture", Namespace: "default"}},
	}
	host := models.Host{
		ID: "host-1", Hostname: "host-1", Status: "online", IntervalSeconds: 15,
		LastSeen: now.Add(-90 * time.Second),
		RAID:     []models.HostRAIDArray{{Device: "md0", Operation: "resync", RebuildPercent: 35}},
	}
	publish := func() {
		m.state.UpsertDockerHost(docker)
		m.state.UpsertKubernetesCluster(cluster)
		m.state.UpsertHost(host)
	}
	publish()

	checkRows := func(healthy bool) models.StateSnapshot {
		t.Helper()
		s := m.state.GetSnapshot()
		status := "offline"
		if healthy {
			status = "online"
		}
		require.Equal(t, map[string]bool{
			dockerConnectionPrefix + docker.ID:      healthy,
			kubernetesConnectionPrefix + cluster.ID: healthy,
			hostConnectionPrefix + host.ID:          healthy,
		}, s.ConnectionHealth)
		require.Len(t, s.DockerHosts, 1)
		require.Len(t, s.KubernetesClusters, 1)
		require.Len(t, s.Hosts, 1)
		require.Equal(t, status, s.DockerHosts[0].Status)
		require.Equal(t, status, s.KubernetesClusters[0].Status)
		require.Equal(t, status, s.Hosts[0].Status)
		require.Equal(t, docker.LastSeen, s.DockerHosts[0].LastSeen)
		require.Equal(t, cluster.LastSeen, s.KubernetesClusters[0].LastSeen)
		require.Equal(t, host.LastSeen, s.Hosts[0].LastSeen)
		require.Equal(t, docker.Containers[0].ID, s.DockerHosts[0].Containers[0].ID)
		require.Equal(t, cluster.Pods[0].UID, s.KubernetesClusters[0].Pods[0].UID)
		require.Equal(t, host.RAID[0].Device, s.Hosts[0].RAID[0].Device)
		return s
	}

	sweepAgentLivenessFixture(m, now)
	s := checkRows(true)
	require.Equal(t, "resync", s.Hosts[0].RAID[0].Operation)
	require.Empty(t, m.alertManager.GetActiveAlerts())
	for i := 1; i <= 3; i++ {
		sweepAgentLivenessFixture(m, now.Add(time.Duration(i)*time.Second))
		if i < 3 {
			require.Empty(t, m.alertManager.GetActiveAlerts(), "legacy confirmation count changed")
		}
	}
	s = checkRows(false)
	require.Empty(t, s.Hosts[0].RAID[0].Operation)
	require.Zero(t, s.Hosts[0].RAID[0].RebuildPercent)
	active := m.alertManager.GetActiveAlerts()
	require.Len(t, active, 2)
	byResource := make(map[string]alerts.Alert, len(active))
	for _, alert := range active {
		byResource[alert.ResourceID] = alert
	}
	dockerOffline, ok := byResource["docker:"+docker.ID]
	require.True(t, ok, "Docker connectivity alert missing")
	require.Equal(t, "docker-host-offline", dockerOffline.Type)
	require.Equal(t, docker.DisplayName, dockerOffline.ResourceName)
	require.Equal(t, docker.AgentID, dockerOffline.Metadata["agentId"])
	hostOffline, ok := byResource["agent:"+host.ID]
	require.True(t, ok, "host connectivity alert missing")
	require.Equal(t, alerts.HostOfflineAlertType, hostOffline.Type)
	require.Equal(t, "Pulse agent on 'host-1' stopped reporting", hostOffline.Message)

	recovered := now.Add(4 * time.Second)
	docker.LastSeen, cluster.LastSeen, host.LastSeen = recovered, recovered, recovered
	publish()
	sweepAgentLivenessFixture(m, recovered)
	s = checkRows(true)
	require.Equal(t, "resync", s.Hosts[0].RAID[0].Operation)
	require.Empty(t, m.alertManager.GetActiveAlerts())
	t.Log("equal-to-lease reports stayed online; stale rows retained inventory, expired transient RAID evidence and opened both confirmed alerts; fresh reports restored the rows and cleared both alerts")
}

func BenchmarkAgentLivenessSweep(b *testing.B) {
	for _, workloadCount := range []int{0, 50} {
		name := "empty"
		if workloadCount > 0 {
			name = "populated"
		}
		b.Run(name, func(b *testing.B) {
			m, now := agentLivenessFixture(workloadCount)
			m.alertManager = alerts.NewManagerWithDataDir(b.TempDir())
			b.Cleanup(m.alertManager.Stop)
			sweepAgentLivenessFixture(m, now)
			b.ReportAllocs()
			for b.Loop() {
				sweepAgentLivenessFixture(m, now)
			}
		})
	}
}

func sweepAgentLivenessFixture(m *Monitor, now time.Time) {
	m.evaluateDockerAgents(now)
	m.evaluateKubernetesAgents(now)
	m.evaluateHostAgents(now)
}

func agentLivenessFixture(workloads int) (*Monitor, time.Time) {
	m := &Monitor{state: models.NewState()}
	now := time.Unix(1780000000, 0).UTC()
	for i := 0; i < 6; i++ {
		host := models.DockerHost{
			ID: fmt.Sprintf("docker-%d", i), Status: "online", IntervalSeconds: 15, LastSeen: now,
		}
		for j := 0; j < workloads; j++ {
			host.Containers = append(host.Containers, models.DockerContainer{
				ID:    fmt.Sprintf("container-%d", j),
				Image: "fixture:v1", State: "running",
				Labels: map[string]string{"com.docker.compose.service": "fixture"},
			})
		}
		m.state.UpsertDockerHost(host)
	}
	for i := 0; i < 3; i++ {
		cluster := models.KubernetesCluster{
			ID: fmt.Sprintf("cluster-%d", i), Status: "online", IntervalSeconds: 15, LastSeen: now,
		}
		for j := 0; j < workloads*4; j++ {
			name := fmt.Sprintf("pod-%d", j)
			cluster.Pods = append(cluster.Pods, models.KubernetesPod{
				UID: name, Name: name, Namespace: "default", Phase: "Running",
				Labels:     map[string]string{"app": "fixture"},
				Containers: []models.KubernetesPodContainer{{Name: "app", Ready: true}},
			})
		}
		m.state.UpsertKubernetesCluster(cluster)
	}
	for i := 0; i < 12; i++ {
		host := models.Host{
			ID: fmt.Sprintf("host-%d", i), Status: "online", IntervalSeconds: 15, LastSeen: now,
		}
		if workloads > 0 {
			host.Libvirt = &models.HostLibvirtInventory{CollectedAt: now}
			for j := 0; j < workloads/2; j++ {
				host.Libvirt.Domains = append(host.Libvirt.Domains, models.HostLibvirtDomain{
					ID: fmt.Sprintf("vm-%d", j), State: "running",
				})
			}
		}
		m.state.UpsertHost(host)
	}
	return m, now
}
