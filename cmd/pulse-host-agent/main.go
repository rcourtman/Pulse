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

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
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
	envURL := strings.TrimSpace(os.Getenv("PULSE_URL"))
	envToken := strings.TrimSpace(os.Getenv("PULSE_TOKEN"))
	envInterval := strings.TrimSpace(os.Getenv("PULSE_INTERVAL"))
	envHostname := strings.TrimSpace(os.Getenv("PULSE_HOSTNAME"))
	envAgentID := strings.TrimSpace(os.Getenv("PULSE_AGENT_ID"))
	envInsecure := strings.TrimSpace(os.Getenv("PULSE_INSECURE_SKIP_VERIFY"))
	envTags := strings.TrimSpace(os.Getenv("PULSE_TAGS"))
	envRunOnce := strings.TrimSpace(os.Getenv("PULSE_ONCE"))

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
	insecureFlag := flag.Bool("insecure", parseBool(envInsecure), "Skip TLS certificate verification")
	runOnceFlag := flag.Bool("once", parseBool(envRunOnce), "Collect and send a single report, then exit")
	showVersion := flag.Bool("version", false, "Print the agent version and exit")

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

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
