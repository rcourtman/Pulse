package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog"
)

const ActionRunnerRuntimeRole = agentexec.RuntimeRoleActionRunner

type ActionRunnerClientConfig struct {
	PulseURL                         string
	APIToken                         string
	StateDir                         string
	HealthPath                       string
	ActivationNonce                  string
	InsecureSkipVerify               bool
	CACertPath                       string
	ServerFingerprint                string
	Logger                           *zerolog.Logger
	DockerContainerUpdater           DockerContainerUpdater
	DockerContainerLifecycleOperator DockerContainerLifecycleOperator
}

// ActionRunnerCredentialLifecycleConfig carries the exact non-browser inputs
// used by installer recovery and uninstall. The bearer is read from a private
// file by the runner command and is never passed in argv or to curl.
type ActionRunnerCredentialLifecycleConfig struct {
	PulseURL           string
	APIToken           string
	InsecureSkipVerify bool
	CACertPath         string
	ServerFingerprint  string
}

func normalizeActionRunnerHTTPBaseURL(raw string) (*url.URL, error) {
	parsed, err := securityutil.NormalizePulseHTTPBaseURL(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" {
		return parsed, nil
	}
	hostname := strings.TrimSpace(strings.ToLower(parsed.Hostname()))
	if hostname == "localhost" {
		return parsed, nil
	}
	ip := net.ParseIP(hostname)
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("plaintext action-runner URL requires a literal loopback host")
	}
	return parsed, nil
}

// ValidateActionRunnerPulseURL enforces the runner's stricter plaintext
// boundary. Unlike generic URL handling, *.localhost is not accepted because
// a resolver can direct that name away from the local host.
func ValidateActionRunnerPulseURL(raw string) error {
	_, err := normalizeActionRunnerHTTPBaseURL(raw)
	return err
}

func effectiveActionRunnerInsecureMode(pulseURL string, insecure bool, caCertPath, serverFingerprint string) (bool, error) {
	parsed, err := normalizeActionRunnerHTTPBaseURL(pulseURL)
	if err != nil {
		return false, err
	}
	if parsed.Scheme != "https" || !insecure {
		return insecure, nil
	}
	if strings.TrimSpace(serverFingerprint) != "" {
		return true, nil // agenttls replaces insecure verification with the exact DER pin.
	}
	if strings.TrimSpace(caCertPath) != "" {
		return false, nil // custom CA verification must remain enabled.
	}
	return false, fmt.Errorf("generic insecure HTTPS is forbidden for the action runner")
}

// newActionRunnerHTTPClient deliberately bypasses ambient proxy variables.
// Runner bearers are host-bound control-plane credentials and must never be
// disclosed to an operator-unconfigured HTTP_PROXY intermediary.
func newActionRunnerHTTPClient(caCertPath string, insecureSkipVerify bool, serverFingerprint string) (*http.Client, error) {
	client, err := newAgentHTTPClient(caCertPath, insecureSkipVerify, serverFingerprint)
	if err != nil {
		return nil, err
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		return nil, fmt.Errorf("action-runner HTTP transport is unavailable")
	}
	direct := transport.Clone()
	direct.Proxy = nil
	client.Transport = direct
	return client, nil
}

func performActionRunnerCredentialLifecycle(ctx context.Context, config ActionRunnerCredentialLifecycleConfig, method, path, agentID, hostname string, includeBindingBody bool) error {
	baseURL, err := normalizeActionRunnerHTTPBaseURL(config.PulseURL)
	if err != nil {
		return fmt.Errorf("validate action-runner lifecycle URL: %w", err)
	}
	effectiveInsecure, err := effectiveActionRunnerInsecureMode(config.PulseURL, config.InsecureSkipVerify, config.CACertPath, config.ServerFingerprint)
	if err != nil {
		return fmt.Errorf("validate action-runner lifecycle TLS mode: %w", err)
	}
	token := strings.TrimSpace(config.APIToken)
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("action-runner lifecycle bearer is invalid")
	}
	var body io.Reader
	if includeBindingBody {
		encoded, err := json.Marshal(map[string]string{"agentId": strings.TrimSpace(agentID), "hostname": strings.TrimSpace(hostname)})
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL.String(), "/")+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if includeBindingBody {
		request.Header.Set("Content-Type", "application/json")
	}
	client, err := newActionRunnerHTTPClient(config.CACertPath, effectiveInsecure, config.ServerFingerprint)
	if err != nil {
		return fmt.Errorf("build action-runner lifecycle client: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("action-runner lifecycle request: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("action-runner lifecycle request: server returned %s", response.Status)
	}
	return nil
}

