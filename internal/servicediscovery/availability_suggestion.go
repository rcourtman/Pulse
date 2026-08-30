package servicediscovery

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"
)

var (
	ErrAvailabilityProposalNotFound        = errors.New("availability proposal not found")
	ErrAvailabilityProposalEvidenceChanged = errors.New("availability proposal evidence changed")
)

// tcpServiceDefaults maps service types to their default TCP probe port.
// These are services without a web interface that are still commonly
// monitored for availability (databases, message brokers, caches).
var tcpServiceDefaults = map[string]int{
	"mqtt":       1883,
	"mosquitto":  1883,
	"postgres":   5432,
	"postgresql": 5432,
	"mysql":      3306,
	"mariadb":    3306,
	"redis":      6379,
	"memcached":  11211,
	"zookeeper":  2181,
	"nats":       4222,
	"etcd":       2379,
	"rabbitmq":   5672,
	"activemq":   61616,
}

// SuggestAvailabilityProbe generates a suggested availability probe
// configuration for a discovered resource. It first checks webServiceDefaults
// (HTTP/HTTPS probes for services with web interfaces), then falls back to
// tcpServiceDefaults (TCP probes for databases and message brokers).
// Returns nil if no suitable probe can be suggested.
func SuggestAvailabilityProbe(discovery *ResourceDiscovery, hostIP string) *AvailabilityProbeSuggestion {
	if discovery == nil || hostIP == "" {
		return nil
	}

	normalized := normalizeServiceTypeForLookup(discovery.ServiceType)

	// 1. Web services → HTTP/HTTPS probe
	if defaults, matched, ok := lookupWebServiceDefault(normalized); ok {
		return finalizeAvailabilityProbeSuggestion(&AvailabilityProbeSuggestion{
			Protocol:    defaults.Protocol,
			Address:     hostIP,
			Port:        defaults.Port,
			Path:        defaults.Path,
			ServiceName: pickServiceName(discovery, matched),
			Reason:      fmt.Sprintf("service default: %s", matched),
		})
	}

	// 2. TCP-only services (databases, brokers) → TCP probe
	if port, matched, ok := lookupTCPServiceDefault(normalized); ok {
		return finalizeAvailabilityProbeSuggestion(&AvailabilityProbeSuggestion{
			Protocol:    "tcp",
			Address:     hostIP,
			Port:        port,
			ServiceName: pickServiceName(discovery, matched),
			Reason:      fmt.Sprintf("tcp service default: %s", matched),
		})
	}

	// 3. Fallback: try hostname/name against the defaults maps.
	// This covers containers where the deep scan hasn't populated ServiceType yet.
	hostnameNormalized := normalizeServiceTypeForLookup(discovery.Hostname)
	if hostnameNormalized != "" && hostnameNormalized != normalized {
		if defaults, matched, ok := lookupWebServiceDefault(hostnameNormalized); ok {
			return finalizeAvailabilityProbeSuggestion(&AvailabilityProbeSuggestion{
				Protocol:    defaults.Protocol,
				Address:     hostIP,
				Port:        defaults.Port,
				Path:        defaults.Path,
				ServiceName: pickServiceName(discovery, matched),
				Reason:      fmt.Sprintf("hostname match: %s", matched),
			})
		}
		if port, matched, ok := lookupTCPServiceDefault(hostnameNormalized); ok {
			return finalizeAvailabilityProbeSuggestion(&AvailabilityProbeSuggestion{
				Protocol:    "tcp",
				Address:     hostIP,
				Port:        port,
				ServiceName: pickServiceName(discovery, matched),
				Reason:      fmt.Sprintf("hostname tcp match: %s", matched),
			})
		}
	}

	return nil
}

// finalizeAvailabilityProbeSuggestion normalizes a suggestion and binds it to
// the exact evidence an operator is being asked to review. The fingerprint is
// intentionally independent of discovery timestamps and unrelated facts: a
// dismissal reopens only when the proposed endpoint, behavior, identity, or
// inference reason materially changes.
func finalizeAvailabilityProbeSuggestion(suggestion *AvailabilityProbeSuggestion) *AvailabilityProbeSuggestion {
	if suggestion == nil {
		return nil
	}
	suggestion.Protocol = strings.ToLower(strings.TrimSpace(suggestion.Protocol))
	suggestion.Address = normalizeAvailabilitySuggestionAddress(suggestion.Address)
	suggestion.Path = normalizeAvailabilitySuggestionPath(suggestion.Path)
	suggestion.ServiceName = strings.TrimSpace(suggestion.ServiceName)
	suggestion.Reason = strings.TrimSpace(suggestion.Reason)
	fingerprintInput := strings.Join([]string{
		suggestion.Protocol,
		suggestion.Address,
		strconv.Itoa(suggestion.Port),
		suggestion.Path,
		strings.ToLower(suggestion.ServiceName),
		strings.ToLower(suggestion.Reason),
	}, "\x00")
	sum := sha256.Sum256([]byte(fingerprintInput))
	suggestion.EvidenceFingerprint = fmt.Sprintf("sha256:%x", sum[:])
	return suggestion
}

func normalizeAvailabilitySuggestionAddress(address string) string {
	trimmed := strings.TrimSpace(address)
	if parsed := net.ParseIP(strings.Trim(trimmed, "[]")); parsed != nil {
		return parsed.String()
	}
	return strings.ToLower(strings.TrimSuffix(trimmed, "."))
}

func normalizeAvailabilitySuggestionPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if normalized == "/" && trimmed != "/" {
		return ""
	}
	return normalized
}

// normalizeServiceTypeForLookup normalizes a service type string for map lookup.
func normalizeServiceTypeForLookup(serviceType string) string {
	s := strings.ToLower(strings.TrimSpace(serviceType))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// lookupWebServiceDefault checks webServiceDefaults for an exact or variation match.
func lookupWebServiceDefault(normalized string) (webServiceDefault, string, bool) {
	if defaults, ok := webServiceDefaults[normalized]; ok {
		return defaults, normalized, true
	}
	for _, variation := range serviceTypeVariations(normalized) {
		if defaults, ok := webServiceDefaults[variation]; ok {
			return defaults, variation, true
		}
	}
	return webServiceDefault{}, "", false
}

// lookupTCPServiceDefault checks tcpServiceDefaults for an exact or variation match.
func lookupTCPServiceDefault(normalized string) (int, string, bool) {
	if port, ok := tcpServiceDefaults[normalized]; ok {
		return port, normalized, true
	}
	for _, variation := range serviceTypeVariations(normalized) {
		if port, ok := tcpServiceDefaults[variation]; ok {
			return port, variation, true
		}
	}
	return 0, "", false
}

// serviceTypeVariations generates common variations of a normalized service type
// for fuzzy map lookup.
func serviceTypeVariations(normalized string) []string {
	return []string{
		normalized,
		strings.ReplaceAll(normalized, "-", ""),
		strings.TrimSuffix(normalized, "-server"),
		strings.TrimSuffix(normalized, "server"),
	}
}

// pickServiceName returns the discovery's human-readable service name, falling
// back to the matched service type key.
func pickServiceName(discovery *ResourceDiscovery, matchedKey string) string {
	if discovery.ServiceName != "" {
		return discovery.ServiceName
	}
	return matchedKey
}
