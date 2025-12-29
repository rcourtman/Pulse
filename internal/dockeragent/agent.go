package dockeragent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// TargetConfig describes a single Pulse backend the agent should report to.
type TargetConfig struct {
	URL                string
	Token              string
	InsecureSkipVerify bool
}

// Config describes runtime configuration for the Docker agent.
type Config struct {
	PulseURL           string
	APIToken           string
	Interval           time.Duration
	HostnameOverride   string
	AgentID            string
	AgentType          string // "unified" when running as part of pulse-agent, empty for standalone
	AgentVersion       string // Version to report; if empty, uses dockeragent.Version
	InsecureSkipVerify bool
	DisableAutoUpdate  bool
	Targets            []TargetConfig
	ContainerStates    []string
	SwarmScope         string
	Runtime            string
	IncludeServices    bool
	IncludeTasks       bool
	IncludeContainers  bool
	CollectDiskMetrics bool
	LogLevel           zerolog.Level
	Logger             *zerolog.Logger
}

var allowedContainerStates = map[string]string{
	"created":    "created",
	"restarting": "restarting",
	"running":    "running",
	"removing":   "removing",
	"paused":     "paused",
	"exited":     "exited",
	"dead":       "dead",
	"stopped":    "exited",
}

type RuntimeKind string

const (
	RuntimeAuto   RuntimeKind = "auto"
	RuntimeDocker RuntimeKind = "docker"
	RuntimePodman RuntimeKind = "podman"
)

// Agent collects Docker metrics and posts them to Pulse.
type Agent struct {
	cfg                 Config
	docker              dockerClient
	daemonHost          string
	daemonID            string // Cached at init; Podman can return unstable IDs across calls
	runtime             RuntimeKind
	runtimeVer          string
	agentVersion        string
	supportsSwarm       bool
	httpClients         map[bool]*http.Client
	logger              zerolog.Logger
	machineID           string
	hostName            string
	cpuCount            int
	targets             []TargetConfig
	allowedStates       map[string]struct{}
	stateFilters        []string
	hostID              string
	prevContainerCPU    map[string]cpuSample
	preCPUStatsFailures int
	reportBuffer        *buffer.Queue[agentsdocker.Report]
	registryChecker     *RegistryChecker // For checking container image updates
}

// ErrStopRequested indicates the agent should terminate gracefully after acknowledging a stop command.
var ErrStopRequested = errors.New("docker host stop requested")

type cpuSample struct {
	totalUsage  uint64
	systemUsage uint64
	onlineCPUs  uint32
	read        time.Time
}

// New creates a new Docker agent instance.
func New(cfg Config) (*Agent, error) {
	targets, err := normalizeTargetsFn(cfg.Targets)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		url := strings.TrimSpace(cfg.PulseURL)
		token := strings.TrimSpace(cfg.APIToken)
		if url == "" || token == "" {
			return nil, errors.New("at least one Pulse target is required")
		}

		targets, err = normalizeTargetsFn([]TargetConfig{{
			URL:                url,
			Token:              token,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		}})
		if err != nil {
			return nil, err
		}
	}

	cfg.Targets = targets
	cfg.PulseURL = targets[0].URL
	cfg.APIToken = targets[0].Token
	cfg.InsecureSkipVerify = targets[0].InsecureSkipVerify

	stateFilters, err := normalizeContainerStates(cfg.ContainerStates)
	if err != nil {
		return nil, err
	}
	cfg.ContainerStates = stateFilters

	scope, err := normalizeSwarmScope(cfg.SwarmScope)
	if err != nil {
		return nil, err
	}
	cfg.SwarmScope = scope

	if !cfg.IncludeContainers && !cfg.IncludeServices && !cfg.IncludeTasks {
		cfg.IncludeContainers = true
		cfg.IncludeServices = true
		cfg.IncludeTasks = true
	}

	logger := cfg.Logger
	if zerolog.GlobalLevel() == zerolog.DebugLevel && cfg.LogLevel != zerolog.DebugLevel {
		zerolog.SetGlobalLevel(cfg.LogLevel)
	}

	if logger == nil {
		defaultLogger := zerolog.New(os.Stdout).Level(cfg.LogLevel).With().Timestamp().Str("component", "pulse-docker-agent").Logger()
		logger = &defaultLogger
	} else {
		scoped := logger.With().Str("component", "pulse-docker-agent").Logger()
		logger = &scoped
	}

	runtimePref, err := normalizeRuntime(cfg.Runtime)
	if err != nil {
		return nil, err
	}

	dockerClient, info, runtimeKind, err := connectRuntimeFn(runtimePref, logger)
	if err != nil {
		return nil, err
	}
	cfg.Runtime = string(runtimeKind)

	if runtimeKind == RuntimePodman {
		if cfg.IncludeServices {
			logger.Warn().Msg("Podman runtime detected; disabling Swarm service collection")
		}
		if cfg.IncludeTasks {
			logger.Warn().Msg("Podman runtime detected; disabling Swarm task collection")
		}
		cfg.IncludeServices = false
		cfg.IncludeTasks = false
	}

	logger.Info().
		Str("runtime", string(runtimeKind)).
		Str("daemon_host", dockerClient.DaemonHost()).
		Str("version", info.ServerVersion).
		Msg("Connected to container runtime")

	hasSecure := false
	hasInsecure := false
	for _, target := range cfg.Targets {
		if target.InsecureSkipVerify {
			hasInsecure = true
		} else {
			hasSecure = true
		}
	}

	httpClients := make(map[bool]*http.Client, 2)
	if hasSecure {
		httpClients[false] = newHTTPClient(false)
	}
	if hasInsecure {
		httpClients[true] = newHTTPClient(true)
	}

	machineID, _ := readMachineID()
	hostName := cfg.HostnameOverride
	if hostName == "" {
		if h, err := os.Hostname(); err == nil {
			hostName = h
		}
	}

	// Use configured version or fall back to package version
	agentVersion := cfg.AgentVersion
	if agentVersion == "" {
		agentVersion = Version
	}

	const bufferCapacity = 60

	agent := &Agent{
		cfg:              cfg,
		docker:           dockerClient,
		daemonHost:       dockerClient.DaemonHost(),
		daemonID:         info.ID, // Cache at init for stable agent ID
		runtime:          runtimeKind,
		runtimeVer:       info.ServerVersion,
		agentVersion:     agentVersion,
		supportsSwarm:    runtimeKind == RuntimeDocker,
		httpClients:      httpClients,
		logger:           *logger,
		machineID:        machineID,
		hostName:         hostName,
		targets:          cfg.Targets,
		allowedStates:    make(map[string]struct{}, len(stateFilters)),
		stateFilters:     stateFilters,
		prevContainerCPU: make(map[string]cpuSample),
		reportBuffer:     buffer.New[agentsdocker.Report](bufferCapacity),
		registryChecker:  NewRegistryChecker(*logger),
	}

	for _, state := range stateFilters {
		agent.allowedStates[state] = struct{}{}
	}

	return agent, nil
}

