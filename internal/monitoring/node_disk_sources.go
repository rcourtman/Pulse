package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) resolveNodeDisk(
	instanceName string,
	nodeID string,
	nodeName string,
	node proxmox.Node,
	nodeInfo *proxmox.NodeStatus,
) (models.Disk, string) {
	if linkedHost := m.linkedHostForNode(instanceName, nodeID, nodeName); linkedHost != nil {
		if disk, ok := models.SummaryDisk(linkedHost.Disks); ok {
			resolved := models.Disk{
				Total: disk.Total,
				Used:  disk.Used,
				Free:  disk.Free,
				Usage: disk.Usage,
			}
			log.Debug().
				Str("instance", instanceName).
				Str("node", nodeName).
				Str("hostAgentID", linkedHost.ID).
				Int64("total", resolved.Total).
				Int64("used", resolved.Used).
				Float64("usage", resolved.Usage).
				Msg("Node disk: using linked Pulse host agent disk summary")
			return resolved, "agent"
		}
	}

	if nodeInfo != nil && nodeInfo.RootFS != nil && nodeInfo.RootFS.Total > 0 {
		resolved := models.Disk{
			Total: int64(nodeInfo.RootFS.Total),
			Used:  int64(nodeInfo.RootFS.Used),
			Free:  int64(nodeInfo.RootFS.Free),
			Usage: safePercentage(float64(nodeInfo.RootFS.Used), float64(nodeInfo.RootFS.Total)),
		}
		log.Debug().
			Str("instance", instanceName).
			Str("node", nodeName).
			Uint64("rootfsUsed", nodeInfo.RootFS.Used).
			Uint64("rootfsTotal", nodeInfo.RootFS.Total).
			Float64("rootfsUsage", resolved.Usage).
			Msg("Node disk: using Proxmox rootfs metrics")
		return resolved, "node-status-rootfs"
	}

	if node.MaxDisk > 0 {
		resolved := models.Disk{
			Total: int64(node.MaxDisk),
			Used:  int64(node.Disk),
			Free:  int64(node.MaxDisk - node.Disk),
			Usage: safePercentage(float64(node.Disk), float64(node.MaxDisk)),
		}
		log.Debug().
			Str("instance", instanceName).
			Str("node", nodeName).
			Uint64("disk", node.Disk).
			Uint64("maxDisk", node.MaxDisk).
			Float64("usage", resolved.Usage).
			Msg("Node disk: using /nodes endpoint metrics")
		return resolved, "nodes-endpoint"
	}

	return models.Disk{}, ""
}

func (m *Monitor) linkedHostForNode(instanceName, nodeID, nodeName string) *models.Host {
	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return nil
	}

	linkedAgentID := ""
	for _, existingNode := range nodesForInstanceFromReadState(readState, instanceName) {
		if existingNode.ID == nodeID || strings.EqualFold(existingNode.Name, nodeName) {
			linkedAgentID = strings.TrimSpace(existingNode.LinkedAgentID)
			break
		}
	}
	if linkedAgentID == "" {
		return nil
	}

	for _, host := range hostsFromReadState(readState) {
		if strings.TrimSpace(host.ID) != linkedAgentID {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(host.Status), "online") {
			return nil
		}
		resolved := host
		return &resolved
	}

	return nil
}
