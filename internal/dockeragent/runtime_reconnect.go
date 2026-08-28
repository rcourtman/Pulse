package dockeragent

import (
	"context"
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

	cli, info, runtimeKind, connErr := connectRuntimeFn(a.runtimePref, &a.logger)
	if connErr != nil {
		a.logger.Warn().
			Err(connErr).
			Int("consecutive_failures", a.runtimeGoneStreak).
			Msg("Container runtime endpoint unavailable; reconnect attempt failed")
		return false
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

// adoptRuntimeConnection swaps in a freshly discovered runtime connection and
// refreshes the runtime identity fields. Called under collectMu; returns the
// replaced client so the caller can close it.
func (a *Agent) adoptRuntimeConnection(cli dockerClient, info systemtypes.Info, runtimeKind RuntimeKind) dockerClient {
	var previous dockerClient
	if sw, ok := a.docker.(*swappableDockerClient); ok {
		previous = sw.swap(cli)
	} else {
		previous = a.docker
		a.docker = cli
	}

	a.daemonHost = cli.DaemonHost()
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
