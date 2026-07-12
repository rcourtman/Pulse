package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenttls"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

// Config controls the behaviour of the runtime-side Unified Agent.
type Config struct {
	PulseURL           string
	APIToken           string
	Interval           time.Duration
	HostnameOverride   string
	AgentID            string
	AgentType          string // "unified" when running as part of pulse-agent, empty for standalone
	AgentVersion       string // Version to report; if empty, uses the Unified Agent binary version
	Tags               []string
	InsecureSkipVerify bool
	CACertPath         string
	ServerFingerprint  string
	RunOnce            bool
	LogLevel           zerolog.Level
	Logger             *zerolog.Logger

	// Proxmox integration
	EnableProxmox bool   // If true, creates Proxmox API token and registers node on startup
	ProxmoxType   string // "pve", "pbs", or "" for auto-detect

	// Security options
	EnableCommands bool // If true, enables the command execution feature (AI auto-fix)

	// Enrollment
	Enroll bool // If true, exchange bootstrap token for runtime token on startup

	// Disk filtering
	DiskExclude []string // Mount points or path prefixes to exclude from disk monitoring

	// State directory for persistent files (agent-id, proxmox registration, etc.)
	StateDir string // Default: /var/lib/pulse-agent

	// Deploy SSH configuration for peer-node install fan-out.
	DeploySSHUser string // Default: root. Non-root users must support passwordless sudo for install steps.

	// Network configuration
	ReportIP    string // IP address to report instead of auto-detected (for multi-NIC systems)
	DisableCeph bool   // If true, disables local Ceph status polling

	// AppliedConfig is the non-secret fingerprint of the managed config that
	// was applied before this runtime started. UpdateStatus supplies live
	// self-update state without coupling report collection to updater internals.
	AppliedConfig *agentshost.ConfigFingerprint
	UpdateStatus  func() agentupdate.Status
	ModuleStatus  func() []agentshost.ModuleStatus

	Collector SystemCollector // Optional: override default system information collector (for testing)

	newCommandClientFn   func(Config, string, string, string, string) *CommandClient
	runCommandClientFn   func(*CommandClient, context.Context) error
	updatedFromVersionFn func() string
	packageUpdates       *packageUpdateManager
	storageCleanup       *storageCleanupManager
}

// Agent is responsible for collecting host metrics and shipping them to Pulse.
type Agent struct {
	cfg        Config
	logger     zerolog.Logger
	httpClient *http.Client

	hostInfo               *gohost.InfoStat
	hostname               string
	displayName            string
	platform               string
	osName                 string
	osVersion              string
	kernelVersion          string
	architecture           string
	machineID              string
	agentID                string
	agentVersion           string
	updatedFrom            string // Previous version if recently auto-updated (reported once)
	reportIP               string // User-specified IP to report (for multi-NIC systems)
	interval               time.Duration
	stateDir               string
	trimmedPulseURL        string
	configMu               sync.RWMutex
	remoteConfigChanged    chan struct{}
	reportBuffer           *utils.Queue[agentshost.Report]
	commandClient          *CommandClient
	commandClientMu        sync.Mutex
	commandClientRunCancel context.CancelFunc
	commandClientParentCtx context.Context
	collector              SystemCollector
	newCommandClient       func(Config, string, string, string, string) *CommandClient
	runCommandClient       func(*CommandClient, context.Context) error
	packageUpdates         *packageUpdateManager
	storageCleanup         *storageCleanupManager

	// lastAuthFailureLog throttles the actionable 401 error so a permanently
	// rejected token does not spam the log every report interval. Only touched
	// from the single-threaded report loop (process/flushBuffer).
	lastAuthFailureLog time.Time
}

const defaultInterval = 30 * time.Second
const defaultStateDir = "/var/lib/pulse-agent"
const agentReportEndpoint = "/api/agents/agent/report"

// authFailureLogInterval bounds how often the agent re-emits the actionable
// "token rejected" error while the server keeps returning 401.
const authFailureLogInterval = 5 * time.Minute

type reportHTTPStatusError struct {
	Endpoint   string
	Status     string
	StatusCode int
}

func (e *reportHTTPStatusError) Error() string {
	return fmt.Sprintf("pulse responded with status %s", e.Status)
}

