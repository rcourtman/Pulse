package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// EnsurePVEClusterNodeIdentities migrates and repairs persisted cluster-node
// identities in place. Existing IDs are never derived again once recorded.
func EnsurePVEClusterNodeIdentities(instances []PVEInstance) bool {
	before := clonePVEInstances(instances)
	clusterNameCounts := make(map[string]int)
	for i := range instances {
		if !instances[i].IsCluster {
			continue
		}
		if name := strings.ToLower(strings.TrimSpace(instances[i].ClusterName)); name != "" {
			clusterNameCounts[name]++
		}
	}

	for i := range instances {
		ensurePVEClusterNodeIdentitiesForInstance(instances, &instances[i], clusterNameCounts)
	}
	return !reflect.DeepEqual(before, instances)
}

func ensurePVEClusterNodeIdentitiesForInstance(instances []PVEInstance, instance *PVEInstance, clusterNameCounts map[string]int) {
	if instance == nil || !instance.IsCluster {
		return
	}

	// The cluster name is what the endpoint reports about itself. It scopes
	// node identities only while no other cluster reports it and no other
	// connection is named it: a connection's name scopes its own nodes.
	connectionScope := strings.TrimSpace(instance.Name)
	scope := connectionScope
	clusterName := strings.TrimSpace(instance.ClusterName)
	if clusterName != "" && clusterNameCounts[strings.ToLower(clusterName)] == 1 &&
		!pveConnectionNameHeldElsewhere(instances, instance.Name, clusterName) {
		scope = clusterName
	}
	if scope == "" {
		scope = "proxmox"
	}

	normalized := make([]PVEClusterNodeIdentity, 0, len(instance.ClusterNodeIdentities)+len(instance.ClusterEndpoints))
	byID := make(map[string]int)
	for _, identity := range instance.ClusterNodeIdentities {
		identity.ID = strings.TrimSpace(identity.ID)
		identity.NativeName = strings.TrimSpace(identity.NativeName)
		identity.NativeAliases = appendUniquePVEIdentityAliases(nil, identity.NativeAliases...)
		identity.DisplayName = strings.TrimSpace(identity.DisplayName)
		if identity.ID == "" {
			continue
		}
		if existingIdx, exists := byID[identity.ID]; exists {
			existing := &normalized[existingIdx]
			if existing.NativeNodeID == 0 {
				existing.NativeNodeID = identity.NativeNodeID
			}
			if existing.NativeName == "" {
				existing.NativeName = identity.NativeName
			}
			if existing.DisplayName == "" {
				existing.DisplayName = identity.DisplayName
			}
			existing.NativeAliases = appendUniquePVEIdentityAliases(existing.NativeAliases, identity.NativeAliases...)
			continue
		}
		byID[identity.ID] = len(normalized)
		normalized = append(normalized, identity)
	}
	instance.ClusterNodeIdentities = normalized

	usedIdentity := make(map[string]int)
	for endpointIdx := range instance.ClusterEndpoints {
		endpoint := &instance.ClusterEndpoints[endpointIdx]
		endpoint.NodeIdentity = strings.TrimSpace(endpoint.NodeIdentity)
		endpoint.NodeName = strings.TrimSpace(endpoint.NodeName)

		identityIdx := findPVEClusterNodeIdentityIndex(*instance, *endpoint)
		if identityIdx < 0 && endpoint.NodeIdentity != "" {
			identityIdx = len(instance.ClusterNodeIdentities)
			instance.ClusterNodeIdentities = append(instance.ClusterNodeIdentities, PVEClusterNodeIdentity{
				ID:           endpoint.NodeIdentity,
				NativeNodeID: endpoint.NativeNodeID,
				NativeName:   endpoint.NodeName,
			})
		}
		if identityIdx < 0 {
			seed := endpoint.NodeName
			if seed == "" {
				seed = endpoint.NodeID
			}
			candidate := scope + "-" + seed
			if connectionScope != "" && scope != connectionScope &&
				pveNodeIdentityHeldElsewhere(instances, instance.Name, scope, candidate) {
				// "<cluster>-<node>" splits at any hyphen, so a reported name
				// can still spell another connection's node. The connection's
				// own name scopes this cluster from here on.
				scope = connectionScope
				candidate = scope + "-" + seed
			}
			if seed == "" || identityIDConflicts(instance.ClusterNodeIdentities, candidate, endpoint.NativeNodeID, endpoint.NodeName) ||
				pveNodeIdentityHeldElsewhere(instances, instance.Name, scope, candidate) {
				candidate = deterministicPVEClusterNodeIdentity(scope, *endpoint, endpointIdx)
				for suffix := 1; identityIDExists(instance.ClusterNodeIdentities, candidate) ||
					pveNodeIdentityRecordedElsewhere(instances, instance.Name, candidate); suffix++ {
					candidate = deterministicPVEClusterNodeIdentity(scope, *endpoint, endpointIdx+suffix)
				}
			}
			identityIdx = len(instance.ClusterNodeIdentities)
			instance.ClusterNodeIdentities = append(instance.ClusterNodeIdentities, PVEClusterNodeIdentity{
				ID:           candidate,
				NativeNodeID: endpoint.NativeNodeID,
				NativeName:   endpoint.NodeName,
			})
		}

		identity := &instance.ClusterNodeIdentities[identityIdx]
		if previousEndpointIdx, duplicate := usedIdentity[identity.ID]; duplicate &&
			!samePVEClusterNode(instance.ClusterEndpoints[previousEndpointIdx], *endpoint) {
			candidate := deterministicPVEClusterNodeIdentity(scope, *endpoint, endpointIdx)
			for suffix := 1; identityIDExists(instance.ClusterNodeIdentities, candidate) ||
				pveNodeIdentityRecordedElsewhere(instances, instance.Name, candidate); suffix++ {
				candidate = deterministicPVEClusterNodeIdentity(scope, *endpoint, endpointIdx+suffix)
			}
			identityIdx = len(instance.ClusterNodeIdentities)
			instance.ClusterNodeIdentities = append(instance.ClusterNodeIdentities, PVEClusterNodeIdentity{
				ID:           candidate,
				NativeNodeID: endpoint.NativeNodeID,
				NativeName:   endpoint.NodeName,
			})
			identity = &instance.ClusterNodeIdentities[identityIdx]
		}

		endpoint.NodeIdentity = identity.ID
		if endpoint.NativeNodeID != 0 {
			identity.NativeNodeID = endpoint.NativeNodeID
		} else if identity.NativeNodeID != 0 {
			endpoint.NativeNodeID = identity.NativeNodeID
		}
		if endpoint.NodeName != "" {
			if identity.NativeName != "" && identity.NativeName != endpoint.NodeName {
				identity.NativeAliases = appendUniquePVEIdentityAliases(identity.NativeAliases, identity.NativeName)
			}
			identity.NativeName = endpoint.NodeName
		}
		usedIdentity[identity.ID] = endpointIdx
	}
}

