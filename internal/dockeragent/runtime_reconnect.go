package dockeragent

import (
	"context"
	"fmt"
	"io"
	"sync"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// runtimeReconnectFailureThreshold is how many consecutive daemon-unavailable
// collect cycles are tolerated before the agent tries to re-run runtime
// discovery. A bound socket can disappear for good mid-run (issue #1647: a
// socket-activated rootless podman API socket vanishes when the login session
// ends) and without a reconnect the agent errors forever until restarted.
const runtimeReconnectFailureThreshold = 3

func validateCollectorDirectRuntime(cli dockerClient, info systemtypes.Info) error {
	if cli == nil {
		return fmt.Errorf("collector direct runtime client is nil")
	}
	if !collectorOwnsRootlessEndpoint(cli.DaemonHost()) {
		return fmt.Errorf("direct runtime endpoint %q is not a collector-owned rootless Unix socket", cli.DaemonHost())
	}
	if !runtimeInfoIsRootless(info) {
		return fmt.Errorf("direct runtime endpoint %q did not attest rootless mode", cli.DaemonHost())
	}
	return nil
}

// swappableDockerClient lets the agent replace its runtime connection while
// concurrent goroutines (orphan cleanup, container update commands) keep using
// the same dockerClient handle. Only the inner pointer swaps, under the lock.
type swappableDockerClient struct {
	mu    sync.RWMutex
	inner dockerClient
}

func newSwappableDockerClient(inner dockerClient) *swappableDockerClient {
	return &swappableDockerClient{inner: inner}
}

func (s *swappableDockerClient) get() dockerClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inner
}

func (s *swappableDockerClient) swap(next dockerClient) dockerClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.inner
	s.inner = next
	return previous
}

func (s *swappableDockerClient) Info(ctx context.Context) (systemtypes.Info, error) {
	return s.get().Info(ctx)
}

func (s *swappableDockerClient) DaemonHost() string {
	return s.get().DaemonHost()
}

func (s *swappableDockerClient) ContainerList(ctx context.Context, options dockerContainerListOptions) ([]containertypes.Summary, error) {
	return s.get().ContainerList(ctx, options)
}

func (s *swappableDockerClient) ContainerInspectWithRaw(ctx context.Context, containerID string, size bool) (containertypes.InspectResponse, []byte, error) {
	return s.get().ContainerInspectWithRaw(ctx, containerID, size)
}

func (s *swappableDockerClient) ContainerStatsOneShot(ctx context.Context, containerID string) (dockerStatsResponseReader, error) {
	return s.get().ContainerStatsOneShot(ctx, containerID)
}

func (s *swappableDockerClient) ContainerInspect(ctx context.Context, containerID string) (containertypes.InspectResponse, error) {
	return s.get().ContainerInspect(ctx, containerID)
}

func (s *swappableDockerClient) ImagePull(ctx context.Context, ref string, options dockerImagePullOptions) (io.ReadCloser, error) {
	return s.get().ImagePull(ctx, ref, options)
}

func (s *swappableDockerClient) ContainerStop(ctx context.Context, containerID string, options dockerContainerStopOptions) error {
	return s.get().ContainerStop(ctx, containerID, options)
}

func (s *swappableDockerClient) ContainerRestart(ctx context.Context, containerID string, options dockerContainerRestartOptions) error {
	return s.get().ContainerRestart(ctx, containerID, options)
}

func (s *swappableDockerClient) ContainerRename(ctx context.Context, containerID, newName string) error {
	return s.get().ContainerRename(ctx, containerID, newName)
}

func (s *swappableDockerClient) ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (containertypes.CreateResponse, error) {
	return s.get().ContainerCreate(ctx, config, hostConfig, networkingConfig, platform, containerName)
}

func (s *swappableDockerClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	return s.get().NetworkConnect(ctx, networkID, containerID, config)
}

func (s *swappableDockerClient) ContainerStart(ctx context.Context, containerID string, options dockerContainerStartOptions) error {
	return s.get().ContainerStart(ctx, containerID, options)
}

func (s *swappableDockerClient) ContainerRemove(ctx context.Context, containerID string, options dockerContainerRemoveOptions) error {
	return s.get().ContainerRemove(ctx, containerID, options)
}

func (s *swappableDockerClient) ImageList(ctx context.Context, options dockerImageListOptions) ([]image.Summary, error) {
	return s.get().ImageList(ctx, options)
}

