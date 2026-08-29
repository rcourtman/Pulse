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