func normalizeTargets(raw []TargetConfig) ([]TargetConfig, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	normalized := make([]TargetConfig, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))

	for _, target := range raw {
		url := strings.TrimSpace(target.URL)
		token := strings.TrimSpace(target.Token)
		if url == "" && token == "" {
			continue
		}

		if url == "" {
			return nil, errors.New("pulse target URL is required")
		}
		if token == "" {
			return nil, fmt.Errorf("pulse target %s is missing API token", url)
		}

		url = strings.TrimRight(url, "/")
		key := fmt.Sprintf("%s|%s|%t", url, token, target.InsecureSkipVerify)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		normalized = append(normalized, TargetConfig{
			URL:                url,
			Token:              token,
			InsecureSkipVerify: target.InsecureSkipVerify,
		})
	}

	return normalized, nil
}

func normalizeContainerStates(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))

	for _, value := range raw {
		state := strings.ToLower(strings.TrimSpace(value))
		if state == "" {
			continue
		}

		canonical, ok := allowedContainerStates[state]
		if !ok {
			return nil, fmt.Errorf("unsupported container state %q", value)
		}

		if _, exists := seen[canonical]; exists {
			continue
		}

		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}

	return normalized, nil
}

func normalizeRuntime(value string) (RuntimeKind, error) {
	runtime := strings.ToLower(strings.TrimSpace(value))
	switch runtime {
	case "", string(RuntimeAuto), "default":
		return RuntimeAuto, nil
	case string(RuntimeDocker):
		return RuntimeDocker, nil
	case string(RuntimePodman):
		return RuntimePodman, nil
	default:
		return "", fmt.Errorf("unsupported runtime %q: must be auto, docker, or podman", value)
	}
}

type runtimeCandidate struct {
	host           string
	label          string
	applyDockerEnv bool
}

func connectRuntime(preference RuntimeKind, logger *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
	candidates := buildRuntimeCandidatesFn(preference)
	var attempts []string

	for _, candidate := range candidates {
		opts := []client.Opt{client.WithAPIVersionNegotiation()}
		if candidate.applyDockerEnv {
			opts = append(opts, client.FromEnv)
		}
		if candidate.host != "" {
			opts = append(opts, client.WithHost(candidate.host))
		}

		cli, info, err := tryRuntimeCandidateFn(opts)
		if err != nil {
			attempts = append(attempts, fmt.Sprintf("%s: %v", candidate.label, err))
			continue
		}

		endpoint := cli.DaemonHost()
		runtime := detectRuntime(info, endpoint, preference)

		if preference != RuntimeAuto && runtime != preference {
			attempts = append(attempts, fmt.Sprintf("%s: detected %s runtime", candidate.label, runtime))
			_ = cli.Close()
			continue
		}

		if logger != nil {
			logger.Debug().Str("host", endpoint).Str("runtime", string(runtime)).Msg("Connected to container runtime")
		}

		return cli, info, runtime, nil
	}

	if len(attempts) == 0 {
		return nil, systemtypes.Info{}, RuntimeAuto, errors.New("no container runtime endpoints to try")
	}

	return nil, systemtypes.Info{}, RuntimeAuto, fmt.Errorf("failed to connect to container runtime: %s", strings.Join(attempts, "; "))
}

