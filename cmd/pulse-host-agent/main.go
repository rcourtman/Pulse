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

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog"
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
	cfg := loadConfig()

	zerolog.SetGlobalLevel(cfg.LogLevel)

	logger := zerolog.New(os.Stdout).Level(cfg.LogLevel).With().Timestamp().Logger()
	cfg.Logger = &logger

	// Check if we should run as a Windows service
	if err := runAsWindowsService(cfg, logger); err != nil {
		logger.Fatal().Err(err).Msg("Windows service failed")
	}

	// If runAsWindowsService returns nil without error, we're not running as a service
	// or we're on a non-Windows platform, so run normally
	agent, err := hostagent.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialise host agent")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info().
		Str("pulse_url", cfg.PulseURL).
		Str("agent_id", cfg.AgentID).
		Dur("interval", cfg.Interval).
		Msg("Starting Pulse host agent")

	if err := agent.Run(ctx); err != nil && err != context.Canceled {
		logger.Fatal().Err(err).Msg("host agent terminated with error")
	}

	logger.Info().Msg("Host agent stopped")
}

func loadConfig() hostagent.Config {
	envURL := utils.GetenvTrim("PULSE_URL")
	envToken := utils.GetenvTrim("PULSE_TOKEN")
	envInterval := utils.GetenvTrim("PULSE_INTERVAL")
	envHostname := utils.GetenvTrim("PULSE_HOSTNAME")
	envAgentID := utils.GetenvTrim("PULSE_AGENT_ID")
	envInsecure := utils.GetenvTrim("PULSE_INSECURE_SKIP_VERIFY")
	envTags := utils.GetenvTrim("PULSE_TAGS")
	envRunOnce := utils.GetenvTrim("PULSE_ONCE")
	envLogLevel := utils.GetenvTrim("LOG_LEVEL")

	defaultInterval := 30 * time.Second
	if envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval); err == nil {
			defaultInterval = parsed
		}
	}

	urlFlag := flag.String("url", envURL, "Pulse server URL (e.g. https://pulse.example.com)")
	tokenFlag := flag.String("token", envToken, "Pulse API token (required)")
	intervalFlag := flag.Duration("interval", defaultInterval, "Reporting interval (e.g. 30s, 1m)")
	hostnameFlag := flag.String("hostname", envHostname, "Override hostname reported to Pulse")
	agentIDFlag := flag.String("agent-id", envAgentID, "Override agent identifier")
	insecureFlag := flag.Bool("insecure", utils.ParseBool(envInsecure), "Skip TLS certificate verification")
	runOnceFlag := flag.Bool("once", utils.ParseBool(envRunOnce), "Collect and send a single report, then exit")
	showVersion := flag.Bool("version", false, "Print the agent version and exit")
	logLevelFlag := flag.String("log-level", defaultLogLevel(envLogLevel), "Log level: debug, info, warn, error")

	var tagFlags multiValue
	flag.Var(&tagFlags, "tag", "Tag to apply to this host (repeatable)")

	flag.Parse()

	if *showVersion {
		fmt.Println(hostagent.Version)
		os.Exit(0)
	}

	pulseURL := strings.TrimSpace(*urlFlag)
	if pulseURL == "" {
		pulseURL = "http://localhost:7655"
	}

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		fmt.Fprintln(os.Stderr, "error: Pulse API token is required (via --token or PULSE_TOKEN)")
		os.Exit(1)
	}

	interval := *intervalFlag
	if interval <= 0 {
		interval = 30 * time.Second
	}

	logLevel, err := parseLogLevel(*logLevelFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	tags := gatherTags(envTags, tagFlags)

	return hostagent.Config{
		PulseURL:           pulseURL,
		APIToken:           token,
		Interval:           interval,
		HostnameOverride:   strings.TrimSpace(*hostnameFlag),
		AgentID:            strings.TrimSpace(*agentIDFlag),
		Tags:               tags,
		InsecureSkipVerify: *insecureFlag,
		RunOnce:            *runOnceFlag,
		LogLevel:           logLevel,
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