// New constructs a fully initialised runtime-side Unified Agent.
func New(cfg Config) (*Agent, error) {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}

	if strings.TrimSpace(cfg.StateDir) == "" {
		cfg.StateDir = defaultStateDir
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

	logger := cfg.Logger.Level(cfg.LogLevel).With().Str("component", "agent").Logger()

	if cfg.Enroll && strings.TrimSpace(cfg.APIToken) == "" {
		return nil, fmt.Errorf("api token is required when enrollment is enabled")
	}

	pulseURL := strings.TrimSpace(cfg.PulseURL)
	if pulseURL == "" {
		pulseURL = "http://localhost:7655"
	}
	var err error
	pulseURL, err = normalizePulseURL(pulseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid pulse URL: %w", err)
	}
	cfg.PulseURL = pulseURL

	cfg.ProxmoxType, err = normalizeProxmoxType(cfg.ProxmoxType)
	if err != nil {
		return nil, err
	}

	cfg.ReportIP, err = normalizeReportIP(cfg.ReportIP)
	if err != nil {
		return nil, err
	}

	cfg.DeploySSHUser, err = NormalizeDeploySSHUser(cfg.DeploySSHUser)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collector := cfg.Collector
	if collector == nil {
		collector = &defaultCollector{}
	}
	newCommandClientFn := cfg.newCommandClientFn
	if newCommandClientFn == nil {
		newCommandClientFn = NewCommandClient
	}
	runCommandClientFn := cfg.runCommandClientFn
	if runCommandClientFn == nil {
		runCommandClientFn = func(client *CommandClient, ctx context.Context) error {
			return client.Run(ctx)
		}
	}
	updatedFromVersionFn := cfg.updatedFromVersionFn
	if updatedFromVersionFn == nil {
		updatedFromVersionFn = agentupdate.GetUpdatedFromVersion
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
	osName, osVersion = resolveHostOSIdentity(collector, osName, osVersion)
	if profile, ok := platformsupport.AgentHostProfileForIdentity(osName, platform); ok {
		platform = platformsupport.NormalizeRuntimePlatformForAgentHostProfile(profile.ID, platform)
	}
	kernelVersion := strings.TrimSpace(info.KernelVersion)
	arch := strings.TrimSpace(info.KernelArch)
	if arch == "" {
		arch = runtime.GOARCH
	}
	tlsConfig, err := agenttls.NewClientTLSConfig(cfg.CACertPath, cfg.InsecureSkipVerify, cfg.ServerFingerprint)
	if err != nil {
		return nil, fmt.Errorf("invalid TLS configuration: %w", err)
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
	updatedFrom := updatedFromVersionFn()
	if updatedFrom != "" {
		logger.Info().
			Str("previousVersion", updatedFrom).
			Str("currentVersion", agentVersion).
			Msg("Agent was auto-updated")
	}

	packageUpdates := cfg.packageUpdates
	storageCleanup := cfg.storageCleanup
	packageUpdates, storageCleanup = configurePackageManagers(platform, packageUpdates, storageCleanup)
	cfg.packageUpdates = packageUpdates
	cfg.storageCleanup = storageCleanup

	agent := &Agent{
		cfg:                 cfg,
		logger:              logger,
		httpClient:          client,
		hostInfo:            info,
		hostname:            hostname,
		displayName:         displayName,
		platform:            platform,
		osName:              osName,
		osVersion:           osVersion,
		kernelVersion:       kernelVersion,
		architecture:        arch,
		machineID:           machineID,
		agentID:             agentID,
		agentVersion:        agentVersion,
		updatedFrom:         updatedFrom,
		reportIP:            cfg.ReportIP,
		stateDir:            cfg.StateDir,
		interval:            cfg.Interval,
		trimmedPulseURL:     pulseURL,
		remoteConfigChanged: make(chan struct{}, 1),
		reportBuffer:        utils.New[agentshost.Report](bufferCapacity),
		collector:           collector,
		newCommandClient:    newCommandClientFn,
		runCommandClient:    runCommandClientFn,
		packageUpdates:      packageUpdates,
		storageCleanup:      storageCleanup,
	}

	// Create command client for AI command execution (only if enabled)
	if cfg.EnableCommands {
		agent.commandClient = newCommandClientFn(cfg, agentID, hostname, platform, agentVersion)
		cfg.Logger.Info().Msg("Command execution enabled via --enable-commands flag")
	} else {
		cfg.Logger.Info().Msg("Command execution disabled (use --enable-commands to enable)")
	}

	return agent, nil
}

func normalizeProxmoxType(raw string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "", "auto":
		return "", nil
	case "pve", "pbs":
		return value, nil
	default:
		return "", fmt.Errorf("invalid proxmox type %q: must be pve, pbs, or auto", raw)
	}
}

func normalizeReportIP(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}

	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", fmt.Errorf("invalid report IP %q: must be a valid IPv4 or IPv6 address", raw)
	}
	if addr.IsUnspecified() {
		return "", fmt.Errorf("invalid report IP %q: unspecified addresses are not allowed", raw)
	}

	return addr.String(), nil
}

// Run executes the agent until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if a.cfg.RunOnce {
		return a.runOnce(ctx)
	}

	a.commandClientMu.Lock()
	a.commandClientParentCtx = ctx
	commandClient := a.commandClient
	a.commandClientMu.Unlock()
	defer func() {
		a.commandClientMu.Lock()
		a.commandClientParentCtx = nil
		a.commandClientMu.Unlock()
		a.stopCommandClient(true)
	}()

	// Enrollment: exchange bootstrap token for runtime token (must run before
	// Proxmox setup and reporting, which require the runtime token's scopes).
	if a.cfg.Enroll {
		if err := a.runEnrollmentLoop(ctx); err != nil {
			return fmt.Errorf("enrollment failed: %w", err)
		}
		// Re-create command client with the new runtime token, since the
		// original was built with the bootstrap token during New().
		if a.cfg.EnableCommands {
			a.commandClientMu.Lock()
			a.commandClient = a.newCommandClient(a.cfg, a.agentID, a.hostname, a.platform, a.agentVersion)
			commandClient = a.commandClient
			a.commandClientMu.Unlock()
		}
	}

	// Proxmox setup (if enabled)
	if a.cfg.EnableProxmox {
		a.runProxmoxSetup(ctx)
		go a.runProxmoxHealthCheckLoop(ctx)
	}

	// Start command client in background for AI command execution
	if commandClient != nil {
		a.startCommandClient(commandClient)
	}

	// Load any reports buffered from a previous shutdown
	a.loadPersistedBuffer()

	ticker := time.NewTicker(a.currentInterval())
	defer ticker.Stop()
	defer a.persistBuffer()

	if err := a.process(ctx); err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error().
			Err(err).
			Str("hostname", a.hostname).
			Msg("Initial host report failed")
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.remoteConfigChanged:
			ticker.Reset(a.currentInterval())
		case <-ticker.C:
			if err := a.process(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
				a.logger.Error().
					Err(err).
					Str("hostname", a.hostname).
					Msg("Failed to process host report")
			}
		}
	}
}

type runtimeConfigSnapshot struct {
	agentType       string
	interval        time.Duration
	commandsEnabled bool
	diskExclude     []string
	tags            []string
	reportIP        string
	disableCeph     bool
	appliedConfig   *agentshost.ConfigFingerprint
}

