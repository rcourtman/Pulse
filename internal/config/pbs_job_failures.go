package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

const (
	pbsJobFailuresFileName         = "pbs_job_failures.json"
	maxPBSJobFailuresFileSizeBytes = 1 * 1024 * 1024
	// A PBS server with more failing jobs than this is far past what one
	// record needs to carry; the cap keeps the file well inside its read limit.
	maxPBSJobFailuresPerInstance = 256
)

// PBSJobFailureStore keeps, per PBS instance, the jobs whose last completed run
// failed, so the PBS poller can still carry a failure after Pulse restarts.
// PBS's /admin job status drops a job's previous result while its next run is
// in progress; without this record, the first poll after a restart that meets
// a running job would read as recovery and clear the job's alert, only for it
// to fire again if that run failed too.
type PBSJobFailureStore struct {
	mu       sync.RWMutex
	entries  map[string]pbsJobFailureRecord
	dataPath string
	fs       FileSystem
	loadErr  error
}

// Instance ids come from connection names, so a record is bound to the server
// it was read from: a different server added under a reused name starts clean.
type pbsJobFailureRecord struct {
	Host     string                        `json:"host"`
	Failures []models.PBSJobHealthEvidence `json:"failures"`
}

func NewPBSJobFailureStore(dataPath string, fs FileSystem) *PBSJobFailureStore {
	store := &PBSJobFailureStore{
		entries:  make(map[string]pbsJobFailureRecord),
		dataPath: dataPath,
		fs:       fs,
	}
	if store.fs == nil {
		store.fs = defaultFileSystem{}
	}
	if err := store.Load(); err != nil {
		store.loadErr = err
		log.Warn().Err(err).Msg("Failed to load PBS job failure record")
	}
	return store
}

func (s *PBSJobFailureStore) LoadError() error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadErr
}

func (s *PBSJobFailureStore) Load() error {
	if strings.TrimSpace(s.dataPath) == "" {
		return nil
	}
	filePath := filepath.Join(s.dataPath, pbsJobFailuresFileName)
	data, err := readLimitedRegularFileFS(s.fs, filePath, maxPBSJobFailuresFileSizeBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read PBS job failure record: %w", err)
	}

	entries := make(map[string]pbsJobFailureRecord)
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal PBS job failure record: %w", err)
	}
	normalized := make(map[string]pbsJobFailureRecord, len(entries))
	for instanceID, record := range entries {
		instanceID = strings.TrimSpace(instanceID)
		failures := stablePBSJobFailures(record.Failures)
		if instanceID != "" && len(failures) > 0 {
			normalized[instanceID] = pbsJobFailureRecord{Host: normalizePBSJobFailureHost(record.Host), Failures: failures}
		}
	}

	s.mu.Lock()
	s.entries = normalized
	s.loadErr = nil
	s.mu.Unlock()
	return nil
}

// Get returns a copy of the failures recorded for one PBS instance, or none
// when the record was read from a different server.
func (s *PBSJobFailureStore) Get(instanceID, host string) []models.PBSJobHealthEvidence {
	instanceID = strings.TrimSpace(instanceID)
	if s == nil || instanceID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.entries[instanceID]
	if !ok || record.Host != normalizePBSJobFailureHost(host) {
		return nil
	}
	return append([]models.PBSJobHealthEvidence(nil), record.Failures...)
}

// Set records the instance's failed jobs, keeping only failures and only the
// fields that describe the failed run. Poll times are left out so an
// unchanged failure does not rewrite the file on every poll; an empty set
// removes the instance.
func (s *PBSJobFailureStore) Set(instanceID, host string, evidence []models.PBSJobHealthEvidence) error {
	instanceID = strings.TrimSpace(instanceID)
	if s == nil || instanceID == "" {
		return nil
	}
	next := pbsJobFailureRecord{Host: normalizePBSJobFailureHost(host), Failures: stablePBSJobFailures(evidence)}

	s.mu.Lock()
	defer s.mu.Unlock()
	previous, existed := s.entries[instanceID]
	if !existed && len(next.Failures) == 0 {
		return nil
	}
	if existed && len(next.Failures) > 0 && samePBSJobFailureRecord(previous, next) {
		return nil
	}
	if s.entries == nil {
		s.entries = make(map[string]pbsJobFailureRecord)
	}
	if len(next.Failures) == 0 {
		delete(s.entries, instanceID)
	} else {
		s.entries[instanceID] = next
	}
	if err := s.saveLocked(); err != nil {
		if existed {
			s.entries[instanceID] = previous
		} else {
			delete(s.entries, instanceID)
		}
		return err
	}
	return nil
}