func tryRuntimeCandidate(opts []client.Opt) (dockerClient, systemtypes.Info, error) {
	cli, err := newDockerClientFn(opts...)
	if err != nil {
		return nil, systemtypes.Info{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := cli.Info(ctx)
	if err != nil {
		_ = cli.Close()
		return nil, systemtypes.Info{}, err
	}

	return cli, info, nil
}

func buildRuntimeCandidates(preference RuntimeKind) []runtimeCandidate {
	candidates := make([]runtimeCandidate, 0, 6)
	seen := make(map[string]struct{})

	add := func(candidate runtimeCandidate) {
		hostKey := candidate.host
		if hostKey == "" {
			hostKey = "__default__"
		}
		if _, ok := seen[hostKey]; ok {
			return
		}
		seen[hostKey] = struct{}{}
		candidates = append(candidates, candidate)
	}

	add(runtimeCandidate{
		label:          "environment defaults",
		applyDockerEnv: true,
	})

	if host := utils.GetenvTrim("DOCKER_HOST"); host != "" {
		add(runtimeCandidate{
			host:           host,
			label:          "DOCKER_HOST",
			applyDockerEnv: true,
		})
	}

	if host := utils.GetenvTrim("CONTAINER_HOST"); host != "" {
		add(runtimeCandidate{
			host:  host,
			label: "CONTAINER_HOST",
		})
	}

	if host := utils.GetenvTrim("PODMAN_HOST"); host != "" {
		add(runtimeCandidate{
			host:  host,
			label: "PODMAN_HOST",
		})
	}

	if preference == RuntimePodman || preference == RuntimeAuto {
		rootless := fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", os.Getuid())
		add(runtimeCandidate{
			host:  rootless,
			label: "podman rootless socket",
		})

		add(runtimeCandidate{
			host:  "unix:///run/podman/podman.sock",
			label: "podman system socket",
		})
	}

	if preference == RuntimeDocker || preference == RuntimeAuto {
		add(runtimeCandidate{
			host:           "unix:///var/run/docker.sock",
			label:          "default docker socket",
			applyDockerEnv: true,
		})
	}

	return candidates
}

func detectRuntime(info systemtypes.Info, endpoint string, preference RuntimeKind) RuntimeKind {
	if preference == RuntimePodman {
		return RuntimePodman
	}

	lowerEndpoint := strings.ToLower(endpoint)
	if strings.Contains(lowerEndpoint, "podman") || strings.Contains(lowerEndpoint, "libpod") {
		return RuntimePodman
	}

	if strings.Contains(strings.ToLower(info.InitBinary), "podman") {
		return RuntimePodman
	}

	if strings.Contains(strings.ToLower(info.ServerVersion), "podman") {
		return RuntimePodman
	}

	for _, pair := range info.DriverStatus {
		if strings.Contains(strings.ToLower(pair[0]), "podman") || strings.Contains(strings.ToLower(pair[1]), "podman") {
			return RuntimePodman
		}
	}

	for _, option := range info.SecurityOptions {
		if strings.Contains(strings.ToLower(option), "podman") {
			return RuntimePodman
		}
	}

	if preference == RuntimeDocker {
		return RuntimeDocker
	}

	return RuntimeDocker
}

// Run starts the collection loop until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	interval := a.cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
		a.cfg.Interval = interval
	}

	ticker := newTickerFn(interval)
	defer ticker.Stop()

	const (
		updateInterval        = 24 * time.Hour
		startupJitterWindow   = 2 * time.Minute
		recurringJitterWindow = 5 * time.Minute
	)

	initialDelay := 5*time.Second + randomDurationFn(startupJitterWindow)
	updateTimer := newTimerFn(initialDelay)
	defer stopTimer(updateTimer)

	if err := a.collectOnce(ctx); err != nil {
		if errors.Is(err, ErrStopRequested) {
			return nil
		}
		a.logger.Error().Err(err).Msg("Failed to send initial report")
	}

	for {
		select {
		case <-ctx.Done():
			stopTimer(updateTimer)
			return ctx.Err()
		case <-ticker.C:
			if err := a.collectOnce(ctx); err != nil {
				if errors.Is(err, ErrStopRequested) {
					return nil
				}
				a.logger.Error().Err(err).Msg("Failed to send docker report")
			}
		case <-updateTimer.C:
			go a.checkForUpdates(ctx)
			nextDelay := updateInterval + randomDurationFn(recurringJitterWindow)
			if nextDelay <= 0 {
				nextDelay = updateInterval
			}
			updateTimer.Reset(nextDelay)
		}
	}
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (a *Agent) collectOnce(ctx context.Context) error {
	report, err := a.buildReport(ctx)
	if err != nil {
		return err
	}

	if err := a.sendReport(ctx, report); err != nil {
		if errors.Is(err, ErrStopRequested) {
			return nil
		}
		a.logger.Warn().Err(err).Msg("Failed to send docker report, buffering")
		a.reportBuffer.Push(report)
		return nil
	}

	a.flushBuffer(ctx)
	return nil
}

func (a *Agent) flushBuffer(ctx context.Context) {
	report, ok := a.reportBuffer.Peek()
	if !ok {
		return
	}

	a.logger.Info().Int("count", a.reportBuffer.Len()).Msg("Flushing buffered docker reports")

	for {
		if err := a.sendReport(ctx, report); err != nil {
			if errors.Is(err, ErrStopRequested) {
				return
			}
			a.logger.Warn().Err(err).Msg("Failed to flush buffered docker report, stopping flush")
			return
		}
		a.reportBuffer.Pop()

		report, ok = a.reportBuffer.Peek()
		if !ok {
			return
		}
	}
}

func (a *Agent) buildReport(ctx context.Context) (agentsdocker.Report, error) {
	info, err := a.docker.Info(ctx)
	if err != nil {
		return agentsdocker.Report{}, fmt.Errorf("failed to query docker info: %w", err)
	}

	a.runtimeVer = info.ServerVersion
	if a.daemonHost == "" {
		a.daemonHost = a.docker.DaemonHost()
	}

	newRuntime := detectRuntime(info, a.daemonHost, RuntimeAuto)
	if newRuntime != a.runtime {
		if a.runtime != "" {
			a.logger.Info().
				Str("runtime_previous", string(a.runtime)).
				Str("runtime_current", string(newRuntime)).
				Msg("Detected container runtime change")
		}
		a.runtime = newRuntime
		a.supportsSwarm = newRuntime == RuntimeDocker
		if newRuntime == RuntimePodman {
			if a.cfg.IncludeServices {
				a.logger.Warn().Msg("Podman runtime detected during report; disabling Swarm service collection")
			}
			if a.cfg.IncludeTasks {
				a.logger.Warn().Msg("Podman runtime detected during report; disabling Swarm task collection")
			}
			a.cfg.IncludeServices = false
			a.cfg.IncludeTasks = false
		}
		a.cfg.Runtime = string(newRuntime)
	}

	a.cpuCount = info.NCPU

	agentID := a.cfg.AgentID
	if agentID == "" {
		// Use cached daemon ID from init rather than info.ID from current call.
		// Podman can return different/empty IDs across calls, causing token
		// binding conflicts on the server.
		agentID = a.daemonID
	}
	if agentID == "" {
		agentID = a.machineID
	}
	if agentID == "" {
		agentID = a.hostName
	}
	a.hostID = agentID

	hostName := a.hostName
	if hostName == "" {
		hostName = info.Name
	}

	uptime := readSystemUptime()

	metricsCtx, metricsCancel := context.WithTimeout(ctx, 10*time.Second)
	snapshot, err := hostmetricsCollect(metricsCtx, nil)
	metricsCancel()
	if err != nil {
		return agentsdocker.Report{}, fmt.Errorf("collect host metrics: %w", err)
	}

	collectContainers := a.cfg.IncludeContainers
	if !collectContainers && (a.cfg.IncludeServices || a.cfg.IncludeTasks) && !info.Swarm.ControlAvailable {
		collectContainers = true
	}

	var containers []agentsdocker.Container
	if collectContainers {
		var err error
		containers, err = a.collectContainers(ctx)
		if err != nil {
			return agentsdocker.Report{}, err
		}
	}

	services, tasks, swarmInfo := a.collectSwarmData(ctx, info, containers)

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              agentID,
			Version:         a.agentVersion,
			Type:            a.cfg.AgentType,
			IntervalSeconds: int(a.cfg.Interval / time.Second),
		},
		Host: agentsdocker.HostInfo{
			Hostname:         hostName,
			Name:             info.Name,
			MachineID:        a.machineID,
			OS:               info.OperatingSystem,
			Runtime:          string(a.runtime),
			RuntimeVersion:   a.runtimeVer,
			KernelVersion:    info.KernelVersion,
			Architecture:     info.Architecture,
			DockerVersion:    info.ServerVersion,
			TotalCPU:         info.NCPU,
			TotalMemoryBytes: info.MemTotal,
			UptimeSeconds:    uptime,
			CPUUsagePercent:  safeFloat(snapshot.CPUUsagePercent),
			LoadAverage:      append([]float64(nil), snapshot.LoadAverage...),
			Memory:           snapshot.Memory,
			Disks:            append([]agentsdocker.Disk(nil), snapshot.Disks...),
			Network:          append([]agentsdocker.NetworkInterface(nil), snapshot.Network...),
		},
		Timestamp: time.Now().UTC(),
	}

	if swarmInfo != nil {
		report.Host.Swarm = swarmInfo
	}

	if a.cfg.IncludeContainers {
		report.Containers = containers
	}
	if a.cfg.IncludeServices && len(services) > 0 {
		report.Services = services
	}
	if a.cfg.IncludeTasks && len(tasks) > 0 {
		report.Tasks = tasks
	}

	if report.Agent.IntervalSeconds <= 0 {
		report.Agent.IntervalSeconds = int(30 * time.Second / time.Second)
	}

	return report, nil
}