func (a *Agent) currentInterval() time.Duration {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	if a.interval <= 0 {
		return time.Minute
	}
	return a.interval
}

func (a *Agent) currentReportIP() string {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return a.reportIP
}

func (a *Agent) runtimeConfigSnapshot() runtimeConfigSnapshot {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return runtimeConfigSnapshot{
		agentType:       a.cfg.AgentType,
		interval:        a.interval,
		commandsEnabled: a.cfg.EnableCommands,
		diskExclude:     append([]string(nil), a.cfg.DiskExclude...),
		tags:            append([]string(nil), a.cfg.Tags...),
		reportIP:        a.reportIP,
		disableCeph:     a.cfg.DisableCeph,
		appliedConfig:   cloneConfigFingerprint(a.cfg.AppliedConfig),
	}
}

func cloneConfigFingerprint(value *agentshost.ConfigFingerprint) *agentshost.ConfigFingerprint {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (a *Agent) currentUpdateStatus() *agentshost.UpdateStatus {
	if a.cfg.UpdateStatus == nil {
		return nil
	}
	status := a.cfg.UpdateStatus()
	return &agentshost.UpdateStatus{
		State:            status.State,
		AutoUpdate:       status.AutoUpdate,
		AvailableVersion: status.AvailableVersion,
		LastCheckedAt:    status.LastCheckedAt,
		LastAttemptAt:    status.LastAttemptAt,
		LastSuccessAt:    status.LastSuccessAt,
		LastError:        status.LastError,
	}
}

func (a *Agent) currentModuleStatus() []agentshost.ModuleStatus {
	if a.cfg.ModuleStatus == nil {
		return nil
	}
	return append([]agentshost.ModuleStatus(nil), a.cfg.ModuleStatus()...)
}

func (a *Agent) signalRemoteConfigChanged() {
	select {
	case a.remoteConfigChanged <- struct{}{}:
	default:
	}
}

func (a *Agent) currentOperationReceiptVersion() int {
	a.commandClientMu.Lock()
	defer a.commandClientMu.Unlock()
	if a.commandClient == nil {
		return 0
	}
	return a.commandClient.operationReceiptVersion()
}

func (a *Agent) startCommandClient(client *CommandClient) bool {
	if client == nil {
		return false
	}

	a.commandClientMu.Lock()
	if a.commandClientRunCancel != nil {
		a.commandClientMu.Unlock()
		return true
	}

	parentCtx := a.commandClientParentCtx
	if parentCtx == nil {
		a.commandClientMu.Unlock()
		return false
	}
	runCtx, cancel := context.WithCancel(parentCtx)
	a.commandClientRunCancel = cancel
	a.commandClientMu.Unlock()

	go func() {
		if err := a.runCommandClient(client, runCtx); err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Error().Err(err).Msg("Command client stopped with error")
		}
	}()
	return true
}

func (a *Agent) stopCommandClient(clearClient bool) {
	a.commandClientMu.Lock()
	client := a.commandClient
	cancel := a.commandClientRunCancel
	a.commandClientRunCancel = nil
	if clearClient {
		a.commandClient = nil
	}
	a.commandClientMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if client != nil {
		if err := client.Close(); err != nil {
			a.logger.Debug().Err(err).Msg("Error closing command client connection")
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
		var statusErr *reportHTTPStatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusForbidden {
			a.logger.Error().
				Err(err).
				Str("endpoint", statusErr.Endpoint).
				Int("status_code", statusErr.StatusCode).
				Msg("Failed to send Unified Agent report (403 Forbidden). API token may lack 'Unified Agent reporting' scope. Set PULSE_ENABLE_HOST=false if host monitoring is not needed.")
			return nil
		}
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusUnauthorized {
			// 401 means the server does not recognise this agent's API token
			// (for example after a Pulse restore or a v5 -> v6 upgrade that did
			// not carry the token across). Retrying with the same token loops
			// forever, so drop the report instead of buffering it and surface an
			// actionable error the operator can act on. Throttle it so a
			// permanently rejected token does not flood the log.
			a.logAuthFailure(statusErr)
			return nil
		}

		a.reportBuffer.Push(report)
		event := a.logger.Warn().
			Err(err).
			Str("endpoint", agentReportEndpoint).
			Int("buffered_reports", a.reportBuffer.Len())
		if errors.As(err, &statusErr) {
			event.Str("endpoint", statusErr.Endpoint)
			event.Int("status_code", statusErr.StatusCode)
		}
		event.Msg("Failed to send report, buffering")
		return nil
	}

	// A successful report means the token is accepted again; reset the auth
	// failure throttle so a later rejection is reported promptly.
	a.lastAuthFailureLog = time.Time{}

	// If successful, try to flush buffer
	a.flushBuffer(ctx)

	a.logger.Debug().
		Str("hostname", report.Host.Hostname).
		Str("platform", report.Host.Platform).
		Msg("Unified Agent report sent")
	return nil
}

// logAuthFailure emits an actionable, throttled error when the server rejects
// the agent's API token with 401. It is only called from the single-threaded
// report loop, so lastAuthFailureLog needs no additional synchronisation.
func (a *Agent) logAuthFailure(statusErr *reportHTTPStatusError) {
	if time.Since(a.lastAuthFailureLog) < authFailureLogInterval {
		return
	}
	a.lastAuthFailureLog = time.Now()

	endpoint := agentReportEndpoint
	statusCode := http.StatusUnauthorized
	if statusErr != nil {
		if statusErr.Endpoint != "" {
			endpoint = statusErr.Endpoint
		}
		statusCode = statusErr.StatusCode
	}

	a.logger.Error().
		Str("endpoint", endpoint).
		Int("status_code", statusCode).
		Str("pulse_url", a.trimmedPulseURL).
		Msg("Pulse rejected this agent's API token (401 Unauthorized). The token is no longer valid on the server, which usually means Pulse was restored/reinstalled or upgraded (for example v5 -> v6) without carrying the token across. Re-run the agent install command from the Pulse UI to mint a fresh token. Reports are dropped until the token is replaced.")
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
			var statusErr *reportHTTPStatusError
			if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusUnauthorized {
				// The token is rejected; buffered reports will never be
				// accepted with it. Drop them so the buffer cannot grow
				// unbounded across restarts, and surface the actionable error.
				a.logAuthFailure(statusErr)
				for {
					if _, ok := a.reportBuffer.Pop(); !ok {
						break
					}
				}
				return
			}
			event := a.logger.Warn().
				Err(err).
				Str("endpoint", agentReportEndpoint).
				Int("remaining_buffered_reports", a.reportBuffer.Len())
			if errors.As(err, &statusErr) {
				event.Str("endpoint", statusErr.Endpoint)
				event.Int("status_code", statusErr.StatusCode)
			}
			event.Msg("Failed to flush buffered report, stopping flush")
			return
		}

		// Pop only on success
		a.reportBuffer.Pop()
	}
}

