package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// localSMARTProvider reuses the canonical SMART collector inside the
// network-isolated helper process. Its protocol request has no path or
// argument fields; device discovery and command construction remain entirely
// provider-owned.
type localSMARTProvider struct{}

func (localSMARTProvider) Snapshot(ctx context.Context) (json.RawMessage, error) {
	disks, err := hostagent.CollectSMARTLocal(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("collect SMART snapshot: %w", err)
	}
	result, err := json.Marshal(struct {
		Disks []hostagent.DiskSMART `json:"disks"`
	}{Disks: disks})
	if err != nil {
		return nil, fmt.Errorf("encode SMART snapshot: %w", err)
	}
	return result, nil
}

// localProxmoxProvider runs the existing bounded node-local inventory as one
// typed operation. The collector cannot select a VMID, filesystem path,
// command, or pct argument across the helper boundary.
type localProxmoxProvider struct{}

func (localProxmoxProvider) LXCFilesystems(ctx context.Context) (json.RawMessage, error) {
	inventory := hostagent.CollectProxmoxLXCFilesystemsLocal(ctx)
	result, err := json.Marshal(struct {
		Inventory *agentshost.ProxmoxLXCInventory `json:"inventory"`
	}{Inventory: inventory})
	if err != nil {
		return nil, fmt.Errorf("encode Proxmox LXC filesystem inventory: %w", err)
	}
	return result, nil
}
