package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

func TestIntegrationTenantRuntimeStartsRootlessWithHardenedHostConfig(t *testing.T) {
	if os.Getenv("PULSE_RUN_DOCKER_INTEGRATION") != "1" {
		t.Skip("set PULSE_RUN_DOCKER_INTEGRATION=1 to run live Docker tenant-runtime proof")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	suffix := dockerIntegrationSuffix(t)
	imageTag := "pulse-it-rootless-" + suffix + ":latest"
	providerNetwork := "pulse-it-provider-" + suffix
	tenantNetworkPrefix := "pulse-it-tenant-" + suffix
	tenantID := "t-rootless-" + suffix
	scratchRoot, err := dockerIntegrationScratchRoot()
	if err != nil {
		t.Fatalf("resolve docker-visible scratch root: %v", err)
	}
	tenantDataDir, err := os.MkdirTemp(scratchRoot, "pulse-it-rootless-"+suffix+"-")
	if err != nil {
		t.Fatalf("create docker-visible tenant data dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tenantDataDir) })

	if err := os.MkdirAll(filepath.Join(tenantDataDir, "secrets"), 0o755); err != nil {
		t.Fatalf("mkdir tenant secrets: %v", err)
	}
	for _, path := range []string{
		filepath.Join(tenantDataDir, "billing.json"),
		filepath.Join(tenantDataDir, "secrets", "handoff.key"),
		filepath.Join(tenantDataDir, ".cloud_handoff_key"),
	} {
		if err := os.WriteFile(path, []byte("secret"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	mgr, err := NewManager(ManagerConfig{
		Image:                 imageTag,
		Network:               providerNetwork,
		IsolateTenantNetworks: true,
		TenantNetworkPrefix:   tenantNetworkPrefix,
		BaseDomain:            "msp.example.test",
		TenantRuntimeUID:      os.Getuid(),
		TenantRuntimeGID:      os.Getgid(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	dockerIntegrationBuildRootlessProofImage(t, ctx, mgr, imageTag)
	t.Cleanup(func() {
		_, _ = mgr.cli.ImageRemove(context.Background(), imageTag, client.ImageRemoveOptions{Force: true, PruneChildren: true})
	})

	if _, err := mgr.cli.NetworkCreate(ctx, providerNetwork, client.NetworkCreateOptions{
		Driver: "bridge",
		Labels: map[string]string{"pulse.integration": "tenant-rootless-proof"},
	}); err != nil {
		t.Fatalf("create provider network: %v", err)
	}
	t.Cleanup(func() {
		_, _ = mgr.cli.NetworkRemove(context.Background(), providerNetwork, client.NetworkRemoveOptions{})
	})

	dockerIntegrationCreateSupportContainer(t, ctx, mgr, providerNetwork, providerSupportTraefikLabel)
	dockerIntegrationCreateSupportContainer(t, ctx, mgr, providerNetwork, providerSupportControlPlaneLabel)

	containerID, err := mgr.CreateAndStart(ctx, tenantID, tenantDataDir)
	if err != nil {
		t.Fatalf("CreateAndStart: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Remove(context.Background(), containerID) })

	inspect, err := mgr.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect tenant runtime: %v", err)
	}
	if inspect.Container.Config == nil {
		t.Fatal("tenant runtime Config is nil")
	}
	if inspect.Container.Config.User != tenantRuntimeUserFor(mgr.cfg) {
		t.Fatalf("tenant runtime Config.User = %q, want %q", inspect.Container.Config.User, tenantRuntimeUserFor(mgr.cfg))
	}
	if inspect.Container.HostConfig == nil {
		t.Fatal("tenant runtime HostConfig is nil")
	}
	if !inspect.Container.HostConfig.ReadonlyRootfs {
		t.Fatal("tenant runtime root filesystem is not read-only")
	}
	if got := inspect.Container.HostConfig.CapDrop; len(got) != 1 || got[0] != "ALL" {
		t.Fatalf("tenant runtime CapDrop = %v, want [ALL]", got)
	}
	if len(inspect.Container.HostConfig.CapAdd) != 0 {
		t.Fatalf("tenant runtime CapAdd = %v, want none", inspect.Container.HostConfig.CapAdd)
	}

	dockerIntegrationWaitForRootlessStartupProof(t, ctx, mgr, containerID, filepath.Join(tenantDataDir, "rootless-startup-proof"), tenantRuntimeUserFor(mgr.cfg))
}

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

// This exercises exact checked-out manager source against a fresh Docker daemon.
// The release-line push check preloads a pinned helper image; final candidate
// qualification uses its verified control-plane image instead. Unit fixtures
// cover the API requests; this checks their effects with two installations.
func TestIntegrationProviderPairNetworkIsolation(t *testing.T) {
	if os.Getenv("PULSE_RUN_PROVIDER_PAIR_DOCKER_INTEGRATION") != "1" {
		t.Skip("set PULSE_RUN_PROVIDER_PAIR_DOCKER_INTEGRATION=1 for live two-provider Docker proof")
	}
	image := strings.TrimSpace(os.Getenv("PULSE_DOCKER_INTEGRATION_IMAGE"))
	if image == "" {
		t.Fatal("PULSE_DOCKER_INTEGRATION_IMAGE must name a preloaded test image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	suffix := dockerIntegrationSuffix(t)
	tenantID := "t-shared-" + suffix
	managerFor := func(provider string) *Manager {
		t.Helper()
		mgr, err := NewManager(ManagerConfig{
			Image:                 image,
			Network:               "pulse-it-provider-" + provider + "-" + suffix,
			IsolateTenantNetworks: true,
		})
		if err != nil {
			t.Fatalf("create provider %s manager: %v", provider, err)
		}
		t.Cleanup(func() { _ = mgr.Close() })
		if _, err := mgr.cli.ImageInspect(ctx, image); err != nil {
			t.Fatalf("inspect preloaded test image %q: %v", image, err)
		}
		if _, err := mgr.cli.NetworkCreate(ctx, mgr.cfg.Network, client.NetworkCreateOptions{
			Driver: "bridge",
			Labels: map[string]string{"pulse.integration": "provider-pair-network-proof"},
		}); err != nil {
			t.Fatalf("create provider %s ingress network: %v", provider, err)
		}
		t.Cleanup(func() {
			_, _ = mgr.cli.NetworkRemove(context.Background(), mgr.cfg.Network, client.NetworkRemoveOptions{})
		})
		return mgr
	}
	a, b := managerFor("a"), managerFor("b")
	aSupport := []string{
		dockerIntegrationCreateSupportContainer(t, ctx, a, a.cfg.Network, providerSupportTraefikLabel),
		dockerIntegrationCreateSupportContainer(t, ctx, a, a.cfg.Network, providerSupportControlPlaneLabel),
	}
	bSupport := []string{
		dockerIntegrationCreateSupportContainer(t, ctx, b, b.cfg.Network, providerSupportTraefikLabel),
		dockerIntegrationCreateSupportContainer(t, ctx, b, b.cfg.Network, providerSupportControlPlaneLabel),
	}

	aTenant, err := a.ensureTenantNetwork(ctx, tenantID)
	if err != nil {
		t.Fatalf("provision A tenant network: %v", err)
	}
	t.Cleanup(func() { _ = a.removeTenantNetwork(context.Background(), aTenant) })
	bTenant, err := b.ensureTenantNetwork(ctx, tenantID)
	if err != nil {
		t.Fatalf("provision B tenant network: %v", err)
	}
	t.Cleanup(func() { _ = b.removeTenantNetwork(context.Background(), bTenant) })
	if err := a.connectSupportContainersToTenantNetwork(ctx, aTenant); err != nil {
		t.Fatalf("attach A support: %v", err)
	}
	if err := b.connectSupportContainersToTenantNetwork(ctx, bTenant); err != nil {
		t.Fatalf("attach B support: %v", err)
	}
	createTenantContainer := func(mgr *Manager, provider, networkName string) string {
		t.Helper()
		resp, err := mgr.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
			Config: &container.Config{
				Image:      image,
				Entrypoint: []string{"sleep"},
				Cmd:        []string{"300"},
				Labels: map[string]string{
					"pulse.managed":   "true",
					"pulse.tenant.id": tenantID,
				},
			},
			NetworkingConfig: &network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{networkName: {}},
			},
			Name: "pulse-it-client-" + provider + "-" + suffix,
		})
		if err != nil {
			t.Fatalf("create provider %s tenant container: %v", provider, err)
		}
		t.Cleanup(func() {
			_, _ = mgr.cli.ContainerRemove(context.Background(), resp.ID, client.ContainerRemoveOptions{Force: true})
		})
		if _, err := mgr.cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
			t.Fatalf("start provider %s tenant container: %v", provider, err)
		}
		return resp.ID
	}
	aClient := createTenantContainer(a, "a", aTenant)
	bClient := createTenantContainer(b, "b", bTenant)
	for _, id := range aSupport {
		dockerIntegrationAssertAttached(t, ctx, a, id, aTenant)
		dockerIntegrationAssertNotAttached(t, ctx, a, id, bTenant)
	}
	for _, id := range bSupport {
		dockerIntegrationAssertAttached(t, ctx, b, id, bTenant)
		dockerIntegrationAssertNotAttached(t, ctx, b, id, aTenant)
	}

	// An unrelated unlabelled network with our derived name must not be adopted.
	collision := a.tenantNetworkName("t-collision-" + suffix)
	if _, err := a.cli.NetworkCreate(ctx, collision, client.NetworkCreateOptions{Driver: "bridge"}); err != nil {
		t.Fatalf("create unowned collision network: %v", err)
	}
	t.Cleanup(func() { _, _ = a.cli.NetworkRemove(context.Background(), collision, client.NetworkRemoveOptions{}) })
	if _, err := a.ensureTenantNetwork(ctx, "t-collision-"+suffix); err == nil {
		t.Fatal("provider A adopted an unowned same-name network")
	}
	if _, err := a.cli.NetworkInspect(ctx, collision, client.NetworkInspectOptions{}); err != nil {
		t.Fatalf("unowned collision network was changed: %v", err)
	}

	if err := a.Remove(ctx, aClient); err != nil {
		t.Fatalf("remove A tenant container: %v", err)
	}
	if _, err := a.cli.NetworkInspect(ctx, aTenant, client.NetworkInspectOptions{}); !errdefs.IsNotFound(err) {
		t.Fatalf("A tenant network remains or lookup failed unexpectedly: %v", err)
	}
	for _, id := range aSupport {
		dockerIntegrationAssertNotAttached(t, ctx, a, id, aTenant)
		dockerIntegrationAssertAttached(t, ctx, a, id, a.cfg.Network)
	}
	if _, err := b.cli.NetworkInspect(ctx, bTenant, client.NetworkInspectOptions{}); err != nil {
		t.Fatalf("A cleanup removed B tenant network: %v", err)
	}
	for _, id := range bSupport {
		dockerIntegrationAssertAttached(t, ctx, b, id, bTenant)
		dockerIntegrationAssertAttached(t, ctx, b, id, b.cfg.Network)
	}
	if err := b.Remove(ctx, bClient); err != nil {
		t.Fatalf("remove B tenant container: %v", err)
	}
	if _, err := b.cli.NetworkInspect(ctx, bTenant, client.NetworkInspectOptions{}); !errdefs.IsNotFound(err) {
		t.Fatalf("B tenant network remains or lookup failed unexpectedly: %v", err)
	}
	for _, id := range bSupport {
		dockerIntegrationAssertNotAttached(t, ctx, b, id, bTenant)
		dockerIntegrationAssertAttached(t, ctx, b, id, b.cfg.Network)
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

func dockerIntegrationScratchRoot() (string, error) {
	if value := strings.TrimSpace(os.Getenv("PULSE_DOCKER_INTEGRATION_SCRATCH")); value != "" {
		if err := os.MkdirAll(value, 0o700); err != nil {
			return "", err
		}
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, ".cache", "pulse-docker-integration")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func dockerIntegrationBuildRootlessProofImage(t *testing.T, ctx context.Context, mgr *Manager, imageTag string) {
	t.Helper()

	entrypointPath := filepath.Join(dockerIntegrationRepoRoot(t), "docker-entrypoint.sh")
	entrypoint, err := os.ReadFile(entrypointPath)
	if err != nil {
		t.Fatalf("read docker-entrypoint.sh: %v", err)
	}

	dockerfile := []byte(`FROM alpine:3.20
RUN apk --no-cache add su-exec
RUN addgroup -g 1000 pulse && adduser -D -u 1000 -G pulse pulse
RUN mkdir -p /etc/pulse /data && chown -R pulse:pulse /etc/pulse /data
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["sh", "-c", "printf '%s:%s\n' \"$(id -u)\" \"$(id -g)\" > /etc/pulse/rootless-startup-proof && sleep 300"]
`)

	var contextTar bytes.Buffer
	tw := tar.NewWriter(&contextTar)
	dockerIntegrationAddTarFile(t, tw, "Dockerfile", 0o644, dockerfile)
	dockerIntegrationAddTarFile(t, tw, "docker-entrypoint.sh", 0o755, entrypoint)
	if err := tw.Close(); err != nil {
		t.Fatalf("close image build context: %v", err)
	}

	build, err := mgr.cli.ImageBuild(ctx, &contextTar, client.ImageBuildOptions{
		Tags:        []string{imageTag},
		Dockerfile:  "Dockerfile",
		Remove:      true,
		ForceRemove: true,
	})
	if err != nil {
		t.Fatalf("build rootless proof image: %v", err)
	}
	defer build.Body.Close()
	buildOutput, err := io.ReadAll(build.Body)
	if err != nil {
		t.Fatalf("read rootless proof image build output: %v", err)
	}
	if _, err := mgr.cli.ImageInspect(ctx, imageTag); err != nil {
		t.Fatalf("inspect built rootless proof image: %v\n%s", err, string(buildOutput))
	}
}

func dockerIntegrationRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docker-entrypoint.sh")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repo root from %s", dir)
		}
		dir = parent
	}
}

func dockerIntegrationAddTarFile(t *testing.T, tw *tar.Writer, name string, mode int64, content []byte) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(content))}); err != nil {
		t.Fatalf("write tar header %s: %v", name, err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content %s: %v", name, err)
	}
}

func dockerIntegrationCreateSupportContainer(t *testing.T, ctx context.Context, mgr *Manager, networkName, label string) string {
	t.Helper()
	key, value, ok := strings.Cut(label, "=")
	if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		t.Fatalf("invalid support label %q", label)
	}
	name := fmt.Sprintf("pulse-it-support-%s-%s", dockerIntegrationSuffix(t), strings.ReplaceAll(key, ".", "-"))
	resp, err := mgr.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image:      mgr.cfg.Image,
			Entrypoint: []string{"sleep"},
			Cmd:        []string{"300"},
			Labels: map[string]string{
				strings.TrimSpace(key): strings.TrimSpace(value),
				"pulse.integration":    "tenant-rootless-proof",
			},
		},
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},
		Name: name,
	})
	if err != nil {
		t.Fatalf("create support container %s: %v", label, err)
	}
	t.Cleanup(func() {
		_, _ = mgr.cli.ContainerRemove(context.Background(), resp.ID, client.ContainerRemoveOptions{Force: true})
	})
	if _, err := mgr.cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		t.Fatalf("start support container %s: %v", label, err)
	}
	return resp.ID
}