func (a *Agent) collectContainers(ctx context.Context) ([]agentsdocker.Container, error) {
	options := containertypes.ListOptions{All: true}
	if len(a.stateFilters) > 0 {
		filterArgs := filters.NewArgs()
		for _, state := range a.stateFilters {
			filterArgs.Add("status", state)
		}
		options.Filters = filterArgs
	}

	list, err := a.docker.ContainerList(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	containers := make([]agentsdocker.Container, 0, len(list))
	active := make(map[string]struct{}, len(list))
	for _, summary := range list {
		if len(a.allowedStates) > 0 {
			if _, ok := a.allowedStates[strings.ToLower(summary.State)]; !ok {
				continue
			}
		}

		active[summary.ID] = struct{}{}

		container, err := a.collectContainer(ctx, summary)
		if err != nil {
			a.logger.Warn().Str("container", strings.Join(summary.Names, ",")).Err(err).Msg("Failed to collect container stats")
			continue
		}
		containers = append(containers, container)
	}
	a.pruneStaleCPUSamples(active)
	return containers, nil
}

func (a *Agent) pruneStaleCPUSamples(active map[string]struct{}) {
	if len(a.prevContainerCPU) == 0 {
		return
	}

	for id := range a.prevContainerCPU {
		if _, ok := active[id]; !ok {
			delete(a.prevContainerCPU, id)
		}
	}
}

func (a *Agent) collectContainer(ctx context.Context, summary containertypes.Summary) (agentsdocker.Container, error) {
	const perContainerTimeout = 15 * time.Second

	containerCtx, cancel := context.WithTimeout(ctx, perContainerTimeout)
	defer cancel()

	requestSize := a.cfg.CollectDiskMetrics
	inspect, _, err := a.docker.ContainerInspectWithRaw(containerCtx, summary.ID, requestSize)
	if err != nil {
		return agentsdocker.Container{}, fmt.Errorf("inspect: %w", err)
	}

	var (
		cpuPercent float64
		memUsage   int64
		memLimit   int64
		memPercent float64
		blockIO    *agentsdocker.ContainerBlockIO
	)

	if inspect.State.Running || inspect.State.Paused {
		statsResp, err := a.docker.ContainerStatsOneShot(containerCtx, summary.ID)
		if err != nil {
			return agentsdocker.Container{}, fmt.Errorf("stats: %w", err)
		}
		defer statsResp.Body.Close()

		var stats containertypes.StatsResponse
		if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
			return agentsdocker.Container{}, fmt.Errorf("decode stats: %w", err)
		}

		cpuPercent = a.calculateContainerCPUPercent(summary.ID, stats)
		memUsage, memLimit, memPercent = calculateMemoryUsage(stats)
		blockIO = summarizeBlockIO(stats)
	} else {
		delete(a.prevContainerCPU, summary.ID)
	}

	createdAt := time.Unix(summary.Created, 0)

	startedAt := parseTime(inspect.State.StartedAt)
	finishedAt := parseTime(inspect.State.FinishedAt)

	uptimeSeconds := int64(0)
	if !startedAt.IsZero() && inspect.State.Running {
		uptimeSeconds = int64(time.Since(startedAt).Seconds())
		if uptimeSeconds < 0 {
			uptimeSeconds = 0
		}
	}

	health := ""
	if inspect.State.Health != nil {
		health = inspect.State.Health.Status
	}

	ports := make([]agentsdocker.ContainerPort, len(summary.Ports))
	for i, port := range summary.Ports {
		ports[i] = agentsdocker.ContainerPort{
			PrivatePort: int(port.PrivatePort),
			PublicPort:  int(port.PublicPort),
			Protocol:    port.Type,
			IP:          port.IP,
		}
	}

	labels := make(map[string]string, len(summary.Labels))
	for k, v := range summary.Labels {
		labels[k] = v
	}

	networks := make([]agentsdocker.ContainerNetwork, 0)
	if inspect.NetworkSettings != nil {
		for name, cfg := range inspect.NetworkSettings.Networks {
			networks = append(networks, agentsdocker.ContainerNetwork{
				Name: name,
				IPv4: cfg.IPAddress,
				IPv6: cfg.GlobalIPv6Address,
			})
		}
	}

	var startedPtr, finishedPtr *time.Time
	if !startedAt.IsZero() {
		started := startedAt
		startedPtr = &started
	}
	if !finishedAt.IsZero() && !inspect.State.Running {
		finished := finishedAt
		finishedPtr = &finished
	}

	var writableLayerBytes int64
	if inspect.SizeRw != nil {
		writableLayerBytes = *inspect.SizeRw
	}

	var rootFsBytes int64
	if inspect.SizeRootFs != nil {
		rootFsBytes = *inspect.SizeRootFs
	}

	var mounts []agentsdocker.ContainerMount
	if len(inspect.Mounts) > 0 {
		mounts = make([]agentsdocker.ContainerMount, 0, len(inspect.Mounts))
		for _, mount := range inspect.Mounts {
			mounts = append(mounts, agentsdocker.ContainerMount{
				Type:        string(mount.Type),
				Source:      mount.Source,
				Destination: mount.Destination,
				Mode:        mount.Mode,
				RW:          mount.RW,
				Propagation: string(mount.Propagation),
				Name:        mount.Name,
				Driver:      mount.Driver,
			})
		}
	}

	container := agentsdocker.Container{
		ID:                  summary.ID,
		Name:                trimLeadingSlash(summary.Names),
		Image:               summary.Image,
		ImageDigest:         summary.ImageID, // sha256:... digest of the image
		CreatedAt:           createdAt,
		State:               summary.State,
		Status:              summary.Status,
		Health:              health,
		CPUPercent:          cpuPercent,
		MemoryUsageBytes:    memUsage,
		MemoryLimitBytes:    memLimit,
		MemoryPercent:       memPercent,
		UptimeSeconds:       uptimeSeconds,
		RestartCount:        inspect.RestartCount,
		ExitCode:            inspect.State.ExitCode,
		StartedAt:           startedPtr,
		FinishedAt:          finishedPtr,
		Ports:               ports,
		Labels:              labels,
		Env:                 maskSensitiveEnvVars(inspect.Config.Env),
		Networks:            networks,
		WritableLayerBytes:  writableLayerBytes,
		RootFilesystemBytes: rootFsBytes,
		BlockIO:             blockIO,
		Mounts:              mounts,
	}

	if a.runtime == RuntimePodman {
		if meta := extractPodmanMetadata(labels); meta != nil {
			container.Podman = meta
		}
	}

	// Check for image updates if registry checker is enabled
	if a.registryChecker != nil && a.registryChecker.Enabled() {
		// Get the actual manifest digest (RepoDigest) from the image for accurate comparison.
		// The ImageID is a local content-addressable ID that differs from the registry manifest digest.
		digestForComparison := a.getImageRepoDigest(containerCtx, summary.ImageID, summary.Image)
		if digestForComparison == "" {
			// Fall back to ImageID if we can't get RepoDigest (shouldn't compare as equal)
			digestForComparison = summary.ImageID
		}
		result := a.registryChecker.CheckImageUpdate(ctx, container.Image, digestForComparison)
		if result != nil {
			container.UpdateStatus = &agentsdocker.UpdateStatus{
				UpdateAvailable: result.UpdateAvailable,
				CurrentDigest:   result.CurrentDigest,
				LatestDigest:    result.LatestDigest,
				LastChecked:     result.CheckedAt,
				Error:           result.Error,
			}
		}
	}

	if requestSize {
		a.logger.Debug().
			Str("container", container.Name).
			Int64("writableLayerBytes", writableLayerBytes).
			Int64("rootFilesystemBytes", rootFsBytes).
			Int("mountCount", len(mounts)).
			Msg("Collected container disk metrics")
	}

	return container, nil
}

// getImageRepoDigest retrieves the RepoDigest for an image, which is the actual
// manifest digest from the registry. This is necessary because Docker's ImageID
// is a local content-addressable hash that differs from the registry manifest digest.
// For multi-arch images, the registry returns a manifest list digest, while Docker
// stores the platform-specific image config digest locally.
func (a *Agent) getImageRepoDigest(ctx context.Context, imageID, imageName string) string {
	imageInspect, _, err := a.docker.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		a.logger.Debug().
			Err(err).
			Str("imageID", imageID).
			Str("imageName", imageName).
			Msg("Failed to inspect image for RepoDigest")
		return ""
	}

	if len(imageInspect.RepoDigests) == 0 {
		// Locally built images won't have RepoDigests
		return ""
	}

	// Try to find a RepoDigest that matches the image reference
	// RepoDigests format: "registry/repo@sha256:..."
	for _, repoDigest := range imageInspect.RepoDigests {
		// Extract just the digest part (after @)
		if idx := strings.LastIndex(repoDigest, "@"); idx >= 0 {
			repoRef := repoDigest[:idx] // e.g., "docker.io/library/nginx"
			digest := repoDigest[idx+1:] // e.g., "sha256:abc..."

			// Check if this RepoDigest matches our image reference
			// Normalize both for comparison
			if matchesImageReference(imageName, repoRef) {
				return digest
			}
		}
	}

	// If no exact match, return the first RepoDigest's digest
	// This handles cases where the image was pulled with a different tag
	if idx := strings.LastIndex(imageInspect.RepoDigests[0], "@"); idx >= 0 {
		return imageInspect.RepoDigests[0][idx+1:]
	}

	return ""
}