func findPVEClusterNodeIdentityIndex(instance PVEInstance, endpoint ClusterEndpoint) int {
	if endpoint.NodeIdentity != "" {
		for i := range instance.ClusterNodeIdentities {
			if instance.ClusterNodeIdentities[i].ID == endpoint.NodeIdentity {
				return i
			}
		}
	}
	if endpoint.NativeNodeID != 0 {
		match := -1
		for i := range instance.ClusterNodeIdentities {
			if instance.ClusterNodeIdentities[i].NativeNodeID != endpoint.NativeNodeID {
				continue
			}
			if match >= 0 {
				return -1
			}
			match = i
		}
		return match
	}
	if endpoint.NodeName != "" {
		match := -1
		for i := range instance.ClusterNodeIdentities {
			if !strings.EqualFold(instance.ClusterNodeIdentities[i].NativeName, endpoint.NodeName) {
				continue
			}
			if match >= 0 {
				return -1
			}
			match = i
		}
		return match
	}
	return -1
}

// pveConnectionNameHeldElsewhere reports whether a connection other than
// owner is configured under name.
func pveConnectionNameHeldElsewhere(instances []PVEInstance, owner, name string) bool {
	owner = strings.TrimSpace(owner)
	name = strings.TrimSpace(name)
	for i := range instances {
		other := strings.TrimSpace(instances[i].Name)
		if !strings.EqualFold(other, owner) && strings.EqualFold(other, name) {
			return true
		}
	}
	return false
}

// pveNodeIdentityRecordedElsewhere reports whether a connection other than
// owner already records id for one of its cluster nodes.
func pveNodeIdentityRecordedElsewhere(instances []PVEInstance, owner, id string) bool {
	owner = strings.TrimSpace(owner)
	for i := range instances {
		if strings.EqualFold(strings.TrimSpace(instances[i].Name), owner) {
			continue
		}
		for _, identity := range instances[i].ClusterNodeIdentities {
			if strings.EqualFold(identity.ID, id) {
				return true
			}
		}
	}
	return false
}

// pveNodeIdentityHeldElsewhere reports whether id, derived under scope for
// the connection named owner, reads as a node of another connection. Node
// state, history and agent links are keyed on the identity alone, so it must
// name one connection's node only. Another connection holds id when it
// records it, or when id lies in its "<connection name>-" namespace, the
// scope of every node a standalone connection reports. Connection names may
// nest ("pve" and "pve-b"); a connection scoped by its own name keeps the
// namespace below it and the shorter name gives way.
func pveNodeIdentityHeldElsewhere(instances []PVEInstance, owner, scope, id string) bool {
	owner = strings.ToLower(strings.TrimSpace(owner))
	scope = strings.ToLower(strings.TrimSpace(scope))
	id = strings.ToLower(id)
	for i := range instances {
		other := strings.ToLower(strings.TrimSpace(instances[i].Name))
		if other == "" || other == owner || !strings.HasPrefix(id, other+"-") {
			continue
		}
		if scope != owner || !strings.HasPrefix(owner, other+"-") {
			return true
		}
	}
	return pveNodeIdentityRecordedElsewhere(instances, owner, id)
}

