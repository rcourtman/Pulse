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
	"sort"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
	gocpu "github.com/shirou/gopsutil/v4/cpu"
	godisk "github.com/shirou/gopsutil/v4/disk"
	gohost "github.com/shirou/gopsutil/v4/host"
	goload "github.com/shirou/gopsutil/v4/load"
	gomem "github.com/shirou/gopsutil/v4/mem"
	gonet "github.com/shirou/gopsutil/v4/net"
)

// Config controls the behaviour of the host agent.
type Config struct {
	PulseURL           string
	APIToken           string
	Interval           time.Duration
	HostnameOverride   string
	AgentID            string
	Tags               []string
	InsecureSkipVerify bool
	RunOnce            bool
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
	interval        time.Duration
	trimmedPulseURL string
}

const defaultInterval = 30 * time.Second

// New constructs a fully initialised host Agent.
func New(cfg Config) (*Agent, error) {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}

	if cfg.Logger == nil {
		defaultLogger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
		cfg.Logger = &defaultLogger
	}

	logger := cfg.Logger.With().Str("component", "host-agent").Logger()

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
	loadAvg, _ := goload.AvgWithContext(collectCtx)
	cpuCount, _ := gocpu.CountsWithContext(collectCtx, true)
	cpuUsage, err := a.calculateCPUUsage(collectCtx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("failed to compute cpu usage")
	}

	memStats, err := gomem.VirtualMemoryWithContext(collectCtx)
	if err != nil {
		return agentshost.Report{}, fmt.Errorf("memory stats: %w", err)
	}

	disks := a.collectDisks(collectCtx)
	network := a.collectNetwork(collectCtx)

	var loadValues []float64
	if loadAvg != nil {
		loadValues = []float64{loadAvg.Load1, loadAvg.Load5, loadAvg.Load15}
	}

	swapUsed := int64(0)
	if memStats.SwapTotal > memStats.SwapFree {
		swapUsed = int64(memStats.SwapTotal - memStats.SwapFree)
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              a.agentID,
			Version:         Version,
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
			CPUCount:      cpuCount,
			UptimeSeconds: int64(uptime),
			LoadAverage:   loadValues,
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: cpuUsage,
			Memory: agentshost.MemoryMetric{
				TotalBytes: int64(memStats.Total),
				UsedBytes:  int64(memStats.Used),
				FreeBytes:  int64(memStats.Free),
				Usage:      memStats.UsedPercent,
				SwapTotal:  int64(memStats.SwapTotal),
				SwapUsed:   swapUsed,
			},
		},
		Disks:     disks,
		Network:   network,
		Sensors:   agentshost.Sensors{},
		Tags:      append([]string(nil), a.cfg.Tags...),
		Timestamp: time.Now().UTC(),
	}

	return report, nil
}

func (a *Agent) calculateCPUUsage(ctx context.Context) (float64, error) {
	// Use Percent() with a 1 second measurement interval for cross-platform compatibility
	// This works reliably on macOS ARM64 where Times() is not implemented
	percentages, err := gocpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return 0, err
	}
	if len(percentages) == 0 {
		return 0, nil
	}

	usage := percentages[0]
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}

	return usage, nil
}

func (a *Agent) collectDisks(ctx context.Context) []agentshost.Disk {
	partitions, err := godisk.PartitionsWithContext(ctx, true)
	if err != nil {
		a.logger.Debug().Err(err).Msg("failed to fetch disk partitions")
		return nil
	}

	disks := make([]agentshost.Disk, 0, len(partitions))
	seen := make(map[string]struct{}, len(partitions))

	for _, part := range partitions {
		if part.Mountpoint == "" {
			continue
		}
		if _, ok := seen[part.Mountpoint]; ok {
			continue
		}
		seen[part.Mountpoint] = struct{}{}

		usage, err := godisk.UsageWithContext(ctx, part.Mountpoint)
		if err != nil {
			continue
		}
		if usage.Total == 0 {
			continue
		}

		disks = append(disks, agentshost.Disk{
			Device:     part.Device,
			Mountpoint: part.Mountpoint,
			Filesystem: part.Fstype,
			Type:       part.Fstype,
			TotalBytes: int64(usage.Total),
			UsedBytes:  int64(usage.Used),
			FreeBytes:  int64(usage.Free),
			Usage:      usage.UsedPercent,
		})
	}

	sort.Slice(disks, func(i, j int) bool { return disks[i].Mountpoint < disks[j].Mountpoint })
	return disks
}

func (a *Agent) collectNetwork(ctx context.Context) []agentshost.NetworkInterface {
	ifaces, err := gonet.InterfacesWithContext(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("failed to fetch network interfaces")
		return nil
	}

	ioCounters, err := gonet.IOCountersWithContext(ctx, true)
	if err != nil {
		a.logger.Debug().Err(err).Msg("failed to fetch network counters")
	}
	ioMap := make(map[string]gonet.IOCountersStat, len(ioCounters))
	for _, stat := range ioCounters {
		ioMap[stat.Name] = stat
	}

	interfaces := make([]agentshost.NetworkInterface, 0, len(ifaces))

	for _, iface := range ifaces {
		if len(iface.Addrs) == 0 {
			continue
		}
		if isLoopback(iface.Flags) {
			continue
		}

		addresses := make([]string, 0, len(iface.Addrs))
		for _, addr := range iface.Addrs {
			if addr.Addr != "" {
				addresses = append(addresses, addr.Addr)
			}
		}
		if len(addresses) == 0 {
			continue
		}

		counter := ioMap[iface.Name]
		ifaceEntry := agentshost.NetworkInterface{
			Name:      iface.Name,
			MAC:       iface.HardwareAddr,
			Addresses: addresses,
			RXBytes:   counter.BytesRecv,
			TXBytes:   counter.BytesSent,
		}

		interfaces = append(interfaces, ifaceEntry)
	}

	sort.Slice(interfaces, func(i, j int) bool { return interfaces[i].Name < interfaces[j].Name })
	return interfaces
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

func isLoopback(flags []string) bool {
	for _, flag := range flags {
		if strings.EqualFold(flag, "loopback") {
			return true
		}
	}
	return false
}
