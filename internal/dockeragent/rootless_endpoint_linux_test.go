//go:build linux

package dockeragent

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestCollectorOwnsRootlessEndpointRequiresOwnedRuntimeSocket(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
	})

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(runtimeDir, "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if !collectorOwnsRootlessEndpoint("unix://" + socketPath) {
		t.Fatal("collector-owned rootless Unix socket was rejected")
	}
	if collectorOwnsRootlessEndpoint("unix:///var/run/docker.sock") {
		t.Fatal("rootful system socket was accepted")
	}
	if collectorOwnsRootlessEndpoint("tcp://127.0.0.1:2375") {
		t.Fatal("non-Unix runtime endpoint was accepted")
	}

	symlink := filepath.Join(runtimeDir, "linked.sock")
	if err := os.Symlink(socketPath, symlink); err != nil {
		t.Fatal(err)
	}
	if collectorOwnsRootlessEndpoint("unix://" + symlink) {
		t.Fatal("symlinked runtime endpoint was accepted")
	}
}

func TestCollectorRootlessRuntimeCandidatesRequireOneExactOwnedSocket(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
	})
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("CONTAINER_HOST", "")
	t.Setenv("PODMAN_HOST", "")

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(filepath.Join(runtimeDir, "podman"), 0o700); err != nil {
		t.Fatal(err)
	}

	dockerPath := filepath.Join(runtimeDir, "docker.sock")
	dockerListener, err := net.Listen("unix", dockerPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerListener.Close()

	candidates, err := collectorRootlessRuntimeCandidates(RuntimeAuto)
	if err != nil {
		t.Fatalf("single Docker candidate: %v", err)
	}
	if len(candidates) != 1 || candidates[0].host != "unix://"+dockerPath {
		t.Fatalf("Docker candidates = %+v", candidates)
	}

	podmanPath := filepath.Join(runtimeDir, "podman", "podman.sock")
	podmanListener, err := net.Listen("unix", podmanPath)
	if err != nil {
		t.Fatal(err)
	}
	defer podmanListener.Close()

	if _, err := collectorRootlessRuntimeCandidates(RuntimeAuto); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("auto discovery with Docker and Podman = %v, want ambiguity", err)
	}

	candidates, err = collectorRootlessRuntimeCandidates(RuntimeDocker)
	if err != nil {
		t.Fatalf("explicit Docker preference: %v", err)
	}
	if len(candidates) != 1 || candidates[0].host != "unix://"+dockerPath {
		t.Fatalf("Docker preference candidates = %+v", candidates)
	}

	t.Setenv("DOCKER_HOST", "unix://"+dockerPath)
	candidates, err = collectorRootlessRuntimeCandidates(RuntimeAuto)
	if err != nil {
		t.Fatalf("explicit endpoint pin: %v", err)
	}
	if len(candidates) != 1 || candidates[0].label != "DOCKER_HOST" {
		t.Fatalf("explicit candidates = %+v", candidates)
	}
}

func TestCollectorRootlessRuntimeCandidatesRejectCrossUserAndConflictingPins(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
	})

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(filepath.Join(runtimeDir, "podman"), 0o700); err != nil {
		t.Fatal(err)
	}

	dockerPath := filepath.Join(runtimeDir, "docker.sock")
	dockerListener, err := net.Listen("unix", dockerPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerListener.Close()
	podmanPath := filepath.Join(runtimeDir, "podman", "podman.sock")
	podmanListener, err := net.Listen("unix", podmanPath)
	if err != nil {
		t.Fatal(err)
	}
	defer podmanListener.Close()

	t.Setenv("DOCKER_HOST", "unix://"+dockerPath)
	t.Setenv("PODMAN_HOST", "unix://"+podmanPath)
	t.Setenv("CONTAINER_HOST", "")
	if _, err := collectorRootlessRuntimeCandidates(RuntimeAuto); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting live pins = %v, want conflict rejection", err)
	}

	t.Setenv("DOCKER_HOST", "unix://"+filepath.Join(runtimeDir, "custom.sock"))
	t.Setenv("PODMAN_HOST", "")
	if _, err := collectorRootlessRuntimeCandidates(RuntimeAuto); err == nil || !strings.Contains(err.Error(), "exact collector-owned endpoint") {
		t.Fatalf("custom same-UID pin = %v, want exact-path rejection", err)
	}

	t.Setenv("DOCKER_HOST", "")
	t.Setenv("PODMAN_HOST", "")
	effectiveUID = func() int { return os.Geteuid() + 1 }
	if _, err := collectorRootlessRuntimeCandidates(RuntimeAuto); err == nil || !strings.Contains(err.Error(), "no live collector-owned") {
		t.Fatalf("cross-user sockets = %v, want fail-closed rejection", err)
	}
}

