package config

import (
	"net"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// ConsolidatePVEInstances canonicalizes PVE instance configuration by:
//   - merging duplicate cluster definitions with the same ClusterName
//   - removing standalone instances whose endpoint is already represented by
//     a unique cluster endpoint
//
// It returns a normalized copy of the slice and whether any change was applied.
func ConsolidatePVEInstances(instances []PVEInstance) ([]PVEInstance, bool) {
	if len(instances) < 2 {
		return instances, false
	}

	working := clonePVEInstances(instances)
	changed := mergeDuplicateClusterInstances(working)
	working = dedupeClusterInstances(working)

	standaloneMerged := mergeStandalonePVEIntoClusters(working)
	if standaloneMerged {
		working = removeMergedStandaloneInstances(working)
		changed = true
	}

	if !changed {
		return instances, false
	}
	return working, true
}

func clonePVEInstances(instances []PVEInstance) []PVEInstance {
	out := make([]PVEInstance, len(instances))
	copy(out, instances)
	for i := range out {
		if len(instances[i].ClusterEndpoints) > 0 {
			out[i].ClusterEndpoints = make([]ClusterEndpoint, len(instances[i].ClusterEndpoints))
			copy(out[i].ClusterEndpoints, instances[i].ClusterEndpoints)
		}
	}
	return out
}

func mergeDuplicateClusterInstances(instances []PVEInstance) bool {
	clusterGroups := make(map[string][]int)
	for i, instance := range instances {
		if instance.IsCluster && strings.TrimSpace(instance.ClusterName) != "" {
			clusterGroups[strings.TrimSpace(instance.ClusterName)] = append(clusterGroups[strings.TrimSpace(instance.ClusterName)], i)
		}
	}

	mergedAny := false
	for clusterName, indices := range clusterGroups {
		if len(indices) < 2 {
			continue
		}

		log.Warn().
			Str("cluster", clusterName).
			Int("duplicates", len(indices)).
			Msg("Detected duplicate cluster instances - consolidating")

		primaryIdx := indices[0]
		primary := &instances[primaryIdx]

		existingEndpoints := make(map[string]int)
		for idx, ep := range primary.ClusterEndpoints {
			existingEndpoints[strings.TrimSpace(strings.ToLower(ep.NodeName))] = idx
		}

		for _, dupIdx := range indices[1:] {
			duplicate := instances[dupIdx]
			log.Info().
				Str("cluster", clusterName).
				Str("primary", primary.Name).
				Str("duplicate", duplicate.Name).
				Msg("Merging duplicate cluster instance")

			mergePVEInstanceData(primary, duplicate)

			for _, ep := range duplicate.ClusterEndpoints {
				nodeKey := strings.TrimSpace(strings.ToLower(ep.NodeName))
				if existingIdx, ok := existingEndpoints[nodeKey]; ok {
					mergeClusterEndpointData(&primary.ClusterEndpoints[existingIdx], ep)
					continue
				}
				primary.ClusterEndpoints = append(primary.ClusterEndpoints, ep)
				existingEndpoints[nodeKey] = len(primary.ClusterEndpoints) - 1
				log.Info().
					Str("cluster", clusterName).
					Str("endpoint", ep.NodeName).
					Msg("Added endpoint from duplicate cluster instance")
			}
		}

		mergedAny = true
	}

	return mergedAny
}

func mergeClusterEndpointData(dst *ClusterEndpoint, src ClusterEndpoint) {
	if dst == nil {
		return
	}
	if dst.Host == "" && strings.TrimSpace(src.Host) != "" {
		dst.Host = strings.TrimSpace(src.Host)
	}
	if dst.GuestURL == "" && strings.TrimSpace(src.GuestURL) != "" {
		dst.GuestURL = strings.TrimSpace(src.GuestURL)
	}
	if dst.IP == "" && strings.TrimSpace(src.IP) != "" {
		dst.IP = strings.TrimSpace(src.IP)
	}
	if dst.IPOverride == "" && strings.TrimSpace(src.IPOverride) != "" {
		dst.IPOverride = strings.TrimSpace(src.IPOverride)
	}
	if dst.Fingerprint == "" && strings.TrimSpace(src.Fingerprint) != "" {
		dst.Fingerprint = strings.TrimSpace(src.Fingerprint)
	}
	if !dst.Online && src.Online {
		dst.Online = true
	}
	if dst.LastSeen.IsZero() || src.LastSeen.After(dst.LastSeen) {
		dst.LastSeen = src.LastSeen
	}
	if dst.PulseReachable == nil && src.PulseReachable != nil {
		reachable := *src.PulseReachable
		dst.PulseReachable = &reachable
	}
	if dst.LastPulseCheck == nil && src.LastPulseCheck != nil {
		lastCheck := *src.LastPulseCheck
		dst.LastPulseCheck = &lastCheck
	}
	if dst.PulseError == "" && strings.TrimSpace(src.PulseError) != "" {
		dst.PulseError = strings.TrimSpace(src.PulseError)
	}
}

func mergePVEInstanceData(dst *PVEInstance, src PVEInstance) {
	if dst == nil {
		return
	}

	if dst.Host == "" && strings.TrimSpace(src.Host) != "" {
		dst.Host = strings.TrimSpace(src.Host)
	}
	if dst.GuestURL == "" && strings.TrimSpace(src.GuestURL) != "" {
		dst.GuestURL = strings.TrimSpace(src.GuestURL)
	}
	if dst.Fingerprint == "" && strings.TrimSpace(src.Fingerprint) != "" {
		dst.Fingerprint = strings.TrimSpace(src.Fingerprint)
	}
	if dst.Source == "" && strings.TrimSpace(src.Source) != "" {
		dst.Source = strings.TrimSpace(src.Source)
	}
	if !dst.VerifySSL && src.VerifySSL {
		dst.VerifySSL = true
	}
	if dst.TemperatureMonitoringEnabled == nil && src.TemperatureMonitoringEnabled != nil {
		enabled := *src.TemperatureMonitoringEnabled
		dst.TemperatureMonitoringEnabled = &enabled
	}
	if dst.MonitorPhysicalDisks == nil && src.MonitorPhysicalDisks != nil {
		enabled := *src.MonitorPhysicalDisks
		dst.MonitorPhysicalDisks = &enabled
	}
	if dst.SSHPort == 0 && src.SSHPort != 0 {
		dst.SSHPort = src.SSHPort
	}
	if dst.PhysicalDiskPollingMinutes == 0 && src.PhysicalDiskPollingMinutes != 0 {
		dst.PhysicalDiskPollingMinutes = src.PhysicalDiskPollingMinutes
	}

	switch {
	case dst.TokenName != "" || dst.TokenValue != "":
		if dst.TokenName == "" && strings.TrimSpace(src.TokenName) != "" {
			dst.TokenName = strings.TrimSpace(src.TokenName)
		}
		if dst.TokenValue == "" && strings.TrimSpace(src.TokenValue) != "" {
			dst.TokenValue = strings.TrimSpace(src.TokenValue)
		}
	case dst.User != "" || dst.Password != "":
		if dst.User == "" && strings.TrimSpace(src.User) != "" {
			dst.User = strings.TrimSpace(src.User)
		}
		if dst.Password == "" && src.Password != "" {
			dst.Password = src.Password
		}
	case strings.TrimSpace(src.TokenName) != "" && strings.TrimSpace(src.TokenValue) != "":
		dst.TokenName = strings.TrimSpace(src.TokenName)
		dst.TokenValue = strings.TrimSpace(src.TokenValue)
		dst.User = ""
		dst.Password = ""
	case strings.TrimSpace(src.User) != "" && src.Password != "":
		dst.User = strings.TrimSpace(src.User)
		dst.Password = src.Password
	}
}

func dedupeClusterInstances(instances []PVEInstance) []PVEInstance {
	out := make([]PVEInstance, 0, len(instances))
	seenClusters := make(map[string]bool)

	for _, instance := range instances {
		clusterName := strings.TrimSpace(instance.ClusterName)
		if instance.IsCluster && clusterName != "" {
			if seenClusters[clusterName] {
				log.Info().
					Str("cluster", clusterName).
					Str("instance", instance.Name).
					Msg("Removing duplicate cluster instance")
				continue
			}
			seenClusters[clusterName] = true
		}
		out = append(out, instance)
	}

	return out
}

func normalizePVEEndpointIdentity(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if !strings.HasPrefix(strings.ToLower(value), "http://") && !strings.HasPrefix(strings.ToLower(value), "https://") {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}

	host := strings.TrimSpace(strings.ToLower(parsed.Hostname()))
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}

	port := parsed.Port()
	if port == "" {
		port = DefaultPVEPort
	}

	return net.JoinHostPort(host, port)
}

