package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/kubernetesagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

var (
	Version = "dev"

	// Prometheus metrics
	agentInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pulse_agent_info",
		Help: "Information about the Pulse agent",
	}, []string{"version", "host_enabled", "docker_enabled", "kubernetes_enabled"})

	agentUp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pulse_agent_up",
		Help: "Whether the Pulse agent is running (1 = up, 0 = down)",
	})
)

type multiValue []string

func (m *multiValue) String() string {
	return strings.Join(*m, ",")
}

func (m *multiValue) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	// 1. Parse Configuration
	cfg := loadConfig()

	// 2. Setup Logging
	zerolog.SetGlobalLevel(cfg.LogLevel)
	logger := zerolog.New(os.Stdout).Level(cfg.LogLevel).With().Timestamp().Logger()
	cfg.Logger = &logger

	// 2a. Handle Self-Test
	if cfg.SelfTest {
		logger.Info().Msg("Self-test passed: config loaded and logger initialized")
		os.Exit(0)
	}

	// 3. Check if running as Windows service
	ranAsService, err := runAsWindowsService(cfg, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Windows service failed")
	}
	if ranAsService {
		return
	}

	// 4. Setup Context & Signal Handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	logger.Info().
		Str("version", Version).
		Str("pulse_url", cfg.PulseURL).
		Bool("host_agent", cfg.EnableHost).
		Bool("docker_agent", cfg.EnableDocker).
		Bool("kubernetes_agent", cfg.EnableKubernetes).
		Bool("proxmox_mode", cfg.EnableProxmox).
		Bool("auto_update", !cfg.DisableAutoUpdate).
		Msg("Starting Pulse Unified Agent")

	// 5. Set prometheus info metric
	agentInfo.WithLabelValues(
		Version,
		fmt.Sprintf("%t", cfg.EnableHost),
		fmt.Sprintf("%t", cfg.EnableDocker),
		fmt.Sprintf("%t", cfg.EnableKubernetes),
	).Set(1)
	agentUp.Set(1)

	// 6. Start Health/Metrics Server
	var ready atomic.Bool
	if cfg.HealthAddr != "" {
		startHealthServer(ctx, cfg.HealthAddr, &ready, &logger)
	}

	// 7. Start Auto-Updater
	updater := agentupdate.New(agentupdate.Config{
		PulseURL:           cfg.PulseURL,
		APIToken:           cfg.APIToken,
		AgentName:          "pulse-agent",
		CurrentVersion:     Version,
		CheckInterval:      1 * time.Hour,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		Logger:             &logger,
		Disabled:           cfg.DisableAutoUpdate,
	})

	g.Go(func() error {
		updater.RunLoop(ctx)
		return nil
	})

	// 8. Start Host Agent (if enabled)
	if cfg.EnableHost {
		hostCfg := hostagent.Config{
			PulseURL:           cfg.PulseURL,
			APIToken:           cfg.APIToken,
			Interval:           cfg.Interval,
			HostnameOverride:   cfg.HostnameOverride,
			AgentID:            cfg.AgentID,
			AgentType:          "unified",
			AgentVersion:       Version,
			Tags:               cfg.Tags,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			LogLevel:           cfg.LogLevel,
			Logger:             &logger,
			EnableProxmox:      cfg.EnableProxmox,
			ProxmoxType:        cfg.ProxmoxType,
			EnableCommands:     cfg.EnableCommands,
			DiskExclude:        cfg.DiskExclude,
			ReportIP:           cfg.ReportIP,
		}

		agent, err := hostagent.New(hostCfg)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to initialize host agent")
		}

		g.Go(func() error {
			logger.Info().Msg("Host agent module started")
			return agent.Run(ctx)
		})
	}

	// Auto-detect Docker/Podman if not explicitly configured
	if !cfg.EnableDocker && !cfg.DockerConfigured {
		// Check for docker binary
		if _, err := exec.LookPath("docker"); err == nil {
			logger.Info().Msg("Auto-detected Docker binary, enabling Docker monitoring")
			cfg.EnableDocker = true
		} else if _, err := exec.LookPath("podman"); err == nil {
			logger.Info().Msg("Auto-detected Podman binary, enabling Docker monitoring")
			cfg.EnableDocker = true
		} else {
			logger.Debug().Msg("Docker/Podman not found, skipping Docker monitoring")
		}
	}

	// 9. Start Docker Agent (if enabled)
	var dockerAgent *dockeragent.Agent
	if cfg.EnableDocker {
		dockerCfg := dockeragent.Config{
			PulseURL:           cfg.PulseURL,
			APIToken:           cfg.APIToken,
			Interval:           cfg.Interval,
			HostnameOverride:   cfg.HostnameOverride,
			AgentID:            cfg.AgentID,
			AgentType:          "unified",
			AgentVersion:       Version,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			DisableAutoUpdate:  cfg.DisableAutoUpdate,
			LogLevel:           cfg.LogLevel,
			Logger:             &logger,
			SwarmScope:         "node",
			IncludeContainers:  true,
			IncludeServices:    true,
			IncludeTasks:       true,
			CollectDiskMetrics: false,
		}

		dockerAgent, err = dockeragent.New(dockerCfg)
		if err != nil {
			// Docker isn't available yet - start retry loop in background
			logger.Warn().Err(err).Msg("Docker not available, will retry with exponential backoff")

			g.Go(func() error {
				agent := initDockerWithRetry(ctx, dockerCfg, &logger)
				if agent != nil {
					dockerAgent = agent
					logger.Info().Msg("Docker agent module started (after retry)")
					return agent.Run(ctx)
				}
				// Docker never became available, continue without it
				return nil
			})
		} else {
			g.Go(func() error {
				logger.Info().Msg("Docker agent module started")
				return dockerAgent.Run(ctx)
			})
		}
	}

	// 10. Start Kubernetes Agent (if enabled)
	if cfg.EnableKubernetes {
		kubeCfg := kubernetesagent.Config{
			PulseURL:           cfg.PulseURL,
			APIToken:           cfg.APIToken,
			Interval:           cfg.Interval,
			AgentID:            cfg.AgentID,
			AgentType:          "unified",
			AgentVersion:       Version,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			LogLevel:           cfg.LogLevel,
			Logger:             &logger,
			KubeconfigPath:     cfg.KubeconfigPath,
			KubeContext:        cfg.KubeContext,
			IncludeNamespaces:     cfg.KubeIncludeNamespaces,
			ExcludeNamespaces:     cfg.KubeExcludeNamespaces,
			IncludeAllPods:        cfg.KubeIncludeAllPods,
			IncludeAllDeployments: cfg.KubeIncludeAllDeployments,
			MaxPods:               cfg.KubeMaxPods,
		}

		agent, err := kubernetesagent.New(kubeCfg)
		if err != nil {
			logger.Warn().Err(err).Msg("Kubernetes not available, will retry with exponential backoff")

			g.Go(func() error {
				retried := initKubernetesWithRetry(ctx, kubeCfg, &logger)
				if retried != nil {
					logger.Info().Msg("Kubernetes agent module started (after retry)")
					return retried.Run(ctx)
				}
				return nil
			})
		} else {
			g.Go(func() error {
				logger.Info().Msg("Kubernetes agent module started")
				return agent.Run(ctx)
			})
		}
	}

	// Mark as ready after all agents started
	ready.Store(true)

	// 11. Wait for all agents to exit
	if err := g.Wait(); err != nil && err != context.Canceled {
		logger.Error().Err(err).Msg("Agent terminated with error")
		agentUp.Set(0)
		cleanupDockerAgent(dockerAgent, &logger)
		os.Exit(1)
	}

	// 12. Cleanup
	agentUp.Set(0)
	cleanupDockerAgent(dockerAgent, &logger)

	logger.Info().Msg("Pulse Unified Agent stopped")
}

