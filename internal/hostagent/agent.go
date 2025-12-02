package hostagent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/mdadm"
	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

// Config controls the behaviour of the host agent.
type Config struct {
	PulseURL           string
	APIToken           string
	Interval           time.Duration
	HostnameOverride   string
	AgentID            string
	AgentType          string // "unified" when running as part of pulse-agent, empty for standalone
	AgentVersion       string // Version to report; if empty, uses hostagent.Version
	Tags               []string
	InsecureSkipVerify bool
	RunOnce            bool
	LogLevel           zerolog.Level
	Logger             *zerolog.Logger
}

// Agent is responsible for collecting host metrics and shipping them to Pulse.
type Agent struct {
	cfg        Config
	logger     zerolog.Logger
	httpClient *http.Client

	hostInfo        *gohost.InfoStat
	hostname        string
	displayName     string
	platform        string
	osName          string
	osVersion       string
	kernelVersion   string
	architecture    string
	machineID       string
	agentID         string
	agentVersion    string
	interval        time.Duration
	trimmedPulseURL string
}

const defaultInterval = 30 * time.Second

// New constructs a fully initialised host Agent.
func New(cfg Config) (*Agent, error) {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}

	if zerolog.GlobalLevel() == zerolog.DebugLevel && cfg.LogLevel != zerolog.DebugLevel {
		zerolog.SetGlobalLevel(cfg.LogLevel)
	}

	if cfg.Logger == nil {
		defaultLogger := zerolog.New(zerolog.NewConsoleWriter()).
			Level(cfg.LogLevel).
			With().
			Timestamp().
			Logger()
		cfg.Logger = &defaultLogger
	}

	logger := cfg.Logger.Level(cfg.LogLevel).With().Str("component", "host-agent").Logger()

	if strings.TrimSpace(cfg.APIToken) == "" {
		return nil, fmt.Errorf("api token is required")
	}

	pulseURL := cfg.PulseURL
	if strings.TrimSpace(pulseURL) == "" {
		pulseURL = "http://localhost:7655"
	}
	pulseURL = strings.TrimRight(pulseURL, "/")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := gohost.InfoWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch host info: %w", err)
	}

	hostname := strings.TrimSpace(cfg.HostnameOverride)
	if hostname == "" {
		hostname = strings.TrimSpace(info.Hostname)
	}
	if hostname == "" {
		hostname = "unknown-host"
	}

	displayName := hostname

	machineID := strings.TrimSpace(info.HostID)

	agentID := strings.TrimSpace(cfg.AgentID)
	if agentID == "" {
		agentID = machineID
	}
	if agentID == "" {
		agentID = hostname
	}

	platform := normalisePlatform(info.Platform)
	osName := strings.TrimSpace(info.PlatformFamily)
	if osName == "" {
		osName = strings.TrimSpace(info.Platform)
	}
	osVersion := strings.TrimSpace(info.PlatformVersion)
	kernelVersion := strings.TrimSpace(info.KernelVersion)
	arch := strings.TrimSpace(info.KernelArch)
	if arch == "" {
		arch = runtime.GOARCH
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.InsecureSkipVerify {
		//nolint:gosec // Insecure mode is explicitly user-controlled.
		tlsConfig.InsecureSkipVerify = true
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
	}

	trimmedTags := make([]string, 0, len(cfg.Tags))
	seenTags := make(map[string]struct{}, len(cfg.Tags))
	for _, tag := range cfg.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, exists := seenTags[tag]; exists {
			continue
		}
		seenTags[tag] = struct{}{}
		trimmedTags = append(trimmedTags, tag)
	}
	cfg.Tags = trimmedTags

	// Use configured version or fall back to package version
	agentVersion := cfg.AgentVersion
	if agentVersion == "" {
		agentVersion = Version
	}

	return &Agent{
		cfg:             cfg,
		logger:          logger,
		httpClient:      client,
		hostInfo:        info,
		hostname:        hostname,
		displayName:     displayName,
		platform:        platform,
		osName:          osName,
		osVersion:       osVersion,
		kernelVersion:   kernelVersion,
		architecture:    arch,
		machineID:       machineID,
		agentID:         agentID,
		agentVersion:    agentVersion,
		interval:        cfg.Interval,
		trimmedPulseURL: pulseURL,
	}, nil
}

// Run executes the agent until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if a.cfg.RunOnce {
		return a.runOnce(ctx)
	}

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	if err := a.process(ctx); err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error().Err(err).Msg("initial report failed")
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.process(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
				a.logger.Error().Err(err).Msg("failed to send report")
			}
		}
	}
}

func (a *Agent) runOnce(ctx context.Context) error {
	return a.process(ctx)
}

func (a *Agent) process(ctx context.Context) error {
	report, err := a.buildReport(ctx)
	if err != nil {
		return fmt.Errorf("build report: %w", err)
	}
	if err := a.sendReport(ctx, report); err != nil {
		return fmt.Errorf("send report: %w", err)
	}
	a.logger.Debug().
		Str("hostname", report.Host.Hostname).
		Str("platform", report.Host.Platform).
		Msg("host report sent")
	return nil
}

