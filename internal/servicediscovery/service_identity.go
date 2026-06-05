package servicediscovery

import (
	"fmt"
	"strings"
)

type knownServiceIdentity struct {
	ServiceType string
	ServiceName string
	Category    ServiceCategory
	Aliases     []string
	Confidence  float64
}

var knownServiceIdentities = []knownServiceIdentity{
	{
		ServiceType: "esphome",
		ServiceName: "ESPHome",
		Category:    CategoryHomeAuto,
		Aliases:     []string{"esphome", "esp-home", "esp home"},
		Confidence:  0.85,
	},
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
