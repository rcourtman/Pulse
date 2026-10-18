package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// ProxmoxDiskAgentSMARTSplit reports whether the operator split a Proxmox
// disk observation from the disk a host agent reports in one SMART row:
// report-merge on the merged disk (the drawer's Split merged resource)
// recorded an exclusion that separates them by the rule the linked disk join
// applies (physicalDiskSplitLocked). The PVE disk poller asks before it pairs
// the agent's row with the Proxmox disk on the agent's node, so a split
// Proxmox row carries what Proxmox reported and the PVE disk check judges it.
//
// It reads the exclusions this generation loaded, and whether it holds the two
// observations as one disk, never what the poller's pairing would fill into
// the Proxmox row: the pairing re-keys a Proxmox disk whose serial it fills or
// promotes, so a decision read from those fields would flip with it. The
// generation holds them as one disk only through a manual link, which is the
// operator's later decision, so pairing resumes while it stands, and the link's
// removal or a new report-merge splits them again. A registry without a store
// carries no operator decisions. It takes no lock monitoring holds.
func (a *MonitorAdapter) ProxmoxDiskAgentSMARTSplit(disk models.PhysicalDisk, host models.Host, smart models.HostDiskSMART) bool {
	registry := a.currentRegistry()
	if registry == nil {
		return false
	}
	return registry.proxmoxDiskAgentSMARTSplit(disk, host, smart)
}

// proxmoxDiskAgentSMARTSplit judges the pair as the linked disk join would:
// the Proxmox observation by its source-specific candidate ID (its slot) and
// the identity Proxmox reported, the agent's disk by the ID this generation
// gave it, or, absent from it, by the identity its row reports.
func (rr *ResourceRegistry) proxmoxDiskAgentSMARTSplit(disk models.PhysicalDisk, host models.Host, smart models.HostDiskSMART) bool {
	proxmoxSourceID := normalizeSourceID(disk.ID)
	agentSourceID := normalizeSourceID(HostSMARTDiskSourceID(host, smart))
	if proxmoxSourceID == "" || agentSourceID == "" {
		return false
	}

	rr.mu.RLock()
	defer rr.mu.RUnlock()
	if len(rr.exclusions) == 0 {
		return false
	}
	agentDiskID := rr.bySource[SourceAgent][agentSourceID]
	if agentDiskID != "" && agentDiskID == rr.bySource[SourceProxmox][proxmoxSourceID] {
		return false
	}
	agentDisk := rr.resources[agentDiskID]
	if agentDisk == nil {
		resource, identity := resourceFromHostSMARTDisk(host, smart)
		resource.Identity = identity
		// A row without hardware identity takes its candidate ID, which
		// report-merge records for it, as the merged disk's ID.
		resource.ID = rr.sourceSpecificID(ResourceTypePhysicalDisk, SourceAgent, agentSourceID)
		if hostID := rr.bySource[SourceAgent][normalizeSourceID(host.ID)]; hostID != "" {
			resource.ParentID = &hostID
		}
		agentDisk = &resource
	}
	observation, identity := resourceFromPhysicalDisk(disk)
	observation.Identity = identity
	if nodeID := rr.bySource[SourceProxmox][proxmoxNodeSourceID(disk.Instance, disk.Node)]; nodeID != "" {
		observation.ParentID = &nodeID
	}
	return rr.physicalDiskSplitLocked(agentDisk, &observation, rr.sourceSpecificID(ResourceTypePhysicalDisk, SourceProxmox, proxmoxSourceID))
}
