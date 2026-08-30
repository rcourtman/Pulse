package hostagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

const ActionRunnerRuntimeRole = agentexec.RuntimeRoleActionRunner

type ActionRunnerClientConfig struct {
	PulseURL                         string
	APIToken                         string
	StateDir                         string
	HealthPath                       string
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
	Registered   bool      `json:"registered"`
	RuntimeRole  string    `json:"runtime_role"`
	Server       string    `json:"server"`
	HostID       string    `json:"host_id"`
	Hostname     string    `json:"hostname"`
	Capabilities []string  `json:"capabilities"`
	RegisteredAt time.Time `json:"registered_at"`
}

func (c *CommandClient) writeActionRunnerHealth() error {
	if c == nil || !c.actionRunnerOnly || strings.TrimSpace(c.healthPath) == "" {
		return fmt.Errorf("action-runner health path is required")
	}
	capabilities := append([]string(nil), c.healthCapabilities...)
	sort.Strings(capabilities)
	health := actionRunnerHealth{
		Registered: true, RuntimeRole: agentexec.RuntimeRoleActionRunner,
		Server: c.pulseURL, HostID: c.agentID, Hostname: c.hostname,
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
	temp, err := os.CreateTemp(dir, ".health-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
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
	return syncActionRunnerHealthDirectory(dir)
}
