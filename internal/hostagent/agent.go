package hostagent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
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

	// Proxmox integration
	EnableProxmox bool   // If true, creates Proxmox API token and registers node on startup
	ProxmoxType   string // "pve", "pbs", or "" for auto-detect

	// Security options
	EnableCommands bool // If true, enables the command execution feature (AI auto-fix)

	// Disk filtering
	DiskExclude []string // Mount points or path prefixes to exclude from disk monitoring

	// Network configuration
	ReportIP    string // IP address to report instead of auto-detected (for multi-NIC systems)
	DisableCeph bool   // If true, disables local Ceph status polling

	Collector SystemCollector // Optional: override default system information collector (for testing)
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
	updatedFrom     string // Previous version if recently auto-updated (reported once)
	reportIP        string // User-specified IP to report (for multi-NIC systems)
	interval        time.Duration
	trimmedPulseURL string
	reportBuffer    *utils.Queue[agentshost.Report]
	commandClient   *CommandClient
	collector       SystemCollector
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
	cfg.PulseURL = pulseURL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collector := cfg.Collector
	if collector == nil {
		collector = &defaultCollector{}
	}

	info, err := collector.HostInfo(ctx)
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

	machineID := GetReliableMachineID(collector, info.HostID, logger)

	agentID := strings.TrimSpace(cfg.AgentID)
	if agentID == "" {
		agentID = machineID
	}
	if agentID == "" {
		agentID = hostname
	}

	platform := normalisePlatform(info.Platform)
	// Use Platform (specific distro like "ubuntu") over PlatformFamily (distro family like "debian")
	// This ensures Ubuntu shows as "ubuntu 24.04" not "debian 24.04" (refs #927)
	osName := strings.TrimSpace(info.Platform)
	if osName == "" {
		osName = strings.TrimSpace(info.PlatformFamily)
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
		// Disallow redirects for agent API calls. If a reverse proxy redirects
		// HTTP to HTTPS, Go's default behavior converts POST to GET (per HTTP spec),
		// causing 405 errors. Return an error with guidance instead.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("server returned redirect to %s - if using a reverse proxy, ensure you use the correct protocol (https:// instead of http://) in your --url flag", req.URL)
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

	const bufferCapacity = 60

	// Check if agent was recently auto-updated (only reported once per restart)
	updatedFrom := agentupdate.GetUpdatedFromVersion()
	if updatedFrom != "" {
		logger.Info().
			Str("previousVersion", updatedFrom).
			Str("currentVersion", agentVersion).
			Msg("Agent was auto-updated")
	}

	agent := &Agent{
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
		updatedFrom:     updatedFrom,
		reportIP:        strings.TrimSpace(cfg.ReportIP),
		interval:        cfg.Interval,
		trimmedPulseURL: pulseURL,
		reportBuffer:    utils.NewQueue[agentshost.Report](bufferCapacity),
		collector:       collector,
	}

	// Create command client for AI command execution (only if enabled)
	if cfg.EnableCommands {
		agent.commandClient = NewCommandClient(cfg, agentID, hostname, platform, agentVersion)
		cfg.Logger.Info().Msg("Command execution enabled via --enable-commands flag")
	} else {
		cfg.Logger.Info().Msg("Command execution disabled (use --enable-commands to enable)")
	}

	return agent, nil
}

// Run executes the agent until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if a.cfg.RunOnce {
		return a.runOnce(ctx)
	}

	// Proxmox setup (if enabled)
	if a.cfg.EnableProxmox {
		a.runProxmoxSetup(ctx)
	}

	// Start command client in background for AI command execution
	if a.commandClient != nil {
		go func() {
			if err := a.commandClient.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error().Err(err).Msg("Command client stopped with error")
			}
		}()
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
		if strings.Contains(err.Error(), "403 Forbidden") {
			a.logger.Error().Msg("Failed to send host report (403 Forbidden). API token may lack 'Host agent reporting' scope. Set PULSE_ENABLE_HOST=false if host monitoring is not needed.")
			return nil
		}
		a.logger.Warn().Err(err).Msg("Failed to send report, buffering")
		a.reportBuffer.Push(report)
		return nil
	}

	// If successful, try to flush buffer
	a.flushBuffer(ctx)

	a.logger.Debug().
		Str("hostname", report.Host.Hostname).
		Str("platform", report.Host.Platform).
		Msg("host report sent")
	return nil
}