func dockerIntegrationWaitForRootlessStartupProof(t *testing.T, ctx context.Context, mgr *Manager, containerID, proofPath, wantUser string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if content, err := os.ReadFile(proofPath); err == nil {
			if got := strings.TrimSpace(string(content)); got == wantUser {
				return
			}
			t.Fatalf("rootless startup proof = %q, want %q", strings.TrimSpace(string(content)), wantUser)
		}

		inspect, err := mgr.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
		if err == nil && inspect.Container.State != nil && !inspect.Container.State.Running {
			t.Fatalf("tenant runtime exited before rootless proof; state=%s exit=%d logs=%s",
				inspect.Container.State.Status,
				inspect.Container.State.ExitCode,
				dockerIntegrationContainerLogs(t, ctx, mgr, containerID),
			)
		}

		select {
		case <-time.After(250 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("wait for rootless startup proof: %v", ctx.Err())
		}
	}
	t.Fatalf("tenant runtime did not write rootless proof at %s; logs=%s", proofPath, dockerIntegrationContainerLogs(t, ctx, mgr, containerID))
}

func dockerIntegrationContainerLogs(t *testing.T, ctx context.Context, mgr *Manager, containerID string) string {
	t.Helper()
	logs, err := mgr.cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Tail: "50"})
	if err != nil {
		return err.Error()
	}
	defer logs.Close()
	content, err := io.ReadAll(logs)
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(string(content))
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

func dockerIntegrationAssertAttached(t *testing.T, ctx context.Context, mgr *Manager, containerID, networkName string) {
	t.Helper()
	inspect, err := mgr.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspect container %s: %v", containerID, err)
	}
	if inspect.Container.NetworkSettings == nil || inspect.Container.NetworkSettings.Networks[networkName] == nil {
		t.Fatalf("container %s is not attached to network %s", containerID[:12], networkName)
	}
}