func clusterEndpointIdentityKeys(endpoint ClusterEndpoint) []string {
	keys := make([]string, 0, 3)
	if key := normalizePVEEndpointIdentity(endpoint.Host); key != "" {
		keys = append(keys, key)
	}
	if key := normalizePVEEndpointIdentity(endpoint.IP); key != "" {
		keys = append(keys, key)
	}
	if key := normalizePVEEndpointIdentity(endpoint.IPOverride); key != "" {
		keys = append(keys, key)
	}
	return keys
}

func mergeStandalonePVEIntoClusters(instances []PVEInstance) bool {
	type endpointRef struct {
		clusterIdx  int
		endpointIdx int
	}

	endpointOwners := make(map[string]endpointRef)
	ambiguousKeys := make(map[string]struct{})
	registerKey := func(key string, ref endpointRef) {
		if key == "" {
			return
		}
		if _, ambiguous := ambiguousKeys[key]; ambiguous {
			return
		}
		if existing, ok := endpointOwners[key]; ok && existing != ref {
			delete(endpointOwners, key)
			ambiguousKeys[key] = struct{}{}
			return
		}
		endpointOwners[key] = ref
	}

	for clusterIdx := range instances {
		if !instances[clusterIdx].IsCluster {
			continue
		}
		for endpointIdx, endpoint := range instances[clusterIdx].ClusterEndpoints {
			for _, key := range clusterEndpointIdentityKeys(endpoint) {
				registerKey(key, endpointRef{clusterIdx: clusterIdx, endpointIdx: endpointIdx})
			}
		}
	}

	mergedAny := false
	for idx := range instances {
		if instances[idx].IsCluster {
			continue
		}

		key := normalizePVEEndpointIdentity(instances[idx].Host)
		if key == "" {
			continue
		}
		if _, ambiguous := ambiguousKeys[key]; ambiguous {
			continue
		}
		ref, ok := endpointOwners[key]
		if !ok {
			continue
		}
		if ref.endpointIdx >= len(instances[ref.clusterIdx].ClusterEndpoints) {
			continue
		}

		endpoint := &instances[ref.clusterIdx].ClusterEndpoints[ref.endpointIdx]
		mergeClusterEndpointData(endpoint, ClusterEndpoint{
			Host:        instances[idx].Host,
			GuestURL:    instances[idx].GuestURL,
			Fingerprint: instances[idx].Fingerprint,
		})
		mergePVEInstanceData(&instances[ref.clusterIdx], instances[idx])
		instances[idx].Source = "__merged_into_cluster__"

		log.Warn().
			Str("standalone", instances[idx].Name).
			Str("standaloneHost", instances[idx].Host).
			Str("cluster", instances[ref.clusterIdx].Name).
			Str("node", endpoint.NodeName).
			Msg("Detected standalone PVE instance already covered by cluster endpoint - consolidating")
		mergedAny = true
	}

	return mergedAny
}

func removeMergedStandaloneInstances(instances []PVEInstance) []PVEInstance {
	out := make([]PVEInstance, 0, len(instances))
	for _, instance := range instances {
		if instance.Source == "__merged_into_cluster__" {
			continue
		}
		out = append(out, instance)
	}
	return out
}
