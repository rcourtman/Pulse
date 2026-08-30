package hostagent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const privilegeHelperOperationDeadline = 30 * time.Second

// PrivilegedTelemetry is the collector-side view of the no-network helper.
// It intentionally exposes complete typed snapshots rather than commands,
// executable paths, VMIDs, device paths, or caller-selected arguments.
type PrivilegedTelemetry interface {
	Health(context.Context) error
	SMARTSnapshot(context.Context) ([]DiskSMART, error)
	ProxmoxLXCFilesystems(context.Context) (*agentshost.ProxmoxLXCInventory, error)
}

type privilegeHelperTelemetry struct {
	client *agenthelper.Client
}

// NewPrivilegeHelperTelemetry creates a local-only client for the fixed helper
// socket selected by the installer. An empty path disables the helper.
func NewPrivilegeHelperTelemetry(socketPath string) (PrivilegedTelemetry, error) {
	if strings.TrimSpace(socketPath) == "" {
		return nil, nil
	}
	client, err := agenthelper.NewClient(agenthelper.ClientConfig{
		SocketPath:  socketPath,
		MaxDeadline: privilegeHelperOperationDeadline,
	})
	if err != nil {
		return nil, err
	}
	return &privilegeHelperTelemetry{client: client}, nil
}

// Health proves that the configured socket is serving the exact typed-helper
// protocol expected by this collector. A listening systemd socket alone is
// not sufficient: socket activation can succeed while the helper binary is
// missing, incompatible, or unable to handle requests.
func (c *privilegeHelperTelemetry) Health(ctx context.Context) error {
	var response agenthelper.HealthResult
	_, err := c.client.Call(
		ctx,
		agenthelper.OperationHealth,
		agenthelper.OperationVersion1,
		privilegeHelperOperationDeadline,
		struct{}{},
		&response,
	)
	if err != nil {
		return err
	}
	if response.Status != "ok" {
		return errors.New("helper health response is not ok")
	}
	if response.ProtocolVersion != agenthelper.ProtocolVersion {
		return errors.New("helper health protocol version does not match")
	}
	return nil
}

func (c *privilegeHelperTelemetry) SMARTSnapshot(ctx context.Context) ([]DiskSMART, error) {
	var response struct {
		Disks []DiskSMART `json:"disks"`
	}
	_, err := c.client.Call(
		ctx,
		agenthelper.OperationSMARTSnapshot,
		agenthelper.OperationVersion1,
		privilegeHelperOperationDeadline,
		struct{}{},
		&response,
	)
	if err != nil {
		return nil, err
	}
	return response.Disks, nil
}

func (c *privilegeHelperTelemetry) ProxmoxLXCFilesystems(ctx context.Context) (*agentshost.ProxmoxLXCInventory, error) {
	var response struct {
		Inventory *agentshost.ProxmoxLXCInventory `json:"inventory"`
	}
	_, err := c.client.Call(
		ctx,
		agenthelper.OperationProxmoxLXCFilesystems,
		agenthelper.OperationVersion1,
		privilegeHelperOperationDeadline,
		struct{}{},
		&response,
	)
	if err != nil {
		return nil, err
	}
	if response.Inventory == nil {
		return nil, errors.New("helper returned no Proxmox LXC filesystem inventory")
	}
	return response.Inventory, nil
}

// collectProxmoxLXCFilesystemsForReport preserves the privilege boundary once
// a helper is configured. A helper failure must omit this best-effort snapshot
// rather than silently retrying the same collection in the unprivileged
// collector process (or widening that process's privileges to make it work).
func (a *Agent) collectProxmoxLXCFilesystemsForReport(ctx context.Context) *agentshost.ProxmoxLXCInventory {
	if a.privilegedTelemetry == nil {
		return a.collectProxmoxLXCFilesystems(ctx)
	}
	inventory, err := a.privilegedTelemetry.ProxmoxLXCFilesystems(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Typed helper could not collect Proxmox LXC filesystems")
		return nil
	}
	return inventory
}
