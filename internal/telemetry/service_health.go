package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

type serviceHealthRecord struct {
	SchemaVersion           int    `json:"schema_version"`
	CurrentVersion          string `json:"current_version"`
	CurrentObserved         bool   `json:"current_observed"`
	CurrentHealthy          bool   `json:"current_healthy"`
	CurrentFailureCategory  string `json:"current_failure_category,omitempty"`
	PreviousVersion         string `json:"previous_version,omitempty"`
	PreviousObserved        bool   `json:"previous_observed"`
	PreviousHealthy         bool   `json:"previous_healthy"`
	PreviousFailureCategory string `json:"previous_failure_category,omitempty"`
}

var serviceHealthMu sync.Mutex

var storedServiceVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

func applyServiceHealth(ping *Ping, dataDir string, observe func() ServiceHealthObservation) {
	if ping == nil || observe == nil {
		return
	}

	observation := normalizeServiceHealthObservation(observe())
	serviceHealthMu.Lock()
	defer serviceHealthMu.Unlock()

	record := readServiceHealthRecord(dataDir)
	switch {
	case record.CurrentVersion == "":
		ping.ServiceHealthCohort = ServiceHealthCohortFirstObservation
	case record.CurrentVersion != ping.Version:
		ping.ServiceHealthCohort = ServiceHealthCohortVersionChange
		record.PreviousVersion = record.CurrentVersion
		record.PreviousObserved = record.CurrentObserved
		record.PreviousHealthy = record.CurrentHealthy
		record.PreviousFailureCategory = record.CurrentFailureCategory
	default:
		ping.ServiceHealthCohort = ServiceHealthCohortSameVersion
	}

	record.SchemaVersion = 1
	record.CurrentVersion = ping.Version
	record.CurrentObserved = observation.Observed
	record.CurrentHealthy = observation.Healthy
	record.CurrentFailureCategory = observation.FailureCategory

	ping.ServiceHealthObserved = observation.Observed
	ping.ServiceHealthHealthy = observation.Healthy
	ping.ServiceHealthFailureCategory = observation.FailureCategory
	ping.ServiceHealthPreviousVersion = record.PreviousVersion
	ping.ServiceHealthPreviousObserved = record.PreviousObserved
	ping.ServiceHealthPreviousHealthy = record.PreviousHealthy

	if err := writeServiceHealthRecord(dataDir, record); err != nil {
		log.Debug().Err(err).Msg("Could not persist coarse telemetry service-health observation")
	}
}

func normalizeServiceHealthObservation(observation ServiceHealthObservation) ServiceHealthObservation {
	if !observation.Observed {
		return ServiceHealthObservation{}
	}
	if observation.Healthy {
		return ServiceHealthObservation{Observed: true, Healthy: true}
	}
	observation.FailureCategory = canonicalServiceHealthFailureCategory(observation.FailureCategory)
	return observation
}

func canonicalServiceHealthFailureCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ServiceHealthFailureListener,
		ServiceHealthFailureStartup,
		ServiceHealthFailureRuntime,
		ServiceHealthFailureAPIConnectivity,
		ServiceHealthFailureAPIStatus,
		ServiceHealthFailureUIStatus,
		ServiceHealthFailureFrontendAssets:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ServiceHealthFailureUnknown
	}
}

func readServiceHealthRecord(dataDir string) serviceHealthRecord {
	data, err := os.ReadFile(filepath.Join(dataDir, serviceHealthStateFile))
	if err != nil {
		return serviceHealthRecord{}
	}
	var record serviceHealthRecord
	if err := json.Unmarshal(data, &record); err != nil || record.SchemaVersion != 1 {
		return serviceHealthRecord{}
	}
	record.CurrentVersion = canonicalStoredServiceVersion(record.CurrentVersion)
	if record.CurrentVersion == "" {
		record.CurrentObserved = false
		record.CurrentHealthy = false
		record.CurrentFailureCategory = ""
	}
	record.PreviousVersion = canonicalStoredServiceVersion(record.PreviousVersion)
	if record.PreviousVersion == "" {
		record.PreviousObserved = false
		record.PreviousHealthy = false
		record.PreviousFailureCategory = ""
	}
	record.CurrentFailureCategory = storedServiceHealthFailureCategory(
		record.CurrentObserved,
		record.CurrentHealthy,
		record.CurrentFailureCategory,
	)
	record.PreviousFailureCategory = storedServiceHealthFailureCategory(
		record.PreviousObserved,
		record.PreviousHealthy,
		record.PreviousFailureCategory,
	)
	return record
}

func canonicalStoredServiceVersion(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	if len(value) > 64 || !storedServiceVersionPattern.MatchString(value) {
		return ""
	}
	return value
}

func storedServiceHealthFailureCategory(observed, healthy bool, category string) string {
	if !observed || healthy {
		return ""
	}
	return canonicalServiceHealthFailureCategory(category)
}

func writeServiceHealthRecord(dataDir string, record serviceHealthRecord) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dataDir, ".telemetry_service_health-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(encoded, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, filepath.Join(dataDir, serviceHealthStateFile))
}
