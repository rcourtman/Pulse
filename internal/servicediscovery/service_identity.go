package servicediscovery

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type knownServiceIdentity struct {
	ServiceType string
	ServiceName string
	Category    ServiceCategory
	Aliases     []string
	// Ports lists DISTINCTIVE listening ports that belong to essentially one
	// service (8123→HA, 32400→Plex). The port fast-path uses these to identify
	// an un-named workload without the model. Ambiguous ports (80, 443, 8080,
	// 3000, 5000) are deliberately omitted — a bad port guess is worse than
	// letting the model decide.
	Ports      []int
	Confidence float64
}

// knownServiceIdentities drives the deterministic surface fast-paths
// (inferSurfaceIdentity by name and inferSurfaceIdentityFromPorts by distinctive
// listening port, both run before the model) and the post-model identity
// improver (applyKnownServiceIdentity). Common homelab services are usually
// named after themselves and/or listen on a distinctive port, so they identify
// instantly — no model call. Aliases match the normalized name; Ports match
// exact listening ports.
var knownServiceIdentities = []knownServiceIdentity{
	{ServiceType: "home-assistant", ServiceName: "Home Assistant", Category: CategoryHomeAuto, Aliases: []string{"home assistant", "homeassistant", "hassio", "hass"}, Ports: []int{8123}, Confidence: 0.9},
	{ServiceType: "esphome", ServiceName: "ESPHome", Category: CategoryHomeAuto, Aliases: []string{"esphome", "esp-home", "esp home"}, Ports: []int{6052}, Confidence: 0.85},
	{ServiceType: "zigbee2mqtt", ServiceName: "Zigbee2MQTT", Category: CategoryHomeAuto, Aliases: []string{"zigbee2mqtt", "zigbee 2 mqtt"}, Confidence: 0.9},
	{ServiceType: "frigate", ServiceName: "Frigate NVR", Category: CategoryNVR, Aliases: []string{"frigate"}, Confidence: 0.9},
	{ServiceType: "mosquitto", ServiceName: "Mosquitto MQTT", Category: CategoryNetwork, Aliases: []string{"mosquitto", "mqtt"}, Ports: []int{1883, 8883}, Confidence: 0.85},
	{ServiceType: "postgresql", ServiceName: "PostgreSQL", Category: CategoryDatabase, Aliases: []string{"postgresql", "postgres"}, Ports: []int{5432}, Confidence: 0.9},
	{ServiceType: "mariadb", ServiceName: "MariaDB", Category: CategoryDatabase, Aliases: []string{"mariadb", "mysqld", "mysql"}, Ports: []int{3306}, Confidence: 0.85},
	{ServiceType: "redis", ServiceName: "Redis", Category: CategoryCache, Aliases: []string{"redis", "redis server"}, Ports: []int{6379}, Confidence: 0.9},
	{ServiceType: "influxdb", ServiceName: "InfluxDB", Category: CategoryDatabase, Aliases: []string{"influxdb", "influx"}, Ports: []int{8086}, Confidence: 0.85},
	{ServiceType: "plex", ServiceName: "Plex Media Server", Category: CategoryMedia, Aliases: []string{"plex media server", "plexmediaserver", "plex"}, Ports: []int{32400}, Confidence: 0.9},
	{ServiceType: "jellyfin", ServiceName: "Jellyfin", Category: CategoryMedia, Aliases: []string{"jellyfin"}, Ports: []int{8096}, Confidence: 0.9},
	{ServiceType: "nginx", ServiceName: "Nginx", Category: CategoryWebServer, Aliases: []string{"nginx"}, Confidence: 0.8},
	{ServiceType: "grafana", ServiceName: "Grafana", Category: CategoryMonitoring, Aliases: []string{"grafana"}, Confidence: 0.9},
	{ServiceType: "prometheus", ServiceName: "Prometheus", Category: CategoryMonitoring, Aliases: []string{"prometheus"}, Ports: []int{9090}, Confidence: 0.9},
	{ServiceType: "uptime-kuma", ServiceName: "Uptime Kuma", Category: CategoryMonitoring, Aliases: []string{"uptime kuma", "uptimekuma"}, Confidence: 0.9},
	{ServiceType: "pihole", ServiceName: "Pi-hole", Category: CategoryNetwork, Aliases: []string{"pi hole", "pihole"}, Confidence: 0.9},
	{ServiceType: "adguard", ServiceName: "AdGuard Home", Category: CategoryNetwork, Aliases: []string{"adguard home", "adguardhome", "adguard"}, Confidence: 0.9},
	{ServiceType: "tailscale", ServiceName: "Tailscale", Category: CategoryNetwork, Aliases: []string{"tailscale"}, Confidence: 0.85},
	{ServiceType: "unifi", ServiceName: "UniFi Controller", Category: CategoryNetwork, Aliases: []string{"unifi controller", "unifi"}, Confidence: 0.85},
	{ServiceType: "nextcloud", ServiceName: "Nextcloud", Category: CategoryStorage, Aliases: []string{"nextcloud"}, Confidence: 0.85},
}

