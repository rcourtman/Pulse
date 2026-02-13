package dockeragent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	systemtypes "github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
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
	PulseURL            string
	APIToken            string
	Interval            time.Duration
	HostnameOverride    string
	AgentID             string
	AgentType           string // "unified" when running as part of pulse-agent, empty for standalone
	AgentVersion        string // Version to report; if empty, uses dockeragent.Version
	InsecureSkipVerify  bool
	DisableAutoUpdate   bool
	DisableUpdateChecks bool // Disable Docker image update detection (registry checks)
	Targets             []TargetConfig
	ContainerStates     []string
	SwarmScope          string
	Runtime             string
	IncludeServices     bool
	IncludeTasks        bool
	IncludeContainers   bool
	CollectDiskMetrics  bool
	DiskExclude         []string // Mount points or path prefixes to exclude from disk monitoring
	LogLevel            zerolog.Level
	Logger              *zerolog.Logger
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

// backupContainerMarker is the substring used to identify backup containers
// created during container updates.
const backupContainerMarker = "_pulse_backup_"

// isBackupContainer reports whether any of the given container names contains
// the Pulse backup marker (e.g. "myapp_pulse_backup_20240101_120000").
func isBackupContainer(names []string) bool {
	for _, name := range names {
		if strings.Contains(name, backupContainerMarker) {
			return true
		}
	}
	return false
}

// setAgentHeaders sets the standard authentication and metadata headers for
// requests from the Docker agent to a Pulse backend.
func setAgentHeaders(req *http.Request, token string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", token)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "pulse-docker-agent/"+Version)
}

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
	cpuMu               sync.Mutex // protects prevContainerCPU and preCPUStatsFailures
	preCPUStatsFailures int
	reportBuffer        *utils.Queue[agentsdocker.Report]
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
		return nil, fmt.Errorf("dockeragent.New: normalize targets: %w", err)
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
			return nil, fmt.Errorf("dockeragent.New: normalize fallback target: %w", err)
		}
	}

	cfg.Targets = targets
	cfg.PulseURL = targets[0].URL
	cfg.APIToken = targets[0].Token
	cfg.InsecureSkipVerify = targets[0].InsecureSkipVerify

	stateFilters, err := normalizeContainerStates(cfg.ContainerStates)
	if err != nil {
		return nil, fmt.Errorf("dockeragent.New: normalize container states: %w", err)
	}
	cfg.ContainerStates = stateFilters

	scope, err := normalizeSwarmScope(cfg.SwarmScope)
	if err != nil {
		return nil, fmt.Errorf("dockeragent.New: normalize swarm scope: %w", err)
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
		return nil, fmt.Errorf("dockeragent.New: normalize runtime: %w", err)
	}

	dockerClient, info, runtimeKind, err := connectRuntimeFn(runtimePref, logger)
	if err != nil {
		return nil, fmt.Errorf("dockeragent.New: connect runtime: %w", err)
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
		reportBuffer:     utils.NewQueue[agentsdocker.Report](bufferCapacity),
		registryChecker:  newRegistryCheckerWithConfig(*logger, !cfg.DisableUpdateChecks),
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
		targetURL := strings.TrimSpace(target.URL)
		token := strings.TrimSpace(target.Token)
		if targetURL == "" && token == "" {
			continue
		}

		if targetURL == "" {
			return nil, errors.New("pulse target URL is required")
		}
		if token == "" {
			return nil, fmt.Errorf("pulse target %s is missing API token", targetURL)
		}

		normalizedURL, err := normalizeTargetURL(url)
		if err != nil {
			return nil, fmt.Errorf("invalid pulse target URL %q: %w", url, err)
		}

		key := fmt.Sprintf("%s|%s|%t", normalizedURL, token, target.InsecureSkipVerify)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		normalized = append(normalized, TargetConfig{
			URL:                normalizedURL,
			Token:              token,
			InsecureSkipVerify: target.InsecureSkipVerify,
		})
	}

	return normalized, nil
}

func normalizeTargetURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("must include http:// or https:// with a valid host")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}

	if parsed.User != nil {
		return "", errors.New("userinfo is not supported")
	}

	if parsed.RawQuery != "" {
		return "", errors.New("query parameters are not supported")
	}

	if parsed.Fragment != "" {
		return "", errors.New("fragments are not supported")
	}

	if parsed.Hostname() == "" {
		return "", errors.New("host is required")
	}

	if port := parsed.Port(); port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum < 1 || portNum > 65535 {
			return "", fmt.Errorf("invalid port %q: must be between 1 and 65535", port)
		}
	}

	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""

	normalized := strings.TrimRight(parsed.String(), "/")
	if normalized == "" {
		return "", errors.New("URL is empty after normalization")
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
			if closeErr := cli.Close(); closeErr != nil {
				attempts = append(attempts, fmt.Sprintf("%s: close client after runtime mismatch: %v", candidate.label, closeErr))
			}
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
		if closeErr := cli.Close(); closeErr != nil {
			return nil, systemtypes.Info{}, errors.Join(
				err,
				fmt.Errorf("close runtime client after info failure: %w", closeErr),
			)
		}
		return nil, systemtypes.Info{}, err
	}

	return cli, info, nil
}

