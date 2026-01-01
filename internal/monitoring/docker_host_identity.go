package monitoring

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// tokenHintFromRecord returns a redacted token hint for display purposes.
func tokenHintFromRecord(record *config.APITokenRecord) string {
	if record == nil {
		return ""
	}
	switch {
	case record.Prefix != "" && record.Suffix != "":
		return fmt.Sprintf("%s…%s", record.Prefix, record.Suffix)
	case record.Prefix != "":
		return record.Prefix + "…"
	case record.Suffix != "":
		return "…" + record.Suffix
	default:
		return ""
	}
}

// resolveDockerHostIdentifier determines a unique identifier for a Docker host
// based on its report and existing hosts. Returns the identifier, fallback identifiers,
// the existing host (if matched), and whether a match was found.
func resolveDockerHostIdentifier(report agentsdocker.Report, tokenRecord *config.APITokenRecord, hosts []models.DockerHost) (string, []string, models.DockerHost, bool) {
	base := strings.TrimSpace(report.AgentKey())
	fallbacks := uniqueNonEmptyStrings(
		base,
		strings.TrimSpace(report.Agent.ID),
		strings.TrimSpace(report.Host.MachineID),
		strings.TrimSpace(report.Host.Hostname),
	)

	if existing, ok := findMatchingDockerHost(hosts, report, tokenRecord); ok {
		return existing.ID, fallbacks, existing, true
	}

	identifier := base
	if identifier == "" {
		identifier = strings.TrimSpace(report.Host.MachineID)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(report.Host.Hostname)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(report.Agent.ID)
	}
	if identifier == "" {
		identifier = fallbackDockerHostID(report, tokenRecord)
	}
	if identifier == "" {
		identifier = "docker-host"
	}

	if dockerHostIDExists(identifier, hosts) {
		identifier = generateDockerHostIdentifier(identifier, report, tokenRecord, hosts)
	}

	return identifier, fallbacks, models.DockerHost{}, false
}

// findMatchingDockerHost searches for an existing host that matches the report.
func findMatchingDockerHost(hosts []models.DockerHost, report agentsdocker.Report, tokenRecord *config.APITokenRecord) (models.DockerHost, bool) {
	agentID := strings.TrimSpace(report.Agent.ID)
	tokenID := ""
	if tokenRecord != nil {
		tokenID = strings.TrimSpace(tokenRecord.ID)
	}
	machineID := strings.TrimSpace(report.Host.MachineID)
	hostname := strings.TrimSpace(report.Host.Hostname)

	if agentID != "" {
		for _, host := range hosts {
			if strings.TrimSpace(host.AgentID) != agentID {
				continue
			}

			existingToken := strings.TrimSpace(host.TokenID)
			if tokenID == "" || existingToken == tokenID {
				return host, true
			}
		}
	}

	if machineID != "" && hostname != "" {
		for _, host := range hosts {
			if strings.TrimSpace(host.MachineID) == machineID && strings.TrimSpace(host.Hostname) == hostname {
				if tokenID == "" || strings.TrimSpace(host.TokenID) == tokenID {
					return host, true
				}
			}
		}
	}

	// Fallback: match by Hostname and Token only (when MachineID is missing)
	// This fixes issues where containerized agents without persistent machine-id
	// reconnect with the same token but are treated as new agents.
	if hostname != "" && tokenID != "" {
		for _, host := range hosts {
			if strings.TrimSpace(host.Hostname) == hostname && strings.TrimSpace(host.TokenID) == tokenID {
				return host, true
			}
		}
	}

	if machineID != "" && tokenID == "" {
		for _, host := range hosts {
			if strings.TrimSpace(host.MachineID) == machineID && strings.TrimSpace(host.TokenID) == "" {
				return host, true
			}
		}
	}

	if hostname != "" && tokenID == "" {
		for _, host := range hosts {
			if strings.TrimSpace(host.Hostname) == hostname && strings.TrimSpace(host.TokenID) == "" {
				return host, true
			}
		}
	}

	return models.DockerHost{}, false
}

