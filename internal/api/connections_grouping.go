package api

import (
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type connectionSystemGroup struct {
	id         string
	typ        ConnectionType
	name       string
	components map[string]ConnectionSystemComponent
}

func buildConnectionSystems(
	connections []Connection,
	monitor *monitoring.Monitor,
) []ConnectionSystem {
	if len(connections) == 0 {
		return nil
	}

	connectionByID := make(map[string]Connection, len(connections))
	for _, connection := range connections {
		connectionByID[connection.ID] = connection
	}

	hostIndex := buildConnectionHostIndex(connections)
	agentAttachments := resolveAgentAttachments(connectionByID, hostIndex, monitor)

	groups := make(map[string]*connectionSystemGroup, len(connections))
	ensureGroup := func(primary Connection) *connectionSystemGroup {
		group := groups[primary.ID]
		if group != nil {
			return group
		}
		group = &connectionSystemGroup{
			id:         primary.ID,
			typ:        primary.Type,
			name:       strings.ToLower(primary.Name),
			components: make(map[string]ConnectionSystemComponent, 2),
		}
		group.components[primary.ID] = ConnectionSystemComponent{
			ConnectionID: primary.ID,
			Type:         primary.Type,
			Role:         ConnectionSystemComponentRolePrimary,
		}
		groups[primary.ID] = group
		return group
	}

	for _, connection := range connections {
		if connection.Type == ConnectionTypeAgent {
			if primaryID, ok := agentAttachments[connection.ID]; ok && primaryID != "" {
				primary, exists := connectionByID[primaryID]
				if !exists {
					continue
				}
				group := ensureGroup(primary)
				group.components[connection.ID] = ConnectionSystemComponent{
					ConnectionID: connection.ID,
					Type:         connection.Type,
					Role:         ConnectionSystemComponentRoleAttachment,
				}
				continue
			}
		}

		ensureGroup(connection)
	}

	out := make([]ConnectionSystem, 0, len(groups))
	for _, group := range groups {
		components := make([]ConnectionSystemComponent, 0, len(group.components))
		for _, component := range group.components {
			components = append(components, component)
		}
		sort.Slice(components, func(i, j int) bool {
			if components[i].Role != components[j].Role {
				return connectionComponentRolePriority(components[i].Role) <
					connectionComponentRolePriority(components[j].Role)
			}
			if components[i].Type != components[j].Type {
				return connectionTypePriority(components[i].Type) < connectionTypePriority(components[j].Type)
			}
			return components[i].ConnectionID < components[j].ConnectionID
		})
		out = append(out, ConnectionSystem{
			ID:         group.id,
			Type:       group.typ,
			Components: components,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return connectionTypePriority(out[i].Type) < connectionTypePriority(out[j].Type)
		}
		leftName := groups[out[i].ID].name
		rightName := groups[out[j].ID].name
		if leftName == rightName {
			return out[i].ID < out[j].ID
		}
		return leftName < rightName
	})

	return out
}

func connectionComponentRolePriority(role ConnectionSystemComponentRole) int {
	switch role {
	case ConnectionSystemComponentRolePrimary:
		return 0
	case ConnectionSystemComponentRoleAttachment:
		return 1
	default:
		return 100
	}
}

func resolveAgentAttachments(
	connectionByID map[string]Connection,
	hostIndex connectionHostIndex,
	monitor *monitoring.Monitor,
) map[string]string {
	attachments := make(map[string]string)
	if monitor == nil {
		return attachments
	}

	register := func(agentID, primaryID string, allowOverride bool) {
		agentID = strings.TrimSpace(agentID)
		primaryID = strings.TrimSpace(primaryID)
		if agentID == "" || primaryID == "" || agentID == primaryID {
			return
		}
		if _, ok := connectionByID[agentID]; !ok {
			return
		}
		primary, ok := connectionByID[primaryID]
		if !ok || primary.Type == ConnectionTypeAgent {
			return
		}

		existing, exists := attachments[agentID]
		if !exists || existing == "" {
			attachments[agentID] = primaryID
			return
		}
		if existing == primaryID {
			return
		}
		if allowOverride {
			attachments[agentID] = primaryID
			return
		}
		attachments[agentID] = ""
	}

	for _, node := range monitor.NodesSnapshot() {
		if strings.TrimSpace(node.LinkedAgentID) == "" || strings.TrimSpace(node.Instance) == "" {
			continue
		}
		register("agent:"+strings.TrimSpace(node.LinkedAgentID), "pve:"+strings.TrimSpace(node.Instance), false)
	}

	resources, _ := monitor.UnifiedResourceSnapshot()
	for _, resource := range resources {
		if resource.Agent == nil {
			continue
		}
		agentID := "agent:" + strings.TrimSpace(resource.Agent.AgentID)
		if _, ok := connectionByID[agentID]; !ok {
			continue
		}
		if _, exists := attachments[agentID]; exists {
			continue
		}
		if primaryID := primaryConnectionIDForResource(resource, hostIndex); primaryID != "" {
			register(agentID, primaryID, false)
		}
	}

	for agentID, primaryID := range attachments {
		if primaryID == "" {
			delete(attachments, agentID)
		}
	}

	return attachments
}

type connectionHostIndex struct {
	byType map[ConnectionType]map[string][]string
}

func buildConnectionHostIndex(connections []Connection) connectionHostIndex {
	index := connectionHostIndex{byType: make(map[ConnectionType]map[string][]string)}
	for _, connection := range connections {
		host := normalizedConnectionHost(connection)
		if host == "" {
			continue
		}
		typeBucket := index.byType[connection.Type]
		if typeBucket == nil {
			typeBucket = make(map[string][]string)
			index.byType[connection.Type] = typeBucket
		}
		typeBucket[host] = append(typeBucket[host], connection.ID)
	}
	return index
}

func primaryConnectionIDForResource(
	resource unified.Resource,
	hostIndex connectionHostIndex,
) string {
	switch {
	case resource.Proxmox != nil:
		if instance := strings.TrimSpace(resource.Proxmox.Instance); instance != "" {
			return "pve:" + instance
		}
	case resource.VMware != nil:
		if connectionID := strings.TrimSpace(resource.VMware.ConnectionID); connectionID != "" {
			return "vmware:" + connectionID
		}
	case resource.TrueNAS != nil:
		for _, candidate := range []string{
			strings.TrimSpace(resource.TrueNAS.Hostname),
			canonicalResourceHostname(resource),
			strings.TrimSpace(resource.Name),
		} {
			if matched := hostIndex.uniqueMatch(ConnectionTypeTrueNAS, candidate); matched != "" {
				return matched
			}
		}
	case resource.PBS != nil:
		if instanceID := strings.TrimSpace(resource.PBS.InstanceID); instanceID != "" {
			return "pbs:" + instanceID
		}
	case resource.PMG != nil:
		if instanceID := strings.TrimSpace(resource.PMG.InstanceID); instanceID != "" {
			return "pmg:" + instanceID
		}
	}

	return ""
}

func (i connectionHostIndex) uniqueMatch(typ ConnectionType, raw string) string {
	host := normalizeHost(raw)
	if host == "" {
		return ""
	}
	typeBucket := i.byType[typ]
	if typeBucket == nil {
		return ""
	}
	matches := typeBucket[host]
	if len(matches) != 1 {
		return ""
	}
	return matches[0]
}

func normalizedConnectionHost(connection Connection) string {
	for _, candidate := range []string{connection.Address, connection.Name} {
		if normalized := normalizeHost(candidate); normalized != "" {
			return normalized
		}
	}
	return ""
}

func normalizeHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if parsedURL, err := url.Parse(value); err == nil && parsedURL.Host != "" {
		value = parsedURL.Hostname()
	} else if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}

	if parsedIP := net.ParseIP(value); parsedIP != nil {
		return parsedIP.String()
	}

	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, ".")
	return value
}

func canonicalResourceHostname(resource unified.Resource) string {
	if resource.Canonical == nil {
		return ""
	}
	return strings.TrimSpace(resource.Canonical.Hostname)
}

func connectionTypePriority(typ ConnectionType) int {
	switch typ {
	case ConnectionTypePVE:
		return 0
	case ConnectionTypeVMware:
		return 1
	case ConnectionTypeTrueNAS:
		return 2
	case ConnectionTypeDocker:
		return 3
	case ConnectionTypeAgent:
		return 4
	case ConnectionTypePBS:
		return 10
	case ConnectionTypePMG:
		return 11
	case ConnectionTypeKubernetes:
		return 12
	default:
		return 100
	}
}