// CancelPendingActionRunnerCredential durably cancels a prepared replacement.
// Only a nil result (HTTP 204) is safe authority to restore a predecessor.
func CancelPendingActionRunnerCredential(ctx context.Context, config ActionRunnerCredentialLifecycleConfig) error {
	return performActionRunnerCredentialLifecycle(ctx, config, http.MethodDelete, "/api/agents/action-runner/credential/activation", "", "", false)
}

// RevokeActionRunnerCredential removes the exact active runner credential.
func RevokeActionRunnerCredential(ctx context.Context, config ActionRunnerCredentialLifecycleConfig, agentID, hostname string) error {
	return performActionRunnerCredentialLifecycle(ctx, config, http.MethodDelete, "/api/agents/action-runner/credential", agentID, hostname, true)
}

// NewActionRunnerClient constructs the separately credentialed, typed-only
// action transport. It does not create a collector, reporter, deploy client,
// arbitrary command executor, or unrestricted file reader.
func NewActionRunnerClient(config ActionRunnerClientConfig, agentID, hostname, version string) *CommandClient {
	logger := config.Logger
	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
	}
	lease := newPackageManagerLease()
	client := NewCommandClient(Config{
		PulseURL: config.PulseURL, APIToken: config.APIToken, StateDir: config.StateDir,
		InsecureSkipVerify: config.InsecureSkipVerify, CACertPath: config.CACertPath,
		ServerFingerprint: config.ServerFingerprint, Logger: logger,
		packageUpdates:                   newPackageUpdateManager(runtime.GOOS, lease),
		storageCleanup:                   newStorageCleanupManager(runtime.GOOS, lease),
		DockerContainerUpdater:           config.DockerContainerUpdater,
		DockerContainerLifecycleOperator: config.DockerContainerLifecycleOperator,
	}, agentID, hostname, runtime.GOOS, version)
	client.actionRunnerOnly = true
	client.runtimeRole = agentexec.RuntimeRoleActionRunner
	client.actionCapability = agentexec.ActionCapabilityTypedV1
	if config.DockerContainerLifecycleOperator == nil {
		// The privileged runner never falls back to a Docker/Podman CLI. A
		// daemon may already have accepted a mutation after its CLI exits, so
		// process containment cannot establish the operation's terminal state.
		client.dockerLifecycle = nil
	}
	client.healthPath = strings.TrimSpace(config.HealthPath)
	client.actionActivationNonce = strings.TrimSpace(config.ActivationNonce)
	client.healthCapabilities = []string{
		"host.storage_cleanup.v1",
		"host.update.v1",
		"proxmox.guest.lifecycle.v1",
		agentexec.ActionCapabilityTypedV1,
	}
	if config.DockerContainerUpdater != nil && config.DockerContainerLifecycleOperator != nil {
		client.healthCapabilities = append(client.healthCapabilities, "container.lifecycle.v1", "container.update.v1")
	}
	return client
}

func allowedActionRunnerMessage(message messageType) bool {
	switch message {
	case msgTypePong,
		msgTypeHostStorageCleanup, msgTypeHostUpdate, msgTypeProxmoxGuestLifecycle,
		msgTypeDockerContainerLifecycle, msgTypeDockerContainerUpdate,
		msgTypeDockerContainerObserve, msgTypeActionPreflight,
		msgTypeOperationQuery, msgTypeCancelCmd:
		return true
	default:
		return false
	}
}

