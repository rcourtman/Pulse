package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// HostContinuityEntry stores the minimum durable identity and report watermark
// needed to recognise an existing standalone host and reject older telemetry
// across restart and upgrade boundaries before the next live report arrives.
type HostContinuityEntry struct {
	HostID            string    `json:"hostId"`
	ReportHostID      string    `json:"reportHostId,omitempty"`
	AgentReportedID   string    `json:"agentReportedId,omitempty"`
	Hostname          string    `json:"hostname,omitempty"`
	DisplayName       string    `json:"displayName,omitempty"`
	MachineID         string    `json:"machineId,omitempty"`
	TokenID           string    `json:"tokenId,omitempty"`
	AgentVersion      string    `json:"agentVersion,omitempty"`
	Platform          string    `json:"platform,omitempty"`
	LinkedNodeID      string    `json:"linkedNodeId,omitempty"`
	LinkedVMID        string    `json:"linkedVmId,omitempty"`
	LinkedContainerID string    `json:"linkedContainerId,omitempty"`
	IsLegacy          bool      `json:"isLegacy,omitempty"`
	LastSeen          time.Time `json:"lastSeen,omitempty"`
	IntervalSeconds   int       `json:"intervalSeconds,omitempty"`
	// Report ordering and transport activity are persisted independently from
	// accepted telemetry freshness. LastSeen is the server receipt time of the
	// last accepted report; ReportLastReceivedAt includes rejected stale or
	// duplicate arrivals, while ReportObservedAt is the agent-authored clock.
	ReportObservedAt       time.Time `json:"reportObservedAt,omitempty"`
	ReportLastReceivedAt   time.Time `json:"reportLastReceivedAt,omitempty"`
	ReportStreamID         string    `json:"reportStreamId,omitempty"`
	ReportSequence         uint64    `json:"reportSequence,omitempty"`
	RetiredReportStreamIDs []string  `json:"retiredReportStreamIds,omitempty"`
}

// HostContinuityStore persists recent standalone host identity and report
// ordering so licensing, grandfather-floor, and telemetry-transition
// continuity survive process restarts.
type HostContinuityStore struct {
	mu       sync.RWMutex
	entries  map[string]HostContinuityEntry
	dataPath string
	fs       FileSystem
}

func NewHostContinuityStore(dataPath string, fs FileSystem) *HostContinuityStore {
	store := &HostContinuityStore{
		entries:  make(map[string]HostContinuityEntry),
		dataPath: dataPath,
		fs:       fs,
	}
	if store.fs == nil {
		store.fs = defaultFileSystem{}
	}
	if err := store.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load host continuity state")
	}
	return store
}

func normalizeHostContinuityEntry(entry HostContinuityEntry) (HostContinuityEntry, bool) {
	entry.HostID = strings.TrimSpace(entry.HostID)
	if entry.HostID == "" {
		return HostContinuityEntry{}, false
	}

	entry.ReportHostID = strings.TrimSpace(entry.ReportHostID)
	entry.AgentReportedID = strings.TrimSpace(entry.AgentReportedID)
	entry.Hostname = strings.TrimSpace(entry.Hostname)
	entry.DisplayName = strings.TrimSpace(entry.DisplayName)
	entry.MachineID = strings.TrimSpace(entry.MachineID)
	entry.TokenID = strings.TrimSpace(entry.TokenID)
	entry.AgentVersion = strings.TrimSpace(entry.AgentVersion)
	entry.Platform = strings.TrimSpace(entry.Platform)
	entry.LinkedNodeID = strings.TrimSpace(entry.LinkedNodeID)
	entry.LinkedVMID = strings.TrimSpace(entry.LinkedVMID)
	entry.LinkedContainerID = strings.TrimSpace(entry.LinkedContainerID)
	entry.ReportStreamID = strings.TrimSpace(entry.ReportStreamID)
	entry.RetiredReportStreamIDs = uniqueTrimmedStrings(entry.RetiredReportStreamIDs...)
	if len(entry.RetiredReportStreamIDs) > 8 {
		entry.RetiredReportStreamIDs = append([]string(nil), entry.RetiredReportStreamIDs[len(entry.RetiredReportStreamIDs)-8:]...)
	}
	if entry.ReportStreamID == "" {
		entry.ReportSequence = 0
	}
	return entry, true
}

func (s *HostContinuityStore) Load() error {
	filePath := filepath.Join(s.dataPath, "host_continuity.json")

	data, err := readLimitedRegularFileFS(s.fs, filePath, maxHostContinuityFileSizeBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read continuity file: %w", err)
	}

	entries := make(map[string]HostContinuityEntry)
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal continuity: %w", err)
	}

	normalized := make(map[string]HostContinuityEntry, len(entries))
	for _, entry := range entries {
		if normalizedEntry, ok := normalizeHostContinuityEntry(entry); ok {
			normalized[normalizedEntry.HostID] = normalizedEntry
		}
	}

	s.mu.Lock()
	s.entries = normalized
	s.mu.Unlock()
	return nil
}

func (s *HostContinuityStore) save() error {
	data, err := json.Marshal(s.entries)
	if err != nil {
		return fmt.Errorf("failed to marshal continuity: %w", err)
	}
	if err := persistMetadata(s.fs, s.dataPath, "host_continuity.json", data); err != nil {
		return err
	}
	return nil
}

func (s *HostContinuityStore) Upsert(entry HostContinuityEntry) error {
	normalized, ok := normalizeHostContinuityEntry(entry)
	if !ok {
		return fmt.Errorf("host continuity entry requires host ID")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[normalized.HostID] = normalized
	return s.save()
}

func (s *HostContinuityStore) Delete(hostID string) error {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, hostID)
	return s.save()
}

func (s *HostContinuityStore) Get(hostID string) (HostContinuityEntry, bool) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return HostContinuityEntry{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[hostID]
	return entry, ok
}

func (s *HostContinuityStore) RecentEntries(since time.Time) []HostContinuityEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]HostContinuityEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if !since.IsZero() && entry.LastSeen.Before(since) {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastSeen.Equal(out[j].LastSeen) {
			return out[i].HostID < out[j].HostID
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}

func (s *HostContinuityStore) Match(
	reportHostID string,
	machineID string,
	agentID string,
	hostname string,
	tokenID string,
	since time.Time,
) (HostContinuityEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := uniqueTrimmedStrings(reportHostID, machineID, agentID)
	hostname = strings.TrimSpace(hostname)
	tokenID = strings.TrimSpace(tokenID)

	var best HostContinuityEntry
	matched := false

	for _, entry := range s.entries {
		if !since.IsZero() && entry.LastSeen.Before(since) {
			continue
		}

		matchedByAlias := false
		for _, candidate := range candidates {
			if entry.HostID == candidate ||
				entry.ReportHostID == candidate ||
				entry.MachineID == candidate ||
				entry.AgentReportedID == candidate {
				if !matched || entry.LastSeen.After(best.LastSeen) {
					best = entry
					matched = true
				}
				matchedByAlias = true
				break
			}
		}

		if matchedByAlias {
			continue
		}
		if !hostContinuityHostnameMatches(entry.Hostname, hostname) {
			continue
		}
		if tokenID != "" && (entry.TokenID == "" || entry.TokenID != tokenID) {
			continue
		}
		if !matched || entry.LastSeen.After(best.LastSeen) {
			best = entry
			matched = true
		}
	}

	return best, matched
}

func hostContinuityHostnameMatches(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return strings.EqualFold(left, right) || unifiedresources.HostnamesEquivalent(left, right)
}

func uniqueTrimmedStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