// matchesImageReference checks if a RepoDigest repository matches an image reference.
// It handles Docker Hub's various naming conventions.
func matchesImageReference(imageName, repoRef string) bool {
	// Normalize image name by removing tag
	if idx := strings.LastIndex(imageName, ":"); idx >= 0 {
		// Make sure it's a tag, not a port (check if there's a / after it)
		if !strings.Contains(imageName[idx:], "/") {
			imageName = imageName[:idx]
		}
	}

	// Direct match
	if imageName == repoRef {
		return true
	}

	// Docker Hub library images: "nginx" == "docker.io/library/nginx"
	if repoRef == "docker.io/library/"+imageName {
		return true
	}

	// Docker Hub with namespace: "myuser/myapp" == "docker.io/myuser/myapp"
	if repoRef == "docker.io/"+imageName {
		return true
	}

	// Registry prefix matching (e.g., "ghcr.io/user/repo" matches "ghcr.io/user/repo")
	// Already handled by direct match above

	return false
}

func extractPodmanMetadata(labels map[string]string) *agentsdocker.PodmanContainer {
	if len(labels) == 0 {
		return nil
	}

	meta := &agentsdocker.PodmanContainer{}

	if v := strings.TrimSpace(labels["io.podman.annotations.pod.name"]); v != "" {
		meta.PodName = v
	}

	if v := strings.TrimSpace(labels["io.podman.annotations.pod.id"]); v != "" {
		meta.PodID = v
	}

	if v := strings.TrimSpace(labels["io.podman.annotations.pod.infra"]); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			meta.Infra = parsed
		} else if strings.EqualFold(v, "yes") || strings.EqualFold(v, "true") {
			meta.Infra = true
		}
	}

	if v := strings.TrimSpace(labels["io.podman.compose.project"]); v != "" {
		meta.ComposeProject = v
	}

	if v := strings.TrimSpace(labels["io.podman.compose.service"]); v != "" {
		meta.ComposeService = v
	}

	if v := strings.TrimSpace(labels["io.podman.compose.working_dir"]); v != "" {
		meta.ComposeWorkdir = v
	}

	if v := strings.TrimSpace(labels["io.podman.compose.config-hash"]); v != "" {
		meta.ComposeConfig = v
	}

	if v := strings.TrimSpace(labels["io.containers.autoupdate"]); v != "" {
		meta.AutoUpdatePolicy = v
	}

	if v := strings.TrimSpace(labels["io.containers.autoupdate.restart"]); v != "" {
		meta.AutoUpdateRestart = v
	}

	if v := strings.TrimSpace(labels["io.podman.annotations.userns"]); v != "" {
		meta.UserNS = v
	} else if v := strings.TrimSpace(labels["io.containers.userns"]); v != "" {
		meta.UserNS = v
	}

	if meta.PodName == "" && meta.PodID == "" && meta.ComposeProject == "" && meta.AutoUpdatePolicy == "" && meta.UserNS == "" && !meta.Infra {
		return nil
	}

	return meta
}

// sensitiveEnvPatterns are substrings that, when found in an env var name (case-insensitive),
// indicate the value should be masked for security.
var sensitiveEnvPatterns = []string{
	"password", "passwd", "secret", "key", "token", "credential", "auth",
	"api_key", "apikey", "private", "access_token", "refresh_token",
	"database_url", "connection_string", "encryption",
}

// maskSensitiveEnvVars returns a copy of the environment variables with sensitive values masked.
// Environment variables whose names contain sensitive keywords will have their values replaced with "***".
func maskSensitiveEnvVars(envVars []string) []string {
	if len(envVars) == 0 {
		return nil
	}

	result := make([]string, 0, len(envVars))
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			result = append(result, env)
			continue
		}

		name := parts[0]
		value := parts[1]

		// Check if the environment variable name contains a sensitive pattern
		lowerName := strings.ToLower(name)
		isSensitive := false
		for _, pattern := range sensitiveEnvPatterns {
			if strings.Contains(lowerName, pattern) {
				isSensitive = true
				break
			}
		}

		if isSensitive && value != "" {
			result = append(result, name+"=***")
		} else {
			result = append(result, env)
		}
	}

	return result
}

func (a *Agent) sendReport(ctx context.Context, report agentsdocker.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	var errs []error
	containerCount := len(report.Containers)

	for _, target := range a.targets {
		err := a.sendReportToTarget(ctx, target, payload, containerCount)
		if err == nil {
			continue
		}
		if errors.Is(err, ErrStopRequested) {
			return ErrStopRequested
		}
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	// Warn if payload is approaching the server's 512KB limit
	const warnThresholdKB = 400
	payloadSizeKB := len(payload) / 1024
	if payloadSizeKB >= warnThresholdKB {
		a.logger.Warn().
			Int("containers", containerCount).
			Int("payloadSizeKB", payloadSizeKB).
			Msg("Report payload is large and approaching the 512KB limit. Consider reducing container count or running 'docker container prune' to remove stopped containers.")
	}

	a.logger.Debug().
		Int("containers", containerCount).
		Int("payloadSizeKB", payloadSizeKB).
		Int("payloadBytes", len(payload)).
		Int("targets", len(a.targets)).
		Msg("Report sent to Pulse targets")
	return nil
}

func (a *Agent) sendReportToTarget(ctx context.Context, target TargetConfig, payload []byte, _ int) error {
	url := fmt.Sprintf("%s/api/agents/docker/report", target.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("target %s: create request: %w", target.URL, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", target.Token)
	req.Header.Set("Authorization", "Bearer "+target.Token)
	req.Header.Set("User-Agent", "pulse-docker-agent/"+Version)

	client := a.httpClientFor(target)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("target %s: send report: %w", target.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if hostRemoved := detectHostRemovedError(bodyBytes); hostRemoved != "" {
			a.logger.Warn().
				Str("hostID", a.hostID).
				Str("pulseURL", target.URL).
				Str("detail", hostRemoved).
				Msg("Pulse rejected docker report because this host was previously removed. Allow the host to re-enroll from the Pulse UI or rerun the installer with a docker:manage token.")
			return ErrStopRequested
		}
		errMsg := strings.TrimSpace(string(bodyBytes))
		if errMsg == "" {
			errMsg = resp.Status
		}
		return fmt.Errorf("target %s: pulse responded %s: %s", target.URL, resp.Status, errMsg)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("target %s: read response: %w", target.URL, err)
	}

	if len(body) == 0 {
		return nil
	}

	var reportResp agentsdocker.ReportResponse
	if err := json.Unmarshal(body, &reportResp); err != nil {
		a.logger.Warn().Err(err).Str("target", target.URL).Msg("Failed to decode Pulse response")
		return nil
	}

	for _, command := range reportResp.Commands {
		err := a.handleCommand(ctx, target, command)
		if err == nil {
			continue
		}
		if errors.Is(err, ErrStopRequested) {
			return ErrStopRequested
		}
		return err
	}

	return nil
}

func (a *Agent) handleCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	switch strings.ToLower(command.Type) {
	case agentsdocker.CommandTypeStop:
		return a.handleStopCommand(ctx, target, command)
	case agentsdocker.CommandTypeUpdateContainer:
		return a.handleUpdateContainerCommand(ctx, target, command)
	default:
		a.logger.Warn().Str("command", command.Type).Msg("Received unsupported control command")
		return nil
	}
}

func (a *Agent) handleStopCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	a.logger.Info().Str("commandID", command.ID).Msg("Received stop command from Pulse")

	if err := a.disableSelf(ctx); err != nil {
		a.logger.Error().Err(err).Msg("Failed to disable pulse-docker-agent service")
		if ackErr := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusFailed, err.Error()); ackErr != nil {
			a.logger.Error().Err(ackErr).Msg("Failed to send failure acknowledgement to Pulse")
		}
		return nil
	}

	if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusCompleted, "Agent shutting down"); err != nil {
		return fmt.Errorf("send stop acknowledgement: %w", err)
	}

	a.logger.Info().Msg("Stop command acknowledged; terminating agent")

	// After sending the acknowledgement, stop the systemd service to prevent restart.
	// This is done after the ack to ensure the acknowledgement is sent before the
	// process is terminated by systemctl stop.
	go func() {
		// Small delay to ensure the ack response completes
		sleepFn(1 * time.Second)
		stopServiceCtx := context.Background()
		if err := stopSystemdService(stopServiceCtx, "pulse-docker-agent"); err != nil {
			a.logger.Warn().Err(err).Msg("Failed to stop systemd service, agent will exit normally")
		}
	}()

	return ErrStopRequested
}