const bufferFileName = "report-buffer.json"

// persistBuffer writes buffered reports to disk on shutdown so they can be
// retransmitted on the next startup. Uses atomic write (tmp + rename) to
// prevent corruption if the process is killed mid-write.
func (a *Agent) persistBuffer() {
	items := a.reportBuffer.Items()
	if len(items) == 0 {
		return
	}

	if a.stateDir == "" {
		a.logger.Debug().Msg("No state dir configured, skipping buffer persistence")
		return
	}

	data, err := json.Marshal(items)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to marshal report buffer for persistence")
		return
	}

	if err := os.MkdirAll(a.stateDir, 0700); err != nil {
		a.logger.Warn().Err(err).Str("dir", a.stateDir).Msg("Failed to create state dir for buffer persistence")
		return
	}

	path := filepath.Join(a.stateDir, bufferFileName)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		a.logger.Warn().Err(err).Str("path", tmpPath).Msg("Failed to write report buffer temp file")
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		a.logger.Warn().Err(err).Str("path", path).Msg("Failed to rename report buffer file")
		_ = os.Remove(tmpPath)
		return
	}

	a.logger.Info().Int("count", len(items)).Str("path", path).Msg("Persisted report buffer to disk")
}

// loadPersistedBuffer loads buffered reports from a previous shutdown and
// attempts to flush them. The file is deleted after loading regardless of
// whether the flush succeeds (items are pushed back into the in-memory buffer).
func (a *Agent) loadPersistedBuffer() {
	if a.stateDir == "" {
		return
	}

	path := filepath.Join(a.stateDir, bufferFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			a.logger.Warn().Err(err).Str("path", path).Msg("Failed to read persisted report buffer")
		}
		return
	}

	// Always delete the file after reading
	_ = os.Remove(path)

	var items []agentshost.Report
	if err := json.Unmarshal(data, &items); err != nil {
		a.logger.Warn().Err(err).Str("path", path).Msg("Corrupt report buffer file, discarding")
		return
	}

	if len(items) == 0 {
		return
	}

	for _, item := range items {
		a.reportBuffer.Push(item)
	}

	a.logger.Info().Int("count", len(items)).Msg("Loaded persisted report buffer from disk")
}

func (a *Agent) buildReport(ctx context.Context) (agentshost.Report, error) {
	collectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	runtimeConfig := a.runtimeConfigSnapshot()
	uptime, err := a.collector.HostUptime(collectCtx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect host uptime; defaulting to 0")
	}
	snapshot, err := a.collector.Metrics(collectCtx, runtimeConfig.diskExclude)
	if err != nil {
		return agentshost.Report{}, fmt.Errorf("collect metrics: %w", err)
	}

	// Collect temperature data (best effort - don't fail if unavailable)
	sensorData := a.collectTemperatures(collectCtx)

	// Collect S.M.A.R.T. disk data (best effort - don't fail if unavailable)
	smartData := a.collectSMARTData(collectCtx, runtimeConfig.diskExclude)
	if len(smartData) > 0 {
		sensorData.SMART = smartData
	}

	// Collect RAID array data (best effort - don't fail if unavailable)
	raidData := a.collectRAIDArrays(collectCtx)

	// Collect Unraid array topology (best effort - only on Unraid hosts).
	unraidData := a.collectUnraidStorage(collectCtx)

	// Collect Ceph cluster data (best effort - only on Ceph nodes)
	cephData := a.collectCephStatus(collectCtx, runtimeConfig.disableCeph)

	// Collect temperature data from Proxmox cluster peers via SSH (best effort).
	// Uses parent ctx, not collectCtx — cluster SSH has its own 15s budget that
	// would be capped by collectCtx's 10s timeout.
	clusterSensors := a.collectClusterSensors(ctx)

	packageUpdateCtx, cancelPackageUpdates := context.WithTimeout(ctx, 20*time.Second)
	packageUpdates := a.currentPackageUpdateStatus(packageUpdateCtx)
	cancelPackageUpdates()
	storageCleanupCtx, cancelStorageCleanup := context.WithTimeout(ctx, 20*time.Second)
	storageCleanup := a.currentStorageCleanupStatus(storageCleanupCtx)
	cancelStorageCleanup()

	// Carry updated_from on the first freshly built v6 report only. If that
	// report is buffered, the buffered copy still retains the field for retry.
	updatedFrom := a.updatedFrom
	if updatedFrom != "" {
		a.updatedFrom = ""
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:                      a.agentID,
			Version:                 a.agentVersion,
			Type:                    runtimeConfig.agentType,
			IntervalSeconds:         int(runtimeConfig.interval / time.Second),
			Hostname:                a.hostname,
			UpdatedFrom:             updatedFrom,
			CommandsEnabled:         runtimeConfig.commandsEnabled,
			OperationReceiptVersion: a.currentOperationReceiptVersion(),
			DiskExclude:             append([]string(nil), runtimeConfig.diskExclude...),
			AppliedConfig:           runtimeConfig.appliedConfig,
			Update:                  a.currentUpdateStatus(),
			Modules:                 a.currentModuleStatus(),
		},
		Host: agentshost.HostInfo{
			ID:             a.machineID,
			Hostname:       a.hostname,
			DisplayName:    a.displayName,
			MachineID:      a.machineID,
			Platform:       a.platform,
			OSName:         a.osName,
			OSVersion:      a.osVersion,
			KernelVersion:  a.kernelVersion,
			Architecture:   a.architecture,
			CPUModel:       "",
			CPUCount:       snapshot.CPUCount,
			UptimeSeconds:  int64(uptime),
			LoadAverage:    append([]float64(nil), snapshot.LoadAverage...),
			ReportIP:       runtimeConfig.reportIP,
			PackageUpdates: packageUpdates,
			StorageCleanup: storageCleanup,
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: snapshot.CPUUsagePercent,
			Memory:          snapshot.Memory,
		},
		Disks:          append([]agentshost.Disk(nil), snapshot.Disks...),
		DiskIO:         append([]agentshost.DiskIO(nil), snapshot.DiskIO...),
		Network:        append([]agentshost.NetworkInterface(nil), snapshot.Network...),
		Sensors:        sensorData,
		RAID:           raidData,
		Unraid:         unraidData,
		Ceph:           cephData,
		ClusterSensors: clusterSensors,
		Tags:           append([]string(nil), runtimeConfig.tags...),
		Timestamp:      a.collector.Now(),
	}

	return report, nil
}