func cleanupDockerAgent(agent *dockeragent.Agent, logger *zerolog.Logger) {
	if agent == nil {
		return
	}
	if err := agent.Close(); err != nil {
		logger.Warn().Err(err).Msg("Failed to close docker agent")
	}
}

func healthHandler(ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()

	// Liveness probe - always returns 200 if server is running
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Readiness probe - returns 200 only when agents are initialized
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if ready.Load() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
		}
	})

	// Prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func startHealthServer(ctx context.Context, addr string, ready *atomic.Bool, logger *zerolog.Logger) {
	srv := &http.Server{
		Addr:         addr,
		Handler:      healthHandler(ready),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
			logger.Warn().Err(err).Msg("Failed to shut down health server")
		}
	}()

	go func() {
		logger.Info().Str("addr", addr).Msg("Health/metrics server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Warn().Err(err).Msg("Health server stopped unexpectedly")
		}
	}()
}

type Config struct {
	PulseURL           string
	APIToken           string
	Interval           time.Duration
	HostnameOverride   string
	AgentID            string
	Tags               []string
	InsecureSkipVerify bool
	LogLevel           zerolog.Level
	Logger             *zerolog.Logger

	// Module flags
	EnableHost       bool
	EnableDocker     bool
	DockerConfigured bool
	EnableKubernetes bool
	EnableProxmox    bool
	ProxmoxType      string // "pve", "pbs", or "" for auto-detect

	// Auto-update
	DisableAutoUpdate bool

	// Security
	EnableCommands  bool // Enable command execution for AI auto-fix (disabled by default)

	// Disk filtering
	DiskExclude []string // Mount points or patterns to exclude from disk monitoring

	// Network configuration
	ReportIP string // IP address to report (for multi-NIC systems)
	SelfTest bool   // Perform self-test and exit

	// Health/metrics server
	HealthAddr string

	// Kubernetes
	KubeconfigPath        string
	KubeContext           string
	KubeIncludeNamespaces     []string
	KubeExcludeNamespaces     []string
	KubeIncludeAllPods        bool
	KubeIncludeAllDeployments bool
	KubeMaxPods               int
}

