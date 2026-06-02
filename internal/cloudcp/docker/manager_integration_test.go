package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

func TestIntegrationTenantNetworksAreNotMutuallyReachable(t *testing.T) {
	if os.Getenv("PULSE_RUN_DOCKER_INTEGRATION") != "1" {
		t.Skip("set PULSE_RUN_DOCKER_INTEGRATION=1 to run live Docker tenant-network proof")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suffix := dockerIntegrationSuffix(t)
	providerNetwork := "pulse-it-provider-" + suffix
	tenantNetworkPrefix := "pulse-it-tenant-" + suffix

	mgr, err := NewManager(ManagerConfig{
		Image:                 "busybox:1.36",
		Network:               providerNetwork,
		IsolateTenantNetworks: true,
		TenantNetworkPrefix:   tenantNetworkPrefix,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	if _, err := mgr.cli.NetworkCreate(ctx, providerNetwork, client.NetworkCreateOptions{
		Driver: "bridge",
		Labels: map[string]string{"pulse.integration": "tenant-network-proof"},
	}); err != nil {
		t.Fatalf("create provider network: %v", err)
	}
	t.Cleanup(func() {
		_, _ = mgr.cli.NetworkRemove(context.Background(), providerNetwork, client.NetworkRemoveOptions{})
	})

	if _, _, err := mgr.ensureRuntimeImageAvailable(ctx, true); err != nil {
		t.Fatalf("prepare busybox image: %v", err)
	}

	tenantA, err := mgr.ensureTenantNetwork(ctx, "t-A")
	if err != nil {
		t.Fatalf("create tenant A network: %v", err)
	}
	t.Cleanup(func() { _, _ = mgr.cli.NetworkRemove(context.Background(), tenantA, client.NetworkRemoveOptions{}) })

	tenantB, err := mgr.ensureTenantNetwork(ctx, "t-B")
	if err != nil {
		t.Fatalf("create tenant B network: %v", err)
	}
	t.Cleanup(func() { _, _ = mgr.cli.NetworkRemove(context.Background(), tenantB, client.NetworkRemoveOptions{}) })

	targetID := dockerIntegrationCreateContainer(t, ctx, mgr, tenantB, []string{"sleep", "300"})
	targetIP := dockerIntegrationContainerIP(t, ctx, mgr, targetID, tenantB)
	if targetIP == "" {
		t.Fatal("tenant B target container has no IP on its tenant network")
	}
	dockerIntegrationAssertNotAttached(t, ctx, mgr, targetID, providerNetwork)

	if code := dockerIntegrationRunContainer(t, ctx, mgr, tenantB, []string{"ping", "-c", "1", "-W", "1", targetIP}); code != 0 {
		t.Fatalf("same-tenant probe exit status = %d, want 0", code)
	}
	if code := dockerIntegrationRunContainer(t, ctx, mgr, tenantA, []string{"ping", "-c", "1", "-W", "1", targetIP}); code == 0 {
		t.Fatal("cross-tenant probe unexpectedly reached tenant B container")
	}
}

func dockerIntegrationSuffix(t *testing.T) string {
	t.Helper()
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return strings.ToLower(hex.EncodeToString(raw[:]))
}

func dockerIntegrationCreateContainer(t *testing.T, ctx context.Context, mgr *Manager, networkName string, cmd []string) string {
	t.Helper()
	name := fmt.Sprintf("pulse-it-%s-%s", dockerIntegrationSuffix(t), strings.ReplaceAll(networkName, "_", "-"))
	resp, err := mgr.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image:  mgr.cfg.Image,
			Cmd:    cmd,
			Labels: map[string]string{"pulse.integration": "tenant-network-proof"},
		},
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},
		Name: name,
	})
	if err != nil {
		t.Fatalf("create container on %s: %v", networkName, err)
	}
	t.Cleanup(func() {
		_, _ = mgr.cli.ContainerRemove(context.Background(), resp.ID, client.ContainerRemoveOptions{Force: true})
	})
	if _, err := mgr.cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		t.Fatalf("start container on %s: %v", networkName, err)
	}
	return resp.ID
}

func dockerIntegrationRunContainer(t *testing.T, ctx context.Context, mgr *Manager, networkName string, cmd []string) int64 {
	t.Helper()
	id := dockerIntegrationCreateContainer(t, ctx, mgr, networkName, cmd)
	wait := mgr.cli.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
	select {
	case err := <-wait.Error:
		if err != nil {
			t.Fatalf("wait for probe on %s: %v", networkName, err)
		}
	case result := <-wait.Result:
		return result.StatusCode
	case <-ctx.Done():
		t.Fatalf("probe on %s did not finish: %v", networkName, ctx.Err())
	}
	return 1
}

func dockerIntegrationContainerIP(t *testing.T, ctx context.Context, mgr *Manager, containerID, networkName string) string {
	t.Helper()
	inspect, err := mgr.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect container %s: %v", containerID, err)
	}
	if inspect.Container.NetworkSettings == nil {
		return ""
	}
	endpoint := inspect.Container.NetworkSettings.Networks[networkName]
	if endpoint == nil {
		return ""
	}
	return strings.TrimSpace(endpoint.IPAddress.String())
}

func dockerIntegrationAssertNotAttached(t *testing.T, ctx context.Context, mgr *Manager, containerID, networkName string) {
	t.Helper()
	inspect, err := mgr.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect container %s: %v", containerID, err)
	}
	if inspect.Container.NetworkSettings == nil {
		return
	}
	if endpoint := inspect.Container.NetworkSettings.Networks[networkName]; endpoint != nil {
		t.Fatalf("container %s is unexpectedly attached to provider network %s", containerID[:12], networkName)
	}
}