func applyKnownServiceIdentity(
	discovery *ResourceDiscovery,
	req DiscoveryRequest,
	metadata map[string]any,
	commandOutputs map[string]string,
) (bool, string) {
	identity, evidence, ok := inferKnownServiceIdentity(discovery, req, metadata, commandOutputs)
	if !ok || discovery == nil {
		return false, ""
	}
	if !shouldApplyKnownServiceIdentity(discovery, identity) {
		return false, ""
	}

	changed := false
	if strings.TrimSpace(discovery.ServiceType) != identity.ServiceType {
		discovery.ServiceType = identity.ServiceType
		changed = true
	}
	if isLowValueServiceIdentity(discovery.ServiceName) {
		discovery.ServiceName = identity.ServiceName
		changed = true
	}
	if discovery.Category == "" || discovery.Category == CategoryUnknown {
		discovery.Category = identity.Category
		changed = true
	}
	if discovery.Confidence < identity.Confidence {
		discovery.Confidence = identity.Confidence
		changed = true
	}

	if changed && evidence != "" {
		note := fmt.Sprintf("Known service identity inferred from %s.", evidence)
		discovery.AIReasoning = appendDiscoveryReasoningNote(discovery.AIReasoning, note)
	}

	return changed, evidence
}

func shouldApplyKnownServiceIdentity(discovery *ResourceDiscovery, identity knownServiceIdentity) bool {
	if discovery == nil {
		return false
	}

	identityType := normalizeKnownServiceEvidence(identity.ServiceType)
	identityName := normalizeKnownServiceEvidence(identity.ServiceName)
	serviceType := normalizeKnownServiceEvidence(discovery.ServiceType)
	serviceName := normalizeKnownServiceEvidence(discovery.ServiceName)

	if serviceType != "" && serviceType != identityType && !isLowValueServiceIdentity(discovery.ServiceType) {
		return false
	}
	if serviceName != "" && serviceName != identityName && !isLowValueServiceIdentity(discovery.ServiceName) {
		return false
	}
	return true
}

func knownServiceIdentityWouldImprove(
	discovery *ResourceDiscovery,
	req DiscoveryRequest,
	metadata map[string]any,
	commandOutputs map[string]string,
) bool {
	if discovery == nil {
		return false
	}
	copy := cloneResourceDiscovery(discovery)
	changed, _ := applyKnownServiceIdentity(copy, req, metadata, commandOutputs)
	return changed
}

func inferKnownServiceIdentity(
	discovery *ResourceDiscovery,
	req DiscoveryRequest,
	metadata map[string]any,
	commandOutputs map[string]string,
) (knownServiceIdentity, string, bool) {
	candidates := knownServiceEvidenceCandidates(discovery, req, metadata, commandOutputs)
	for _, identity := range knownServiceIdentities {
		aliases := append([]string{identity.ServiceType, identity.ServiceName}, identity.Aliases...)
		for _, candidate := range candidates {
			normalizedCandidate := normalizeKnownServiceEvidence(candidate.Value)
			if normalizedCandidate == "" {
				continue
			}
			for _, alias := range aliases {
				normalizedAlias := normalizeKnownServiceEvidence(alias)
				if normalizedAlias == "" {
					continue
				}
				if normalizedCandidate == normalizedAlias ||
					strings.Contains(normalizedCandidate, normalizedAlias) {
					return identity, candidate.Source, true
				}
			}
		}
	}

	return knownServiceIdentity{}, "", false
}