func (a *Agent) currentPackageUpdateStatus(ctx context.Context) *agentshost.PackageUpdateStatus {
	if a == nil || a.packageUpdates == nil {
		return nil
	}
	snapshot := a.packageUpdates.Snapshot(ctx, false)
	packages := make([]agentshost.PackageUpdate, len(snapshot.Packages))
	for i, pkg := range snapshot.Packages {
		packages[i] = agentshost.PackageUpdate{
			Name:             pkg.Name,
			InstalledVersion: pkg.InstalledVersion,
			AvailableVersion: pkg.AvailableVersion,
		}
	}
	return &agentshost.PackageUpdateStatus{
		Supported:      snapshot.Supported,
		Manager:        snapshot.Manager,
		InventoryHash:  snapshot.InventoryHash,
		PendingCount:   snapshot.PendingCount,
		Packages:       packages,
		CheckedAt:      snapshot.CheckedAt,
		RebootRequired: snapshot.RebootRequired,
		Error:          snapshot.Error,
	}
}

func (a *Agent) currentStorageCleanupStatus(ctx context.Context) *agentshost.StorageCleanupStatus {
	if a == nil || a.storageCleanup == nil {
		return nil
	}
	snapshot := a.storageCleanup.Snapshot(ctx, false)
	return &agentshost.StorageCleanupStatus{
		Supported:        snapshot.Supported,
		Provider:         snapshot.Provider,
		Fingerprint:      snapshot.Fingerprint,
		ReclaimableBytes: snapshot.ReclaimableBytes,
		CheckedAt:        snapshot.CheckedAt,
		Error:            snapshot.Error,
	}
}