func TestCollectorRootlessRuntimeAmbiguityIsRejectedBeforeDaemonProbe(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	originalNewClient := newDockerClientFn
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
		newDockerClientFn = originalNewClient
	})
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("CONTAINER_HOST", "")
	t.Setenv("PODMAN_HOST", "")

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(filepath.Join(runtimeDir, "podman"), 0o700); err != nil {
		t.Fatal(err)
	}

	listeners := make([]net.Listener, 0, 2)
	for _, path := range []string{
		filepath.Join(runtimeDir, "docker.sock"),
		filepath.Join(runtimeDir, "podman", "podman.sock"),
	} {
		listener, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		listeners = append(listeners, listener)
	}
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()

	probes := 0
	newDockerClientFn = func(...client.Opt) (dockerClient, error) {
		probes++
		return nil, nil
	}
	if _, _, _, err := connectCollectorOwnedRootlessRuntime(RuntimeAuto, nil); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous connection = %v", err)
	}
	if probes != 0 {
		t.Fatalf("ambiguous endpoints reached daemon client construction %d times", probes)
	}
}

func TestCollectorRootlessRuntimeRejectsSymlinkAndUnreadableSocket(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
	})
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("CONTAINER_HOST", "")
	t.Setenv("PODMAN_HOST", "")

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(runtimeDir, "real.sock")
	listener, err := net.Listen("unix", realPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	standardPath := filepath.Join(runtimeDir, "docker.sock")
	if err := os.Symlink(realPath, standardPath); err != nil {
		t.Fatal(err)
	}
	if _, err := collectorRootlessRuntimeCandidates(RuntimeAuto); err == nil {
		t.Fatal("symlinked rootless runtime endpoint was accepted")
	}
	if err := os.Remove(standardPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(realPath, standardPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(standardPath, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := collectorRootlessRuntimeCandidates(RuntimeAuto); err == nil {
		t.Fatal("collector-unreadable rootless runtime endpoint was accepted")
	}
}

func TestCollectorRootlessRuntimeRejectsOwnedSocketBackedByRootfulDaemon(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	originalNewClient := newDockerClientFn
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
		newDockerClientFn = originalNewClient
	})
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("CONTAINER_HOST", "")
	t.Setenv("PODMAN_HOST", "")

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(runtimeDir, "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	closed := false
	newDockerClientFn = func(...client.Opt) (dockerClient, error) {
		return &fakeDockerClient{
			daemonHost: "unix://" + socketPath,
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{SecurityOptions: []string{"name=seccomp,profile=default"}}, nil
			},
			closeFn: func() error { closed = true; return nil },
		}, nil
	}
	if _, _, _, err := connectCollectorOwnedRootlessRuntime(RuntimeDocker, nil); err == nil || !strings.Contains(err.Error(), "did not attest rootless mode") {
		t.Fatalf("owned-path rootful daemon = %v", err)
	}
	if !closed {
		t.Fatal("owned-path rootful daemon client was not closed")
	}

	newDockerClientFn = func(...client.Opt) (dockerClient, error) {
		return &fakeDockerClient{
			daemonHost: "unix://" + socketPath,
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{SecurityOptions: []string{"name=rootless", "name=seccomp,profile=default"}}, nil
			},
		}, nil
	}
	cli, _, runtimeKind, err := connectCollectorOwnedRootlessRuntime(RuntimeDocker, nil)
	if err != nil {
		t.Fatalf("rootless daemon: %v", err)
	}
	if cli == nil || runtimeKind != RuntimeDocker {
		t.Fatalf("rootless daemon connection = cli:%T runtime:%s", cli, runtimeKind)
	}
}

func TestTypedHelperInventoryPromotesToAttestedRootlessRuntime(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	originalConnect := connectCollectorRuntimeFn
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
		connectCollectorRuntimeFn = originalConnect
	})

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(runtimeDir, "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	rootlessClient := &fakeDockerClient{daemonHost: "unix://" + socketPath}
	connectCollectorRuntimeFn = func(RuntimeKind, *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		return rootlessClient, systemtypes.Info{
			ID: "rootless-daemon", ServerVersion: "28.0.0", SecurityOptions: []string{"name=rootless"},
		}, RuntimeDocker, nil
	}
	helper := &helperInventoryStub{}
	agent := &Agent{
		cfg: Config{
			HelperInventory: helper, DisableUpdateChecks: true,
		},
		helperInventory:     helper,
		directDisableChecks: false,
		directServices:      true,
		directTasks:         true,
		runtimePref:         RuntimeAuto,
		runtime:             RuntimeDocker,
		registryChecker:     newRegistryCheckerWithConfig(zerolog.Nop(), false),
		logger:              zerolog.Nop(),
	}

	if !agent.maybePromoteCollectorRootlessRuntime() {
		t.Fatal("attested rootless runtime was not promoted")
	}
	if agent.helperInventory != nil || agent.docker == nil || agent.daemonID != "rootless-daemon" {
		t.Fatalf("promoted state = helper:%T docker:%T daemon:%q", agent.helperInventory, agent.docker, agent.daemonID)
	}
	if agent.cfg.DisableUpdateChecks || !agent.cfg.IncludeServices || !agent.cfg.IncludeTasks || !agent.registryChecker.Enabled() {
		t.Fatalf("direct configuration was not restored: %+v", agent.cfg)
	}
	if agent.ContainerActionsAvailable() {
		t.Fatal("safe collector promotion exposed container action authority")
	}
}