func (a *Agent) disableSelf(ctx context.Context) error {
	if err := disableSystemdService(ctx, "pulse-docker-agent"); err != nil {
		return err
	}

	// Remove Unraid startup script if present to prevent restart on reboot.
	if err := removeFileIfExists(unraidStartupScriptPath); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to remove Unraid startup script")
	}

	// Best-effort log cleanup (ignore errors).
	_ = removeFileIfExists(agentLogPath)

	return nil
}

func disableSystemdService(ctx context.Context, service string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		// Not a systemd environment; nothing to do.
		return nil
	}

	cmd := exec.CommandContext(ctx, "systemctl", "disable", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			trimmedOutput := strings.TrimSpace(string(output))
			lowerOutput := strings.ToLower(trimmedOutput)
			if exitCode == 5 || strings.Contains(lowerOutput, "could not be found") || strings.Contains(lowerOutput, "not-found") {
				return nil
			}
			if strings.Contains(lowerOutput, "access denied") || strings.Contains(lowerOutput, "permission denied") {
				return fmt.Errorf("systemctl disable %s: access denied. Run 'sudo systemctl disable --now %s' or rerun the installer with sudo so it can install the polkit rule (systemctl output: %s)", service, service, trimmedOutput)
			}
		}
		return fmt.Errorf("systemctl disable %s: %w (%s)", service, err, strings.TrimSpace(string(output)))
	}

	return nil
}

func stopSystemdService(ctx context.Context, service string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		// Not a systemd environment; nothing to do.
		return nil
	}

	// Stop the service to terminate the current running instance.
	// This prevents systemd from restarting the service (services stopped via
	// systemctl stop are not restarted even with Restart=always).
	cmd := exec.CommandContext(ctx, "systemctl", "stop", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			trimmedOutput := strings.TrimSpace(string(output))
			lowerOutput := strings.ToLower(trimmedOutput)
			// Ignore "not found" errors since the service might already be stopped
			if exitCode == 5 || strings.Contains(lowerOutput, "could not be found") || strings.Contains(lowerOutput, "not-found") {
				return nil
			}
			if strings.Contains(lowerOutput, "access denied") || strings.Contains(lowerOutput, "permission denied") {
				return fmt.Errorf("systemctl stop %s: access denied. Run 'sudo systemctl stop %s' or rerun the installer with sudo so it can install the polkit rule (systemctl output: %s)", service, service, trimmedOutput)
			}
		}
		return fmt.Errorf("systemctl stop %s: %w (%s)", service, err, strings.TrimSpace(string(output)))
	}

	return nil
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func (a *Agent) sendCommandAck(ctx context.Context, target TargetConfig, commandID, status, message string) error {
	if a.hostID == "" {
		return fmt.Errorf("host identifier unavailable; cannot acknowledge command")
	}

	ackPayload := agentsdocker.CommandAck{
		HostID:  a.hostID,
		Status:  status,
		Message: message,
	}

	body, err := jsonMarshalFn(ackPayload)
	if err != nil {
		return fmt.Errorf("marshal command acknowledgement: %w", err)
	}

	url := fmt.Sprintf("%s/api/agents/docker/commands/%s/ack", target.URL, commandID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create acknowledgement request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", target.Token)
	req.Header.Set("Authorization", "Bearer "+target.Token)
	req.Header.Set("User-Agent", "pulse-docker-agent/"+Version)

	resp, err := a.httpClientFor(target).Do(req)
	if err != nil {
		return fmt.Errorf("send acknowledgement: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pulse responded %s: %s", resp.Status, strings.TrimSpace(string(bodyBytes)))
	}

	return nil
}

func (a *Agent) primaryTarget() TargetConfig {
	if len(a.targets) == 0 {
		return TargetConfig{}
	}
	return a.targets[0]
}

func (a *Agent) httpClientFor(target TargetConfig) *http.Client {
	if client, ok := a.httpClients[target.InsecureSkipVerify]; ok {
		return client
	}
	if client, ok := a.httpClients[false]; ok {
		return client
	}
	if client, ok := a.httpClients[true]; ok {
		return client
	}
	return newHTTPClient(target.InsecureSkipVerify)
}

func newHTTPClient(insecure bool) *http.Client {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if insecure {
		tlsConfig.InsecureSkipVerify = true //nolint:gosec
	}

	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

func summarizeBlockIO(stats containertypes.StatsResponse) *agentsdocker.ContainerBlockIO {
	var readBytes, writeBytes uint64

	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			readBytes += entry.Value
		case "write":
			writeBytes += entry.Value
		}
	}

	if readBytes == 0 && writeBytes == 0 {
		return nil
	}

	return &agentsdocker.ContainerBlockIO{
		ReadBytes:  readBytes,
		WriteBytes: writeBytes,
	}
}