func buildRuntimeCandidates(preference RuntimeKind) []runtimeCandidate {
	candidates := make([]runtimeCandidate, 0, 8)
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

	// When podman is explicitly requested, try podman-specific sockets FIRST
	// before falling back to environment defaults (which try /var/run/docker.sock)
	if preference == RuntimePodman {
		if host := utils.GetenvTrim("PODMAN_HOST"); host != "" {
			add(runtimeCandidate{
				host:  host,
				label: "PODMAN_HOST",
			})
		}

		rootless := fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", os.Getuid())
		add(runtimeCandidate{
			host:  rootless,
			label: "podman rootless socket",
		})

		add(runtimeCandidate{
			host:  "unix:///run/podman/podman.sock",
			label: "podman system socket",
		})

		// Some distros (CoreOS, Fedora) use /var/run/podman instead of /run/podman
		add(runtimeCandidate{
			host:  "unix:///var/run/podman/podman.sock",
			label: "podman system socket (var/run)",
		})
	}

	// Environment defaults (uses Docker client defaults)
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

	// For auto mode, check podman after environment defaults
	if preference == RuntimeAuto {
		if host := utils.GetenvTrim("PODMAN_HOST"); host != "" {
			add(runtimeCandidate{
				host:  host,
				label: "PODMAN_HOST",
			})
		}

		rootless := fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", os.Getuid())
		add(runtimeCandidate{
			host:  rootless,
			label: "podman rootless socket",
		})

		add(runtimeCandidate{
			host:  "unix:///run/podman/podman.sock",
			label: "podman system socket",
		})

		// Some distros (CoreOS, Fedora) use /var/run/podman instead of /run/podman
		add(runtimeCandidate{
			host:  "unix:///var/run/podman/podman.sock",
			label: "podman system socket (var/run)",
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

	// Periodic cleanup of orphaned backups (every 15 minutes)
	cleanupTicker := newTickerFn(15 * time.Minute)
	defer cleanupTicker.Stop()

	// Perform cleanup of orphaned backup containers on startup
	go a.cleanupOrphanedBackups(ctx)

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
		case <-cleanupTicker.C:
			go a.cleanupOrphanedBackups(ctx)
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
		return fmt.Errorf("build docker report: %w", err)
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

	setAgentHeaders(req, target.Token)

	client := a.httpClientFor(target)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("target %s: send report: %w", target.URL, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			a.logger.Warn().Err(closeErr).Str("target", target.URL).Msg("Failed to close report response body")
		}
	}()

	if resp.StatusCode >= 300 {
		bodyBytes, readErr := readBodyWithLimit(resp.Body, maxPulseResponseBodyBytes)
		if readErr != nil {
			return fmt.Errorf("target %s: read error response: %w", target.URL, readErr)
		}
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
		// Detect token-already-in-use error and log a clear warning
		if strings.Contains(errMsg, "already in use") {
			a.logger.Error().
				Str("pulseURL", target.URL).
				Msg("DOCKER REGISTRATION FAILED: This API token is already used by another Docker agent. " +
					"Each Docker host requires its own unique token. " +
					"Generate a new token in Pulse Settings > Agents and reinstall with the new token.")
		}
		return fmt.Errorf("target %s: pulse responded %s: %s", target.URL, resp.Status, errMsg)
	}

	body, err := readBodyWithLimit(resp.Body, maxPulseResponseBodyBytes)
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
		return fmt.Errorf("handle command from %s: %w", target.URL, err)
	}

	return nil
}

func (a *Agent) handleCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	switch strings.ToLower(command.Type) {
	case agentsdocker.CommandTypeStop:
		return a.handleStopCommand(ctx, target, command)
	case agentsdocker.CommandTypeUpdateContainer:
		return a.handleUpdateContainerCommand(ctx, target, command)
	case agentsdocker.CommandTypeCheckUpdates:
		return a.handleCheckUpdatesCommand(ctx, target, command)
	default:
		a.logger.Warn().
			Str("command", command.Type).
			Str("commandID", command.ID).
			Str("target", target.URL).
			Msg("Received unsupported control command")
		return nil
	}
}

func (a *Agent) handleCheckUpdatesCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	a.logger.Info().Str("commandID", command.ID).Msg("Received check updates command from Pulse")

	if a.registryChecker != nil {
		a.registryChecker.ForceCheck()
	}

	// Send intermediate completion ack
	if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusCompleted, "Registry cache cleared; checking for updates on next report cycle"); err != nil {
		return fmt.Errorf("send check updates acknowledgement: %w", err)
	}

	// Trigger an immediate collection cycle to report updates
	go func() {
		// Small delay to ensure the ack response completes
		sleepFn(1 * time.Second)
		a.collectOnce(ctx)
	}()

	return nil
}