func loadConfig() Config {
	// Environment Variables
	envURL := utils.GetenvTrim("PULSE_URL")
	envToken := utils.GetenvTrim("PULSE_TOKEN")
	envInterval := utils.GetenvTrim("PULSE_INTERVAL")
	envHostname := utils.GetenvTrim("PULSE_HOSTNAME")
	envAgentID := utils.GetenvTrim("PULSE_AGENT_ID")
	envInsecure := utils.GetenvTrim("PULSE_INSECURE_SKIP_VERIFY")
	envTags := utils.GetenvTrim("PULSE_TAGS")
	envLogLevel := utils.GetenvTrim("LOG_LEVEL")
	envEnableHost := utils.GetenvTrim("PULSE_ENABLE_HOST")
	envEnableDocker := utils.GetenvTrim("PULSE_ENABLE_DOCKER")
	envEnableKubernetes := utils.GetenvTrim("PULSE_ENABLE_KUBERNETES")
	envEnableProxmox := utils.GetenvTrim("PULSE_ENABLE_PROXMOX")
	envProxmoxType := utils.GetenvTrim("PULSE_PROXMOX_TYPE")
	envDisableAutoUpdate := utils.GetenvTrim("PULSE_DISABLE_AUTO_UPDATE")
	envEnableCommands := utils.GetenvTrim("PULSE_ENABLE_COMMANDS")
	envDisableCommands := utils.GetenvTrim("PULSE_DISABLE_COMMANDS") // deprecated
	envHealthAddr := utils.GetenvTrim("PULSE_HEALTH_ADDR")
	envKubeconfig := utils.GetenvTrim("PULSE_KUBECONFIG")
	envKubeContext := utils.GetenvTrim("PULSE_KUBE_CONTEXT")
	envKubeIncludeNamespaces := utils.GetenvTrim("PULSE_KUBE_INCLUDE_NAMESPACES")
	envKubeExcludeNamespaces := utils.GetenvTrim("PULSE_KUBE_EXCLUDE_NAMESPACES")
	envKubeIncludeAllPods := utils.GetenvTrim("PULSE_KUBE_INCLUDE_ALL_PODS")
	envKubeIncludeAllDeployments := utils.GetenvTrim("PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS")
	envKubeMaxPods := utils.GetenvTrim("PULSE_KUBE_MAX_PODS")
	envDiskExclude := utils.GetenvTrim("PULSE_DISK_EXCLUDE")
	envReportIP := utils.GetenvTrim("PULSE_REPORT_IP")

	// Defaults
	defaultInterval := 30 * time.Second
	if envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval); err == nil {
			defaultInterval = parsed
		}
	}

	defaultEnableHost := true
	if envEnableHost != "" {
		defaultEnableHost = utils.ParseBool(envEnableHost)
	}

	defaultEnableDocker := false
	if envEnableDocker != "" {
		defaultEnableDocker = utils.ParseBool(envEnableDocker)
	}

	defaultEnableKubernetes := false
	if envEnableKubernetes != "" {
		defaultEnableKubernetes = utils.ParseBool(envEnableKubernetes)
	}

	defaultEnableProxmox := false
	if envEnableProxmox != "" {
		defaultEnableProxmox = utils.ParseBool(envEnableProxmox)
	}

	defaultHealthAddr := envHealthAddr
	if defaultHealthAddr == "" {
		defaultHealthAddr = ":9191"
	}

	// Flags
	urlFlag := flag.String("url", envURL, "Pulse server URL")
	tokenFlag := flag.String("token", envToken, "Pulse API token (prefer --token-file for security)")
	tokenFileFlag := flag.String("token-file", "", "Path to file containing Pulse API token (more secure than --token)")
	intervalFlag := flag.Duration("interval", defaultInterval, "Reporting interval")
	hostnameFlag := flag.String("hostname", envHostname, "Override hostname")
	agentIDFlag := flag.String("agent-id", envAgentID, "Override agent identifier")
	insecureFlag := flag.Bool("insecure", utils.ParseBool(envInsecure), "Skip TLS verification")
	logLevelFlag := flag.String("log-level", defaultLogLevel(envLogLevel), "Log level")

	enableHostFlag := flag.Bool("enable-host", defaultEnableHost, "Enable Host Agent module")
	enableDockerFlag := flag.Bool("enable-docker", defaultEnableDocker, "Enable Docker Agent module")
	enableKubernetesFlag := flag.Bool("enable-kubernetes", defaultEnableKubernetes, "Enable Kubernetes Agent module")
	enableProxmoxFlag := flag.Bool("enable-proxmox", defaultEnableProxmox, "Enable Proxmox mode (creates API token, registers node)")
	proxmoxTypeFlag := flag.String("proxmox-type", envProxmoxType, "Proxmox type: pve or pbs (auto-detected if not specified)")
	disableAutoUpdateFlag := flag.Bool("disable-auto-update", utils.ParseBool(envDisableAutoUpdate), "Disable automatic updates")
	enableCommandsFlag := flag.Bool("enable-commands", utils.ParseBool(envEnableCommands), "Enable command execution for AI auto-fix (disabled by default)")
	disableCommandsFlag := flag.Bool("disable-commands", false, "[DEPRECATED] Commands are now disabled by default; use --enable-commands to enable")
	healthAddrFlag := flag.String("health-addr", defaultHealthAddr, "Health/metrics server address (empty to disable)")
	kubeconfigFlag := flag.String("kubeconfig", envKubeconfig, "Path to kubeconfig (optional; uses in-cluster config if available)")
	kubeContextFlag := flag.String("kube-context", envKubeContext, "Kubeconfig context (optional)")
	kubeIncludeAllPodsFlag := flag.Bool("kube-include-all-pods", utils.ParseBool(envKubeIncludeAllPods), "Include all non-succeeded pods (may be large)")
	kubeIncludeAllDeploymentsFlag := flag.Bool("kube-include-all-deployments", utils.ParseBool(envKubeIncludeAllDeployments), "Include all deployments, not just problem ones")
	kubeMaxPodsFlag := flag.Int("kube-max-pods", defaultInt(envKubeMaxPods, 200), "Max pods included in report")
	reportIPFlag := flag.String("report-ip", envReportIP, "IP address to report (for multi-NIC systems)")
	showVersion := flag.Bool("version", false, "Print the agent version and exit")
	selfTest := flag.Bool("self-test", false, "Perform self-test and exit (used during auto-update)")

	var tagFlags multiValue
	flag.Var(&tagFlags, "tag", "Tag to apply (repeatable)")
	var kubeIncludeNamespaceFlags multiValue
	flag.Var(&kubeIncludeNamespaceFlags, "kube-include-namespace", "Namespace to include (repeatable; default is all)")
	var kubeExcludeNamespaceFlags multiValue
	flag.Var(&kubeExcludeNamespaceFlags, "kube-exclude-namespace", "Namespace to exclude (repeatable)")
	var diskExcludeFlags multiValue
	flag.Var(&diskExcludeFlags, "disk-exclude", "Mount point or path prefix to exclude from disk monitoring (repeatable)")

	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	// Validation
	pulseURL := strings.TrimSpace(*urlFlag)
	if pulseURL == "" {
		pulseURL = "http://localhost:7655"
	}

	// Resolve token with priority: --token > --token-file > env > default file
	token := resolveToken(*tokenFlag, *tokenFileFlag, envToken)
	if token == "" {
		fmt.Fprintln(os.Stderr, "error: Pulse API token is required (use --token, --token-file, PULSE_TOKEN env, or /var/lib/pulse-agent/token)")
		os.Exit(1)
	}

	logLevel, err := parseLogLevel(*logLevelFlag)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	tags := gatherTags(envTags, tagFlags)
	kubeIncludeNamespaces := gatherCSV(envKubeIncludeNamespaces, kubeIncludeNamespaceFlags)
	kubeExcludeNamespaces := gatherCSV(envKubeExcludeNamespaces, kubeExcludeNamespaceFlags)
	diskExclude := gatherCSV(envDiskExclude, diskExcludeFlags)

	// Check if Docker was explicitly configured via flag or env
	dockerConfigured := envEnableDocker != ""
	if !dockerConfigured {
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "enable-docker" {
				dockerConfigured = true
			}
		})
	}

	return Config{
		PulseURL:              pulseURL,
		APIToken:              token,
		Interval:              *intervalFlag,
		HostnameOverride:      strings.TrimSpace(*hostnameFlag),
		AgentID:               strings.TrimSpace(*agentIDFlag),
		Tags:                  tags,
		InsecureSkipVerify:    *insecureFlag,
		LogLevel:              logLevel,
		EnableHost:            *enableHostFlag,
		EnableDocker:          *enableDockerFlag,
		DockerConfigured:      dockerConfigured,
		EnableKubernetes:      *enableKubernetesFlag,
		EnableProxmox:         *enableProxmoxFlag,
		ProxmoxType:           strings.TrimSpace(*proxmoxTypeFlag),
		DisableAutoUpdate:     *disableAutoUpdateFlag,
		EnableCommands:        resolveEnableCommands(*enableCommandsFlag, *disableCommandsFlag, envEnableCommands, envDisableCommands),
		HealthAddr:            strings.TrimSpace(*healthAddrFlag),
		KubeconfigPath:        strings.TrimSpace(*kubeconfigFlag),
		KubeContext:           strings.TrimSpace(*kubeContextFlag),
		KubeIncludeNamespaces:     kubeIncludeNamespaces,
		KubeExcludeNamespaces:     kubeExcludeNamespaces,
		KubeIncludeAllPods:        *kubeIncludeAllPodsFlag,
		KubeIncludeAllDeployments: *kubeIncludeAllDeploymentsFlag,
		KubeMaxPods:               *kubeMaxPodsFlag,
		DiskExclude:               diskExclude,
		ReportIP:                  strings.TrimSpace(*reportIPFlag),
		SelfTest:                  *selfTest,
	}
}

