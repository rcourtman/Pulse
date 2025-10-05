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

	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rs/zerolog"
)

func main() {
	cfg := loadConfig()

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	cfg.Logger = &logger

	agent, err := dockeragent.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create docker agent")
	}
	defer agent.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info().Str("pulse_url", cfg.PulseURL).Dur("interval", cfg.Interval).Msg("Starting Pulse Docker agent")

	if err := agent.Run(ctx); err != nil && err != context.Canceled {
		logger.Fatal().Err(err).Msg("Agent terminated with error")
	}

	logger.Info().Msg("Agent stopped")
}

func loadConfig() dockeragent.Config {
	envURL := strings.TrimSpace(os.Getenv("PULSE_URL"))
	envToken := strings.TrimSpace(os.Getenv("PULSE_TOKEN"))
	envInterval := strings.TrimSpace(os.Getenv("PULSE_INTERVAL"))
	envHostname := strings.TrimSpace(os.Getenv("PULSE_HOSTNAME"))
	envAgentID := strings.TrimSpace(os.Getenv("PULSE_AGENT_ID"))
	envInsecure := strings.TrimSpace(os.Getenv("PULSE_INSECURE_SKIP_VERIFY"))

	defaultInterval := 30 * time.Second
	if envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval); err == nil {
			defaultInterval = parsed
		}
	}

	urlFlag := flag.String("url", envURL, "Pulse server URL (e.g. http://pulse:7655)")
	tokenFlag := flag.String("token", envToken, "Pulse API token (required)")
	intervalFlag := flag.Duration("interval", defaultInterval, "Reporting interval (e.g. 30s)")
	hostnameFlag := flag.String("hostname", envHostname, "Override hostname reported to Pulse")
	agentIDFlag := flag.String("agent-id", envAgentID, "Override agent identifier")
	insecureFlag := flag.Bool("insecure", parseBool(envInsecure), "Skip TLS certificate verification")

	flag.Parse()

	pulseURL := *urlFlag
	if pulseURL == "" {
		pulseURL = "http://localhost:7655"
	}

	token := strings.TrimSpace(*tokenFlag)
	if token == "" {
		fmt.Fprintln(os.Stderr, "error: PULSE_TOKEN or --token must be provided")
		flag.Usage()
		os.Exit(1)
	}

	interval := *intervalFlag
	if interval <= 0 {
		interval = 30 * time.Second
	}

	return dockeragent.Config{
		PulseURL:           pulseURL,
		APIToken:           token,
		Interval:           interval,
		HostnameOverride:   strings.TrimSpace(*hostnameFlag),
		AgentID:            strings.TrimSpace(*agentIDFlag),
		InsecureSkipVerify: *insecureFlag,
	}
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