// inferSurfaceIdentity is the discovery fast-path: deterministic, NAME-based
// identification that runs BEFORE the model. When a resource's name (hostname,
// id, or metadata name) clearly names a known service, we identify it instantly
// and skip the model entirely. Conservative on purpose — it only considers
// strong naming signals, never broad command-output text — so the model is
// skipped only on an obvious match; ambiguous workloads still fall through to
// the full analysis.
func inferSurfaceIdentity(req DiscoveryRequest, metadata map[string]any) (knownServiceIdentity, string, bool) {
	candidates := []knownServiceEvidenceCandidate{
		{Source: "resource name", Value: req.Hostname},
		{Source: "resource id", Value: req.ResourceID},
	}
	if name := stringMetadataValue(metadata, "name", "hostname", "display_name"); name != "" {
		candidates = append(candidates, knownServiceEvidenceCandidate{Source: "resource name", Value: name})
	}

	for _, identity := range knownServiceIdentities {
		aliases := append([]string{identity.ServiceType, identity.ServiceName}, identity.Aliases...)
		for _, candidate := range candidates {
			normalizedCandidate := normalizeKnownServiceEvidence(candidate.Value)
			if normalizedCandidate == "" {
				continue
			}
			for _, alias := range aliases {
				normalizedAlias := normalizeKnownServiceEvidence(alias)
				if normalizedAlias == "" {
					continue
				}
				if normalizedCandidate == normalizedAlias ||
					strings.Contains(normalizedCandidate, normalizedAlias) {
					return identity, candidate.Source, true
				}
			}
		}
	}
	return knownServiceIdentity{}, "", false
}

// listeningPortPattern extracts port numbers that appear after a colon in
// ss/netstat output (the local-address column, e.g. "0.0.0.0:8123"). Only
// distinctive ports are matched downstream, so any stray match is harmless.
var listeningPortPattern = regexp.MustCompile(`:(\d{2,5})\b`)

// parseListeningPorts returns the set of port numbers present in ss/netstat
// output.
func parseListeningPorts(output string) map[int]bool {
	ports := map[int]bool{}
	for _, match := range listeningPortPattern.FindAllStringSubmatch(output, -1) {
		if p, err := strconv.Atoi(match[1]); err == nil {
			ports[p] = true
		}
	}
	return ports
}

// inferSurfaceIdentityFromPorts is the second deterministic fast-path: when the
// name doesn't identify the workload, a distinctive listening port often does
// (8123→Home Assistant, 32400→Plex). Only ports that belong to essentially one
// service are in the table, so a match is high-confidence and the model can be
// skipped — important when the configured model is slow and would otherwise time
// out on an un-named workload.
func inferSurfaceIdentityFromPorts(listeningPortsOutput string) (knownServiceIdentity, string, bool) {
	if strings.TrimSpace(listeningPortsOutput) == "" {
		return knownServiceIdentity{}, "", false
	}
	ports := parseListeningPorts(listeningPortsOutput)
	if len(ports) == 0 {
		return knownServiceIdentity{}, "", false
	}
	for _, identity := range knownServiceIdentities {
		for _, p := range identity.Ports {
			if ports[p] {
				return identity, fmt.Sprintf("listening port %d", p), true
			}
		}
	}
	return knownServiceIdentity{}, "", false
}

// surfaceIdentityResponse builds a no-model discovery result from a fast-path
// match. Identity only: config/data/log paths are intentionally left empty —
// the Assistant knows standard service layouts and fetches specifics on demand,
// and cli_access is derived downstream from the resource type.
func surfaceIdentityResponse(identity knownServiceIdentity, evidence string) *AIAnalysisResponse {
	return &AIAnalysisResponse{
		ServiceType: identity.ServiceType,
		ServiceName: identity.ServiceName,
		Category:    identity.Category,
		Confidence:  identity.Confidence,
		Reasoning:   fmt.Sprintf("Identified from %s — fast surface match, no model call.", evidence),
	}
}

// knownServiceAliases returns the name aliases for a service type from the
// identity table (falling back to the type itself), used to match a service
// against nested container names/images.
func knownServiceAliases(serviceType string) []string {
	for _, identity := range knownServiceIdentities {
		if identity.ServiceType == serviceType {
			return append([]string{identity.ServiceType, identity.ServiceName}, identity.Aliases...)
		}
	}
	return []string{serviceType}
}

// nestedContainerForService inspects the nested-container probe output
// ("name|image" lines) and returns the name of a Docker container that appears
// to run the identified service. This is access topology: when the service runs
// in a nested container, the access path must be layered (enter the container),
// not the bare guest shell. Empty string if nothing matches.
func nestedContainerForService(probeOutput, serviceType, serviceName string) string {
	probeOutput = strings.TrimSpace(probeOutput)
	if probeOutput == "" {
		return ""
	}
	aliases := append(knownServiceAliases(serviceType), serviceName)
	normalizedAliases := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if na := normalizeKnownServiceEvidence(alias); na != "" {
			normalizedAliases = append(normalizedAliases, na)
		}
	}
	if len(normalizedAliases) == 0 {
		return ""
	}

	for _, line := range strings.Split(probeOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		image := ""
		if len(parts) > 1 {
			image = strings.TrimSpace(parts[1])
		}
		normalizedName := normalizeKnownServiceEvidence(name)
		normalizedImage := normalizeKnownServiceEvidence(image)
		for _, na := range normalizedAliases {
			if (normalizedName != "" && strings.Contains(normalizedName, na)) ||
				(normalizedImage != "" && strings.Contains(normalizedImage, na)) {
				return name
			}
		}
	}
	return ""
}