func (a *Agent) flushBuffer(ctx context.Context) {
	if a.reportBuffer.IsEmpty() {
		return
	}

	a.logger.Info().Int("count", a.reportBuffer.Len()).Msg("Flushing buffered reports")

	for !a.reportBuffer.IsEmpty() {
		// Peek first
		report, ok := a.reportBuffer.Peek()
		if !ok {
			break
		}

		if err := a.sendReport(ctx, report); err != nil {
			a.logger.Warn().Err(err).Msg("Failed to flush buffered report, stopping flush")
			return
		}

		// Pop only on success
		a.reportBuffer.Pop()
	}
}

func (a *Agent) buildReport(ctx context.Context) (agentshost.Report, error) {
	collectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	uptime, _ := a.collector.HostUptime(collectCtx)
	snapshot, err := a.collector.Metrics(collectCtx, a.cfg.DiskExclude)
	if err != nil {
		return agentshost.Report{}, fmt.Errorf("collect metrics: %w", err)
	}

	// Collect temperature data (best effort - don't fail if unavailable)
	sensorData := a.collectTemperatures(collectCtx)

	// Collect S.M.A.R.T. disk data (best effort - don't fail if unavailable)
	smartData := a.collectSMARTData(collectCtx)
	if len(smartData) > 0 {
		sensorData.SMART = smartData
	}

	// Collect RAID array data (best effort - don't fail if unavailable)
	raidData := a.collectRAIDArrays(collectCtx)

	// Collect Ceph cluster data (best effort - only on Ceph nodes)
	cephData := a.collectCephStatus(collectCtx)

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              a.agentID,
			Version:         a.agentVersion,
			Type:            a.cfg.AgentType,
			IntervalSeconds: int(a.interval / time.Second),
			Hostname:        a.hostname,
			UpdatedFrom:     a.updatedFrom,
			CommandsEnabled: a.cfg.EnableCommands,
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
			ReportIP:      a.reportIP,
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: snapshot.CPUUsagePercent,
			Memory:          snapshot.Memory,
		},
		Disks:     append([]agentshost.Disk(nil), snapshot.Disks...),
		DiskIO:    append([]agentshost.DiskIO(nil), snapshot.DiskIO...),
		Network:   append([]agentshost.NetworkInterface(nil), snapshot.Network...),
		Sensors:   sensorData,
		RAID:      raidData,
		Ceph:      cephData,
		Tags:      append([]string(nil), a.cfg.Tags...),
		Timestamp: a.collector.Now(),
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

	// Parse response to check for server-side config overrides
	var reportResp struct {
		Success bool   `json:"success"`
		HostID  string `json:"hostId"`
		Config  *struct {
			CommandsEnabled *bool `json:"commandsEnabled"`
		} `json:"config,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reportResp); err != nil {
		// Non-fatal: just log and continue
		a.logger.Debug().Err(err).Msg("Failed to parse report response, ignoring config")
		return nil
	}

	// Apply server config overrides
	if reportResp.Config != nil && reportResp.Config.CommandsEnabled != nil {
		a.applyRemoteConfig(*reportResp.Config.CommandsEnabled)
	}

	return nil
}

// applyRemoteConfig applies server-side configuration overrides.
// Currently handles enabling/disabling the command execution feature dynamically.
func (a *Agent) applyRemoteConfig(commandsEnabled bool) {
	// Check if state would change
	currentlyEnabled := a.commandClient != nil

	if commandsEnabled && !currentlyEnabled {
		// Server enabled commands, but we don't have a command client
		// Start the command client
		a.logger.Info().Msg("Server enabled command execution - starting command client")
		client := NewCommandClient(a.cfg, a.agentID, a.hostname, a.platform, a.agentVersion)
		a.commandClient = client
		go func() {
			if err := client.Run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error().Err(err).Msg("Command client stopped with error")
			}
		}()
	} else if !commandsEnabled && currentlyEnabled {
		// Server disabled commands, but we have a command client running
		a.logger.Info().Msg("Server disabled command execution - stopping command client")
		// Properly close the WebSocket connection to stop the client
		if err := a.commandClient.Close(); err != nil {
			a.logger.Debug().Err(err).Msg("Error closing command client connection")
		}
		a.commandClient = nil
	}
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
	if a.collector.GOOS() != "linux" {
		return agentshost.Sensors{}
	}

	// Collect sensor JSON output
	jsonOutput, err := a.collector.SensorsLocal(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect sensor data (lm-sensors may not be installed)")
		return agentshost.Sensors{}
	}

	// Parse the sensor output
	tempData, err := a.collector.SensorsParse(jsonOutput)
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

	// Add fan speeds (RPM)
	if len(tempData.Fans) > 0 {
		result.FanRPM = make(map[string]float64)
		for fanName, rpm := range tempData.Fans {
			result.FanRPM[fanName] = rpm
		}
	}

	// Add other temperatures (DDR5, motherboard, etc.)
	if len(tempData.Other) > 0 {
		result.Additional = make(map[string]float64)
		for sensorName, temp := range tempData.Other {
			result.Additional[sensorName] = temp
		}
	}

	// Collect power consumption data (Intel RAPL, etc.)
	if powerData, err := a.collector.SensorsPower(ctx); err == nil && powerData != nil && powerData.Available {
		result.PowerWatts = make(map[string]float64)
		if powerData.PackageWatts > 0 {
			result.PowerWatts["cpu_package"] = powerData.PackageWatts
		}
		if powerData.CoreWatts > 0 {
			result.PowerWatts["cpu_core"] = powerData.CoreWatts
		}
		if powerData.DRAMWatts > 0 {
			result.PowerWatts["dram"] = powerData.DRAMWatts
		}
		a.logger.Debug().
			Float64("packageWatts", powerData.PackageWatts).
			Str("source", powerData.Source).
			Msg("Collected power data")
	}

	a.logger.Debug().
		Int("temperatureCount", len(result.TemperatureCelsius)).
		Int("fanCount", len(result.FanRPM)).
		Int("powerCount", len(result.PowerWatts)).
		Int("additionalCount", len(result.Additional)).
		Msg("Collected sensor data")

	return result
}

// collectRAIDArrays attempts to collect mdadm RAID array information.
// Returns an empty slice if collection fails (best-effort).
func (a *Agent) collectRAIDArrays(ctx context.Context) []agentshost.RAIDArray {
	// Only collect on Linux (mdadm is Linux-specific)
	if a.collector.GOOS() != "linux" {
		return nil
	}

	arrays, err := a.collector.RAIDArrays(ctx)
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

// collectCephStatus attempts to collect Ceph cluster status.
// Returns nil if Ceph is not available or not configured on this host.
func (a *Agent) collectCephStatus(ctx context.Context) *agentshost.CephCluster {
	if a.cfg.DisableCeph {
		return nil
	}
	// Only collect on Linux
	if a.collector.GOOS() != "linux" {
		return nil
	}

	status, err := a.collector.CephStatus(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect Ceph status")
		return nil
	}
	if status == nil {
		return nil
	}

	// Convert internal ceph types to agent report types
	result := &agentshost.CephCluster{
		FSID: status.FSID,
		Health: agentshost.CephHealth{
			Status: status.Health.Status,
			Checks: make(map[string]agentshost.CephCheck),
		},
		MonMap: agentshost.CephMonitorMap{
			Epoch:   status.MonMap.Epoch,
			NumMons: status.MonMap.NumMons,
		},
		MgrMap: agentshost.CephManagerMap{
			Available: status.MgrMap.Available,
			NumMgrs:   status.MgrMap.NumMgrs,
			ActiveMgr: status.MgrMap.ActiveMgr,
			Standbys:  status.MgrMap.Standbys,
		},
		OSDMap: agentshost.CephOSDMap{
			Epoch:   status.OSDMap.Epoch,
			NumOSDs: status.OSDMap.NumOSDs,
			NumUp:   status.OSDMap.NumUp,
			NumIn:   status.OSDMap.NumIn,
			NumDown: status.OSDMap.NumDown,
			NumOut:  status.OSDMap.NumOut,
		},
		PGMap: agentshost.CephPGMap{
			NumPGs:           status.PGMap.NumPGs,
			BytesTotal:       status.PGMap.BytesTotal,
			BytesUsed:        status.PGMap.BytesUsed,
			BytesAvailable:   status.PGMap.BytesAvailable,
			DataBytes:        status.PGMap.DataBytes,
			UsagePercent:     status.PGMap.UsagePercent,
			DegradedRatio:    status.PGMap.DegradedRatio,
			MisplacedRatio:   status.PGMap.MisplacedRatio,
			ReadBytesPerSec:  status.PGMap.ReadBytesPerSec,
			WriteBytesPerSec: status.PGMap.WriteBytesPerSec,
			ReadOpsPerSec:    status.PGMap.ReadOpsPerSec,
			WriteOpsPerSec:   status.PGMap.WriteOpsPerSec,
		},
		CollectedAt: status.CollectedAt.Format(time.RFC3339),
	}

	// Convert monitors
	for _, mon := range status.MonMap.Monitors {
		result.MonMap.Monitors = append(result.MonMap.Monitors, agentshost.CephMonitor{
			Name:   mon.Name,
			Rank:   mon.Rank,
			Addr:   mon.Addr,
			Status: mon.Status,
		})
	}

	// Convert health checks
	for name, check := range status.Health.Checks {
		result.Health.Checks[name] = agentshost.CephCheck{
			Severity: check.Severity,
			Message:  check.Message,
			Detail:   check.Detail,
		}
	}

	// Convert health summary
	for _, s := range status.Health.Summary {
		result.Health.Summary = append(result.Health.Summary, agentshost.CephHealthSummary{
			Severity: s.Severity,
			Message:  s.Message,
		})
	}

	// Convert pools
	for _, pool := range status.Pools {
		result.Pools = append(result.Pools, agentshost.CephPool{
			ID:             pool.ID,
			Name:           pool.Name,
			BytesUsed:      pool.BytesUsed,
			BytesAvailable: pool.BytesAvailable,
			Objects:        pool.Objects,
			PercentUsed:    pool.PercentUsed,
		})
	}

	// Convert services
	for _, svc := range status.Services {
		result.Services = append(result.Services, agentshost.CephService{
			Type:    svc.Type,
			Running: svc.Running,
			Total:   svc.Total,
			Daemons: svc.Daemons,
		})
	}

	a.logger.Debug().
		Str("fsid", result.FSID).
		Str("health", result.Health.Status).
		Int("osds", result.OSDMap.NumOSDs).
		Int("pools", len(result.Pools)).
		Msg("Collected Ceph cluster status")

	return result
}

// collectSMARTData collects S.M.A.R.T. data from local disks.
// Returns nil if smartctl is not available or no disks are found.
func (a *Agent) collectSMARTData(ctx context.Context) []agentshost.DiskSMART {
	goos := a.collector.GOOS()
	if goos != "linux" && goos != "freebsd" {
		return nil
	}

	smartData, err := a.collector.SMARTLocal(ctx, a.cfg.DiskExclude)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect S.M.A.R.T. data (smartctl may not be installed)")
		return nil
	}

	if len(smartData) == 0 {
		return nil
	}

	// Convert internal smartctl types to agent report types
	result := make([]agentshost.DiskSMART, 0, len(smartData))
	for _, disk := range smartData {
		entry := agentshost.DiskSMART{
			Device:      disk.Device,
			Model:       disk.Model,
			Serial:      disk.Serial,
			WWN:         disk.WWN,
			Type:        disk.Type,
			Temperature: disk.Temperature,
			Health:      disk.Health,
			Standby:     disk.Standby,
		}
		if disk.Attributes != nil {
			entry.Attributes = &agentshost.SMARTAttributes{
				PowerOnHours:         disk.Attributes.PowerOnHours,
				PowerCycles:          disk.Attributes.PowerCycles,
				ReallocatedSectors:   disk.Attributes.ReallocatedSectors,
				PendingSectors:       disk.Attributes.PendingSectors,
				OfflineUncorrectable: disk.Attributes.OfflineUncorrectable,
				UDMACRCErrors:        disk.Attributes.UDMACRCErrors,
				PercentageUsed:       disk.Attributes.PercentageUsed,
				AvailableSpare:       disk.Attributes.AvailableSpare,
				MediaErrors:          disk.Attributes.MediaErrors,
				UnsafeShutdowns:      disk.Attributes.UnsafeShutdowns,
			}
		}
		result = append(result, entry)
	}

	a.logger.Debug().
		Int("diskCount", len(result)).
		Msg("Collected S.M.A.R.T. disk data")

	return result
}

// runProxmoxSetup performs one-time Proxmox API token setup and node registration.
// Supports hosts with multiple Proxmox products (e.g., PVE + PBS on same host).
func (a *Agent) runProxmoxSetup(ctx context.Context) {
	a.logger.Info().Msg("Proxmox mode enabled, checking setup...")

	setup := NewProxmoxSetup(
		a.logger,
		a.httpClient,
		a.collector,
		a.trimmedPulseURL,
		a.cfg.APIToken,
		a.cfg.ProxmoxType,
		a.hostname,
		a.reportIP,
		a.cfg.InsecureSkipVerify,
	)

	// Use RunAll to detect and register all Proxmox products on this host
	results, err := setup.RunAll(ctx)
	if err != nil {
		a.logger.Error().Err(err).Msg("Proxmox setup failed")
		return
	}

	if len(results) == 0 {
		// All types already registered
		a.logger.Info().Msg("All detected Proxmox products already registered")
		return
	}

	// Log results for each registered type
	for _, result := range results {
		if result.Registered {
			a.logger.Info().
				Str("type", result.ProxmoxType).
				Str("host", result.NodeHost).
				Str("token_id", result.TokenID).
				Msg("Proxmox node registered successfully")
		} else {
			a.logger.Warn().
				Str("type", result.ProxmoxType).
				Str("host", result.NodeHost).
				Msg("Proxmox token created but registration failed (node may need manual configuration)")
		}
	}
}

// isLXCContainer detects if we're running inside an LXC container.
// LXC containers share the host's /sys/class/dmi/id/product_uuid, which causes
// gopsutil to return identical HostIDs for all LXC containers on the same host.
func isLXCContainer(c SystemCollector) bool {
	// Check systemd-detect-virt if available
	if data, err := c.ReadFile("/run/systemd/container"); err == nil {
		container := strings.TrimSpace(string(data))
		if strings.Contains(container, "lxc") {
			return true
		}
	}

	// Check /proc/1/environ for container=lxc
	if data, err := c.ReadFile("/proc/1/environ"); err == nil {
		if strings.Contains(string(data), "container=lxc") {
			return true
		}
	}

	// Check /proc/1/cgroup for lxc markers
	if data, err := c.ReadFile("/proc/1/cgroup"); err == nil {
		text := string(data)
		if strings.Contains(text, "/lxc/") || strings.Contains(text, "lxc.payload") {
			return true
		}
	}

	return false
}

// getReliableMachineID returns a machine ID that's unique per container/host.
// On Linux, /etc/machine-id is always preferred over gopsutil's HostID because:
// - LXC containers share the host's /sys/class/dmi/id/product_uuid
// - Cloned VMs/hosts may share the same DMI product UUID
// - Proxmox cluster nodes with identical hardware may have the same UUID
// The /etc/machine-id file is guaranteed unique per installation.
// GetReliableMachineID attempts to find a stable machine ID.
func GetReliableMachineID(c SystemCollector, gopsutilHostID string, logger zerolog.Logger) string {
	gopsutilID := strings.TrimSpace(gopsutilHostID)

	// On Linux, prefer /etc/machine-id when available.
	// This avoids ID collisions from:
	// - LXC containers sharing host's DMI product UUID
	// - Cloned VMs with identical hardware UUIDs
	// - Proxmox cluster nodes with same hardware configuration
	if c.GOOS() == "linux" {
		if data, err := c.ReadFile("/etc/machine-id"); err == nil {
			machineID := strings.TrimSpace(string(data))
			if machineID != "" && len(machineID) >= 8 {
				// Format as UUID if it's a 32-char hex string (like machine-id typically is).
				if len(machineID) == 32 && isHexString(machineID) {
					machineID = fmt.Sprintf("%s-%s-%s-%s-%s",
						machineID[0:8], machineID[8:12], machineID[12:16],
						machineID[16:20], machineID[20:32])
				}
				if isLXCContainer(c) {
					logger.Debug().
						Str("machineID", machineID).
						Msg("LXC container detected, using /etc/machine-id for unique identification")
				} else {
					logger.Debug().
						Str("machineID", machineID).
						Msg("Linux host detected, using /etc/machine-id for unique identification")
				}
				return machineID
			}
		}

		if macID := getPrimaryMACIdentifier(c); macID != "" {
			logger.Debug().
				Str("machineID", macID).
				Msg("Linux host missing usable /etc/machine-id, using MAC address for unique identification")
			return macID
		}
	}

	return gopsutilID
}

func isHexString(input string) bool {
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		default:
			return false
		}
	}
	return input != ""
}

func getPrimaryMACIdentifier(c SystemCollector) string {
	interfaces, err := c.NetInterfaces()
	if err != nil {
		return ""
	}

	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Name < interfaces[j].Name
	})

	// Prefer a stable-looking interface name first to avoid selecting docker bridges
	// or other virtual interfaces when physical interfaces are present.
	for pass := 0; pass < 2; pass++ {
		for _, iface := range interfaces {
			if len(iface.HardwareAddr) == 0 {
				continue
			}
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			if pass == 0 && isLikelyVirtualInterfaceName(iface.Name) {
				continue
			}

			mac := strings.ToLower(iface.HardwareAddr.String())
			normalized := strings.NewReplacer(":", "", "-", "", ".", "").Replace(mac)
			if normalized == "" {
				continue
			}
			return "mac-" + normalized
		}
	}

	return ""
}

func isLikelyVirtualInterfaceName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	switch {
	case name == "":
		return true
	case name == "lo":
		return true
	case strings.HasPrefix(name, "docker"):
		return true
	case strings.HasPrefix(name, "veth"):
		return true
	case strings.HasPrefix(name, "br-"):
		return true
	case strings.HasPrefix(name, "cni"):
		return true
	case strings.HasPrefix(name, "flannel"):
		return true
	case strings.HasPrefix(name, "virbr"):
		return true
	case strings.HasPrefix(name, "zt"):
		return true
	default:
		return false
	}
}
