package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, strings.TrimRight(c.pulseURL, "/")+"/api/agents/action-runner/credential", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiToken)
	request.Header.Set("Content-Type", "application/json")
	client, err := newAgentHTTPClient(c.caCertPath, c.insecureSkipVerify, c.serverFingerprint)
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
