package proxmox

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
)

// GetZFSPoolsWithDetails gets both the list and detailed info for all ZFS pools on a node
// This combines the list and detail endpoints to get complete information
func (c *Client) GetZFSPoolsWithDetails(ctx context.Context, node string) ([]ZFSPoolInfo, error) {
	// First get the list of pools
	pools, err := c.GetZFSPoolStatus(ctx, node)
	if err != nil {
		return nil, fmt.Errorf("failed to list ZFS pools: %w", err)
	}
	
	// Now get details for each pool
	var poolInfos []ZFSPoolInfo
	for _, pool := range pools {
		info := ZFSPoolInfo{
			Name:   pool.Name,
			Health: pool.Health,
			Size:   pool.Size,
			Alloc:  pool.Alloc,
			Free:   pool.Free,
			Frag:   pool.Frag,
			Dedup:  pool.Dedup,
		}
		
		// Try to get detailed info, but don't fail if it's not available
		detail, err := c.GetZFSPoolDetail(ctx, node, pool.Name)
		if err != nil {
			log.Debug().
				Err(err).
				Str("node", node).
				Str("pool", pool.Name).
				Msg("Could not get ZFS pool details, using basic info only")
			// Continue with basic info
		} else {
			info.State = detail.State
			info.Status = detail.Status
			info.Scan = detail.Scan
			info.Errors = detail.Errors
			info.Devices = detail.Children
		}
		
		poolInfos = append(poolInfos, info)
	}
	
	return poolInfos, nil
}

// ZFSPoolInfo combines list and detail info for a complete picture
type ZFSPoolInfo struct {
	// From list endpoint
	Name   string  `json:"name"`
	Health string  `json:"health"`
	Size   uint64  `json:"size"`
	Alloc  uint64  `json:"alloc"`
	Free   uint64  `json:"free"`
	Frag   int     `json:"frag"`
	Dedup  float64 `json:"dedup"`
	
	// From detail endpoint (may be empty if not available)
	State   string          `json:"state,omitempty"`
	Status  string          `json:"status,omitempty"`
	Scan    string          `json:"scan,omitempty"`
	Errors  string          `json:"errors,omitempty"`
	Devices []ZFSPoolDevice `json:"devices,omitempty"`
}

// ConvertToModelZFSPool converts the combined pool info to our model
func (p *ZFSPoolInfo) ConvertToModelZFSPool() *ZFSPool {
	if p == nil {
		return nil
	}
	
	// Use State if available, otherwise fall back to Health
	state := p.State
	if state == "" {
		state = p.Health
	}
	
	pool := &ZFSPool{
		Name:   p.Name,
		State:  state,
		Health: p.Health,
		Status: p.Status,
		Scan:   p.Scan,
		Errors: p.Errors,
	}
	
	// Extract error counts from devices if available
	pool.Devices = make([]ZFSDevice, 0)
	for _, dev := range p.Devices {
		pool.Devices = append(pool.Devices, convertDeviceRecursive(dev)...)
	}
	
	// Calculate total errors from all devices
	for _, dev := range pool.Devices {
		pool.ReadErrors += dev.ReadErrors
		pool.WriteErrors += dev.WriteErrors
		pool.ChecksumErrors += dev.ChecksumErrors
	}
	
	return pool
}

// ZFSPool represents complete ZFS pool information for monitoring
type ZFSPool struct {
	Name           string      `json:"name"`
	State          string      `json:"state"`
	Health         string      `json:"health"`
	Status         string      `json:"status"`
	Scan           string      `json:"scan"`
	Errors         string      `json:"errors"`
	ReadErrors     int64       `json:"readErrors"`
	WriteErrors    int64       `json:"writeErrors"`
	ChecksumErrors int64       `json:"checksumErrors"`
	Devices        []ZFSDevice `json:"devices"`
}

// ZFSDevice represents a device in the pool (flattened from tree structure)
type ZFSDevice struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	State          string `json:"state"`
	ReadErrors     int64  `json:"readErrors"`
	WriteErrors    int64  `json:"writeErrors"`
	ChecksumErrors int64  `json:"checksumErrors"`
	IsLeaf         bool   `json:"isLeaf"`
}

// convertDeviceRecursive flattens the device tree into a list
func convertDeviceRecursive(dev ZFSPoolDevice) []ZFSDevice {
	var devices []ZFSDevice
	
	// Determine device type based on name and structure
	deviceType := "disk"
	if dev.Leaf == 0 && len(dev.Children) > 0 {
		// It's a vdev (mirror, raidz, etc.)
		if dev.Name == "mirror" || (len(dev.Name) >= 6 && dev.Name[:6] == "mirror") {
			deviceType = "mirror"
		} else if len(dev.Name) >= 5 && dev.Name[:5] == "raidz" {
			deviceType = dev.Name // raidz, raidz2, raidz3
		} else {
			deviceType = "vdev"
		}
	}
	
	// Add this device if it has errors or is not healthy
	if dev.State != "ONLINE" || dev.Read > 0 || dev.Write > 0 || dev.Cksum > 0 {
		devices = append(devices, ZFSDevice{
			Name:           dev.Name,
			Type:           deviceType,
			State:          dev.State,
			ReadErrors:     dev.Read,
			WriteErrors:    dev.Write,
			ChecksumErrors: dev.Cksum,
			IsLeaf:         dev.Leaf == 1,
		})
	}
	
	// Process children
	for _, child := range dev.Children {
		devices = append(devices, convertDeviceRecursive(child)...)
	}
	
	return devices
}