// withNestedDockerAccess rewrites cli_access guidance so the Assistant enters
// the nested Docker container that actually runs the service, rather than the
// bare guest shell.
func withNestedDockerAccess(cliAccess, containerName string) string {
	cliAccess = strings.TrimSpace(cliAccess)
	note := fmt.Sprintf(
		"The service runs inside a Docker container named %q on this guest — to reach the service itself, prefix commands with: docker exec %s <your-command>.",
		containerName, containerName,
	)
	if cliAccess == "" {
		return note
	}
	return cliAccess + " " + note
}

type knownServiceEvidenceCandidate struct {
	Source string
	Value  string
}

func knownServiceEvidenceCandidates(
	discovery *ResourceDiscovery,
	req DiscoveryRequest,
	metadata map[string]any,
	commandOutputs map[string]string,
) []knownServiceEvidenceCandidate {
	candidates := []knownServiceEvidenceCandidate{
		{Source: "resource hostname", Value: req.Hostname},
		{Source: "resource id", Value: req.ResourceID},
	}
	if discovery != nil {
		candidates = append(candidates,
			knownServiceEvidenceCandidate{Source: "stored hostname", Value: discovery.Hostname},
			knownServiceEvidenceCandidate{Source: "stored service type", Value: discovery.ServiceType},
			knownServiceEvidenceCandidate{Source: "stored service name", Value: discovery.ServiceName},
		)
		for _, fact := range discovery.Facts {
			candidates = append(candidates,
				knownServiceEvidenceCandidate{Source: "discovered fact " + fact.Key, Value: fact.Key},
				knownServiceEvidenceCandidate{Source: "discovered fact " + fact.Key, Value: fact.Value},
			)
		}
	}

	for key, value := range metadata {
		candidates = appendAnyEvidenceCandidate(candidates, "metadata "+key, value)
	}
	for key, output := range commandOutputs {
		candidates = append(candidates, knownServiceEvidenceCandidate{
			Source: "command output " + key,
			Value:  output,
		})
	}

	return candidates
}

func appendAnyEvidenceCandidate(
	candidates []knownServiceEvidenceCandidate,
	source string,
	value any,
) []knownServiceEvidenceCandidate {
	switch typed := value.(type) {
	case string:
		return append(candidates, knownServiceEvidenceCandidate{Source: source, Value: typed})
	case []string:
		for _, item := range typed {
			candidates = append(candidates, knownServiceEvidenceCandidate{Source: source, Value: item})
		}
	case []any:
		for _, item := range typed {
			candidates = appendAnyEvidenceCandidate(candidates, source, item)
		}
	case map[string]string:
		for key, item := range typed {
			candidates = append(candidates,
				knownServiceEvidenceCandidate{Source: source + " " + key, Value: key},
				knownServiceEvidenceCandidate{Source: source + " " + key, Value: item},
			)
		}
	case map[string]any:
		for key, item := range typed {
			candidates = appendAnyEvidenceCandidate(candidates, source+" "+key, item)
		}
	}
	return candidates
}

func normalizeKnownServiceEvidence(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		".", " ",
		"/", " ",
		"\\", " ",
		":", " ",
		"@", " ",
	)
	normalized = replacer.Replace(normalized)
	return strings.Join(strings.Fields(normalized), " ")
}

func isLowValueServiceIdentity(value string) bool {
	normalized := normalizeKnownServiceEvidence(value)
	if normalized == "" {
		return true
	}
	switch normalized {
	case "detected",
		"app",
		"application",
		"container",
		"generic host",
		"host",
		"linux",
		"lxc",
		"service",
		"system container",
		"unknown",
		"unknown app",
		"unknown application",
		"unknown container",
		"unknown host",
		"unknown service",
		"unknown system container",
		"unknown virtual machine",
		"unknown vm",
		"unknown workload",
		"virtual machine",
		"vm",
		"workload":
		return true
	default:
		return false
	}
}

func appendDiscoveryReasoningNote(reasoning, note string) string {
	reasoning = strings.TrimSpace(reasoning)
	note = strings.TrimSpace(note)
	if note == "" || strings.Contains(reasoning, note) {
		return reasoning
	}
	if reasoning == "" {
		return note
	}
	return reasoning + " " + note
}