func TestTypedHelperInventoryTransitionsThroughRootlessPodman(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	originalConnect := connectCollectorRuntimeFn
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
		connectCollectorRuntimeFn = originalConnect
	})

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()), "podman")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(runtimeDir, "podman.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	connectCollectorRuntimeFn = func(RuntimeKind, *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		return &fakeDockerClient{daemonHost: "unix://" + socketPath}, systemtypes.Info{
			ID: "rootless-podman", ServerVersion: "5.5.0 podman", SecurityOptions: []string{"name=rootless"},
		}, RuntimePodman, nil
	}
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{Runtime: "podman", Available: true}},
	}}
	agent := &Agent{
		cfg:                 Config{HelperInventory: helper, DisableUpdateChecks: true},
		helperInventory:     helper,
		directDisableChecks: false,
		directServices:      true,
		directTasks:         true,
		runtimePref:         RuntimePodman,
		runtime:             RuntimePodman,
		registryChecker:     newRegistryCheckerWithConfig(zerolog.Nop(), false),
		logger:              zerolog.Nop(),
	}
	if !agent.maybePromoteCollectorRootlessRuntime() {
		t.Fatal("attested rootless Podman runtime was not promoted")
	}
	if agent.runtime != RuntimePodman || agent.cfg.IncludeServices || agent.cfg.IncludeTasks || agent.helperInventory != nil {
		t.Fatalf("promoted Podman state = runtime:%s services:%t tasks:%t helper:%T", agent.runtime, agent.cfg.IncludeServices, agent.cfg.IncludeTasks, agent.helperInventory)
	}
	agent.runtimeGoneStreak = runtimeReconnectFailureThreshold
	if !agent.maybeFallbackToHelperInventory(context.Background()) {
		t.Fatal("rootless Podman loss did not fall back to typed helper summary")
	}
	if agent.helperInventory != helper || agent.docker != nil || agent.runtime != RuntimePodman {
		t.Fatalf("Podman fallback state = helper:%T docker:%T runtime:%s", agent.helperInventory, agent.docker, agent.runtime)
	}
}

func TestCycleBoundaryChangeStopsDirectCallsAndFallsBackToHelper(t *testing.T) {
	originalRoot := rootlessRuntimeRoot
	originalUID := effectiveUID
	originalConnect := connectCollectorRuntimeFn
	originalMetrics := hostmetricsCollectWithDiskFilters
	t.Cleanup(func() {
		rootlessRuntimeRoot = originalRoot
		effectiveUID = originalUID
		connectCollectorRuntimeFn = originalConnect
		hostmetricsCollectWithDiskFilters = originalMetrics
	})

	rootlessRuntimeRoot = t.TempDir()
	effectiveUID = os.Geteuid
	runtimeDir := filepath.Join(rootlessRuntimeRoot, strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(runtimeDir, "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	postInfoCalls := 0
	direct := &fakeDockerClient{
		daemonHost: "unix://" + socketPath,
		infoFunc: func(context.Context) (systemtypes.Info, error) {
			return systemtypes.Info{SecurityOptions: []string{"name=seccomp,profile=default"}}, nil
		},
		containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
			postInfoCalls++
			return nil, nil
		},
	}
	connectCollectorRuntimeFn = func(RuntimeKind, *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		return nil, systemtypes.Info{}, RuntimeAuto, errors.New("replacement failed rootless admission")
	}
	hostmetricsCollectWithDiskFilters = func(context.Context, []string, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	}
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{Runtime: "docker", Available: true}},
	}}
	agent := &Agent{
		cfg:               Config{HelperInventory: helper, IncludeContainers: true},
		docker:            newSwappableDockerClient(direct),
		directServices:    true,
		directTasks:       true,
		runtime:           RuntimeDocker,
		runtimePref:       RuntimeDocker,
		runtimeGoneStreak: runtimeReconnectFailureThreshold - 1,
		registryChecker:   newRegistryCheckerWithConfig(zerolog.Nop(), false),
		allowedStates:     map[string]struct{}{},
		logger:            zerolog.Nop(),
		reportStreamID:    "boundary-change",
	}

	report, err := agent.collectOnceWithReport(context.Background())
	if err != nil {
		t.Fatalf("boundary fallback collection: %v", err)
	}
	if postInfoCalls != 0 {
		t.Fatalf("changed runtime boundary received %d post-Info daemon calls", postInfoCalls)
	}
	if agent.helperInventory != helper || agent.docker != nil || report.Host.CollectionMode != agentsdocker.CollectionModeTypedHelperSummary {
		t.Fatalf("boundary fallback state = helper:%T docker:%T mode:%q", agent.helperInventory, agent.docker, report.Host.CollectionMode)
	}
}
