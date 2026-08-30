package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionrunner"
	"github.com/rcourtman/pulse-go-rewrite/internal/collectorlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog"
)

var version = "dev"

type runtimeConfig struct {
	PulseURL          string
	TokenFile         string
	StateDir          string
	HealthFile        string
	ActivationNonce   string
	AgentID           string
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
		ActivationNonce:   strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE")),
		AgentID:           strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_AGENT_ID")),
		AgentIDFile:       strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_AGENT_ID_FILE")),
		Hostname:          strings.TrimSpace(os.Getenv("PULSE_AGENT_RUNNER_HOSTNAME")),
		ServerFingerprint: strings.TrimSpace(os.Getenv("PULSE_SERVER_FINGERPRINT")),
		CAFile:            strings.TrimSpace(os.Getenv("SSL_CERT_FILE")),
		Insecure:          strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_INSECURE")), "true"),
	}
	if config.PulseURL == "" || config.TokenFile == "" || config.StateDir == "" || config.HealthFile == "" || (config.AgentID == "" && config.AgentIDFile == "") || len(config.ActivationNonce) < 32 || len(config.ActivationNonce) > 128 {
		return runtimeConfig{}, errors.New("PULSE_URL and the action-runner token, state, health, agent identity, and activation nonce settings are required")
	}
	if config.AgentID != "" && !actionrunner.IsValidBoundedID(config.AgentID) {
		return runtimeConfig{}, errors.New("PULSE_AGENT_RUNNER_AGENT_ID must be a valid agent identity")
	}
	if config.Hostname != "" {
		hostname, err := normalizeRunnerHostname(config.Hostname)
		if err != nil {
			return runtimeConfig{}, err
		}
		config.Hostname = hostname
	}
	if err := actionrunner.ValidatePulseURL(config.PulseURL); err != nil {
		return runtimeConfig{}, fmt.Errorf("PULSE_URL must use HTTPS or an exact literal loopback host: %w", err)
	}
	normalizedURL, err := securityutil.NormalizePulseHTTPBaseURL(config.PulseURL)
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("PULSE_URL must use HTTPS except for loopback local use: %w", err)
	}
	if normalizedURL.Scheme == "https" && config.Insecure && config.ServerFingerprint == "" {
		if config.CAFile == "" {
			return runtimeConfig{}, errors.New("generic insecure HTTPS is forbidden for the action runner; configure a trusted CA or exact server fingerprint")
		}
		config.Insecure = false
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
	agentID, err := resolveRunnerAgentID(config)
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
		ActivationNonce: config.ActivationNonce,
		CACertPath:      config.CAFile, ServerFingerprint: config.ServerFingerprint,
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

func runCredentialLifecycleCommand(command string, args []string) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	pulseURL := flags.String("url", "", "Pulse server URL")
	tokenFile := flags.String("token-file", "", "private runner token file")
	agentID := flags.String("agent-id", "", "bound runner agent identity")
	hostname := flags.String("hostname", "", "bound runner hostname")
	caFile := flags.String("cacert", "", "custom CA certificate")
	serverFingerprint := flags.String("server-fingerprint", "", "pinned server certificate fingerprint")
	insecureLoopback := flags.Bool("insecure-loopback", false, "allow plaintext only for a loopback Pulse URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*pulseURL) == "" || strings.TrimSpace(*tokenFile) == "" {
		return errors.New("lifecycle command requires --url and --token-file")
	}
	normalizedHostname := ""
	if command == "revoke-credential" {
		if !actionrunner.IsValidBoundedID(strings.TrimSpace(*agentID)) {
			return errors.New("revoke command requires --agent-id and --hostname")
		}
		var err error
		normalizedHostname, err = normalizeRunnerHostname(*hostname)
		if err != nil {
			return err
		}
	}
	normalizedURL, err := securityutil.NormalizePulseHTTPBaseURL(*pulseURL)
	if err != nil {
		return fmt.Errorf("action-runner lifecycle URL: %w", err)
	}
	if *insecureLoopback && normalizedURL.Scheme != "http" {
		return errors.New("--insecure-loopback is valid only for a loopback HTTP URL")
	}
	token, err := readPrivateValue(*tokenFile, "runner token")
	if err != nil {
		return err
	}
	config := actionrunner.CredentialLifecycleConfig{
		PulseURL: normalizedURL.String(), APIToken: token,
		InsecureSkipVerify: *insecureLoopback, CACertPath: strings.TrimSpace(*caFile),
		ServerFingerprint: strings.TrimSpace(*serverFingerprint),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	switch command {
	case "cancel-pending-credential":
		return actionrunner.CancelPendingCredential(ctx, config)
	case "revoke-credential":
		return actionrunner.RevokeCredential(ctx, config, strings.TrimSpace(*agentID), normalizedHostname)
	default:
		return fmt.Errorf("unknown action-runner command %q", command)
	}
}

func resolveRunnerAgentID(config runtimeConfig) (string, error) {
	if config.AgentID != "" {
		return config.AgentID, nil
	}
	agentID, err := collectorlifecycle.ReadAgentIDFile(config.AgentIDFile, dedicatedCollectorUID())
	if err != nil {
		return "", err
	}
	if !actionrunner.IsValidBoundedID(agentID) {
		return "", errors.New("runner agent identity file must contain a valid agent identity")
	}
	return agentID, nil
}

func dedicatedCollectorUID() *uint64 {
	account, err := user.Lookup("pulse-agent")
	if err != nil || account == nil {
		return nil
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		return nil
	}
	return &uid
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
	value, err := collectorlifecycle.ReadPrivateValueFile(path, nil)
	if err != nil {
		return "", fmt.Errorf("%s file: %w", label, err)
	}
	return value, nil
}

func main() {
	var err error
	if len(os.Args) > 1 && (os.Args[1] == "cancel-pending-credential" || os.Args[1] == "revoke-credential") {
		err = runCredentialLifecycleCommand(os.Args[1], os.Args[2:])
	} else {
		err = run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