func (a *Agent) calculateContainerCPUPercent(id string, stats containertypes.StatsResponse) float64 {
	current := cpuSample{
		totalUsage:  stats.CPUStats.CPUUsage.TotalUsage,
		systemUsage: stats.CPUStats.SystemUsage,
		onlineCPUs:  stats.CPUStats.OnlineCPUs,
		read:        stats.Read,
	}

	// Try to use PreCPUStats if available
	percent := calculateCPUPercent(stats, a.cpuCount)
	if percent > 0 {
		a.prevContainerCPU[id] = current
		a.logger.Debug().
			Str("container_id", id[:12]).
			Float64("cpu_percent", percent).
			Msg("CPU calculated from PreCPUStats")
		return percent
	}

	// PreCPUStats not available or invalid, use manual tracking
	a.preCPUStatsFailures++
	if a.preCPUStatsFailures == 10 {
		a.logger.Warn().
			Str("runtime", string(a.runtime)).
			Msg("PreCPUStats consistently unavailable from Docker API - using manual CPU tracking (this is normal for one-shot stats)")
	}
	prev, ok := a.prevContainerCPU[id]
	if !ok {
		// First time seeing this container - store current sample and return 0
		// On next collection cycle we'll have a previous sample to compare against
		a.prevContainerCPU[id] = current
		a.logger.Debug().
			Str("container_id", id[:12]).
			Uint64("total_usage", current.totalUsage).
			Uint64("system_usage", current.systemUsage).
			Msg("First CPU sample collected, no previous data for delta calculation")
		return 0
	}

	// We have a previous sample - update it after calculation
	a.prevContainerCPU[id] = current

	var totalDelta float64
	if current.totalUsage >= prev.totalUsage {
		totalDelta = float64(current.totalUsage - prev.totalUsage)
	} else {
		// Counter likely reset (container restart); fall back to current reading.
		totalDelta = float64(current.totalUsage)
	}

	if totalDelta <= 0 {
		return 0
	}

	onlineCPUs := current.onlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = prev.onlineCPUs
	}
	if onlineCPUs == 0 && a.cpuCount > 0 {
		onlineCPUs = uint32(a.cpuCount)
	}
	if onlineCPUs == 0 {
		return 0
	}

	var systemDelta float64
	if current.systemUsage >= prev.systemUsage {
		systemDelta = float64(current.systemUsage - prev.systemUsage)
	}
	// If systemUsage went backward (counter reset), leave systemDelta as 0
	// to fall through to time-based calculation below

	if systemDelta > 0 {
		cpuPercent := safeFloat((totalDelta / systemDelta) * float64(onlineCPUs) * 100.0)
		a.logger.Debug().
			Str("container_id", id[:12]).
			Float64("cpu_percent", cpuPercent).
			Float64("total_delta", totalDelta).
			Float64("system_delta", systemDelta).
			Uint32("online_cpus", onlineCPUs).
			Msg("CPU calculated from system delta")
		return cpuPercent
	}

	// Fall back to time-based calculation
	if !prev.read.IsZero() && !current.read.IsZero() {
		elapsed := current.read.Sub(prev.read).Seconds()
		if elapsed > 0 {
			denominator := elapsed * float64(onlineCPUs) * 1e9
			if denominator > 0 {
				cpuPercent := (totalDelta / denominator) * 100.0
				result := safeFloat(cpuPercent)
				a.logger.Debug().
					Str("container_id", id[:12]).
					Float64("cpu_percent", result).
					Float64("total_delta", totalDelta).
					Float64("elapsed_seconds", elapsed).
					Uint32("online_cpus", onlineCPUs).
					Msg("CPU calculated from time-based delta")
				return result
			}
		}
	}

	a.logger.Debug().
		Str("container_id", id[:12]).
		Float64("total_delta", totalDelta).
		Float64("system_delta", systemDelta).
		Bool("prev_read_zero", prev.read.IsZero()).
		Bool("current_read_zero", current.read.IsZero()).
		Msg("CPU calculation failed: no valid delta method available")
	return 0
}

func calculateCPUPercent(stats containertypes.StatsResponse, hostCPUs int) float64 {
	totalDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if totalDelta <= 0 || systemDelta <= 0 {
		return 0
	}

	onlineCPUs := stats.CPUStats.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 && hostCPUs > 0 {
		onlineCPUs = uint32(hostCPUs)
	}

	if onlineCPUs == 0 {
		return 0
	}

	return safeFloat((totalDelta / systemDelta) * float64(onlineCPUs) * 100.0)
}

func calculateMemoryUsage(stats containertypes.StatsResponse) (usage int64, limit int64, percent float64) {
	usage = int64(stats.MemoryStats.Usage)
	if cache, ok := stats.MemoryStats.Stats["cache"]; ok {
		usage -= int64(cache)
	}
	if usage < 0 {
		usage = int64(stats.MemoryStats.Usage)
	}

	limit = int64(stats.MemoryStats.Limit)
	if limit > 0 {
		percent = (float64(usage) / float64(limit)) * 100.0
	}

	return usage, limit, safeFloat(percent)
}