func (a *Agent) sendReport(ctx context.Context, report agentshost.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	compressed, err := utils.CompressJSON(payload)
	if err != nil {
		return fmt.Errorf("compress report: %w", err)
	}

	endpoint := agentReportEndpoint
	url := fmt.Sprintf("%s%s", a.trimmedPulseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if token := strings.TrimSpace(a.cfg.APIToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-API-Token", token)
	}
	req.Header.Set("User-Agent", "pulse-agent/"+Version)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			a.logger.Debug().Err(closeErr).Msg("Failed to close report response body")
		}
	}()

	if resp.StatusCode >= 300 {
		return &reportHTTPStatusError{
			Endpoint:   endpoint,
			Status:     resp.Status,
			StatusCode: resp.StatusCode,
		}
	}

	// Parse response to check for server-side config overrides
	var reportResp struct {
		Success bool   `json:"success"`
		AgentID string `json:"agentId"`
		Config  *struct {
			CommandsEnabled *bool `json:"commandsEnabled"`
		} `json:"config,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reportResp); err != nil {
		// Non-fatal: just log and continue
		a.logger.Debug().Err(err).Msg("Failed to parse report response, ignoring config")
		return nil
	}

	// Persist the server-acknowledged agent ID so uninstall can deregister.
	canonicalAgentID := strings.TrimSpace(reportResp.AgentID)
	if canonicalAgentID != "" {
		a.persistAgentID(canonicalAgentID)
	}

	// Apply server config overrides
	if reportResp.Config != nil && reportResp.Config.CommandsEnabled != nil {
		a.applyRemoteConfig(*reportResp.Config.CommandsEnabled)
	}

	return nil
}

// persistAgentID writes the server-assigned agent ID to the state directory.
// This file is read by the uninstall script to deregister the agent from the server.
// Errors are debug-logged, never fatal — same resilience pattern as proxmox_setup.go.
func (a *Agent) persistAgentID(agentID string) {
	if a.stateDir == "" {
		return
	}
	if err := a.collector.MkdirAll(a.stateDir, 0700); err != nil {
		a.logger.Debug().Err(err).Msg("Failed to create state directory for agent-id")
		return
	}
	agentIDPath := filepath.Join(a.stateDir, "agent-id")
	if err := a.collector.WriteFile(agentIDPath, []byte(agentID), 0600); err != nil {
		a.logger.Debug().Err(err).Msg("Failed to persist agent-id")
		return
	}
	if err := a.collector.Chmod(agentIDPath, 0600); err != nil {
		a.logger.Debug().Err(err).Msg("Failed to enforce agent-id file permissions")
	}
}

// ApplyRemoteConfig applies server-side configuration overrides that can be
// reconciled safely without restarting the agent process.
func (a *Agent) ApplyRemoteConfig(settings map[string]interface{}, commandsEnabled *bool) {
	if commandsEnabled != nil {
		a.applyRemoteConfig(*commandsEnabled)
	}
	if interval, ok := remoteDurationSetting(settings, "interval"); ok && interval > 0 {
		intervalChanged := false
		a.configMu.Lock()
		if a.interval != interval {
			intervalChanged = true
		}
		a.interval = interval
		a.cfg.Interval = interval
		a.configMu.Unlock()
		if intervalChanged {
			a.signalRemoteConfigChanged()
		}
		a.logger.Info().Dur("interval", interval).Msg("Applied remote host report interval")
	}
	if reportIP, ok := remoteStringSetting(settings, "report_ip"); ok {
		trimmedReportIP := strings.TrimSpace(reportIP)
		a.configMu.Lock()
		a.reportIP = trimmedReportIP
		a.cfg.ReportIP = trimmedReportIP
		a.configMu.Unlock()
		a.logger.Info().Str("report_ip", trimmedReportIP).Msg("Applied remote host report IP override")
	}
	if disableCeph, ok := remoteBoolSetting(settings, "disable_ceph"); ok {
		a.configMu.Lock()
		a.cfg.DisableCeph = disableCeph
		a.configMu.Unlock()
		a.logger.Info().Bool("disable_ceph", disableCeph).Msg("Applied remote Ceph collection setting")
	}
	if remoteConfigAppliedWithoutRestart(settings) && remoteconfig.HasAppliedDesiredConfig(commandsEnabled, settings) {
		metadata, err := remoteconfig.BuildDesiredConfigMetadata(commandsEnabled, settings)
		if err != nil {
			a.logger.Warn().Err(err).Msg("Failed to derive refreshed managed configuration fingerprint")
			return
		}
		a.configMu.Lock()
		a.cfg.AppliedConfig = &agentshost.ConfigFingerprint{Version: metadata.Version, Hash: metadata.Hash}
		a.configMu.Unlock()
	}
}

func remoteConfigAppliedWithoutRestart(settings map[string]interface{}) bool {
	for key := range settings {
		switch key {
		case "interval", "report_ip", "disable_ceph":
			continue
		default:
			return false
		}
	}
	return true
}

// applyRemoteConfig applies server-side command execution overrides.
func (a *Agent) applyRemoteConfig(commandsEnabled bool) {
	var (
		clientToStart *CommandClient
		commandCfg    Config
		shouldStop    bool
	)

	a.configMu.Lock()
	a.cfg.EnableCommands = commandsEnabled
	commandCfg = a.cfg
	a.configMu.Unlock()

	a.commandClientMu.Lock()
	currentlyEnabled := a.commandClient != nil
	switch {
	case commandsEnabled && !currentlyEnabled:
		clientToStart = a.newCommandClient(commandCfg, a.agentID, a.hostname, a.platform, a.agentVersion)
		a.commandClient = clientToStart
	case commandsEnabled && currentlyEnabled:
		clientToStart = a.commandClient
	case !commandsEnabled && currentlyEnabled:
		shouldStop = true
	}
	a.commandClientMu.Unlock()

	if clientToStart != nil {
		if a.startCommandClient(clientToStart) {
			a.logger.Info().Msg("Server enabled command execution - starting command client")
		} else {
			a.logger.Info().Msg("Server enabled command execution - command client configured pending run context")
		}
	} else if shouldStop {
		a.logger.Info().Msg("Server disabled command execution - stopping command client")
		a.stopCommandClient(true)
	}
}

func remoteBoolSetting(settings map[string]interface{}, key string) (bool, bool) {
	if settings == nil {
		return false, false
	}
	value, ok := settings[key]
	if !ok {
		return false, false
	}
	parsed, ok := value.(bool)
	return parsed, ok
}

func remoteStringSetting(settings map[string]interface{}, key string) (string, bool) {
	if settings == nil {
		return "", false
	}
	value, ok := settings[key]
	if !ok {
		return "", false
	}
	parsed, ok := value.(string)
	return parsed, ok
}

func remoteDurationSetting(settings map[string]interface{}, key string) (time.Duration, bool) {
	if settings == nil {
		return 0, false
	}
	value, ok := settings[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case string:
		parsed, err := time.ParseDuration(typed)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case float64:
		return time.Duration(typed * float64(time.Second)), true
	case int:
		return time.Duration(typed) * time.Second, true
	case int64:
		return time.Duration(typed) * time.Second, true
	default:
		return 0, false
	}
}

func normalisePlatform(platform string) string {
	normalized := platformsupport.NormalizeAgentReportedPlatform(platform)
	if runtimePlatform := platformsupport.RuntimePlatformForHostIdentityToken(normalized); runtimePlatform != "" {
		return runtimePlatform
	}
	return normalized
}

func normalizePulseURL(rawURL string) (string, error) {
	parsed, err := securityutil.NormalizePulseHTTPBaseURLWithOptions(rawURL, securityutil.PulseURLValidationOptions{
		AllowLocalNetworkHTTP: true,
	})
	if err != nil {
		return "", err
	}

	return parsed.String(), nil
}

// collectTemperatures attempts to collect temperature data from the local system.
// Returns an empty Sensors struct if collection fails (best-effort).
func (a *Agent) collectTemperatures(ctx context.Context) agentshost.Sensors {
	switch a.collector.GOOS() {
	case "linux":
		// Continue below with lm-sensors path
	case "darwin":
		return a.collectDarwinThermalState(ctx)
	case "freebsd":
		return a.collectFreeBSDTemperatures(ctx)
	default:
		return agentshost.Sensors{}
	}

	// Collect sensor JSON output
	jsonOutput, err := a.collector.SensorsLocal(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect sensor data (lm-sensors may not be installed)")
		return a.collectNVIDIATemperatureSensors(ctx)
	}

	// Parse the sensor output
	tempData, err := a.collector.SensorsParse(jsonOutput)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to parse sensor data")
		return a.collectNVIDIATemperatureSensors(ctx)
	}

	if !tempData.Available {
		a.logger.Debug().Msg("No temperature sensors available on this system")
		return a.collectNVIDIATemperatureSensors(ctx)
	}

	result := convertTemperatureDataToSensors(tempData)
	a.mergeNVIDIATemperatures(ctx, &result)

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

// convertTemperatureDataToSensors converts parsed sensor data into the agent report
// sensor format. This is shared between local collection and cluster peer collection.
func convertTemperatureDataToSensors(tempData *sensors.TemperatureData) agentshost.Sensors {
	result := agentshost.Sensors{
		TemperatureCelsius: make(map[string]float64),
	}

	// Add CPU package temperature
	if tempData.CPUPackage > 0 {
		result.TemperatureCelsius["cpu_package"] = tempData.CPUPackage
	}

	// Add individual core temperatures
	for coreName, temp := range tempData.Cores {
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

	return result
}

// collectDarwinThermalState reads macOS thermal pressure signals. macOS does
// not expose Linux-style raw temperature sensors through a stable unprivileged
// interface, so this reports OS pressure state without inventing Celsius data.
func (a *Agent) collectDarwinThermalState(ctx context.Context) agentshost.Sensors {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := a.collector.CommandCombinedOutput(cmdCtx, "pmset", "-g", "therm")
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect macOS thermal state via pmset")
		return agentshost.Sensors{}
	}

	state := parseDarwinPMSetThermalState(output)
	if state == nil {
		a.logger.Debug().Msg("No macOS thermal state available from pmset")
		return agentshost.Sensors{}
	}

	a.logger.Debug().
		Str("pressure", state.Pressure).
		Int("limitCount", len(state.LimitsPercent)).
		Msg("Collected macOS thermal state")

	return agentshost.Sensors{ThermalState: state}
}

func parseDarwinPMSetThermalState(output string) *agentshost.ThermalState {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}

	state := &agentshost.ThermalState{
		Source:   "pmset",
		Pressure: agentshost.ThermalPressureUnknown,
	}
	seen := false
	constrained := false
	limits := make(map[string]int)

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)

		switch {
		case strings.Contains(lower, "no thermal warning level"):
			state.ThermalWarningLevel = intPtr(0)
			seen = true
			continue
		case strings.Contains(lower, "no performance warning level"):
			state.PerformanceWarningLevel = intPtr(0)
			seen = true
			continue
		case strings.Contains(lower, "no cpu power status"):
			state.CPUPowerStatus = intPtr(0)
			seen = true
			continue
		}

		if value, ok := parseIntegerAfterMarker(line, "thermal warning level"); ok {
			state.ThermalWarningLevel = intPtr(value)
			seen = true
			if value > 0 {
				constrained = true
			}
			continue
		}
		if value, ok := parseIntegerAfterMarker(line, "performance warning level"); ok {
			state.PerformanceWarningLevel = intPtr(value)
			seen = true
			if value > 0 {
				constrained = true
			}
			continue
		}
		if value, ok := parseIntegerAfterMarker(line, "cpu power status"); ok {
			state.CPUPowerStatus = intPtr(value)
			seen = true
			if value > 0 {
				constrained = true
			}
			continue
		}
		if key, value, ok := parsePMSetLimitPercent(line); ok {
			limits[key] = value
			seen = true
			if value >= 0 && value < 100 {
				constrained = true
			}
		}
	}

	if !seen {
		return nil
	}
	if len(limits) > 0 {
		state.LimitsPercent = limits
	}
	if constrained {
		state.Pressure = agentshost.ThermalPressureConstrained
	} else {
		state.Pressure = agentshost.ThermalPressureNominal
	}
	return state
}

func parsePMSetLimitPercent(line string) (string, int, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	key := strings.TrimSpace(parts[0])
	if key == "" || !strings.Contains(strings.ToLower(key), "limit") {
		return "", 0, false
	}
	value, ok := parseFirstInteger(parts[1])
	if !ok {
		return "", 0, false
	}
	return normalizePMSetLimitKey(key), value, true
}

func normalizePMSetLimitKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	replacer := strings.NewReplacer(" ", "_", "-", "_")
	key = replacer.Replace(key)
	for strings.Contains(key, "__") {
		key = strings.ReplaceAll(key, "__", "_")
	}
	return strings.Trim(key, "_")
}

func parseIntegerAfterMarker(line, marker string) (int, bool) {
	lower := strings.ToLower(line)
	idx := strings.Index(lower, strings.ToLower(marker))
	if idx < 0 {
		return 0, false
	}
	return parseFirstInteger(line[idx+len(marker):])
}

func parseFirstInteger(input string) (int, bool) {
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if (ch < '0' || ch > '9') && ch != '-' {
			continue
		}
		j := i + 1
		for j < len(input) && input[j] >= '0' && input[j] <= '9' {
			j++
		}
		value, err := strconv.Atoi(input[i:j])
		if err != nil {
			continue
		}
		return value, true
	}
	return 0, false
}

func intPtr(value int) *int {
	return &value
}

// collectFreeBSDTemperatures reads CPU and ACPI thermal zone temperatures
// from sysctl on FreeBSD. Returns an empty Sensors struct on any error.
func (a *Agent) collectFreeBSDTemperatures(ctx context.Context) agentshost.Sensors {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sysctl", "-a")
	output, err := cmd.Output()
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect FreeBSD temperature data via sysctl")
		return agentshost.Sensors{}
	}

	result := parseFreeBSDSysctlTemperatures(string(output))

	if len(result.TemperatureCelsius) > 0 {
		a.logger.Debug().
			Int("temperatureCount", len(result.TemperatureCelsius)).
			Msg("Collected FreeBSD temperature data via sysctl")
	}

	return result
}

// parseFreeBSDSysctlTemperatures parses sysctl output for temperature readings.
// FreeBSD exposes CPU temps via dev.cpu.N.temperature and ACPI thermal zones via
// hw.acpi.thermal.tzN.temperature. Values are formatted as "45.0C".
func parseFreeBSDSysctlTemperatures(sysctlOutput string) agentshost.Sensors {
	result := agentshost.Sensors{
		TemperatureCelsius: make(map[string]float64),
	}

	for _, line := range strings.Split(sysctlOutput, "\n") {
		if !strings.Contains(line, ".temperature:") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])
		valStr = strings.TrimSuffix(valStr, "C")

		temp, err := strconv.ParseFloat(valStr, 64)
		if err != nil || temp <= 0 {
			continue
		}

		keyParts := strings.Split(key, ".")
		switch {
		case strings.HasPrefix(key, "dev.cpu.") && len(keyParts) >= 3:
			result.TemperatureCelsius["cpu_core_"+keyParts[2]] = temp
		case strings.HasPrefix(key, "hw.acpi.thermal.") && len(keyParts) >= 4:
			result.TemperatureCelsius["acpi_"+keyParts[3]] = temp
		default:
			normalized := strings.ReplaceAll(key, ".", "_")
			normalized = strings.TrimSuffix(normalized, "_temperature")
			result.TemperatureCelsius[normalized] = temp
		}
	}

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

// collectUnraidStorage attempts to collect Unraid array topology.
// Returns nil when not running on Unraid or if collection fails.
func (a *Agent) collectUnraidStorage(ctx context.Context) *agentshost.UnraidStorage {
	if a.collector.GOOS() != "linux" {
		return nil
	}

	storage, err := a.collector.UnraidStorage(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect Unraid storage data")
		return nil
	}
	if storage == nil {
		return nil
	}

	a.logger.Debug().
		Bool("arrayStarted", storage.ArrayStarted).
		Int("diskCount", len(storage.Disks)).
		Msg("Collected Unraid storage data")

	return storage
}

// collectCephStatus attempts to collect Ceph cluster status.
// Returns nil if Ceph is not available or not configured on this host.
func (a *Agent) collectCephStatus(ctx context.Context, disableCeph bool) *agentshost.CephCluster {
	if disableCeph {
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
			Status: string(status.Health.Status),
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
			Severity: string(check.Severity),
			Message:  check.Message,
			Detail:   check.Detail,
		}
	}

	// Convert health summary
	for _, s := range status.Health.Summary {
		result.Health.Summary = append(result.Health.Summary, agentshost.CephHealthSummary{
			Severity: string(s.Severity),
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
			Type:    string(svc.Type),
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
func (a *Agent) collectSMARTData(ctx context.Context, diskExclude []string) []agentshost.DiskSMART {
	goos := a.collector.GOOS()
	if goos != "linux" && goos != "freebsd" {
		return nil
	}

	smartData, err := a.collector.SMARTLocal(ctx, diskExclude)
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
			SizeBytes:   disk.SizeBytes,
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

	if pools, err := ZFSDiskPoolMap(ctx); err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect ZFS pool membership for SMART annotation")
	} else if len(pools) > 0 {
		annotateSMARTWithZFSPools(result, pools)
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
		a.currentReportIP(),
		a.stateDir,
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

const (
	proxmoxHealthCheckInitialDelay = 2 * time.Minute
	proxmoxHealthCheckInterval     = 5 * time.Minute
)

// runProxmoxHealthCheckLoop periodically verifies that Proxmox nodes registered
// at startup are still connected to Pulse. This closes the cold-startup race
// where the monitor may not have connection data at the time of the initial
// registration check, causing a stale-token node to be silently skipped.
func (a *Agent) runProxmoxHealthCheckLoop(ctx context.Context) {
	// Wait for the monitor to build initial connection-health state before
	// the first check so that a stale token is detectable.
	select {
	case <-ctx.Done():
		return
	case <-time.After(proxmoxHealthCheckInitialDelay):
	}

	ticker := time.NewTicker(proxmoxHealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			setup := NewProxmoxSetup(
				a.logger,
				a.httpClient,
				a.collector,
				a.trimmedPulseURL,
				a.cfg.APIToken,
				a.cfg.ProxmoxType,
				a.hostname,
				a.currentReportIP(),
				a.stateDir,
				a.cfg.InsecureSkipVerify,
			)
			results, err := setup.RunHealthCheck(ctx)
			if err != nil {
				a.logger.Warn().Err(err).Msg("Proxmox health check failed")
				continue
			}
			for _, result := range results {
				if result.Registered {
					a.logger.Info().
						Str("type", result.ProxmoxType).
						Str("host", result.NodeHost).
						Msg("Proxmox node re-registered via health check")
				} else {
					a.logger.Warn().
						Str("type", result.ProxmoxType).
						Str("host", result.NodeHost).
						Msg("Proxmox health check: token rotated but registration failed")
				}
			}
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
				if len(machineID) == 32 && utils.IsHexString(machineID) {
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
