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
		ensurePVEClusterNodeIdentitiesForInstance(&instances[i], clusterNameCounts)
	}
	return !reflect.DeepEqual(before, instances)
}

func ensurePVEClusterNodeIdentitiesForInstance(instance *PVEInstance, clusterNameCounts map[string]int) {
	if instance == nil || !instance.IsCluster {
		return
	}

	scope := strings.TrimSpace(instance.Name)
	clusterName := strings.TrimSpace(instance.ClusterName)
	if clusterName != "" && clusterNameCounts[strings.ToLower(clusterName)] == 1 {
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
			if seed == "" || identityIDConflicts(instance.ClusterNodeIdentities, candidate, endpoint.NativeNodeID, endpoint.NodeName) {
				candidate = deterministicPVEClusterNodeIdentity(scope, *endpoint, endpointIdx)
				for suffix := 1; identityIDExists(instance.ClusterNodeIdentities, candidate); suffix++ {
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
			for suffix := 1; identityIDExists(instance.ClusterNodeIdentities, candidate); suffix++ {
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
