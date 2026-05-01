package api

import (
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type connectionSystemGroup struct {
	id          string
	typ         ConnectionType
	name        string
	clusterName string
	components  map[string]ConnectionSystemComponent
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

	clusterNames := buildProxmoxClusterNames(connectionByID, monitor)
	hostIndex := buildConnectionHostIndex(connections)
	agentAttachments := resolveAgentAttachments(connectionByID, hostIndex, monitor)
	systemMembers := buildConnectionSystemMembers(connectionByID, agentAttachments, monitor)

	groups := make(map[string]*connectionSystemGroup, len(connections))
	ensureGroup := func(primary Connection) *connectionSystemGroup {
		group := groups[primary.ID]
		if group != nil {
			return group
		}
		clusterName := strings.TrimSpace(clusterNames[primary.ID])
		sortName := strings.ToLower(primary.Name)
		if clusterName != "" {
			sortName = strings.ToLower(clusterName)
		}
		group = &connectionSystemGroup{
			id:          primary.ID,
			typ:         primary.Type,
			name:        sortName,
			clusterName: clusterName,
			components:  make(map[string]ConnectionSystemComponent, 2),
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
			ID:          group.id,
			Type:        group.typ,
			ClusterName: group.clusterName,
			Components:  components,
			Members:     systemMembers[group.id],
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

func buildConnectionSystemMembers(
	connectionByID map[string]Connection,
	agentAttachments map[string]string,
	monitor *monitoring.Monitor,
) map[string][]ConnectionSystemMember {
	systemMembers := make(map[string][]ConnectionSystemMember)
	if monitor == nil {
		return systemMembers
	}

	bySystemKey := make(map[string]map[string]ConnectionSystemMember)
	register := func(primaryID string, member ConnectionSystemMember) {
		primaryID = strings.TrimSpace(primaryID)
		member.Name = strings.TrimSpace(member.Name)
		if primaryID == "" || member.Name == "" {
			return
		}
		if _, ok := connectionByID[primaryID]; !ok {
			return
		}

		memberKey := connectionSystemMemberKey(member)
		if memberKey == "" {
			return
		}
		member.ID = strings.TrimSpace(member.ID)
		if member.ID == "" {
			member.ID = memberKey
		}

		bucket := bySystemKey[primaryID]
		if bucket == nil {
			bucket = make(map[string]ConnectionSystemMember)
			bySystemKey[primaryID] = bucket
		}

		existing, exists := bucket[memberKey]
		if !exists {
			bucket[memberKey] = member
			return
		}

		bucket[memberKey] = mergeConnectionSystemMembers(existing, member)
	}

	for _, node := range monitor.NodesSnapshot() {
		member, primaryID, ok := connectionSystemMemberFromNode(node, connectionByID, agentAttachments)
		if !ok {
			continue
		}
		register(primaryID, member)
	}

	resources, _ := monitor.UnifiedResourceSnapshot()
	for _, resource := range resources {
		member, primaryID, ok := connectionSystemMemberFromResource(resource, connectionByID, agentAttachments)
		if !ok {
			continue
		}
		register(primaryID, member)
	}

	for primaryID, membersByKey := range bySystemKey {
		members := make([]ConnectionSystemMember, 0, len(membersByKey))
		for _, member := range membersByKey {
			members = append(members, member)
		}
		sort.Slice(members, func(i, j int) bool {
			if members[i].Primary != members[j].Primary {
				return members[i].Primary
			}
			leftName := strings.ToLower(strings.TrimSpace(members[i].Name))
			rightName := strings.ToLower(strings.TrimSpace(members[j].Name))
			if leftName == rightName {
				return members[i].ID < members[j].ID
			}
			return leftName < rightName
		})
		systemMembers[primaryID] = members
	}

	return systemMembers
}

func connectionSystemMemberFromNode(
	node models.Node,
	connectionByID map[string]Connection,
	agentAttachments map[string]string,
) (ConnectionSystemMember, string, bool) {
	if !node.IsClusterMember {
		return ConnectionSystemMember{}, "", false
	}
	primaryID := "pve:" + strings.TrimSpace(node.Instance)
	primary, ok := connectionByID[primaryID]
	if !ok || primary.Type != ConnectionTypePVE {
		return ConnectionSystemMember{}, "", false
	}

	var lastSeen *time.Time
	if !node.LastSeen.IsZero() {
		t := node.LastSeen
		lastSeen = &t
	}

	return ConnectionSystemMember{
		ID:                strings.TrimSpace(node.ID),
		Name:              firstNonEmptyTrimmed(node.DisplayName, node.Name),
		Endpoint:          strings.TrimSpace(node.Host),
		HostAliases:       connectionSystemMemberHostAliasesFromNode(node),
		State:             connectionSystemMemberStateFromNode(node),
		LastSeen:          lastSeen,
		Primary:           isPrimaryProxmoxSystemMember(primary, node.Name, node.Host),
		AgentConnectionID: attachedAgentConnectionID(node.LinkedAgentID, primaryID, connectionByID, agentAttachments),
	}, primaryID, true
}

func connectionSystemMemberFromResource(
	resource unified.Resource,
	connectionByID map[string]Connection,
	agentAttachments map[string]string,
) (ConnectionSystemMember, string, bool) {
	if resource.Proxmox == nil || !resource.Proxmox.IsClusterMember {
		return ConnectionSystemMember{}, "", false
	}
	nodeName := strings.TrimSpace(resource.Proxmox.NodeName)
	if nodeName == "" {
		return ConnectionSystemMember{}, "", false
	}
	primaryID := "pve:" + strings.TrimSpace(resource.Proxmox.Instance)
	primary, ok := connectionByID[primaryID]
	if !ok || primary.Type != ConnectionTypePVE {
		return ConnectionSystemMember{}, "", false
	}

	linkedAgentID := resource.Proxmox.LinkedAgentID
	if linkedAgentID == "" && resource.Agent != nil {
		linkedAgentID = resource.Agent.AgentID
	}

	var lastSeen *time.Time
	if !resource.LastSeen.IsZero() {
		t := resource.LastSeen
		lastSeen = &t
	}

	return ConnectionSystemMember{
		ID:                strings.TrimSpace(resource.ID),
		Name:              firstNonEmptyTrimmed(resource.Name, nodeName),
		Endpoint:          strings.TrimSpace(resource.Proxmox.HostURL),
		HostAliases:       connectionSystemMemberHostAliasesFromResource(resource),
		State:             connectionSystemMemberStateFromResource(resource),
		LastSeen:          lastSeen,
		Primary:           isPrimaryProxmoxSystemMember(primary, nodeName, resource.Proxmox.HostURL),
		AgentConnectionID: attachedAgentConnectionID(linkedAgentID, primaryID, connectionByID, agentAttachments),
	}, primaryID, true
}

func mergeConnectionSystemMembers(
	existing ConnectionSystemMember,
	candidate ConnectionSystemMember,
) ConnectionSystemMember {
	if strings.TrimSpace(existing.ID) == "" {
		existing.ID = candidate.ID
	}
	if strings.TrimSpace(existing.Name) == "" {
		existing.Name = candidate.Name
	}
	if strings.TrimSpace(existing.Endpoint) == "" {
		existing.Endpoint = candidate.Endpoint
	}
	existing.State = moreSevereConnectionSystemMemberState(existing.State, candidate.State)
	if existing.LastSeen == nil ||
		(candidate.LastSeen != nil && candidate.LastSeen.After(*existing.LastSeen)) {
		existing.LastSeen = candidate.LastSeen
	}
	if !existing.Primary && candidate.Primary {
		existing.Primary = true
	}
	if strings.TrimSpace(existing.AgentConnectionID) == "" {
		existing.AgentConnectionID = candidate.AgentConnectionID
	}
	existing.HostAliases = appendNormalizedHosts(existing.HostAliases, candidate.HostAliases...)
	return existing
}

func moreSevereConnectionSystemMemberState(
	left ConnectionState,
	right ConnectionState,
) ConnectionState {
	if connectionSystemMemberStateSeverity(right) > connectionSystemMemberStateSeverity(left) {
		return right
	}
	return left
}

func connectionSystemMemberStateSeverity(state ConnectionState) int {
	switch state {
	case ConnectionStateActive:
		return 0
	case ConnectionStatePaused:
		return 1
	case ConnectionStatePending:
		return 2
	case ConnectionStateStale:
		return 3
	case ConnectionStateUnauthorized:
		return 4
	case ConnectionStateUnreachable:
		return 5
	default:
		return 2
	}
}

func connectionSystemMemberHostAliasesFromNode(node models.Node) []string {
	return appendNormalizedHosts(
		nil,
		node.Name,
		node.DisplayName,
		node.Host,
	)
}

func connectionSystemMemberHostAliasesFromResource(resource unified.Resource) []string {
	values := []string{
		resource.Name,
		canonicalResourceHostname(resource),
	}
	if resource.Proxmox != nil {
		values = append(values, resource.Proxmox.NodeName, resource.Proxmox.HostURL)
	}
	if resource.Agent != nil {
		values = append(values, resource.Agent.Hostname)
	}
	values = append(values, resource.Identity.Hostnames...)
	values = append(values, resource.Identity.IPAddresses...)
	return appendNormalizedHosts(nil, values...)
}

func connectionSystemMemberKey(member ConnectionSystemMember) string {
	for _, candidate := range []string{
		strings.TrimSpace(member.Name),
		strings.TrimSpace(member.Endpoint),
		strings.TrimSpace(member.ID),
	} {
		if normalized := normalizeHost(candidate); normalized != "" {
			return normalized
		}
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func connectionSystemMemberStateFromNode(node models.Node) ConnectionState {
	status := strings.ToLower(strings.TrimSpace(node.Status))
	health := strings.ToLower(strings.TrimSpace(node.ConnectionHealth))
	switch {
	case status == "offline" || health == "error" || health == "offline" || health == "unhealthy":
		return ConnectionStateUnreachable
	case health == "degraded" || health == "unknown":
		return ConnectionStateStale
	case node.LastSeen.IsZero():
		return ConnectionStatePending
	default:
		return ConnectionStateActive
	}
}

func connectionSystemMemberStateFromResource(resource unified.Resource) ConnectionState {
	switch resource.Status {
	case unified.StatusOffline:
		return ConnectionStateUnreachable
	case unified.StatusWarning:
		return ConnectionStateStale
	case unified.StatusUnknown:
		return ConnectionStatePending
	default:
		if resource.LastSeen.IsZero() {
			return ConnectionStatePending
		}
		return ConnectionStateActive
	}
}

func attachedAgentConnectionID(
	rawAgentID string,
	primaryID string,
	connectionByID map[string]Connection,
	agentAttachments map[string]string,
) string {
	agentID := strings.TrimSpace(rawAgentID)
	if agentID == "" {
		return ""
	}
	connectionID := "agent:" + agentID
	connection, ok := connectionByID[connectionID]
	if !ok || connection.Type != ConnectionTypeAgent {
		return ""
	}
	if attachedPrimary := strings.TrimSpace(agentAttachments[connectionID]); attachedPrimary != "" &&
		attachedPrimary != primaryID {
		return ""
	}
	return connectionID
}

func isPrimaryProxmoxSystemMember(primary Connection, nodeName, endpoint string) bool {
	nodeName = strings.TrimSpace(nodeName)
	if nodeName != "" {
		for _, candidate := range []string{
			primary.Name,
			strings.TrimPrefix(primary.ID, "pve:"),
		} {
			if strings.EqualFold(nodeName, strings.TrimSpace(candidate)) {
				return true
			}
		}
	}

	return connectionsShareHost(
		Connection{Name: nodeName, Address: strings.TrimSpace(endpoint)},
		primary,
	)
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

	register := func(agentID, primaryID string, allowOverride, allowClusterAttach bool) {
		agentID = strings.TrimSpace(agentID)
		primaryID = strings.TrimSpace(primaryID)
		if agentID == "" || primaryID == "" || agentID == primaryID {
			return
		}
		agent, ok := connectionByID[agentID]
		if !ok {
			return
		}
		primary, ok := connectionByID[primaryID]
		if !ok || primary.Type == ConnectionTypeAgent {
			return
		}
		if !allowClusterAttach && !connectionsShareHost(agent, primary) {
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
		register(
			"agent:"+strings.TrimSpace(node.LinkedAgentID),
			"pve:"+strings.TrimSpace(node.Instance),
			false,
			true,
		)
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
			allowClusterAttach := resource.Proxmox != nil &&
				strings.TrimSpace(resource.Agent.LinkedNodeID) != ""
			register(agentID, primaryID, false, allowClusterAttach)
		}
	}

	for agentID, primaryID := range attachments {
		if primaryID == "" {
			delete(attachments, agentID)
		}
	}

	return attachments
}

func buildProxmoxClusterNames(
	connectionByID map[string]Connection,
	monitor *monitoring.Monitor,
) map[string]string {
	clusterNames := make(map[string]string)
	if monitor == nil {
		return clusterNames
	}

	record := func(primaryID, clusterName string) {
		primaryID = strings.TrimSpace(primaryID)
		clusterName = strings.TrimSpace(clusterName)
		if primaryID == "" || clusterName == "" {
			return
		}
		primary, ok := connectionByID[primaryID]
		if !ok || primary.Type != ConnectionTypePVE {
			return
		}
		if _, exists := clusterNames[primaryID]; exists {
			return
		}
		clusterNames[primaryID] = clusterName
	}

	for _, node := range monitor.NodesSnapshot() {
		if !node.IsClusterMember {
			continue
		}
		record("pve:"+strings.TrimSpace(node.Instance), node.ClusterName)
	}

	resources, _ := monitor.UnifiedResourceSnapshot()
	for _, resource := range resources {
		if resource.Proxmox == nil || !resource.Proxmox.IsClusterMember {
			continue
		}
		record("pve:"+strings.TrimSpace(resource.Proxmox.Instance), resource.Proxmox.ClusterName)
	}

	return clusterNames
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

// connectionHostCandidates returns every distinct normalized host string a
// connection could be reached as. Connections frequently carry both a name
// (often a hostname) and an address (often a URL or IP), and we want any
// overlap to count as "same host" for grouping purposes.
func connectionHostCandidates(connection Connection) []string {
	seen := make(map[string]struct{}, 2)
	out := make([]string, 0, 2)
	for _, candidate := range []string{connection.Address, connection.Name} {
		normalized := normalizeHost(candidate)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

// connectionsShareHost reports whether two connections appear to point at the
// same physical host. Used to gate agent→primary attachment so that sibling
// cluster members (e.g. a separate Proxmox node sharing the cluster API
// connection) stay visible as their own systems instead of being absorbed
// into the connection's primary row.
func connectionsShareHost(a, b Connection) bool {
	aHosts := connectionHostCandidates(a)
	if len(aHosts) == 0 {
		return false
	}
	bHosts := connectionHostCandidates(b)
	if len(bHosts) == 0 {
		return false
	}
	bSet := make(map[string]struct{}, len(bHosts))
	for _, host := range bHosts {
		bSet[host] = struct{}{}
	}
	for _, host := range aHosts {
		if _, ok := bSet[host]; ok {
			return true
		}
	}
	return false
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

	if parsedIP, _, err := net.ParseCIDR(value); err == nil && parsedIP != nil {
		return parsedIP.String()
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

func appendNormalizedHosts(existing []string, candidates ...string) []string {
	seen := make(map[string]struct{}, len(existing)+len(candidates))
	out := make([]string, 0, len(existing)+len(candidates))
	for _, value := range existing {
		normalized := normalizeHost(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	for _, candidate := range candidates {
		normalized := normalizeHost(candidate)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
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
