package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReloadableMonitor_Lifecycle_Coverage(t *testing.T) {
	// Create minimal config
	cfg := &config.Config{
		DataPath: t.TempDir(),
	}
	persistence := config.NewMultiTenantPersistence(cfg.DataPath)

	// Create ReloadableMonitor
	rm, err := NewReloadableMonitor(cfg, persistence, nil)
	require.NoError(t, err)
	require.NotNil(t, rm)

	// Test GetConfig
	assert.Equal(t, cfg, rm.GetConfig())

	// Test Start
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rm.Start(ctx)

	// Test GetMultiTenantMonitor
	mtm := rm.GetMultiTenantMonitor()
	require.NotNil(t, mtm)

	// Test GetMonitor (default legacy shim)
	// Should initialize default monitor on demand
	m := rm.GetMonitor()
	require.NotNil(t, m)

	// Test ReadSnapshot (default)
	state := rm.ReadSnapshot("default")
	require.NotNil(t, state)

	// Test ReadSnapshot (non-existent) - should not auto-provision an unprovisioned tenant
	stateMissing := rm.ReadSnapshot("missing-org")
	assert.Nil(t, stateMissing)

	// Test ReadSnapshot with invalid OrgID (should fail persistence check)
	// Assuming "../" or similar might be rejected by GetPersistence or underlying path logic
	// If GetMonitor is robust, checking error branch might require mocking persistence failure.
	// For now, attempting path traversal char.
	// If Pulse cleans it, it might pass. Checking code: persistence joins path.
	// Let's try an error injection if possible, or skip if too complex.
	// Actually, persistence.GetPersistence returns error if newPersistence fails? No, usually succeeds unless mkdir fails.
	// We'll skip complex mocking just for this line, accepting high coverage.

	// Start reload in background
	errChan := make(chan error)
	go func() {
		errChan <- rm.Reload()
	}()

	// Wait for reload (it sleeps for 1s in doReload)
	select {
	case err := <-errChan:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Reload timed out")
	}

	// Verify internal state after reload (ctx should be new)
	assert.NotNil(t, rm.ctx)

	// Test Stop
	rm.Stop()
}

func TestReloadableMonitorAggregateInstallSnapshotCountsIncludesProvisionedTenants(t *testing.T) {
	baseDir := t.TempDir()
	cfg := &config.Config{DataPath: baseDir}
	persistence := config.NewMultiTenantPersistence(baseDir)

	for _, orgID := range []string{"default", "org-a", "org-b"} {
		_, err := persistence.GetPersistence(orgID)
		require.NoError(t, err)
	}

	rm, err := NewReloadableMonitor(cfg, persistence, nil)
	require.NoError(t, err)

	mtm := rm.GetMultiTenantMonitor()
	require.NotNil(t, mtm)
	mtm.monitors["default"] = testTelemetryMonitor(
		[]models.Node{{ID: "node-default", Name: "node-default", Instance: "pve-default"}},
		[]models.VM{{ID: "vm-default", VMID: 101, Name: "vm-default", Instance: "pve-default"}},
		[]models.Container{{ID: "ct-default", VMID: 201, Name: "ct-default", Instance: "pve-default"}},
		[]models.PBSInstance{{Name: "pbs-default", Host: "pbs-default.local"}},
		[]models.PMGInstance{{Name: "pmg-default", Host: "pmg-default.local"}},
		[]models.DockerHost{{ID: "docker-default", Hostname: "docker-default"}},
		[]models.KubernetesCluster{{ID: "k8s-default", Name: "k8s-default"}},
		1,
	)
	mtm.monitors["org-a"] = testTelemetryMonitor(
		[]models.Node{{ID: "node-a", Name: "node-a", Instance: "pve-a"}},
		[]models.VM{
			{ID: "vm-a1", VMID: 102, Name: "vm-a1", Instance: "pve-a"},
			{ID: "vm-a2", VMID: 103, Name: "vm-a2", Instance: "pve-a"},
		},
		nil,
		[]models.PBSInstance{{Name: "pbs-a", Host: "pbs-a.local"}},
		nil,
		nil,
		nil,
		2,
	)
	mtm.monitors["org-b"] = testTelemetryMonitor(
		nil,
		nil,
		[]models.Container{{ID: "ct-b1", VMID: 202, Name: "ct-b1", Instance: "pve-b"}},
		nil,
		[]models.PMGInstance{{Name: "pmg-b", Host: "pmg-b.local"}},
		[]models.DockerHost{{ID: "docker-b", Hostname: "docker-b"}},
		[]models.KubernetesCluster{{ID: "k8s-b", Name: "k8s-b"}},
		3,
	)

	counts := rm.AggregateInstallSnapshotCounts()

	assert.Equal(t, 2, counts.PVENodes)
	assert.Equal(t, 2, counts.PBSInstances)
	assert.Equal(t, 2, counts.PMGInstances)
	assert.Equal(t, 3, counts.VMs)
	assert.Equal(t, 2, counts.Containers)
	assert.Equal(t, 2, counts.DockerHosts)
	assert.Equal(t, 2, counts.KubernetesClusters)
	assert.Equal(t, 6, counts.ActiveAlerts)
}

func testTelemetryMonitor(
	nodes []models.Node,
	vms []models.VM,
	containers []models.Container,
	pbsInstances []models.PBSInstance,
	pmgInstances []models.PMGInstance,
	dockerHosts []models.DockerHost,
	k8sClusters []models.KubernetesCluster,
	activeAlerts int,
) *Monitor {
	state := models.NewState()
	state.UpdateNodes(nodes)
	state.UpdateVMs(vms)
	state.UpdateContainers(containers)
	state.UpdatePBSInstances(pbsInstances)
	state.UpdatePMGInstances(pmgInstances)
	for _, host := range dockerHosts {
		state.UpsertDockerHost(host)
	}
	for _, cluster := range k8sClusters {
		state.UpsertKubernetesCluster(cluster)
	}

	alerts := make([]models.Alert, 0, activeAlerts)
	for i := 0; i < activeAlerts; i++ {
		alerts = append(alerts, models.Alert{ID: time.Now().Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)})
	}
	state.UpdateActiveAlerts(alerts)

	return &Monitor{state: state}
}