type actionRunnerHealth struct {
	Registered      bool      `json:"registered"`
	Activated       bool      `json:"activated"`
	ActivationNonce string    `json:"activation_nonce"`
	RuntimeRole     string    `json:"runtime_role"`
	Server          string    `json:"server"`
	HostID          string    `json:"host_id"`
	Hostname        string    `json:"hostname"`
	Capabilities    []string  `json:"capabilities"`
	RegisteredAt    time.Time `json:"registered_at"`
}

func (c *CommandClient) persistActionRunnerHealth(activated bool) error {
	if c != nil && c.actionHealthWriter != nil {
		return c.actionHealthWriter(activated)
	}
	return c.writeActionRunnerHealth(activated)
}

func (c *CommandClient) writeActionRunnerHealth(activated bool) error {
	if c == nil || !c.actionRunnerOnly || strings.TrimSpace(c.healthPath) == "" {
		return fmt.Errorf("action-runner health path is required")
	}
	if nonce := strings.TrimSpace(c.actionActivationNonce); len(nonce) < 32 || len(nonce) > 128 {
		return fmt.Errorf("action-runner activation nonce is invalid")
	}
	capabilities := append([]string(nil), c.healthCapabilities...)
	sort.Strings(capabilities)
	health := actionRunnerHealth{
		Registered: true, Activated: activated, ActivationNonce: c.actionActivationNonce,
		RuntimeRole: agentexec.RuntimeRoleActionRunner,
		Server:      c.pulseURL, HostID: c.agentID, Hostname: c.hostname,
		Capabilities: capabilities, RegisteredAt: time.Now().UTC(),
	}
	encoded, err := json.Marshal(health)
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.healthPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := securityutil.HardenPrivatePath(dir, 0o700); err != nil {
		return fmt.Errorf("harden action-runner health directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".health-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := securityutil.HardenPrivatePath(tempPath, 0o600); err != nil {
		temp.Close()
		return fmt.Errorf("harden action-runner health marker: %w", err)
	}
	if _, err := temp.Write(encoded); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := replaceActionRunnerHealthFile(tempPath, c.healthPath); err != nil {
		return err
	}
	info, err := os.Lstat(c.healthPath)
	if err != nil {
		return fmt.Errorf("inspect replaced action-runner health marker: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("action-runner health marker is not a real regular file")
	}
	if err := securityutil.ValidatePrivatePath(c.healthPath, info); err != nil {
		return fmt.Errorf("validate private action-runner health marker: %w", err)
	}
	return syncActionRunnerHealthDirectory(dir)
}

func (c *CommandClient) activateActionRunnerCredential(ctx context.Context) error {
	if c == nil || !c.actionRunnerOnly {
		return fmt.Errorf("action-runner activation requires the typed runner")
	}
	body, err := json.Marshal(map[string]string{"agentId": c.agentID, "hostname": c.hostname})
	if err != nil {
		return err
	}
	baseURL, err := securityutil.NormalizePulseHTTPBaseURL(c.pulseURL)
	if err != nil {
		return fmt.Errorf("validate action-runner activation URL: %w", err)
	}
	effectiveInsecure, err := effectiveActionRunnerInsecureMode(c.pulseURL, c.insecureSkipVerify, c.caCertPath, c.serverFingerprint)
	if err != nil {
		return fmt.Errorf("validate action-runner activation TLS mode: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, strings.TrimRight(baseURL.String(), "/")+"/api/agents/action-runner/credential", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiToken)
	request.Header.Set("Content-Type", "application/json")
	client, err := newActionRunnerHTTPClient(c.caCertPath, effectiveInsecure, c.serverFingerprint)
	if err != nil {
		return fmt.Errorf("build action-runner activation client: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("activate action-runner credential: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("activate action-runner credential: server returned %s", response.Status)
	}
	return nil
}
