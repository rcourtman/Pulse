package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionrunner"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rs/zerolog"
)

var version = "dev"

type runtimeConfig struct {
	PulseURL          string
	TokenFile         string
	StateDir          string
	HealthFile        string
	AgentIDFile       string
	Hostname          string
	ServerFingerprint string
	CAFile            string
	Insecure          bool
}

func loadConfig() (runtimeConfig, error) {
	config := runtimeConfig{
		PulseURL:          strings.TrimSpace(os.Getenv("PULSE_URL")),
		TokenFile:         strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_TOKEN_FILE")),
		StateDir:          strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_STATE_DIR")),
		HealthFile:        strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_HEALTH_FILE")),
		AgentIDFile:       strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_AGENT_ID_FILE")),
		Hostname:          strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_HOSTNAME")),
		ServerFingerprint: strings.TrimSpace(os.Getenv("PULSE_SERVER_FINGERPRINT")),
		CAFile:            strings.TrimSpace(os.Getenv("SSL_CERT_FILE")),
		Insecure:          strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_INSECURE")), "true"),
	}
	if config.PulseURL == "" || config.TokenFile == "" || config.StateDir == "" || config.HealthFile == "" || config.AgentIDFile == "" {
		return runtimeConfig{}, errors.New("PULSE_URL and the action-runner token, state, health, and agent identity file settings are required")
	}
	if config.Hostname != "" {
		hostname, err := normalizeRunnerHostname(config.Hostname)
		if err != nil {
			return runtimeConfig{}, err
		}
		config.Hostname = hostname
	}
	parsed, err := url.Parse(config.PulseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !(config.Insecure && parsed.Scheme == "http")) {
		return runtimeConfig{}, errors.New("PULSE_URL must be HTTPS unless PULSE_INSECURE=true explicitly permits HTTP")
	}
	if _, err := readPrivateValue(config.TokenFile, "runner token"); err != nil {
		return runtimeConfig{}, err
	}
	if err := os.MkdirAll(config.StateDir, 0700); err != nil {
		return runtimeConfig{}, fmt.Errorf("runner state directory: %w", err)
	}
	resolvedState, err := filepath.Abs(config.StateDir)
	if err != nil {
		return runtimeConfig{}, err
	}
	config.StateDir = resolvedState
	resolvedHealth, err := filepath.Abs(config.HealthFile)
	if err != nil {
		return runtimeConfig{}, err
	}
	if filepath.Dir(resolvedHealth) != resolvedState || filepath.Base(resolvedHealth) != "health.json" {
		return runtimeConfig{}, errors.New("action-runner health file must be health.json inside PULSE_AGENT_RUNNER_STATE_DIR")
	}
	config.HealthFile = resolvedHealth
	return config, nil
}

func run() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	token, err := readPrivateValue(config.TokenFile, "runner token")
	if err != nil {
		return err
	}
	agentID, err := readPrivateValue(config.AgentIDFile, "runner agent identity")
	if err != nil {
		return err
	}
	hostname := config.Hostname
	if hostname == "" {
		hostname, err = os.Hostname()
		hostname = strings.TrimSpace(hostname)
		if err != nil || hostname == "" {
			return errors.New("determine action-runner hostname")
		}
	}
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("component", "action-runner").Logger()
	transportConfig := actionrunner.TransportConfig{
		PulseURL: config.PulseURL, APIToken: token, StateDir: config.StateDir,
		HealthPath: config.HealthFile, InsecureSkipVerify: config.Insecure,
		CACertPath: config.CAFile, ServerFingerprint: config.ServerFingerprint,
		Logger: &logger,
	}
	containerRuntime, runtimeErr := dockeragent.NewActionRuntime(strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_CONTAINER_RUNTIME")), &logger)
	if runtimeErr != nil {
		logger.Warn().Err(runtimeErr).Msg("Docker/Podman action capability unavailable")
	} else {
		defer containerRuntime.Close()
		transportConfig.DockerContainerLifecycleOperator = containerRuntime
		transportConfig.DockerContainerUpdater = containerRuntime
	}
	client := actionrunner.NewClient(transportConfig, agentID, hostname, version+"-"+runtime.GOOS+"-"+runtime.GOARCH)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	defer client.Close()
	if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func normalizeRunnerHostname(value string) (string, error) {
	hostname := strings.ToLower(strings.TrimSpace(value))
	hostname = strings.TrimRight(hostname, ".")
	if hostname == "" || len(hostname) > 253 {
		return "", errors.New("PULSE_AGENT_RUNNER_HOSTNAME must be a valid hostname or IP address")
	}
	if net.ParseIP(hostname) != nil {
		return hostname, nil
	}
	for _, label := range strings.Split(hostname, ".") {
		if len(label) == 0 || len(label) > 63 || !runnerHostnameLabelByte(label[0]) || !runnerHostnameLabelByte(label[len(label)-1]) {
			return "", errors.New("PULSE_AGENT_RUNNER_HOSTNAME must be a valid hostname or IP address")
		}
		for index := 1; index < len(label)-1; index++ {
			if !runnerHostnameLabelByte(label[index]) && label[index] != '-' {
				return "", errors.New("PULSE_AGENT_RUNNER_HOSTNAME must be a valid hostname or IP address")
			}
		}
	}
	return hostname, nil
}

func runnerHostnameLabelByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func readPrivateValue(path, label string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("%s file: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("%s file must be regular and inaccessible to group/other", label)
	}
	value, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(value)) == "" {
		return "", fmt.Errorf("%s file is unreadable or empty", label)
	}
	return strings.TrimSpace(string(value)), nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