func gatherTags(env string, flags []string) []string {
	tags := make([]string, 0)
	if env != "" {
		for _, tag := range strings.Split(env, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	for _, tag := range flags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func gatherCSV(env string, flags []string) []string {
	values := make([]string, 0)
	if env != "" {
		for _, value := range strings.Split(env, ",") {
			value = strings.TrimSpace(value)
			if value != "" {
				values = append(values, value)
			}
		}
	}
	for _, value := range flags {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func defaultInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseLogLevel(value string) (zerolog.Level, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return zerolog.InfoLevel, nil
	}
	return zerolog.ParseLevel(normalized)
}

func defaultLogLevel(envValue string) string {
	if strings.TrimSpace(envValue) == "" {
		return "info"
	}
	return envValue
}

// resolveEnableCommands determines whether command execution should be enabled.
// Priority: --enable-commands > --disable-commands (deprecated) > PULSE_ENABLE_COMMANDS > PULSE_DISABLE_COMMANDS (deprecated)
// Default: disabled (false) for security
func resolveEnableCommands(enableFlag, disableFlag bool, envEnable, envDisable string) bool {
	// If --enable-commands is explicitly set, use it
	if enableFlag {
		return true
	}

	// Backwards compat: if --disable-commands was used, log deprecation but respect it
	// (disableFlag being true means commands should be disabled, which is already the default)
	if disableFlag {
		fmt.Fprintln(os.Stderr, "warning: --disable-commands is deprecated and no longer needed (commands are disabled by default). Use --enable-commands to enable.")
		return false
	}

	// Check environment variables
	if envEnable != "" {
		return utils.ParseBool(envEnable)
	}

	// Backwards compat: PULSE_DISABLE_COMMANDS=true means commands disabled (already default)
	// PULSE_DISABLE_COMMANDS=false means commands enabled (backwards compat)
	if envDisable != "" {
		fmt.Fprintln(os.Stderr, "warning: PULSE_DISABLE_COMMANDS is deprecated. Use PULSE_ENABLE_COMMANDS=true to enable commands.")
		// Invert: DISABLE=false means enable
		return !utils.ParseBool(envDisable)
	}

	// Default: commands disabled
	return false
}

// resolveToken resolves the API token with priority:
// 1. --token flag (direct value)
// 2. --token-file flag (read from file)
// 3. PULSE_TOKEN environment variable
// 4. Default token file at /var/lib/pulse-agent/token
//
// Reading from a file is more secure than CLI args as tokens won't appear in `ps` output.
func resolveToken(tokenFlag, tokenFileFlag, envToken string) string {
	// 1. Direct token from --token flag
	if t := strings.TrimSpace(tokenFlag); t != "" {
		return t
	}

	// 2. Token from --token-file flag
	if tokenFileFlag != "" {
		if content, err := os.ReadFile(tokenFileFlag); err == nil {
			if t := strings.TrimSpace(string(content)); t != "" {
				return t
			}
		}
	}

	// 3. PULSE_TOKEN environment variable
	if t := strings.TrimSpace(envToken); t != "" {
		return t
	}

	// 4. Default token file (most secure method for systemd services)
	defaultTokenFile := "/var/lib/pulse-agent/token"
	if content, err := os.ReadFile(defaultTokenFile); err == nil {
		if t := strings.TrimSpace(string(content)); t != "" {
			return t
		}
	}

	return ""
}

// initDockerWithRetry attempts to initialize the Docker agent with exponential backoff.
// It returns the agent when Docker becomes available, or nil if the context is cancelled.
// Retry intervals: 5s, 10s, 20s, 40s, 80s, 160s, then cap at 5 minutes.
func initDockerWithRetry(ctx context.Context, cfg dockeragent.Config, logger *zerolog.Logger) *dockeragent.Agent {
	const (
		initialDelay = 5 * time.Second
		maxDelay     = 5 * time.Minute
		multiplier   = 2.0
	)

	delay := initialDelay
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Docker retry cancelled, context done")
			return nil
		default:
		}

		attempt++
		logger.Info().
			Int("attempt", attempt).
			Str("next_retry", delay.String()).
			Msg("Waiting to retry Docker connection")

		select {
		case <-ctx.Done():
			logger.Info().Msg("Docker retry cancelled during wait")
			return nil
		case <-time.After(delay):
		}

		agent, err := dockeragent.New(cfg)
		if err == nil {
			logger.Info().
				Int("attempts", attempt).
				Msg("Successfully connected to Docker after retry")
			return agent
		}

		logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Str("next_retry", (time.Duration(float64(delay) * multiplier)).String()).
			Msg("Docker still not available, will retry")

		// Calculate next delay with exponential backoff, capped at maxDelay
		delay = time.Duration(float64(delay) * multiplier)
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

// initKubernetesWithRetry attempts to initialize the Kubernetes agent with exponential backoff.
// It returns the agent when Kubernetes becomes available, or nil if the context is cancelled.
// Retry intervals: 5s, 10s, 20s, 40s, 80s, 160s, then cap at 5 minutes.
func initKubernetesWithRetry(ctx context.Context, cfg kubernetesagent.Config, logger *zerolog.Logger) *kubernetesagent.Agent {
	const (
		initialDelay = 5 * time.Second
		maxDelay     = 5 * time.Minute
		multiplier   = 2.0
	)

	delay := initialDelay
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Kubernetes retry cancelled, context done")
			return nil
		default:
		}

		attempt++
		logger.Info().
			Int("attempt", attempt).
			Str("next_retry", delay.String()).
			Msg("Waiting to retry Kubernetes connection")

		select {
		case <-ctx.Done():
			logger.Info().Msg("Kubernetes retry cancelled during wait")
			return nil
		case <-time.After(delay):
		}

		agent, err := kubernetesagent.New(cfg)
		if err == nil {
			logger.Info().
				Int("attempts", attempt).
				Msg("Successfully connected to Kubernetes after retry")
			return agent
		}

		logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Str("next_retry", (time.Duration(float64(delay) * multiplier)).String()).
			Msg("Kubernetes still not available, will retry")

		delay = time.Duration(float64(delay) * multiplier)
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}