func (s *swappableDockerClient) VolumeList(ctx context.Context, options dockerVolumeListOptions) ([]volume.Volume, error) {
	return s.get().VolumeList(ctx, options)
}

func (s *swappableDockerClient) NetworkList(ctx context.Context, options dockerNetworkListOptions) ([]network.Summary, error) {
	return s.get().NetworkList(ctx, options)
}

func (s *swappableDockerClient) DiskUsage(ctx context.Context, options dockerDiskUsageOptions) (client.DiskUsageResult, error) {
	return s.get().DiskUsage(ctx, options)
}

func (s *swappableDockerClient) ServiceList(ctx context.Context, options dockerServiceListOptions) ([]swarmtypes.Service, error) {
	return s.get().ServiceList(ctx, options)
}

func (s *swappableDockerClient) TaskList(ctx context.Context, options dockerTaskListOptions) ([]swarmtypes.Task, error) {
	return s.get().TaskList(ctx, options)
}

func (s *swappableDockerClient) NodeList(ctx context.Context, options dockerNodeListOptions) ([]swarmtypes.Node, error) {
	return s.get().NodeList(ctx, options)
}

func (s *swappableDockerClient) SecretList(ctx context.Context, options dockerSecretListOptions) ([]swarmtypes.Secret, error) {
	return s.get().SecretList(ctx, options)
}

func (s *swappableDockerClient) ConfigList(ctx context.Context, options dockerConfigListOptions) ([]swarmtypes.Config, error) {
	return s.get().ConfigList(ctx, options)
}

func (s *swappableDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	return s.get().ImageInspectWithRaw(ctx, imageID)
}

func (s *swappableDockerClient) Close() error {
	return s.get().Close()
}

// maybeReconnectRuntime is called under collectMu after a failed buildReport.
// It returns true when a fresh runtime connection was established and the
// collect should be retried immediately.
func (a *Agent) maybeReconnectRuntime(err error) bool {
	if !isDockerDaemonUnavailable(err) {
		a.runtimeGoneStreak = 0
		return false
	}

	a.runtimeGoneStreak++
	if a.runtimeGoneStreak < runtimeReconnectFailureThreshold {
		return false
	}

	connect := connectRuntimeFn
	if a.cfg.HelperInventory != nil {
		connect = connectCollectorRuntimeFn
	}
	cli, info, runtimeKind, connErr := connect(a.runtimePref, &a.logger)
	if connErr != nil {
		a.logger.Warn().
			Err(connErr).
			Int("consecutive_failures", a.runtimeGoneStreak).
			Msg("Container runtime endpoint unavailable; reconnect attempt failed")
		return false
	}
	if a.cfg.HelperInventory != nil {
		validationErr := validateCollectorDirectRuntime(cli, info)
		if validationErr != nil {
			if closeErr := cli.Close(); closeErr != nil {
				a.logger.Debug().Err(closeErr).Msg("Failed to close rejected runtime reconnect client")
			}
			a.logger.Warn().
				Err(validationErr).
				Msg("Rejected runtime reconnect outside the collector-owned rootless boundary")
			return false
		}
	}

	previous := a.adoptRuntimeConnection(cli, info, runtimeKind)
	if previous != nil {
		if closeErr := previous.Close(); closeErr != nil {
			a.logger.Debug().Err(closeErr).Msg("Failed to close stale runtime client after reconnect")
		}
	}
	a.runtimeGoneStreak = 0
	a.logger.Info().
		Str("runtime", string(runtimeKind)).
		Str("daemon_host", cli.DaemonHost()).
		Msg("Reconnected to container runtime")
	return true
}

