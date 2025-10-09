package dockeragent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// Config describes runtime configuration for the Docker agent.
type Config struct {
	PulseURL           string
	APIToken           string
	Interval           time.Duration
	HostnameOverride   string
	AgentID            string
	InsecureSkipVerify bool
	Logger             *zerolog.Logger
}

// Agent collects Docker metrics and posts them to Pulse.
type Agent struct {
	cfg        Config
	docker     *client.Client
	httpClient *http.Client
	logger     zerolog.Logger
	machineID  string
	hostName   string
	cpuCount   int
}

// New creates a new Docker agent instance.
func New(cfg Config) (*Agent, error) {
	if cfg.PulseURL == "" {
		return nil, errors.New("pulse URL is required")
	}
	if cfg.APIToken == "" {
		return nil, errors.New("pulse API token is required")
	}

	trimmedURL := strings.TrimSpace(cfg.PulseURL)
	trimmedURL = strings.TrimRight(trimmedURL, "/")
	cfg.PulseURL = trimmedURL

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	httpTransport := &http.Transport{}
	if cfg.InsecureSkipVerify {
		httpTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}

	httpClient := &http.Client{
		Timeout:   15 * time.Second,
		Transport: httpTransport,
	}

	logger := cfg.Logger
	if logger == nil {
		defaultLogger := zerolog.New(os.Stdout).With().Timestamp().Str("component", "pulse-docker-agent").Logger()
		logger = &defaultLogger
	} else {
		scoped := logger.With().Str("component", "pulse-docker-agent").Logger()
		logger = &scoped
	}

	machineID, _ := readMachineID()
	hostName := cfg.HostnameOverride
	if hostName == "" {
		if h, err := os.Hostname(); err == nil {
			hostName = h
		}
	}

	agent := &Agent{
		cfg:        cfg,
		docker:     dockerClient,
		httpClient: httpClient,
		logger:     *logger,
		machineID:  machineID,
		hostName:   hostName,
	}

	return agent, nil
}

// Run starts the collection loop until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	interval := a.cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
		a.cfg.Interval = interval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check for updates on startup
	go a.checkForUpdates(ctx)

	// Check for updates daily
	updateTicker := time.NewTicker(24 * time.Hour)
	defer updateTicker.Stop()

	if err := a.collectOnce(ctx); err != nil {
		a.logger.Error().Err(err).Msg("Failed to send initial report")
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.collectOnce(ctx); err != nil {
				a.logger.Error().Err(err).Msg("Failed to send docker report")
			}
		case <-updateTicker.C:
			go a.checkForUpdates(ctx)
		}
	}
}

func (a *Agent) collectOnce(ctx context.Context) error {
	report, err := a.buildReport(ctx)
	if err != nil {
		return err
	}

	return a.sendReport(ctx, report)
}

func (a *Agent) buildReport(ctx context.Context) (agentsdocker.Report, error) {
	info, err := a.docker.Info(ctx)
	if err != nil {
		return agentsdocker.Report{}, fmt.Errorf("failed to query docker info: %w", err)
	}

	a.cpuCount = info.NCPU

	agentID := a.cfg.AgentID
	if agentID == "" {
		agentID = info.ID
	}
	if agentID == "" {
		agentID = a.machineID
	}
	if agentID == "" {
		agentID = a.hostName
	}

	hostName := a.hostName
	if hostName == "" {
		hostName = info.Name
	}

	uptime := readSystemUptime()

	containers, err := a.collectContainers(ctx)
	if err != nil {
		return agentsdocker.Report{}, err
	}

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              agentID,
			Version:         agentVersion,
			IntervalSeconds: int(a.cfg.Interval / time.Second),
		},
		Host: agentsdocker.HostInfo{
			Hostname:         hostName,
			Name:             info.Name,
			MachineID:        a.machineID,
			OS:               info.OperatingSystem,
			KernelVersion:    info.KernelVersion,
			Architecture:     info.Architecture,
			DockerVersion:    info.ServerVersion,
			TotalCPU:         info.NCPU,
			TotalMemoryBytes: info.MemTotal,
			UptimeSeconds:    uptime,
		},
		Containers: containers,
		Timestamp:  time.Now().UTC(),
	}

	if report.Agent.IntervalSeconds <= 0 {
		report.Agent.IntervalSeconds = int(30 * time.Second / time.Second)
	}

	return report, nil
}