// dockerHostIDExists checks if a host ID is already in use.
func dockerHostIDExists(id string, hosts []models.DockerHost) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	for _, host := range hosts {
		if host.ID == id {
			return true
		}
	}
	return false
}

// generateDockerHostIdentifier creates a unique identifier by appending suffixes.
func generateDockerHostIdentifier(base string, report agentsdocker.Report, tokenRecord *config.APITokenRecord, hosts []models.DockerHost) string {
	if strings.TrimSpace(base) == "" {
		base = fallbackDockerHostID(report, tokenRecord)
	}
	if strings.TrimSpace(base) == "" {
		base = "docker-host"
	}

	used := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		used[host.ID] = struct{}{}
	}

	suffixes := dockerHostSuffixCandidates(report, tokenRecord)
	for _, suffix := range suffixes {
		candidate := fmt.Sprintf("%s::%s", base, suffix)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}

	seed := strings.Join(suffixes, "|")
	if strings.TrimSpace(seed) == "" {
		seed = base
	}
	sum := sha1.Sum([]byte(seed))
	hashSuffix := fmt.Sprintf("hash-%s", hex.EncodeToString(sum[:6]))
	candidate := fmt.Sprintf("%s::%s", base, hashSuffix)
	if _, exists := used[candidate]; !exists {
		return candidate
	}

	for idx := 2; ; idx++ {
		candidate = fmt.Sprintf("%s::%d", base, idx)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

// dockerHostSuffixCandidates returns candidate suffixes for generating unique IDs.
func dockerHostSuffixCandidates(report agentsdocker.Report, tokenRecord *config.APITokenRecord) []string {
	candidates := make([]string, 0, 5)

	if tokenRecord != nil {
		if sanitized := sanitizeDockerHostSuffix(tokenRecord.ID); sanitized != "" {
			candidates = append(candidates, "token-"+sanitized)
		}
	}

	if agentID := sanitizeDockerHostSuffix(report.Agent.ID); agentID != "" {
		candidates = append(candidates, "agent-"+agentID)
	}

	if machineID := sanitizeDockerHostSuffix(report.Host.MachineID); machineID != "" {
		candidates = append(candidates, "machine-"+machineID)
	}

	hostNameSanitized := sanitizeDockerHostSuffix(report.Host.Hostname)
	if hostNameSanitized != "" {
		candidates = append(candidates, "host-"+hostNameSanitized)
	}

	hostDisplay := sanitizeDockerHostSuffix(report.Host.Name)
	if hostDisplay != "" && hostDisplay != hostNameSanitized {
		candidates = append(candidates, "name-"+hostDisplay)
	}

	return uniqueNonEmptyStrings(candidates...)
}

// sanitizeDockerHostSuffix cleans a string for use as a host ID suffix.
func sanitizeDockerHostSuffix(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(value))
	lastHyphen := false
	runeCount := 0

	for _, r := range value {
		if runeCount >= 48 {
			break
		}

		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastHyphen = false
			runeCount++
		default:
			if !lastHyphen {
				builder.WriteRune('-')
				lastHyphen = true
				runeCount++
			}
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return ""
	}
	return result
}

// fallbackDockerHostID generates a hash-based ID when no better identifier exists.
func fallbackDockerHostID(report agentsdocker.Report, tokenRecord *config.APITokenRecord) string {
	seedParts := dockerHostSuffixCandidates(report, tokenRecord)
	if len(seedParts) == 0 {
		seedParts = uniqueNonEmptyStrings(
			report.Host.Hostname,
			report.Host.MachineID,
			report.Agent.ID,
		)
	}
	if len(seedParts) == 0 {
		return ""
	}
	seed := strings.Join(seedParts, "|")
	sum := sha1.Sum([]byte(seed))
	return fmt.Sprintf("docker-host-%s", hex.EncodeToString(sum[:6]))
}

// uniqueNonEmptyStrings returns unique non-empty strings in order of first appearance.
func uniqueNonEmptyStrings(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
