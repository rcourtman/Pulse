package proxmox

import (
	"context"
	"fmt"
	"strings"

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
		pool.Devices = append(pool.Devices, convertDeviceRecursive(dev, "")...)
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
	Message        string `json:"message,omitempty"`
}

// convertDeviceRecursive flattens the device tree into a list
func convertDeviceRecursive(dev ZFSPoolDevice, parentRole string) []ZFSDevice {
	var devices []ZFSDevice

	name := strings.TrimSpace(dev.Name)
	lowerName := strings.ToLower(name)

	role := parentRole
	if role == "" {
		switch {
		case lowerName == "logs" || lowerName == "log" || strings.HasPrefix(lowerName, "slog"):
			role = "log"
		case lowerName == "cache" || strings.HasPrefix(lowerName, "l2arc"):
			role = "cache"
		case lowerName == "spares" || strings.HasPrefix(lowerName, "spare"):
			role = "spare"
		}
	}

	state := strings.ToUpper(strings.TrimSpace(dev.State))
	if state == "" {
		state = "UNKNOWN"
	}

	isVdev := dev.Leaf == 0 && len(dev.Children) > 0

	deviceType := "disk"
	if isVdev {
		deviceType = "vdev"
	}

	switch {
	case isVdev && (lowerName == "mirror" || strings.HasPrefix(lowerName, "mirror")):
		deviceType = "mirror"
	case isVdev && strings.HasPrefix(lowerName, "raidz"):
		deviceType = lowerName // raidz, raidz2, raidz3
	case isVdev && role == "log":
		deviceType = "log"
	case isVdev && role == "cache":
		deviceType = "cache"
	case isVdev && role == "spare":
		deviceType = "spare"
	case role == "log" && dev.Leaf == 1:
		deviceType = "log"
	case role == "cache" && dev.Leaf == 1:
		deviceType = "cache"
	}

	isSpare := lowerName == "spares" || strings.HasPrefix(lowerName, "spare")
	if isSpare {
		if dev.Leaf == 1 {
			deviceType = "spare"
		} else {
			deviceType = "spare-group"
		}
	}

	healthyStates := map[string]bool{
		"ONLINE": true,
		"SPARE":  true,
		"AVAIL":  true,
		"INUSE":  true,
	}

	if state == "UNKNOWN" {
		if role == "log" || role == "cache" || role == "spare" || isSpare {
			healthyStates[state] = true
		}
	}

	// Add this device if it has errors or is not healthy (but skip healthy spares)
	if !healthyStates[state] || dev.Read > 0 || dev.Write > 0 || dev.Cksum > 0 {
		message := strings.TrimSpace(dev.Msg)

		devices = append(devices, ZFSDevice{
			Name:           name,
			Type:           deviceType,
			State:          state,
			ReadErrors:     dev.Read,
			WriteErrors:    dev.Write,
			ChecksumErrors: dev.Cksum,
			IsLeaf:         dev.Leaf == 1,
			Message:        message,
		})
	}

	// Process children
	for _, child := range dev.Children {
		devices = append(devices, convertDeviceRecursive(child, role)...)
	}

	return devices
}