func safeFloat(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

func parseTime(value string) time.Time {
	if value == "" || value == "0001-01-01T00:00:00Z" {
		return time.Time{}
	}
	if strings.Contains(value, ".") {
		if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return t
		}
	} else {
		if t, err := time.Parse(time.RFC3339, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func trimLeadingSlash(names []string) string {
	if len(names) == 0 {
		return ""
	}
	name := names[0]
	return strings.TrimPrefix(name, "/")
}

func (a *Agent) Close() error {
	return a.docker.Close()
}

func readMachineID() (string, error) {
	for _, path := range machineIDPaths {
		data, err := osReadFileFn(path)
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}
	return "", errors.New("machine-id not found")
}

func readSystemUptime() int64 {
	seconds, err := readProcUptime()
	if err != nil {
		return 0
	}
	return int64(seconds)
}

func randomDuration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}

	n, err := randIntFn(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}

	return time.Duration(n.Int64())
}

func detectHostRemovedError(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var payload struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if strings.ToLower(payload.Code) != "invalid_report" {
		return ""
	}
	if !strings.Contains(strings.ToLower(payload.Error), "was removed") {
		return ""
	}
	return payload.Error
}

// checkForUpdates checks if a newer version is available and performs self-update if needed
func (a *Agent) checkForUpdates(ctx context.Context) {
	// Skip updates if disabled via config
	if a.cfg.DisableAutoUpdate {
		a.logger.Info().Msg("Skipping update check - auto-update disabled")
		return
	}

	// Skip updates in development mode to prevent update loops
	if Version == "dev" {
		a.logger.Debug().Msg("Skipping update check - running in development mode")
		return
	}

	a.logger.Debug().Msg("Checking for agent updates")

	target := a.primaryTarget()
	if target.URL == "" {
		a.logger.Debug().Msg("Skipping update check - no Pulse target configured")
		return
	}

	// Get current version from server
	url := fmt.Sprintf("%s/api/agent/version", target.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to create version check request")
		return
	}

	if target.Token != "" {
		req.Header.Set("X-API-Token", target.Token)
		req.Header.Set("Authorization", "Bearer "+target.Token)
	}

	client := a.httpClientFor(target)
	resp, err := client.Do(req)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to check for updates")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Warn().Int("status", resp.StatusCode).Msg("Version endpoint returned non-200 status")
		return
	}

	var versionResp struct {
		Version string `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to decode version response")
		return
	}

	// Skip updates if server is also in development mode
	if versionResp.Version == "dev" {
		a.logger.Debug().Msg("Skipping update - server is in development mode")
		return
	}

	// Compare versions - normalize by stripping "v" prefix for comparison.
	// Server returns version without prefix (e.g., "4.33.1"), but agent's
	// Version may include it (e.g., "v4.33.1") depending on build.
	if utils.NormalizeVersion(versionResp.Version) == utils.NormalizeVersion(Version) {
		a.logger.Debug().Str("version", Version).Msg("Agent is up to date")
		return
	}

	a.logger.Info().
		Str("currentVersion", Version).
		Str("availableVersion", versionResp.Version).
		Msg("New agent version available, performing self-update")

	// Perform self-update
	if err := selfUpdateFunc(a, ctx); err != nil {
		a.logger.Error().Err(err).Msg("Failed to self-update agent")
		return
	}

	a.logger.Info().Msg("Agent updated successfully, restarting...")
}

// isUnraid checks if we're running on Unraid by looking for /etc/unraid-version
func isUnraid() bool {
	_, err := osStatFn(unraidVersionPath)
	return err == nil
}

// resolveSymlink resolves symlinks to get the real path of a file.
// This is needed for self-update because os.Rename() fails across filesystems.
func resolveSymlink(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// verifyELFMagic checks that the file is a valid ELF binary
func verifyELFMagic(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("failed to read magic bytes: %w", err)
	}

	// ELF magic: 0x7f 'E' 'L' 'F'
	if magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F' {
		return nil
	}
	return errors.New("not a valid ELF binary")
}

func determineSelfUpdateArch() string {
	switch goArch {
	case "amd64":
		return "linux-amd64"
	case "arm64":
		return "linux-arm64"
	case "arm":
		return "linux-armv7"
	}

	out, err := unameMachine()
	if err != nil {
		return ""
	}

	normalized := strings.ToLower(strings.TrimSpace(out))
	switch normalized {
	case "x86_64", "amd64":
		return "linux-amd64"
	case "aarch64", "arm64":
		return "linux-arm64"
	case "armv7l", "armhf", "armv7":
		return "linux-armv7"
	default:
		return ""
	}
}

// selfUpdate downloads the new agent binary and replaces the current one
func (a *Agent) selfUpdate(ctx context.Context) error {
	target := a.primaryTarget()
	if target.URL == "" {
		return errors.New("no Pulse target configured for self-update")
	}

	// Get path to current executable
	execPath, err := osExecutableFn()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the real path for atomic rename
	// os.Rename() fails across filesystems, so we need the actual target path
	realExecPath, err := resolveSymlink(execPath)
	if err != nil {
		a.logger.Debug().Err(err).Str("path", execPath).Msg("Failed to resolve symlinks, using original path")
		realExecPath = execPath
	} else if realExecPath != execPath {
		a.logger.Debug().
			Str("symlink", execPath).
			Str("target", realExecPath).
			Msg("Resolved symlink for self-update")
	}

	downloadBase := strings.TrimRight(target.URL, "/") + "/download/pulse-docker-agent"
	archParam := determineSelfUpdateArch()

	type downloadCandidate struct {
		url  string
		arch string
	}

	candidates := make([]downloadCandidate, 0, 2)
	if archParam != "" {
		candidates = append(candidates, downloadCandidate{
			url:  fmt.Sprintf("%s?arch=%s", downloadBase, archParam),
			arch: archParam,
		})
	}
	candidates = append(candidates, downloadCandidate{url: downloadBase})

	client := a.httpClientFor(target)
	var resp *http.Response
	lastErr := errors.New("failed to download new binary")

	for _, candidate := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.url, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create download request: %w", err)
			continue
		}

		if target.Token != "" {
			req.Header.Set("X-API-Token", target.Token)
			req.Header.Set("Authorization", "Bearer "+target.Token)
		}

		response, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to download new binary: %w", err)
			continue
		}

		if response.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("download failed with status: %s", response.Status)
			response.Body.Close()
			continue
		}

		resp = response
		if candidate.arch != "" {
			a.logger.Debug().
				Str("arch", candidate.arch).
				Msg("Self-update: downloaded architecture-specific agent binary")
		} else if archParam != "" {
			a.logger.Debug().Msg("Self-update: falling back to server default agent binary")
		}
		break
	}

	if resp == nil {
		return lastErr
	}
	defer resp.Body.Close()

	checksumHeader := strings.TrimSpace(resp.Header.Get("X-Checksum-Sha256"))

	// Create temporary file in the same directory as the target binary
	// to ensure atomic rename works (os.Rename fails across filesystems)
	targetDir := filepath.Dir(realExecPath)
	tmpFile, err := osCreateTempFn(targetDir, "pulse-docker-agent-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", targetDir, err)
	}
	tmpPath := tmpFile.Name()
	defer osRemoveFn(tmpPath) // Clean up if something goes wrong

	// Write downloaded binary to temp file with size limit (100 MB max)
	const maxBinarySize = 100 * 1024 * 1024
	hasher := sha256.New()
	limitedReader := io.LimitReader(resp.Body, maxBinarySize+1)
	written, err := io.Copy(tmpFile, io.TeeReader(limitedReader, hasher))
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write downloaded binary: %w", err)
	}
	if written > maxBinarySize {
		tmpFile.Close()
		return fmt.Errorf("downloaded binary exceeds maximum size (%d bytes)", maxBinarySize)
	}
	if err := closeFileFn(tmpFile); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify it's a valid ELF binary (basic sanity check for Linux)
	if err := verifyELFMagic(tmpPath); err != nil {
		return fmt.Errorf("downloaded file is not a valid executable: %w", err)
	}

	// Verify checksum (mandatory for security)
	downloadChecksum := hex.EncodeToString(hasher.Sum(nil))
	if checksumHeader == "" {
		return fmt.Errorf("server did not provide checksum header (X-Checksum-Sha256); refusing update for security")
	}

	expected := strings.ToLower(strings.TrimSpace(checksumHeader))
	actual := strings.ToLower(downloadChecksum)
	if expected != actual {
		return fmt.Errorf("checksum verification failed: expected %s, got %s", expected, actual)
	}
	a.logger.Debug().Str("checksum", downloadChecksum).Msg("Self-update: checksum verified")

	// Make temp file executable
	if err := osChmodFn(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to make temp file executable: %w", err)
	}

	// Create backup of current binary (use realExecPath for atomic operations)
	backupPath := realExecPath + ".backup"
	if err := osRenameFn(realExecPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary to current location
	if err := osRenameFn(tmpPath, realExecPath); err != nil {
		// Restore backup on failure
		_ = osRenameFn(backupPath, realExecPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup on success
	_ = osRemoveFn(backupPath)

	// On Unraid, also update the persistent copy on the flash drive
	if isUnraid() {
		persistPath := unraidPersistPath
		if _, err := osStatFn(persistPath); err == nil {
			a.logger.Debug().Str("path", persistPath).Msg("Updating Unraid persistent binary")
			if newBinary, err := osReadFileFn(execPath); err == nil {
				tmpPersist := persistPath + ".tmp"
				if err := osWriteFileFn(tmpPersist, newBinary, 0644); err != nil {
					a.logger.Warn().Err(err).Msg("Failed to write Unraid persistent binary")
				} else if err := osRenameFn(tmpPersist, persistPath); err != nil {
					a.logger.Warn().Err(err).Msg("Failed to rename Unraid persistent binary")
					_ = osRemoveFn(tmpPersist)
				} else {
					a.logger.Info().Str("path", persistPath).Msg("Updated Unraid persistent binary")
				}
			}
		}
	}

	// Restart agent with same arguments
	args := os.Args
	env := os.Environ()

	if err := syscallExecFn(execPath, args, env); err != nil {
		return fmt.Errorf("failed to restart agent: %w", err)
	}

	return nil
}
