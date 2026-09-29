package unifiedresources

import (
	"fmt"
	"strings"
	"time"

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
	vms []models.VM,
) {
	host := uniquePBSHostAgent(instance, hosts, vms)
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
	// Expose the already-corroborated source-native agent identity on the PBS
	// service. The service and host retain separate resources and metric targets;
	// this only lets consumers select the host series when names/IP aliases do
	// not line up (including token-auth PBS connections without nodeName).
	if pbsResource := rr.resources[pbsParentID]; pbsResource != nil && pbsResource.PBS != nil {
		if hostResource := rr.resources[hostCanonicalID]; hostResource != nil &&
			hostResource.Agent != nil && strings.TrimSpace(hostResource.Agent.AgentID) == strings.TrimSpace(host.ID) {
			pbsResource.PBS.LinkedAgentID = strings.TrimSpace(host.ID)
		}
	}
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

func uniquePBSHostAgent(instance models.PBSInstance, hosts []models.Host, vms []models.VM) *models.Host {
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
	if match != nil {
		return match
	}

	// A PBS inside a PVE VM may have an API endpoint IP that the in-guest
	// Pulse Agent does not report as an interface. A state-linked Agent and a
	// PVE guest observed at that exact IP form a safe alternate chain. Names
	// alone do not: a PBS connection label is not host identity. Reject reused
	// guest IPs and multiple agents linked to one guest rather than selecting
	// an arbitrary host.
	endpointIP := NormalizeIP(extractHostname(instance.Host))
	if isNonUniqueIP(endpointIP) {
		return nil
	}
	guestID := ""
	for _, vm := range vms {
		if !strings.EqualFold(strings.TrimSpace(vm.Status), "running") ||
			!pbsGuestLinkObservationFresh(instance.LastSeen, vm.LastSeen) {
			continue
		}
		for _, address := range vm.IPAddresses {
			if NormalizeIP(address) != endpointIP {
				continue
			}
			id := strings.TrimSpace(vm.ID)
			if id == "" || guestID != "" && guestID != id {
				return nil
			}
			guestID = id
			break
		}
	}
	if guestID == "" {
		return nil
	}
	for index := range hosts {
		if strings.TrimSpace(hosts[index].LinkedVMID) != guestID {
			continue
		}
		if !pbsGuestLinkObservationFresh(instance.LastSeen, hosts[index].LastSeen) {
			continue
		}
		if match != nil {
			return nil
		}
		match = &hosts[index]
	}
	return match
}

func pbsGuestLinkObservationFresh(pbsSeen, peerSeen time.Time) bool {
	if pbsSeen.IsZero() || peerSeen.IsZero() {
		return false
	}
	delta := pbsSeen.Sub(peerSeen)
	return delta >= -5*time.Minute && delta <= 5*time.Minute
}

func pbsInstanceCorroboratesHost(instance models.PBSInstance, host models.Host) bool {
	hostName := NormalizeHostname(host.Hostname)
	if hostName == "" {
		return false
	}
	// The node hostname the PBS API reports about itself is machine identity.
	// It is the strongest evidence when the connection is configured by IP or
	// a DNS alias the agent never reports, which the connection label and the
	// configured endpoint cannot corroborate on their own (#1723).
	if nodeName := NormalizeHostname(instance.NodeName); nodeName != "" && nodeName == hostName {
		return true
	}
	// The connection name is operator-chosen display text. Matching it to an
	// Agent hostname can steal the link from the actual guest (and project its
	// SMART disks under the wrong PBS server), so it is not identity evidence.
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
