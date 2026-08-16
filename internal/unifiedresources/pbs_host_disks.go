package unifiedresources

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// associatePBSHostAgentResources projects a corroborated PBS host-agent
// relationship onto the existing host and physical-disk resources. PBS does
// not expose SMART inventory through its API, so the disk facts remain
// agent-owned; the PBS source membership only makes their platform ownership
// explicit for shared Proxmox storage consumers.
func (rr *ResourceRegistry) associatePBSHostAgentResources(
	instance models.PBSInstance,
	hosts []models.Host,
) {
	host := uniquePBSHostAgent(instance, hosts)
	if host == nil {
		return
	}

	pbsSourceID := pbsInstanceSourceID(instance)
	pbsParentID := rr.sourceResourceID(SourcePBS, pbsSourceID)
	if pbsSourceID == "" || pbsParentID == "" {
		return
	}

	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.invalidateSourceTargetsLocked()

	if rr.bySource[SourcePBS] == nil {
		rr.bySource[SourcePBS] = make(map[string]string)
	}

	attach := func(sourceID, canonicalID, parentID string) {
		sourceID = normalizeSourceID(sourceID)
		canonicalID = CanonicalResourceID(canonicalID)
		if sourceID == "" || canonicalID == "" {
			return
		}
		resource := rr.resources[canonicalID]
		if resource == nil {
			return
		}
		resource.Sources = addSource(resource.Sources, SourcePBS)
		if resource.SourceStatus == nil {
			resource.SourceStatus = make(map[DataSource]SourceStatus)
		}
		resource.SourceStatus[SourcePBS] = SourceStatus{
			Status:   sourceSightingStatus(instance.LastSeen),
			LastSeen: instance.LastSeen,
		}
		if parentID != "" {
			rr.setSourceParent(resource, SourcePBS, &parentID)
			resource.ParentID = rr.resolveCanonicalParentID(resource)
		}
		rr.bySource[SourcePBS][sourceID] = canonicalID
	}

	hostCanonicalID := rr.bySource[SourceAgent][normalizeSourceID(host.ID)]
	attach(
		fmt.Sprintf("%s/agent:%s", pbsSourceID, strings.TrimSpace(host.ID)),
		hostCanonicalID,
		"",
	)

	for _, disk := range host.Sensors.SMART {
		agentDiskSourceID := HostSMARTDiskSourceID(*host, disk)
		diskCanonicalID := rr.bySource[SourceAgent][normalizeSourceID(agentDiskSourceID)]
		attach(
			fmt.Sprintf("%s/disk:%s", pbsSourceID, agentDiskSourceID),
			diskCanonicalID,
			pbsParentID,
		)
	}
	rr.viewsDirty = true
}

func uniquePBSHostAgent(instance models.PBSInstance, hosts []models.Host) *models.Host {
	var match *models.Host
	for index := range hosts {
		if !pbsInstanceCorroboratesHost(instance, hosts[index]) {
			continue
		}
		if match != nil {
			return nil
		}
		match = &hosts[index]
	}
	return match
}

func pbsInstanceCorroboratesHost(instance models.PBSInstance, host models.Host) bool {
	hostName := NormalizeHostname(host.Hostname)
	if hostName == "" {
		return false
	}
	if instanceName := NormalizeHostname(instance.Name); instanceName != "" && instanceName == hostName {
		return true
	}

	endpoint := strings.TrimSpace(strings.ToLower(extractHostname(instance.Host)))
	if endpoint == "" {
		return false
	}
	if ip := NormalizeIP(endpoint); ip != "" {
		if NormalizeIP(host.ReportIP) == ip {
			return true
		}
		for _, iface := range host.NetworkInterfaces {
			for _, address := range iface.Addresses {
				if NormalizeIP(address) == ip {
					return true
				}
			}
		}
		return false
	}

	return NormalizeHostname(endpoint) == hostName
}
