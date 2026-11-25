package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAgentReportLeaseRetainsDefaultBoundsAndDeadline(t *testing.T) {
	lease := AgentReportLease{MinimumWindow: 30 * time.Second, MaximumWindow: 10 * time.Minute, GraceMultiplier: 4}
	require.Equal(t, 120*time.Second, lease.Window(0))
	require.Equal(t, 120*time.Second, lease.Window(-1))
	require.Equal(t, 30*time.Second, lease.Window(1))
	require.Equal(t, 60*time.Second, lease.Window(15))
	require.Equal(t, 10*time.Minute, lease.Window(3600))

	now := time.Now()
	require.False(t, lease.current(now, time.Time{}, 15))
	require.True(t, lease.current(now, now.Add(-60*time.Second), 15))
	require.False(t, lease.current(now, now.Add(-60*time.Second-time.Nanosecond), 15))
}

func TestAgentLivenessDetachesExpiredAlertInputs(t *testing.T) {
	state := NewState()
	lease := AgentReportLease{MinimumWindow: 30 * time.Second, MaximumWindow: 10 * time.Minute, GraceMultiplier: 4}
	now := time.Now()
	docker := DockerHost{
		ID: "docker", LastSeen: now.Add(-time.Hour), IntervalSeconds: 15,
		Containers: []DockerContainer{{ID: "old", Labels: map[string]string{"app": "fixture"}}},
	}
	host := Host{
		ID: "host", LastSeen: now.Add(-time.Hour), IntervalSeconds: 15,
		RAID: []HostRAIDArray{{Device: "md0", Operation: "resync", RebuildPercent: 40}},
	}
	state.UpsertDockerHost(docker)
	state.UpsertHost(host)

	dockerLiveness := state.DockerAgentLiveness(now, lease)
	require.Len(t, dockerLiveness, 1)
	require.Equal(t, docker.ID, dockerLiveness[0].ID)
	require.NotNil(t, dockerLiveness[0].Expired)
	dockerLiveness[0].Expired.Containers[0].Labels["app"] = "mutation"
	savedDocker, ok := state.GetDockerHost(docker.ID)
	require.True(t, ok)
	require.Equal(t, "fixture", savedDocker.Containers[0].Labels["app"])

	hostLiveness := state.HostAgentLiveness(now, lease)
	require.Len(t, hostLiveness, 1)
	require.Equal(t, host.ID, hostLiveness[0].ID)
	require.NotNil(t, hostLiveness[0].Expired)
	hostLiveness[0].Expired.RAID[0].Operation = ""
	require.Equal(t, "resync", state.GetHosts()[0].RAID[0].Operation)

	docker.LastSeen, host.LastSeen = now, now
	state.UpsertDockerHost(docker)
	state.UpsertHost(host)
	require.Nil(t, state.DockerAgentLiveness(now, lease)[0].Expired)
	require.Nil(t, state.HostAgentLiveness(now, lease)[0].Expired)
}