// Retain drops records for instances no longer configured, so a removed or
// renamed connection neither grows the file nor lends its failures to a later
// connection of the same name.
func (s *PBSJobFailureStore) Retain(instanceIDs []string) error {
	if s == nil {
		return nil
	}
	keep := make(map[string]bool, len(instanceIDs))
	for _, id := range instanceIDs {
		keep[strings.TrimSpace(id)] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := make(map[string]pbsJobFailureRecord)
	for id, record := range s.entries {
		if !keep[id] {
			removed[id] = record
			delete(s.entries, id)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	if err := s.saveLocked(); err != nil {
		for id, record := range removed {
			s.entries[id] = record
		}
		return err
	}
	return nil
}

func (s *PBSJobFailureStore) saveLocked() error {
	// Without a data directory the record lives in memory only; writing it
	// relative to the working directory would scatter files.
	if strings.TrimSpace(s.dataPath) == "" {
		return nil
	}
	data, err := json.Marshal(s.entries)
	if err != nil {
		return fmt.Errorf("failed to marshal PBS job failure record: %w", err)
	}
	// Never write a record the loader would refuse to read back.
	if len(data) > maxPBSJobFailuresFileSizeBytes {
		return fmt.Errorf("PBS job failure record would be %d bytes, over the %d byte limit", len(data), maxPBSJobFailuresFileSizeBytes)
	}
	if err := persistMetadata(s.fs, s.dataPath, pbsJobFailuresFileName, data); err != nil {
		return fmt.Errorf("failed to save PBS job failure record: %w", err)
	}
	return nil
}

func normalizePBSJobFailureHost(host string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(host)), "/")
}

func stablePBSJobFailures(evidence []models.PBSJobHealthEvidence) []models.PBSJobHealthEvidence {
	out := make([]models.PBSJobHealthEvidence, 0, len(evidence))
	for _, entry := range evidence {
		if entry.Posture != "critical" || entry.PostureReason != "last-run-failed" || strings.TrimSpace(entry.ID) == "" {
			continue
		}
		out = append(out, models.PBSJobHealthEvidence{
			ID:             entry.ID,
			Family:         entry.Family,
			Store:          entry.Store,
			Remote:         entry.Remote,
			Namespace:      entry.Namespace,
			Enabled:        entry.Enabled,
			LastRunState:   entry.LastRunState,
			LastRunUPID:    entry.LastRunUPID,
			LastRunEndtime: entry.LastRunEndtime,
			UPID:           entry.UPID,
			TaskStatus:     entry.TaskStatus,
			TaskEndTime:    entry.TaskEndTime,
			Confidence:     entry.Confidence,
			EvidenceSource: entry.EvidenceSource,
			EvidenceScope:  entry.EvidenceScope,
			Freshness:      models.PBSJobHealthFreshness{LastRunEndTime: entry.Freshness.LastRunEndTime},
			Posture:        entry.Posture,
			PostureReason:  entry.PostureReason,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Family != out[j].Family {
			return out[i].Family < out[j].Family
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > maxPBSJobFailuresPerInstance {
		out = out[:maxPBSJobFailuresPerInstance]
	}
	return out
}

func samePBSJobFailureRecord(left, right pbsJobFailureRecord) bool {
	if left.Host != right.Host || len(left.Failures) != len(right.Failures) {
		return false
	}
	a, errA := json.Marshal(left.Failures)
	b, errB := json.Marshal(right.Failures)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}