// DerivePVENodeIdentity returns the identity of a node that has no recorded
// one: the node of a standalone connection, or a cluster member polled before
// the membership ledger lists it. It is "<scope>-<native name>" unless another
// connection holds that identity.
func DerivePVENodeIdentity(instances []PVEInstance, instanceName, scope, nativeName string) string {
	id := scope + "-" + nativeName
	if !pveNodeIdentityHeldElsewhere(instances, instanceName, scope, id) {
		return id
	}
	connectionScope := strings.TrimSpace(instanceName)
	if id = connectionScope + "-" + nativeName; !pveNodeIdentityHeldElsewhere(instances, instanceName, connectionScope, id) {
		return id
	}
	endpoint := ClusterEndpoint{NodeName: nativeName}
	for ordinal := 0; ; ordinal++ {
		id = deterministicPVEClusterNodeIdentity(connectionScope, endpoint, ordinal)
		if !pveNodeIdentityRecordedElsewhere(instances, instanceName, id) {
			return id
		}
	}
}

func identityIDConflicts(identities []PVEClusterNodeIdentity, id string, nativeID int, nativeName string) bool {
	for _, identity := range identities {
		if identity.ID != id {
			continue
		}
		if nativeID != 0 && identity.NativeNodeID != 0 {
			return nativeID != identity.NativeNodeID
		}
		return !strings.EqualFold(identity.NativeName, nativeName)
	}
	return false
}

// NormalizePVEClusterNodeDisplayName validates and normalizes one
// presentation-only node label. Empty is valid and clears the override.
func NormalizePVEClusterNodeDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 128 {
		return "", fmt.Errorf("must be valid Unicode and at most 128 characters")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("must not contain control characters")
		}
	}
	return value, nil
}

func identityIDExists(identities []PVEClusterNodeIdentity, id string) bool {
	for _, identity := range identities {
		if identity.ID == id {
			return true
		}
	}
	return false
}

func deterministicPVEClusterNodeIdentity(scope string, endpoint ClusterEndpoint, ordinal int) string {
	seed := strings.Join([]string{
		scope,
		endpoint.NodeID,
		endpoint.NodeName,
		endpoint.Host,
		endpoint.IP,
		strconv.Itoa(endpoint.NativeNodeID),
		strconv.Itoa(ordinal),
	}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	return scope + "-node-" + hex.EncodeToString(sum[:6])
}

func samePVEClusterNode(left, right ClusterEndpoint) bool {
	if left.NativeNodeID != 0 && right.NativeNodeID != 0 {
		return left.NativeNodeID == right.NativeNodeID
	}
	return strings.EqualFold(strings.TrimSpace(left.NodeName), strings.TrimSpace(right.NodeName))
}

// PVEClusterNodeIdentityForName returns the durable node identity associated
// with the provider's current native node name.
func PVEClusterNodeIdentityForName(instance *PVEInstance, nativeName string) string {
	if instance == nil {
		return ""
	}
	nativeName = strings.TrimSpace(nativeName)
	for _, endpoint := range instance.ClusterEndpoints {
		if strings.TrimSpace(endpoint.NodeName) == nativeName {
			return endpoint.NodeIdentity
		}
	}
	match := ""
	for _, endpoint := range instance.ClusterEndpoints {
		if !strings.EqualFold(strings.TrimSpace(endpoint.NodeName), nativeName) {
			continue
		}
		if match != "" {
			return ""
		}
		match = endpoint.NodeIdentity
	}
	return match
}

// PVEClusterNodePresentation returns the immutable identity and preferred
// display label for a native node name. The native name is the deterministic
// fallback when no override exists.
func PVEClusterNodePresentation(instance *PVEInstance, nativeName string) (string, string) {
	nativeName = strings.TrimSpace(nativeName)
	if instance == nil {
		return "", nativeName
	}
	identityID := PVEClusterNodeIdentityForName(instance, nativeName)
	for _, identity := range instance.ClusterNodeIdentities {
		if identity.ID != identityID {
			continue
		}
		if identity.DisplayName != "" {
			return identity.ID, identity.DisplayName
		}
		if identity.NativeName != "" {
			return identity.ID, identity.NativeName
		}
		return identity.ID, nativeName
	}
	return identityID, nativeName
}

// PVEClusterNodeIdentityByID returns a copy of one persisted identity.
func PVEClusterNodeIdentityByID(instance *PVEInstance, identityID string) (PVEClusterNodeIdentity, bool) {
	if instance == nil {
		return PVEClusterNodeIdentity{}, false
	}
	for _, identity := range instance.ClusterNodeIdentities {
		if identity.ID == identityID {
			identity.NativeAliases = append([]string(nil), identity.NativeAliases...)
			return identity, true
		}
	}
	return PVEClusterNodeIdentity{}, false
}

// PVEClusterNodeNativeAliases returns prior provider-native names retained for
// diagnostics, history relabeling, and search after a native rename.
func PVEClusterNodeNativeAliases(instance *PVEInstance, nativeName string) []string {
	identityID := PVEClusterNodeIdentityForName(instance, nativeName)
	identity, ok := PVEClusterNodeIdentityByID(instance, identityID)
	if !ok {
		return nil
	}
	return identity.NativeAliases
}