func (a *Agent) handleStopCommand(ctx context.Context, target TargetConfig, command agentsdocker.Command) error {
	a.logger.Info().Str("commandID", command.ID).Msg("Received stop command from Pulse")

	if err := a.disableSelf(ctx); err != nil {
		a.logger.Error().
			Err(err).
			Str("commandID", command.ID).
			Str("target", target.URL).
			Msg("Failed to disable pulse-docker-agent service")
		if ackErr := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusFailed, err.Error()); ackErr != nil {
			a.logger.Error().
				Err(ackErr).
				Str("commandID", command.ID).
				Str("target", target.URL).
				Msg("Failed to send failure acknowledgement to Pulse")
		}
		return nil
	}

	if err := a.sendCommandAck(ctx, target, command.ID, agentsdocker.CommandStatusCompleted, "Agent shutting down"); err != nil {
		return fmt.Errorf("send stop acknowledgement: %w", err)
	}

	a.logger.Info().Str("commandID", command.ID).Msg("Stop command acknowledged; terminating agent")

	// After sending the acknowledgement, stop the systemd service to prevent restart.
	// This is done after the ack to ensure the acknowledgement is sent before the
	// process is terminated by systemctl stop.
	go func() {
		// Small delay to ensure the ack response completes
		sleepFn(1 * time.Second)
		stopServiceCtx := context.Background()
		if err := stopSystemdService(stopServiceCtx, "pulse-docker-agent"); err != nil {
			a.logger.Warn().
				Err(err).
				Str("commandID", command.ID).
				Str("service", "pulse-docker-agent").
				Msg("Failed to stop systemd service, agent will exit normally")
		}
	}()

	return ErrStopRequested
}

func (a *Agent) disableSelf(ctx context.Context) error {
	if err := disableSystemdService(ctx, "pulse-docker-agent"); err != nil {
		return fmt.Errorf("disable systemd service: %w", err)
	}

	// Remove Unraid startup script if present to prevent restart on reboot.
	if err := removeFileIfExists(unraidStartupScriptPath); err != nil {
		a.logger.Warn().
			Err(err).
			Str("path", unraidStartupScriptPath).
			Msg("Failed to remove Unraid startup script")
	}

	// Best-effort log cleanup (ignore errors).
	if err := removeFileIfExists(agentLogPath); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to remove agent log directory")
	}

	return nil
}

func disableSystemdService(ctx context.Context, service string) error {
	return runSystemctlCommand(ctx, "disable", service)
}

func stopSystemdService(ctx context.Context, service string) error {
	// Stop the service to terminate the current running instance.
	// This prevents systemd from restarting the service (services stopped via
	// systemctl stop are not restarted even with Restart=always).
	return runSystemctlCommand(ctx, "stop", service)
}

func runSystemctlCommand(ctx context.Context, action, service string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		// Not a systemd environment; nothing to do.
		return nil
	}

	cmd := exec.CommandContext(ctx, "systemctl", action, service)
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
				if action == "disable" {
					return fmt.Errorf("systemctl disable %s: access denied. Run 'sudo systemctl disable --now %s' or rerun the installer with sudo so it can install the polkit rule (systemctl output: %s)", service, service, trimmedOutput)
				}
				return fmt.Errorf("systemctl %s %s: access denied. Run 'sudo systemctl %s %s' or rerun the installer with sudo so it can install the polkit rule (systemctl output: %s)", action, service, action, service, trimmedOutput)
			}
		}
		return fmt.Errorf("systemctl %s %s: %w (%s)", action, service, err, strings.TrimSpace(string(output)))
	}

	return nil
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("remove %s: %w", path, err)
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

	setAgentHeaders(req, target.Token)

	resp, err := a.httpClientFor(target).Do(req)
	if err != nil {
		return fmt.Errorf("send acknowledgement: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			a.logger.Warn().Err(closeErr).Str("target", target.URL).Msg("Failed to close acknowledgement response body")
		}
	}()

	if resp.StatusCode >= 300 {
		bodyBytes, err := readBodyWithLimit(resp.Body, maxPulseResponseBodyBytes)
		if err != nil {
			return fmt.Errorf("read acknowledgement error response: %w", err)
		}
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
		// Disallow redirects for agent API calls. If a reverse proxy redirects
		// HTTP to HTTPS, Go's default behavior converts POST to GET (per HTTP spec),
		// causing 405 errors. Return an error with guidance instead.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("server returned redirect to %s - if using a reverse proxy, ensure you use the correct protocol (https:// instead of http://) in your --url flag", req.URL)
		},
	}
}

func (a *Agent) Close() error {
	return a.docker.Close()
}

func readMachineID() (string, error) {
	for _, path := range machineIDPaths {
		data, err := osReadFileFn(path)
		if err == nil {
			machineID := strings.TrimSpace(string(data))
			// Format as UUID if it's a 32-char hex string (like machine-id typically is),
			// to match the behavior of the host agent.
			if len(machineID) == 32 && utils.IsHexString(machineID) {
				return fmt.Sprintf("%s-%s-%s-%s-%s",
					machineID[0:8], machineID[8:12], machineID[12:16],
					machineID[16:20], machineID[20:32]), nil
			}
			return machineID, nil
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

// detectHostRemovedError checks if the response body contains a host removal error
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
