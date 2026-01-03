package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

var (
	// Version is the semantic version of the agent, set at build time via ldflags
	Version = "dev"

	osExit = os.Exit

	runAsWindowsServiceFunc = runAsWindowsService

	runFunc = run
)

// Config holds the configuration for the standalone host agent
type Config struct {
	HostConfig        hostagent.Config
	DisableAutoUpdate bool
}

type multiValue []string

func (m *multiValue) String() string {
	return strings.Join(*m, ",")
}

func (m *multiValue) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	cfg, showVersion, err := parseConfig(os.Args[0], os.Args[1:], os.Getenv)
	if err != nil {
		if err == flag.ErrHelp {
			osExit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		osExit(1)
	}

	if showVersion {
		fmt.Println(Version)
		osExit(0)
	}

	if err := runFunc(context.Background(), cfg); err != nil {
		// Log error and exit - logger is set up in run() but we might not have it here
		// Actually, run() handles its own fatal errors for now to match original behavior
		// but we return error for testing.
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		osExit(1)
	}
}

func run(ctx context.Context, cfg Config) error {
	hostCfg := cfg.HostConfig

	zerolog.SetGlobalLevel(hostCfg.LogLevel)

	logger := zerolog.New(os.Stdout).Level(hostCfg.LogLevel).With().Timestamp().Logger()
	hostCfg.Logger = &logger

	// Check if we should run as a Windows service
	if err := runAsWindowsServiceFunc(cfg, logger); err != nil {
		return fmt.Errorf("Windows service failed: %w", err)
	}

	// If runAsWindowsService returns nil without error, we're not running as a service
	// or we're on a non-Windows platform, so run normally

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Deprecation warning
	logger.Warn().Msg("pulse-host-agent is DEPRECATED and will be removed in a future release")
	logger.Warn().Msg("Please migrate to the unified 'pulse-agent' with --enable-host flag")
	logger.Warn().Msg("Example: pulse-agent --url <URL> --token <TOKEN> --enable-host")
	logger.Warn().Msg("")

	logger.Info().
		Str("version", Version).
		Str("pulse_url", hostCfg.PulseURL).
		Str("agent_id", hostCfg.AgentID).
		Dur("interval", hostCfg.Interval).
		Bool("auto_update", !cfg.DisableAutoUpdate).
		Msg("Starting Pulse host agent")

	// Start Auto-Updater
	updater := agentupdate.New(agentupdate.Config{
		PulseURL:           hostCfg.PulseURL,
		APIToken:           hostCfg.APIToken,
		AgentName:          "pulse-host-agent",
		CurrentVersion:     Version,
		CheckInterval:      1 * time.Hour,
		InsecureSkipVerify: hostCfg.InsecureSkipVerify,
		Logger:             &logger,
		Disabled:           cfg.DisableAutoUpdate,
	})

	g.Go(func() error {
		updater.RunLoop(ctx)
		return nil
	})

	// Start the host agent
	agent, err := hostagent.New(hostCfg)
	if err != nil {
		return fmt.Errorf("failed to initialise host agent: %w", err)
	}

	g.Go(func() error {
		return agent.Run(ctx)
	})

	if err := g.Wait(); err != nil && err != context.Canceled {
		return fmt.Errorf("host agent terminated with error: %w", err)
	}

	logger.Info().Msg("Host agent stopped")
	return nil
}

func parseConfig(progName string, args []string, getenv func(string) string) (Config, bool, error) {
	getenvTrim := func(k string) string {
		return strings.TrimSpace(getenv(k))
	}

	envURL := getenvTrim("PULSE_URL")
	envToken := getenvTrim("PULSE_TOKEN")
	envInterval := getenvTrim("PULSE_INTERVAL")
	envHostname := getenvTrim("PULSE_HOSTNAME")
	envAgentID := getenvTrim("PULSE_AGENT_ID")
	envInsecure := getenvTrim("PULSE_INSECURE_SKIP_VERIFY")
	envTags := getenvTrim("PULSE_TAGS")
	envRunOnce := getenvTrim("PULSE_ONCE")
	envLogLevel := getenvTrim("LOG_LEVEL")
	envNoAutoUpdate := getenvTrim("PULSE_NO_AUTO_UPDATE")

	defaultInterval := 30 * time.Second
	if envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval); err == nil {
			defaultInterval = parsed
		}
	}

	fs := flag.NewFlagSet(progName, flag.ContinueOnError)

	urlFlag := fs.String("url", envURL, "Pulse server URL (e.g. https://pulse.example.com)")
	tokenFlag := fs.String("token", envToken, "Pulse API token (required)")
	intervalFlag := fs.Duration("interval", defaultInterval, "Reporting interval (e.g. 30s, 1m)")
	hostnameFlag := fs.String("hostname", envHostname, "Override hostname reported to Pulse")
	agentIDFlag := fs.String("agent-id", envAgentID, "Override agent identifier")
	insecureFlag := fs.Bool("insecure", utils.ParseBool(envInsecure), "Skip TLS certificate verification")
	runOnceFlag := fs.Bool("once", utils.ParseBool(envRunOnce), "Collect and send a single report, then exit")
	noAutoUpdateFlag := fs.Bool("no-auto-update", utils.ParseBool(envNoAutoUpdate), "Disable automatic updates")
	showVersion := fs.Bool("version", false, "Print the agent version and exit")
	logLevelFlag := fs.String("log-level", defaultLogLevel(envLogLevel), "Log level: debug, info, warn, error")

	var tagFlags multiValue
	fs.Var(&tagFlags, "tag", "Tag to apply to this host (repeatable)")

	if err := fs.Parse(args); err != nil {
		return Config{}, false, err
	}

	if *showVersion {
		return Config{}, true, nil
	}

	pulseURL := strings.TrimSpace(*urlFlag)
	if pulseURL == "" {
		pulseURL = "http://localhost:7655"
	}

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		return Config{}, false, fmt.Errorf("Pulse API token is required (via --token or PULSE_TOKEN)")
	}

	interval := *intervalFlag
	if interval <= 0 {
		interval = 30 * time.Second
	}

	logLevel, err := parseLogLevel(*logLevelFlag)
	if err != nil {
		return Config{}, false, err
	}

	tags := gatherTags(envTags, tagFlags)

	return Config{
		HostConfig: hostagent.Config{
			PulseURL:           pulseURL,
			APIToken:           token,
			Interval:           interval,
			HostnameOverride:   strings.TrimSpace(*hostnameFlag),
			AgentID:            strings.TrimSpace(*agentIDFlag),
			Tags:               tags,
			InsecureSkipVerify: *insecureFlag,
			RunOnce:            *runOnceFlag,
			LogLevel:           logLevel,
		},
		DisableAutoUpdate: *noAutoUpdateFlag,
	}, false, nil
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

func parseLogLevel(value string) (zerolog.Level, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return zerolog.InfoLevel, nil
	}

	level, err := zerolog.ParseLevel(normalized)
	if err != nil {
		return zerolog.InfoLevel, fmt.Errorf("invalid log level %q: must be debug, info, warn, or error", value)
	}
	if level < zerolog.DebugLevel || level > zerolog.ErrorLevel {
		return zerolog.InfoLevel, fmt.Errorf("invalid log level %q: must be debug, info, warn, or error", value)
	}

	return level, nil
}

func defaultLogLevel(envValue string) string {
	if strings.TrimSpace(envValue) == "" {
		return "info"
	}
	return envValue
}