func (a *Agent) collectContainers(ctx context.Context) ([]agentsdocker.Container, error) {
	list, err := a.docker.ContainerList(ctx, containertypes.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	containers := make([]agentsdocker.Container, 0, len(list))
	for _, summary := range list {
		container, err := a.collectContainer(ctx, summary)
		if err != nil {
			a.logger.Warn().Str("container", strings.Join(summary.Names, ",")).Err(err).Msg("Failed to collect container stats")
			continue
		}
		containers = append(containers, container)
	}
	return containers, nil
}

func (a *Agent) collectContainer(ctx context.Context, summary types.Container) (agentsdocker.Container, error) {
	const perContainerTimeout = 5 * time.Second

	containerCtx, cancel := context.WithTimeout(ctx, perContainerTimeout)
	defer cancel()

	inspect, err := a.docker.ContainerInspect(containerCtx, summary.ID)
	if err != nil {
		return agentsdocker.Container{}, fmt.Errorf("inspect: %w", err)
	}

	statsResp, err := a.docker.ContainerStatsOneShot(containerCtx, summary.ID)
	if err != nil {
		return agentsdocker.Container{}, fmt.Errorf("stats: %w", err)
	}
	defer statsResp.Body.Close()

	var stats containertypes.StatsResponse
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		return agentsdocker.Container{}, fmt.Errorf("decode stats: %w", err)
	}

	cpuPercent := calculateCPUPercent(stats, a.cpuCount)
	memUsage, memLimit, memPercent := calculateMemoryUsage(stats)

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

	container := agentsdocker.Container{
		ID:               summary.ID,
		Name:             trimLeadingSlash(summary.Names),
		Image:            summary.Image,
		CreatedAt:        createdAt,
		State:            summary.State,
		Status:           summary.Status,
		Health:           health,
		CPUPercent:       cpuPercent,
		MemoryUsageBytes: memUsage,
		MemoryLimitBytes: memLimit,
		MemoryPercent:    memPercent,
		UptimeSeconds:    uptimeSeconds,
		RestartCount:     inspect.RestartCount,
		ExitCode:         inspect.State.ExitCode,
		StartedAt:        startedPtr,
		FinishedAt:       finishedPtr,
		Ports:            ports,
		Labels:           labels,
		Networks:         networks,
	}

	return container, nil
}

func (a *Agent) sendReport(ctx context.Context, report agentsdocker.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	url := fmt.Sprintf("%s/api/agents/docker/report", a.cfg.PulseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", a.cfg.APIToken)
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIToken)
	req.Header.Set("User-Agent", "pulse-docker-agent/"+agentVersion)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("pulse responded with status %s", resp.Status)
	}

	a.logger.Debug().Int("containers", len(report.Containers)).Msg("Report sent")
	return nil
}

const agentVersion = "0.1.0"

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
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
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
	paths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
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

// checkForUpdates checks if a newer version is available and performs self-update if needed
func (a *Agent) checkForUpdates(ctx context.Context) {
	a.logger.Debug().Msg("Checking for agent updates")

	// Get current version from server
	url := fmt.Sprintf("%s/api/agent/version", a.cfg.PulseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to create version check request")
		return
	}

	resp, err := a.httpClient.Do(req)
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

	// Compare versions
	if versionResp.Version == agentVersion {
		a.logger.Debug().Str("version", agentVersion).Msg("Agent is up to date")
		return
	}

	a.logger.Info().
		Str("currentVersion", agentVersion).
		Str("availableVersion", versionResp.Version).
		Msg("New agent version available, performing self-update")

	// Perform self-update
	if err := a.selfUpdate(ctx); err != nil {
		a.logger.Error().Err(err).Msg("Failed to self-update agent")
		return
	}

	a.logger.Info().Msg("Agent updated successfully, restarting...")
}

// selfUpdate downloads the new agent binary and replaces the current one
func (a *Agent) selfUpdate(ctx context.Context) error {
	// Get path to current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Download new binary
	url := fmt.Sprintf("%s/download/pulse-docker-agent", a.cfg.PulseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download new binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "pulse-docker-agent-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up if something goes wrong

	// Write downloaded binary to temp file
	if _, err := tmpFile.ReadFrom(resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write downloaded binary: %w", err)
	}
	tmpFile.Close()

	// Make temp file executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to make temp file executable: %w", err)
	}

	// Create backup of current binary
	backupPath := execPath + ".backup"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary to current location
	if err := os.Rename(tmpPath, execPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup on success
	os.Remove(backupPath)

	// Restart agent with same arguments
	args := os.Args
	env := os.Environ()

	if err := syscall.Exec(execPath, args, env); err != nil {
		return fmt.Errorf("failed to restart agent: %w", err)
	}

	return nil
}
