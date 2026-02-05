package unifiedresources

import (
	"net"
	"sort"
	"strings"
)

// NormalizeHostname lowercases and strips domain suffixes.
func NormalizeHostname(hostname string) string {
	host := strings.TrimSpace(strings.ToLower(hostname))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return ""
	}
	if idx := strings.IndexRune(host, '.'); idx > 0 {
		return host[:idx]
	}
	return host
}

// NormalizeMAC normalizes a MAC address to lower-case colon format.
func NormalizeMAC(mac string) string {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return ""
	}
	parsed, err := net.ParseMAC(mac)
	if err != nil {
		return strings.ToLower(mac)
	}
	return strings.ToLower(parsed.String())
}

// NormalizeIP strips CIDR suffixes and ignores invalid IPs.
func NormalizeIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	if strings.Contains(ip, "/") {
		parts := strings.Split(ip, "/")
		ip = parts[0]
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	return parsed.String()
}

// isNonUniqueIP returns true if the IP is not useful for identity matching.
func isNonUniqueIP(ip string) bool {
	ip = strings.ToLower(ip)
	if ip == "" {
		return true
	}
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "127.") {
		return true
	}
	if strings.HasPrefix(ip, "169.254.") {
		return true
	}
	if strings.HasPrefix(ip, "fe80:") {
		return true
	}
	// Docker bridge networks common to many hosts
	if strings.HasPrefix(ip, "172.17.") || strings.HasPrefix(ip, "172.18.") ||
		strings.HasPrefix(ip, "172.19.") || strings.HasPrefix(ip, "172.20.") ||
		strings.HasPrefix(ip, "172.21.") || strings.HasPrefix(ip, "172.22.") ||
		strings.HasPrefix(ip, "172.23.") || strings.HasPrefix(ip, "172.24.") ||
		strings.HasPrefix(ip, "172.25.") || strings.HasPrefix(ip, "172.26.") ||
		strings.HasPrefix(ip, "172.27.") || strings.HasPrefix(ip, "172.28.") ||
		strings.HasPrefix(ip, "172.29.") || strings.HasPrefix(ip, "172.30.") ||
		strings.HasPrefix(ip, "172.31.") {
		return true
	}
	return false
}

// IdentityMatcher maintains indexes for matching identities to resources.
type IdentityMatcher struct {
	byMachineID map[string]string
	byDMIUUID   map[string]string
	byHostname  map[string]map[string]struct{}
	byIP        map[string]map[string]struct{}
	byMAC       map[string]map[string]struct{}
}

// NewIdentityMatcher creates a new matcher.
func NewIdentityMatcher() *IdentityMatcher {
	return &IdentityMatcher{
		byMachineID: make(map[string]string),
		byDMIUUID:   make(map[string]string),
		byHostname:  make(map[string]map[string]struct{}),
		byIP:        make(map[string]map[string]struct{}),
		byMAC:       make(map[string]map[string]struct{}),
	}
}

// Add indexes a resource identity.
func (m *IdentityMatcher) Add(resourceID string, identity ResourceIdentity) {
	if resourceID == "" {
		return
	}
	if identity.MachineID != "" {
		m.byMachineID[strings.TrimSpace(identity.MachineID)] = resourceID
	}
	if identity.DMIUUID != "" {
		m.byDMIUUID[strings.TrimSpace(identity.DMIUUID)] = resourceID
	}
	for _, hostname := range identity.Hostnames {
		norm := NormalizeHostname(hostname)
		if norm == "" {
			continue
		}
		m.addToIndex(m.byHostname, norm, resourceID)
	}
	for _, ip := range identity.IPAddresses {
		norm := NormalizeIP(ip)
		if norm == "" || isNonUniqueIP(norm) {
			continue
		}
		m.addToIndex(m.byIP, norm, resourceID)
	}
	for _, mac := range identity.MACAddresses {
		norm := NormalizeMAC(mac)
		if norm == "" {
			continue
		}
		m.addToIndex(m.byMAC, norm, resourceID)
	}
}

func (m *IdentityMatcher) addToIndex(index map[string]map[string]struct{}, key, resourceID string) {
	bucket := index[key]
	if bucket == nil {
		bucket = make(map[string]struct{})
		index[key] = bucket
	}
	bucket[resourceID] = struct{}{}
}

// MatchCandidate describes a candidate match.
type MatchCandidate struct {
	ID             string
	Confidence     float64
	Reason         string
	RequiresReview bool
}

// FindCandidates returns possible matches based on the identity signals.
func (m *IdentityMatcher) FindCandidates(identity ResourceIdentity) []MatchCandidate {
	candidates := make(map[string]MatchCandidate)

	// Machine ID match
	if identity.MachineID != "" {
		if existing, ok := m.byMachineID[strings.TrimSpace(identity.MachineID)]; ok {
			candidates[existing] = MatchCandidate{ID: existing, Confidence: 1.0, Reason: "machine_id"}
		}
	}

	// DMI UUID match
	if identity.DMIUUID != "" {
		if existing, ok := m.byDMIUUID[strings.TrimSpace(identity.DMIUUID)]; ok {
			candidates[existing] = promoteCandidate(candidates[existing], MatchCandidate{ID: existing, Confidence: 0.99, Reason: "dmi_uuid"})
		}
	}

	hostnameIDs := m.collectIDs(m.byHostname, identity.Hostnames, NormalizeHostname)
	ipIDs := m.collectIDs(m.byIP, identity.IPAddresses, NormalizeIP)
	macIDs := m.collectIDs(m.byMAC, identity.MACAddresses, NormalizeMAC)

	// Hostname + MAC overlap
	for id := range intersectIDs(hostnameIDs, macIDs) {
		candidates[id] = promoteCandidate(candidates[id], MatchCandidate{ID: id, Confidence: 0.90, Reason: "hostname+mac"})
	}

	// Hostname + IP overlap
	for id := range intersectIDs(hostnameIDs, ipIDs) {
		candidates[id] = promoteCandidate(candidates[id], MatchCandidate{ID: id, Confidence: 0.85, Reason: "hostname+ip"})
	}

	// Hostname only
	for id := range hostnameIDs {
		candidates[id] = promoteCandidate(candidates[id], MatchCandidate{ID: id, Confidence: 0.50, Reason: "hostname", RequiresReview: true})
	}

	// IP only
	for id := range ipIDs {
		candidates[id] = promoteCandidate(candidates[id], MatchCandidate{ID: id, Confidence: 0.40, Reason: "ip", RequiresReview: true})
	}

	list := make([]MatchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		list = append(list, candidate)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Confidence == list[j].Confidence {
			return list[i].ID < list[j].ID
		}
		return list[i].Confidence > list[j].Confidence
	})

	return list
}

func (m *IdentityMatcher) collectIDs(index map[string]map[string]struct{}, values []string, normalize func(string) string) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, value := range values {
		norm := normalize(value)
		if norm == "" {
			continue
		}
		bucket := index[norm]
		for id := range bucket {
			ids[id] = struct{}{}
		}
	}
	return ids
}

func intersectIDs(a, b map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{})
	if len(a) == 0 || len(b) == 0 {
		return out
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	for id := range a {
		if _, ok := b[id]; ok {
			out[id] = struct{}{}
		}
	}
	return out
}

func promoteCandidate(existing MatchCandidate, incoming MatchCandidate) MatchCandidate {
	if existing.ID == "" {
		return incoming
	}
	if incoming.Confidence > existing.Confidence {
		return incoming
	}
	return existing
}