func (a *Agent) buildReport(ctx context.Context) (agentshost.Report, error) {
	collectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	uptime, _ := gohost.UptimeWithContext(collectCtx)
	snapshot, err := hostmetrics.Collect(collectCtx)
	if err != nil {
		return agentshost.Report{}, fmt.Errorf("collect metrics: %w", err)
	}

	// Collect temperature data (best effort - don't fail if unavailable)
	sensorData := a.collectTemperatures(collectCtx)

	// Collect RAID array data (best effort - don't fail if unavailable)
	raidData := a.collectRAIDArrays(collectCtx)

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              a.agentID,
			Version:         a.agentVersion,
			Type:            a.cfg.AgentType,
			IntervalSeconds: int(a.interval / time.Second),
			Hostname:        a.hostname,
		},
		Host: agentshost.HostInfo{
			ID:            a.machineID,
			Hostname:      a.hostname,
			DisplayName:   a.displayName,
			MachineID:     a.machineID,
			Platform:      a.platform,
			OSName:        a.osName,
			OSVersion:     a.osVersion,
			KernelVersion: a.kernelVersion,
			Architecture:  a.architecture,
			CPUModel:      "",
			CPUCount:      snapshot.CPUCount,
			UptimeSeconds: int64(uptime),
			LoadAverage:   append([]float64(nil), snapshot.LoadAverage...),
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: snapshot.CPUUsagePercent,
			Memory:          snapshot.Memory,
		},
		Disks:     append([]agentshost.Disk(nil), snapshot.Disks...),
		Network:   append([]agentshost.NetworkInterface(nil), snapshot.Network...),
		Sensors:   sensorData,
		RAID:      raidData,
		Tags:      append([]string(nil), a.cfg.Tags...),
		Timestamp: time.Now().UTC(),
	}

	return report, nil
}

func (a *Agent) sendReport(ctx context.Context, report agentshost.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	url := fmt.Sprintf("%s/api/agents/host/report", a.trimmedPulseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIToken)
	req.Header.Set("X-API-Token", a.cfg.APIToken)
	req.Header.Set("User-Agent", "pulse-host-agent/"+Version)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("pulse responded with status %s", resp.Status)
	}

	return nil
}

func normalisePlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	switch platform {
	case "darwin":
		return "macos"
	default:
		return platform
	}
}

// collectTemperatures attempts to collect temperature data from the local system.
// Returns an empty Sensors struct if collection fails (best-effort).
func (a *Agent) collectTemperatures(ctx context.Context) agentshost.Sensors {
	// Only collect on Linux for now (lm-sensors is Linux-specific)
	if runtime.GOOS != "linux" {
		return agentshost.Sensors{}
	}

	// Collect sensor JSON output
	jsonOutput, err := sensors.CollectLocal(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect sensor data (lm-sensors may not be installed)")
		return agentshost.Sensors{}
	}

	// Parse the sensor output
	tempData, err := sensors.Parse(jsonOutput)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse sensor data")
		return agentshost.Sensors{}
	}

	if !tempData.Available {
		a.logger.Debug().Msg("No temperature sensors available on this system")
		return agentshost.Sensors{}
	}

	// Convert to host agent sensor format
	result := agentshost.Sensors{
		TemperatureCelsius: make(map[string]float64),
	}

	// Add CPU package temperature
	if tempData.CPUPackage > 0 {
		result.TemperatureCelsius["cpu_package"] = tempData.CPUPackage
	}

	// Add individual core temperatures
	for coreName, temp := range tempData.Cores {
		// Normalize core name (e.g., "Core 0" -> "cpu_core_0")
		normalizedName := strings.ToLower(strings.ReplaceAll(coreName, " ", "_"))
		result.TemperatureCelsius["cpu_"+normalizedName] = temp
	}

	// Add NVMe temperatures
	for nvmeName, temp := range tempData.NVMe {
		result.TemperatureCelsius[nvmeName] = temp
	}

	// Add GPU temperatures
	for gpuName, temp := range tempData.GPU {
		result.TemperatureCelsius[gpuName] = temp
	}

	a.logger.Debug().
		Int("temperatureCount", len(result.TemperatureCelsius)).
		Msg("Collected temperature data")

	return result
}

// collectRAIDArrays attempts to collect mdadm RAID array information.
// Returns an empty slice if collection fails (best-effort).
func (a *Agent) collectRAIDArrays(ctx context.Context) []agentshost.RAIDArray {
	// Only collect on Linux (mdadm is Linux-specific)
	if runtime.GOOS != "linux" {
		return nil
	}

	arrays, err := mdadm.CollectArrays(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect RAID array data (mdadm may not be installed)")
		return nil
	}

	if len(arrays) > 0 {
		a.logger.Debug().
			Int("arrayCount", len(arrays)).
			Msg("Collected RAID array data")
	}

	return arrays
}