// maybePromoteCollectorRootlessRuntime is called under collectMu while the
// agent is using helper summary inventory. Each collection re-evaluates the
// exact collector-owned endpoint set, so a rootless daemon that appears later
// can recover full direct monitoring without a process restart.
func (a *Agent) maybePromoteCollectorRootlessRuntime() bool {
	if a == nil || a.helperInventory == nil || a.cfg.HelperInventory == nil {
		return false
	}

	cli, info, runtimeKind, err := connectCollectorRuntimeFn(a.runtimePref, &a.logger)
	if err != nil {
		return false
	}
	if err := validateCollectorDirectRuntime(cli, info); err != nil {
		if closeErr := cli.Close(); closeErr != nil {
			a.logger.Debug().Err(closeErr).Msg("Failed to close rejected promoted runtime client")
		}
		a.logger.Warn().Err(err).Msg("Rejected promoted runtime outside the collector-owned rootless boundary")
		return false
	}

	a.cfg.DisableUpdateChecks = a.directDisableChecks
	a.cfg.IncludeServices = a.directServices
	a.cfg.IncludeTasks = a.directTasks
	previous := a.adoptRuntimeConnection(cli, info, runtimeKind)
	a.helperInventory = nil
	if a.registryChecker != nil {
		a.registryChecker.SetEnabled(!a.cfg.DisableUpdateChecks)
	}
	if a.cfg.HelperOperationStatus != nil {
		a.cfg.HelperOperationStatus.Record("container.inventory", nil)
	}
	if previous != nil {
		if closeErr := previous.Close(); closeErr != nil {
			a.logger.Debug().Err(closeErr).Msg("Failed to close stale runtime client after helper recovery")
		}
	}
	a.runtimeGoneStreak = 0
	a.logger.Info().
		Str("runtime", string(runtimeKind)).
		Str("daemon_host", cli.DaemonHost()).
		Msg("Recovered direct collector-owned rootless runtime monitoring")
	return true
}

// maybeFallbackToHelperInventory is called under collectMu after direct
// runtime reconnect has reached its failure threshold. It changes mode only
// after a complete helper snapshot is available and no direct-runtime
// background operation remains active.
func (a *Agent) maybeFallbackToHelperInventory(ctx context.Context) bool {
	if a == nil || a.helperInventory != nil || a.cfg.HelperInventory == nil || !a.backgroundTasksIdle() {
		return false
	}

	probeCtx, cancel := context.WithTimeout(ctx, helperInventoryOperationDeadline)
	result, err := a.cfg.HelperInventory.Inventory(probeCtx)
	cancel()
	if err == nil {
		_, err = selectHelperRuntime(result, a.runtimePref)
	}
	if err != nil {
		a.recordHelperInventoryStatus(err)
		a.logger.Warn().Err(err).Msg("Typed helper inventory unavailable during rootless runtime recovery")
		return false
	}

	if a.docker != nil {
		if closeErr := a.docker.Close(); closeErr != nil {
			a.logger.Debug().Err(closeErr).Msg("Failed to close unavailable direct runtime client")
		}
	}
	a.docker = nil
	a.helperInventory = a.cfg.HelperInventory
	a.daemonHost = ""
	a.daemonID = ""
	a.cfg.DisableUpdateChecks = true
	a.cfg.IncludeServices = false
	a.cfg.IncludeTasks = false
	if a.registryChecker != nil {
		a.registryChecker.SetEnabled(false)
	}
	a.clearDockerCollectionCaches()
	a.recordHelperInventoryStatus(nil)
	a.logger.Info().
		Str("collection_mode", "typed-helper-summary").
		Msg("Fell back to typed helper inventory after rootless runtime loss")
	return true
}

func (a *Agent) backgroundTasksIdle() bool {
	a.backgroundMu.Lock()
	defer a.backgroundMu.Unlock()
	return !a.updateCheckRunning && !a.cleanupTaskRunning
}

// adoptRuntimeConnection swaps in a freshly discovered runtime connection and
// refreshes the runtime identity fields. Called under collectMu; returns the
// replaced client so the caller can close it.
func (a *Agent) adoptRuntimeConnection(cli dockerClient, info systemtypes.Info, runtimeKind RuntimeKind) dockerClient {
	var previous dockerClient
	if sw, ok := a.docker.(*swappableDockerClient); ok {
		previous = sw.swap(cli)
	} else {
		previous = a.docker
		a.docker = newSwappableDockerClient(cli)
	}

	a.daemonHost = cli.DaemonHost()
	if info.ID != "" {
		a.daemonID = info.ID
	}
	a.runtimeVer = info.ServerVersion
	if runtimeKind != a.runtime {
		a.logger.Info().
			Str("runtime_previous", string(a.runtime)).
			Str("runtime_current", string(runtimeKind)).
			Msg("Detected container runtime change during reconnect")
	}
	a.runtime = runtimeKind
	a.supportsSwarm = runtimeKind == RuntimeDocker
	if runtimeKind == RuntimePodman {
		a.cfg.IncludeServices = false
		a.cfg.IncludeTasks = false
	}
	a.cfg.Runtime = string(runtimeKind)
	a.clearDockerCollectionCaches()

	return previous
}
