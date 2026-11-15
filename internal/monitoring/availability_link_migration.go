package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// migrateAvailabilityLinksToCanonicalIDs loads the availability target
// configuration, re-homes explicit links whose resource reference was retired
// by a canonical-ID succession or a guest live migration, and persists the
// result. Runs on the same cadence as the alert override migration; once
// every link references a live canonical ID it is a cheap no-op.
func (m *Monitor) migrateAvailabilityLinksToCanonicalIDs(resources []unifiedresources.Resource) {
	if m == nil || m.configPersist == nil || len(resources) == 0 {
		return
	}
	targets, err := m.configPersist.LoadAvailabilityTargets()
	if err != nil || len(targets) == 0 {
		return
	}
	migrated, changed := migrateAvailabilityLinkedResources(targets, resources)
	if !changed {
		return
	}
	if err := m.configPersist.SaveAvailabilityTargets(migrated); err != nil {
		log.Error().Err(err).Msg("failed to persist canonical availability link migration")
		return
	}
	log.Info().Msg("migrated availability links to current canonical resource IDs")
}

// migrateAvailabilityLinkedResources re-homes explicit availability links
// written under a retired canonical resource ID (or under a node-scoped guest
// source ID) onto the resource's current canonical ID. The explicit link is
// documented as authoritative and fail-closed, so only provider-declared
// persistence keys participate: a superseded canonical ID declared by exactly
// one live resource, or a Proxmox guest source reference whose instance+VMID
// matches exactly one live guest. Address or hostname coincidences never
// migrate a link.
//
// The returned slice shares unmodified entries with the input; the boolean
// reports whether any link changed.
func migrateAvailabilityLinkedResources(targets []config.AvailabilityTarget, resources []unifiedresources.Resource) ([]config.AvailabilityTarget, bool) {
	if len(targets) == 0 || len(resources) == 0 {
		return targets, false
	}

	hasLink := false
	for _, target := range targets {
		if strings.TrimSpace(target.LinkedResourceID) != "" || target.DockerPort != nil {
			hasLink = true
			break
		}
	}
	if !hasLink {
		return targets, false
	}

	liveIDs := make(map[string]struct{}, len(resources))
	successors := make(map[string]string)
	ambiguous := make(map[string]struct{})
	guestIDsByKey := make(map[string][]string)
	// A check created from a published container port names its host and
	// container. Docker keeps a container name unique per host, so one live
	// namesake on that host is the container the check belongs to.
	type dockerContainerCandidate struct {
		id    string
		ports []unifiedresources.DockerPortMeta
	}
	containersByKey := make(map[string][]dockerContainerCandidate)

	for _, resource := range resources {
		currentID := unifiedresources.CanonicalResourceID(resource.ID)
		if currentID == "" {
			continue
		}
		liveIDs[currentID] = struct{}{}

		for _, supersededID := range resource.SupersededCanonicalIDs {
			oldID := unifiedresources.CanonicalResourceID(supersededID)
			if oldID == "" || oldID == currentID {
				continue
			}
			if existing, ok := successors[oldID]; ok && existing != currentID {
				delete(successors, oldID)
				ambiguous[oldID] = struct{}{}
				continue
			}
			if _, conflict := ambiguous[oldID]; conflict {
				continue
			}
			successors[oldID] = currentID
		}

		if resource.Docker != nil && unifiedresources.CanonicalResourceType(resource.Type) == unifiedresources.ResourceTypeAppContainer {
			if key := dockerContainerLinkKey(resource.Docker.HostSourceID, resource.Name); key != "" {
				containersByKey[key] = append(containersByKey[key], dockerContainerCandidate{id: currentID, ports: resource.Docker.Ports})
			}
		}

		if resource.Proxmox != nil && resource.Proxmox.VMID > 0 {
			switch unifiedresources.CanonicalResourceType(resource.Type) {
			case unifiedresources.ResourceTypeVM, unifiedresources.ResourceTypeSystemContainer:
				key := unifiedresources.ProxmoxGuestIdentityKey(resource.Proxmox.Instance, resource.Proxmox.VMID)
				if key != "" {
					guestIDsByKey[key] = append(guestIDsByKey[key], currentID)
				}
			}
		}
	}

	changed := false
	out := targets
	for i, target := range targets {
		linkedID := strings.TrimSpace(target.LinkedResourceID)
		if linkedID == "" && target.DockerPort == nil {
			continue
		}
		if _, live := liveIDs[linkedID]; live {
			continue
		}

		newID := successors[linkedID]
		if newID == "" && target.DockerPort != nil {
			// The container was recreated under a new Docker ID. Follow the
			// name on the same host, but only while the namesake still
			// publishes the port this check connects to: recreated with a
			// different port, the old endpoint may now belong to something
			// else. Two namesakes fail closed.
			key := dockerContainerLinkKey(target.DockerPort.HostID, target.DockerPort.ContainerName)
			if candidates := containersByKey[key]; key != "" && len(candidates) == 1 &&
				dockerPortStillPublished(candidates[0].ports, target.DockerPort.PrivatePort, target.Port) {
				newID = candidates[0].id
			}
		}
		if newID == "" && linkedID != "" {
			if instance, _, vmid, ok := unifiedresources.ParseProxmoxGuestSourceID(linkedID); ok {
				// Node segment deliberately ignored: the link must follow the
				// guest across live migrations. Ambiguity fails closed.
				if candidates := guestIDsByKey[unifiedresources.ProxmoxGuestIdentityKey(instance, vmid)]; len(candidates) == 1 {
					newID = candidates[0]
				}
			}
		}
		if newID == "" || newID == linkedID {
			continue
		}

		if !changed {
			out = make([]config.AvailabilityTarget, len(targets))
			copy(out, targets)
			changed = true
		}
		out[i].LinkedResourceID = newID
	}
	return out, changed
}

func dockerContainerLinkKey(hostID, containerName string) string {
	hostID = strings.TrimSpace(hostID)
	containerName = config.NormalizeDockerContainerName(containerName)
	if hostID == "" || containerName == "" {
		return ""
	}
	return hostID + "\x00" + containerName
}

func dockerPortStillPublished(ports []unifiedresources.DockerPortMeta, privatePort, publicPort int) bool {
	for _, port := range ports {
		protocol := strings.ToLower(strings.TrimSpace(port.Protocol))
		if port.PrivatePort == privatePort && port.PublicPort == publicPort && (protocol == "" || protocol == "tcp") {
			return true
		}
	}
	return false
}
