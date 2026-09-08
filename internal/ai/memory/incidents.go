package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// IncidentStatus represents the current state of an incident.
type IncidentStatus string

const (
	IncidentStatusOpen     IncidentStatus = "open"
	IncidentStatusUnknown  IncidentStatus = "unknown"
	IncidentStatusResolved IncidentStatus = "resolved"
)

// IncidentEventType describes a timeline event type.
type IncidentEventType string

const (
	IncidentEventAlertFired          IncidentEventType = "alert_fired"
	IncidentEventAlertAcknowledged   IncidentEventType = "alert_acknowledged"
	IncidentEventAlertUnacknowledged IncidentEventType = "alert_unacknowledged"
	IncidentEventAlertSnoozed        IncidentEventType = "alert_snoozed"
	IncidentEventAlertUnsnoozed      IncidentEventType = "alert_unsnoozed"
	IncidentEventAlertResolved       IncidentEventType = "alert_resolved"
	IncidentEventAnalysis            IncidentEventType = "ai_analysis"
	IncidentEventCommand             IncidentEventType = "command"
	IncidentEventRunbook             IncidentEventType = "runbook"
	IncidentEventNote                IncidentEventType = "note"
)

// IncidentEvent represents a single timeline entry for an incident.
type IncidentEvent struct {
	Source    string                           `json:"source,omitempty"`
	Evidence  *unifiedresources.ResourceChange `json:"evidence,omitempty"`
	ID        string                           `json:"id"`
	Type      IncidentEventType                `json:"type"`
	Timestamp time.Time                        `json:"timestamp"`
	Summary   string                           `json:"summary"`
	Details   map[string]interface{}           `json:"details,omitempty"`
}

// Incident captures an alert occurrence and its investigation timeline.
// It is an alert-scoped memory/projection for investigation support rather than
// the canonical durable resource-change history.
type Incident struct {
	History         *IncidentHistoryCoverage `json:"history,omitempty"`
	ID              string                   `json:"id"`
	AlertIdentifier string                   `json:"alertIdentifier"`
	AlertType       string                   `json:"alertType"`
	Level           string                   `json:"level"`
	ResourceID      string                   `json:"resourceId"`
	ResourceName    string                   `json:"resourceName"`
	ResourceType    string                   `json:"resourceType,omitempty"`
	Node            string                   `json:"node,omitempty"`
	Instance        string                   `json:"instance,omitempty"`
	Message         string                   `json:"message,omitempty"`
	Status          IncidentStatus           `json:"status"`
	OpenedAt        time.Time                `json:"openedAt"`
	ClosedAt        *time.Time               `json:"closedAt,omitempty"`
	Acknowledged    bool                     `json:"acknowledged"`
	AckUser         string                   `json:"ackUser,omitempty"`
	AckTime         *time.Time               `json:"ackTime,omitempty"`
	Events          []IncidentEvent          `json:"events,omitempty"`
}

type incidentShell struct {
	ID                 string
	AlertIdentifier    string
	AlertType          string
	Level              string
	ResourceID         string
	ResourceName       string
	ResourceType       string
	Node               string
	Instance           string
	Message            string
	OpenedAt           time.Time
	OccurrenceClosedAt *time.Time
	Events             []IncidentEvent
}

type incidentJSON struct {
	History         *IncidentHistoryCoverage `json:"history,omitempty"`
	ID              string                   `json:"id"`
	AlertIdentifier string                   `json:"alertIdentifier"`
	AlertType       string                   `json:"alertType"`
	Level           string                   `json:"level"`
	ResourceID      string                   `json:"resourceId"`
	ResourceName    string                   `json:"resourceName"`
	ResourceType    string                   `json:"resourceType,omitempty"`
	Node            string                   `json:"node,omitempty"`
	Instance        string                   `json:"instance,omitempty"`
	Message         string                   `json:"message,omitempty"`
	Status          IncidentStatus           `json:"status"`
	OpenedAt        time.Time                `json:"openedAt"`
	ClosedAt        *time.Time               `json:"closedAt,omitempty"`
	Acknowledged    bool                     `json:"acknowledged"`
	AckUser         string                   `json:"ackUser,omitempty"`
	AckTime         *time.Time               `json:"ackTime,omitempty"`
	Events          []IncidentEvent          `json:"events,omitempty"`
}

type incidentShellJSON struct {
	ID                 string          `json:"id"`
	AlertIdentifier    string          `json:"alertIdentifier"`
	AlertType          string          `json:"alertType"`
	Level              string          `json:"level"`
	ResourceID         string          `json:"resourceId"`
	ResourceName       string          `json:"resourceName"`
	ResourceType       string          `json:"resourceType,omitempty"`
	Node               string          `json:"node,omitempty"`
	Instance           string          `json:"instance,omitempty"`
	Message            string          `json:"message,omitempty"`
	OpenedAt           time.Time       `json:"openedAt"`
	OccurrenceClosedAt *time.Time      `json:"occurrenceClosedAt,omitempty"`
	Events             []IncidentEvent `json:"events,omitempty"`
	Status             IncidentStatus  `json:"status,omitempty"`
	ClosedAt           *time.Time      `json:"closedAt,omitempty"`
	Acknowledged       bool            `json:"acknowledged,omitempty"`
	AckUser            string          `json:"ackUser,omitempty"`
	AckTime            *time.Time      `json:"ackTime,omitempty"`
}

func (i Incident) MarshalJSON() ([]byte, error) {
	alertIdentifier := strings.TrimSpace(i.AlertIdentifier)
	return json.Marshal(incidentJSON{
		History:         i.History,
		ID:              i.ID,
		AlertIdentifier: alertIdentifier,
		AlertType:       i.AlertType,
		Level:           i.Level,
		ResourceID:      i.ResourceID,
		ResourceName:    i.ResourceName,
		ResourceType:    i.ResourceType,
		Node:            i.Node,
		Instance:        i.Instance,
		Message:         i.Message,
		Status:          i.Status,
		OpenedAt:        i.OpenedAt,
		ClosedAt:        i.ClosedAt,
		Acknowledged:    i.Acknowledged,
		AckUser:         i.AckUser,
		AckTime:         i.AckTime,
		Events:          i.Events,
	})
}

func (i *Incident) UnmarshalJSON(data []byte) error {
	if i == nil {
		return nil
	}
	var payload incidentJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	*i = Incident{
		History:         payload.History,
		ID:              payload.ID,
		AlertIdentifier: strings.TrimSpace(payload.AlertIdentifier),
		AlertType:       payload.AlertType,
		Level:           payload.Level,
		ResourceID:      payload.ResourceID,
		ResourceName:    payload.ResourceName,
		ResourceType:    payload.ResourceType,
		Node:            payload.Node,
		Instance:        payload.Instance,
		Message:         payload.Message,
		Status:          payload.Status,
		OpenedAt:        payload.OpenedAt,
		ClosedAt:        payload.ClosedAt,
		Acknowledged:    payload.Acknowledged,
		AckUser:         payload.AckUser,
		AckTime:         payload.AckTime,
		Events:          payload.Events,
	}
	return nil
}

func (s incidentShell) MarshalJSON() ([]byte, error) {
	return json.Marshal(incidentShellJSON{
		ID:                 s.ID,
		AlertIdentifier:    strings.TrimSpace(s.AlertIdentifier),
		AlertType:          s.AlertType,
		Level:              s.Level,
		ResourceID:         s.ResourceID,
		ResourceName:       s.ResourceName,
		ResourceType:       s.ResourceType,
		Node:               s.Node,
		Instance:           s.Instance,
		Message:            s.Message,
		OpenedAt:           s.OpenedAt,
		OccurrenceClosedAt: s.OccurrenceClosedAt,
		Events:             s.Events,
	})
}

func (s *incidentShell) UnmarshalJSON(data []byte) error {
	if s == nil {
		return nil
	}
	var payload incidentShellJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	*s = incidentShell{
		ID:                 payload.ID,
		AlertIdentifier:    strings.TrimSpace(payload.AlertIdentifier),
		AlertType:          payload.AlertType,
		Level:              payload.Level,
		ResourceID:         payload.ResourceID,
		ResourceName:       payload.ResourceName,
		ResourceType:       payload.ResourceType,
		Node:               payload.Node,
		Instance:           payload.Instance,
		Message:            payload.Message,
		OpenedAt:           payload.OpenedAt,
		OccurrenceClosedAt: payload.OccurrenceClosedAt,
		Events:             cloneIncidentEvents(payload.Events),
	}
	if s.OccurrenceClosedAt == nil && payload.ClosedAt != nil {
		s.OccurrenceClosedAt = cloneTime(*payload.ClosedAt)
	}
	if payload.Acknowledged && !hasIncidentEventType(s.Events, IncidentEventAlertAcknowledged, IncidentEventAlertUnacknowledged) {
		ackAt := payload.OpenedAt
		if payload.AckTime != nil && !payload.AckTime.IsZero() {
			ackAt = *payload.AckTime
		}
		s.Events = append(s.Events, IncidentEvent{
			ID:        generateIncidentEventID(),
			Type:      IncidentEventAlertAcknowledged,
			Timestamp: ackAt,
			Summary:   "Alert acknowledged",
			Details: map[string]interface{}{
				"user": strings.TrimSpace(payload.AckUser),
			},
		})
	}
	if (payload.Status == IncidentStatusResolved || payload.ClosedAt != nil) && !hasIncidentEventType(s.Events, IncidentEventAlertResolved) {
		resolvedAt := payload.OpenedAt
		if payload.ClosedAt != nil && !payload.ClosedAt.IsZero() {
			resolvedAt = *payload.ClosedAt
		}
		s.Events = append(s.Events, IncidentEvent{
			ID:        generateIncidentEventID(),
			Type:      IncidentEventAlertResolved,
			Timestamp: resolvedAt,
			Summary:   "Alert resolved",
			Details: map[string]interface{}{
				"resolved_at": resolvedAt.Format(time.RFC3339),
			},
		})
	}
	sortIncidentEvents(s.Events)
	return nil
}

// IncidentStoreConfig configures incident retention and persistence.
type IncidentStoreConfig struct {
	DataDir              string
	MaxIncidents         int
	MaxEventsPerIncident int
	MaxAgeDays           int
}

// IncidentStore maintains alert-scoped incident timelines and persistence for
// investigation memory. Durable resource history belongs to the canonical
// unified-resource change model.
type IncidentStore struct {
	mu                    sync.RWMutex
	saveMu                sync.Mutex
	savePending           atomic.Bool  // a queued save will capture the latest state; further requests coalesce
	savesCompleted        atomic.Int64 // completed JSON replacements, observable by tests
	incidents             []*incidentShell
	maxIncidents          int
	maxEvents             int
	maxAge                time.Duration
	dataDir               string
	filePath              string
	resourceTimelineStore IncidentTimelineStore
}

const (
	defaultIncidentMaxIncidents = 500
	defaultIncidentMaxEvents    = 120
	defaultIncidentMaxAgeDays   = 90
	maxIncidentFileSize         = 20 * 1024 * 1024 // 20MB
	incidentStartMatchTolerance = time.Second
	incidentSnapshotSource      = "alert_history_snapshot"
)

// IncidentTimelineStore exposes the canonical resource timeline used to derive
// incident lifecycle and remediation history.
type IncidentTimelineStore interface {
	GetRecentChangesFiltered(string, time.Time, int, unifiedresources.ResourceChangeFilters) ([]unifiedresources.ResourceChange, error)
	ResourceHistoryIDs(string) ([]string, error)
	GetRecentChanges(canonicalID string, since time.Time, limit int) ([]unifiedresources.ResourceChange, error)
}

// NewIncidentStore creates a new incident store with persistence.
func NewIncidentStore(cfg IncidentStoreConfig) *IncidentStore {
	maxIncidents := cfg.MaxIncidents
	if maxIncidents <= 0 {
		maxIncidents = defaultIncidentMaxIncidents
	}
	maxEvents := cfg.MaxEventsPerIncident
	if maxEvents <= 0 {
		maxEvents = defaultIncidentMaxEvents
	}
	maxAgeDays := cfg.MaxAgeDays
	if maxAgeDays <= 0 {
		maxAgeDays = defaultIncidentMaxAgeDays
	}

	store := &IncidentStore{
		incidents:    make([]*incidentShell, 0),
		maxIncidents: maxIncidents,
		maxEvents:    maxEvents,
		maxAge:       time.Duration(maxAgeDays) * 24 * time.Hour,
		dataDir:      normalizeOptionalMemoryDataDir(cfg.DataDir),
	}

	if store.dataDir != "" {
		filePath, err := memoryPersistencePath(store.dataDir, incidentHistoryFileName)
		if err != nil {
			panic(fmt.Sprintf("invalid AI incident persistence path for %q: %v", store.dataDir, err))
		}
		store.filePath = filePath
		if err := store.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("failed to load incident history from disk")
		} else if len(store.incidents) > 0 {
			log.Info().Int("count", len(store.incidents)).Msg("loaded incident history from disk")
		}
	}

	return store
}

// SetResourceTimelineStore attaches the canonical resource timeline used to
// project durable incident lifecycle and remediation history.
func (s *IncidentStore) SetResourceTimelineStore(store IncidentTimelineStore) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.resourceTimelineStore = store
	s.mu.Unlock()
}

// RecordAlertFired opens or updates an incident for a fired alert.
func (s *IncidentStore) RecordAlertFired(alert *alerts.Alert) {
	if alert == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	shell := s.findLifecycleOccurrenceLocked(alert)
	if alert.StartTime.IsZero() {
		shell = s.findOpenIncidentByAlertIdentifierLocked(alert.ID)
	}
	if shell == nil {
		shell = newIncidentShellFromAlert(alert)
		s.incidents = append(s.incidents, shell)
		if !s.projectsFromCanonicalLocked() {
			s.addEventAtLocked(shell, IncidentEventAlertFired, alert.StartTime, formatAlertSummary(alert), map[string]interface{}{
				"type":      alert.Type,
				"level":     string(alert.Level),
				"value":     alert.Value,
				"threshold": alert.Threshold,
			})
		}
	} else if incidentOccurrenceClosedAt(shell) != nil {
		// A replayed fired event must not reopen a completed occurrence.
		return
	} else {
		updateIncidentShellFromAlert(shell, alert)
	}

	s.trimLocked()
	s.saveAsync()
}

// RecordAlertAcknowledged records an acknowledgement event for an alert.
func (s *IncidentStore) RecordAlertAcknowledged(alert *alerts.Alert, user string) {
	if alert == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	shell := s.ensureIncidentForAlertLocked(alert)
	if !s.projectsFromCanonicalLocked() {
		ackAt := time.Now()
		if alert.AckTime != nil && !alert.AckTime.IsZero() {
			ackAt = *alert.AckTime
		}
		s.addEventAtLocked(shell, IncidentEventAlertAcknowledged, ackAt, "Alert acknowledged", map[string]interface{}{
			"user": user,
		})
	}

	s.trimLocked()
	s.saveAsync()
}

// RecordAlertUnacknowledged records an unacknowledge event for an alert.
func (s *IncidentStore) RecordAlertUnacknowledged(alert *alerts.Alert, user string) {
	if alert == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	shell := s.ensureIncidentForAlertLocked(alert)
	if !s.projectsFromCanonicalLocked() {
		s.addEventAtLocked(shell, IncidentEventAlertUnacknowledged, time.Now(), "Alert unacknowledged", map[string]interface{}{
			"user": user,
		})
	}

	s.trimLocked()
	s.saveAsync()
}

// RecordAlertResolved records a resolved event and closes the incident.
func (s *IncidentStore) RecordAlertResolved(alert *alerts.Alert, resolvedAt time.Time) {
	if alert == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	shell := s.findLifecycleOccurrenceLocked(alert)
	if alert.StartTime.IsZero() {
		shell = s.findOpenIncidentByAlertIdentifierLocked(alert.ID)
	}
	if shell == nil {
		shell = newIncidentShellFromAlert(alert)
		s.incidents = append(s.incidents, shell)
	}

	// Resolution replay is idempotent, including after a checkpoint reload.
	if incidentOccurrenceClosedAt(shell) != nil {
		return
	}

	if resolvedAt.IsZero() {
		now := time.Now()
		resolvedAt = now
	}
	shell.OccurrenceClosedAt = cloneTime(resolvedAt)

	if !s.projectsFromCanonicalLocked() {
		s.addEventAtLocked(shell, IncidentEventAlertResolved, resolvedAt, "Alert resolved", map[string]interface{}{
			"resolved_at": resolvedAt.Format(time.RFC3339),
		})
	}

	s.trimLocked()
	s.saveAsync()
}

// EnsureAlertOccurrence materializes the minimum honest timeline carried by
// an alert snapshot. It is the read-repair boundary for active alerts and
// legacy history entries that predate the canonical resource timeline. When
// canonical events exist, QueryIncidents replaces these snapshot-derived
// lifecycle breadcrumbs with the durable projection.
func (s *IncidentStore) EnsureAlertOccurrence(alert *alerts.Alert, resolvedAt *time.Time) *Incident {
	if s == nil || alert == nil || strings.TrimSpace(alert.ID) == "" {
		return nil
	}

	s.mu.Lock()
	shell := s.findIncidentByAlertAtLocked(alert.ID, alert.StartTime)
	changed := false
	if shell == nil {
		shell = newIncidentShellFromAlert(alert)
		s.incidents = append(s.incidents, shell)
		changed = true
	} else {
		updateIncidentShellFromAlert(shell, alert)
	}

	if !hasIncidentEventType(shell.Events, IncidentEventAlertFired) {
		s.addEventAtLocked(shell, IncidentEventAlertFired, shell.OpenedAt, formatAlertSummary(alert), map[string]interface{}{
			"type":              alert.Type,
			"level":             string(alert.Level),
			"value":             alert.Value,
			"threshold":         alert.Threshold,
			"projection_source": incidentSnapshotSource,
		})
		changed = true
	}

	if alert.Acknowledged && !hasIncidentEventType(shell.Events, IncidentEventAlertAcknowledged) {
		ackAt := shell.OpenedAt
		if alert.AckTime != nil && !alert.AckTime.IsZero() {
			ackAt = *alert.AckTime
		}
		s.addEventAtLocked(shell, IncidentEventAlertAcknowledged, ackAt, "Alert acknowledged", map[string]interface{}{
			"user":              strings.TrimSpace(alert.AckUser),
			"projection_source": incidentSnapshotSource,
		})
		changed = true
	}

	if resolvedAt != nil && !resolvedAt.IsZero() {
		shell.OccurrenceClosedAt = cloneTime(*resolvedAt)
		if !hasIncidentEventType(shell.Events, IncidentEventAlertResolved) {
			s.addEventAtLocked(shell, IncidentEventAlertResolved, *resolvedAt, "Alert resolved", map[string]interface{}{
				"resolved_at":       resolvedAt.Format(time.RFC3339),
				"projection_source": incidentSnapshotSource,
			})
			changed = true
		}
	}

	if changed {
		s.trimLockedPreserving(shell)
		s.saveAsync()
	}
	s.mu.Unlock()

	return s.GetTimelineByAlertAt(alert.ID, alert.StartTime)
}

// RecordAnalysis adds an AI analysis event to the incident for an alert.
func (s *IncidentStore) RecordAnalysis(alertIdentifier, summary string, details map[string]interface{}) {
	if alertIdentifier == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	shell := s.findLatestIncidentByAlertIdentifierLocked(alertIdentifier)
	if shell == nil {
		shell = &incidentShell{
			ID:              generateIncidentID(),
			AlertIdentifier: alertIdentifier,
			OpenedAt:        time.Now(),
		}
		s.incidents = append(s.incidents, shell)
	}

	if summary == "" {
		summary = "Pulse Patrol analysis completed"
	}

	s.addEventLocked(shell, IncidentEventAnalysis, summary, details)
	s.trimLocked()
	s.saveAsync()
}

// RecordCommand adds a command execution event to the incident for an alert.
func (s *IncidentStore) RecordCommand(alertIdentifier, command string, success bool, output string, details map[string]interface{}) {
	if alertIdentifier == "" || command == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	shell := s.findLatestIncidentByAlertIdentifierLocked(alertIdentifier)
	if shell == nil {
		shell = &incidentShell{
			ID:              generateIncidentID(),
			AlertIdentifier: alertIdentifier,
			OpenedAt:        time.Now(),
		}
		s.incidents = append(s.incidents, shell)
	}

	if details == nil {
		details = make(map[string]interface{})
	}
	if shell.ResourceID == "" {
		if resourceID, ok := details["resource_id"].(string); ok {
			shell.ResourceID = strings.TrimSpace(resourceID)
		}
	}
	details["command"] = command
	details["success"] = success
	if output != "" {
		details["output_excerpt"] = truncateOutput(output, 500)
	}

	status := "failed"
	if success {
		status = "succeeded"
	}
	summary := fmt.Sprintf("Command %s: %s", status, command)

	if !s.projectsFromCanonicalLocked() {
		s.addEventLocked(shell, IncidentEventCommand, summary, details)
	}
	s.trimLocked()
	s.saveAsync()
}

// RecordRunbook adds a runbook execution event to the incident for an alert.
func (s *IncidentStore) RecordRunbook(alertIdentifier, runbookID, title string, outcome string, automatic bool, message string) {
	if alertIdentifier == "" || runbookID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	shell := s.findLatestIncidentByAlertIdentifierLocked(alertIdentifier)
	if shell == nil {
		shell = &incidentShell{
			ID:              generateIncidentID(),
			AlertIdentifier: alertIdentifier,
			OpenedAt:        time.Now(),
		}
		s.incidents = append(s.incidents, shell)
	}

	summary := fmt.Sprintf("Runbook %s (%s)", title, outcome)
	details := map[string]interface{}{
		"runbook_id": runbookID,
		"outcome":    outcome,
		"automatic":  automatic,
	}
	if message != "" {
		details["message"] = message
	}

	if !s.projectsFromCanonicalLocked() {
		s.addEventLocked(shell, IncidentEventRunbook, summary, details)
	}
	s.trimLocked()
	s.saveAsync()
}

// RecordNote appends a user note to an incident identified by canonical alert identifier or incident ID.
func (s *IncidentStore) RecordNote(alertIdentifier, incidentID, note, user string) bool {
	note = strings.TrimSpace(note)
	if note == "" {
		return false
	}

	var projected *Incident
	if strings.HasPrefix(incidentID, "projected-") {
		page, err := s.QueryIncidents(IncidentQuery{AlertIdentifier: alertIdentifier, IncidentID: incidentID, Limit: 1})
		if err != nil || len(page.Incidents) != 1 {
			return false
		}
		projected = page.Incidents[0]
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var shell *incidentShell
	if incidentID != "" {
		shell = s.findIncidentByIDLocked(incidentID)
	} else if alertIdentifier != "" {
		shell = s.findLatestIncidentByAlertIdentifierLocked(alertIdentifier)
	}
	if shell == nil && projected != nil {
		shell = &incidentShell{ID: projected.ID, AlertIdentifier: projected.AlertIdentifier, AlertType: projected.AlertType, Level: projected.Level, ResourceID: projected.ResourceID, ResourceName: projected.ResourceName, ResourceType: projected.ResourceType, Node: projected.Node, Instance: projected.Instance, Message: projected.Message, OpenedAt: projected.OpenedAt}
		if projected.ClosedAt != nil {
			closed := *projected.ClosedAt
			shell.OccurrenceClosedAt = &closed
		}
		s.incidents = append(s.incidents, shell)
	}
	if shell == nil {
		return false
	}

	summary := "Note added"
	if user != "" {
		summary = fmt.Sprintf("Note added by %s", user)
	}

	s.addEventLocked(shell, IncidentEventNote, summary, map[string]interface{}{
		"note": note,
		"user": user,
	})

	s.trimLocked()
	s.saveAsync()
	return true
}

// GetTimelineByAlertIdentifier returns the most recent incident for the alert.
func (s *IncidentStore) GetTimelineByAlertIdentifier(alertIdentifier string) *Incident {
	return s.GetTimelineByAlertAt(alertIdentifier, time.Time{})
}

// GetTimelineByAlertAt is retained for local writer callers. Runtime readers
// use QueryIncidents to distinguish an unavailable store from absent history.
func (s *IncidentStore) GetTimelineByAlertAt(alertIdentifier string, startedAt time.Time) *Incident {
	if strings.TrimSpace(alertIdentifier) == "" {
		return nil
	}
	page, err := s.QueryIncidents(IncidentQuery{AlertIdentifier: alertIdentifier, StartedAt: startedAt, Limit: 1})
	if err != nil {
		log.Warn().Err(err).Msg("incident history unavailable")
		return nil
	}
	if len(page.Incidents) == 0 {
		return nil
	}
	return page.Incidents[0]
}

func (s *IncidentStore) findIncidentByAlertAtLocked(alertIdentifier string, startedAt time.Time) *incidentShell {
	if startedAt.IsZero() {
		return s.findLatestIncidentByAlertIdentifierLocked(alertIdentifier)
	}
	best, delta := s.findClosestIncidentByAlertAtLocked(alertIdentifier, startedAt)
	if best == nil || delta > incidentStartMatchTolerance {
		return nil
	}
	return best
}

func (s *IncidentStore) findClosestIncidentByAlertAtLocked(alertIdentifier string, startedAt time.Time) (*incidentShell, time.Duration) {
	var best *incidentShell
	var bestDelta time.Duration
	for _, incident := range s.incidents {
		if incident == nil || incident.AlertIdentifier != alertIdentifier {
			continue
		}
		delta := incident.OpenedAt.Sub(startedAt)
		if delta < 0 {
			delta = -delta
		}
		if best == nil || delta < bestDelta {
			best = incident
			bestDelta = delta
		}
	}
	return best, bestDelta
}

// ListIncidentsByResource returns recent incidents for a resource.
func (s *IncidentStore) ListIncidentsByResource(resourceID string, limit int) []*Incident {
	if strings.TrimSpace(resourceID) == "" {
		return nil
	}
	page, err := s.QueryIncidents(IncidentQuery{ResourceID: resourceID, Limit: limit})
	if err != nil {
		log.Warn().Err(err).Msg("incident history unavailable")
		return nil
	}
	return page.Incidents
}

// FormatForAlert returns a condensed incident timeline for prompt injection.
func (s *IncidentStore) FormatForAlert(alertIdentifier string, maxEvents int) string {
	page, err := s.QueryIncidents(IncidentQuery{AlertIdentifier: alertIdentifier, Limit: 1})
	if err != nil {
		return "\n\nIncident history unavailable: canonical evidence could not be read.\n"
	}
	if len(page.Incidents) == 0 {
		return formatIncidentCoverage(page)
	}
	incident := page.Incidents[0]

	var b strings.Builder
	b.WriteString("\n\n## Incident Memory\n")
	b.WriteString(formatIncidentCoverage(page))
	b.WriteString(fmt.Sprintf("Alert incident for %s (%s, %s)\n",
		incident.ResourceName, incident.AlertType, incident.Level))
	b.WriteString(fmt.Sprintf("Status: %s\n", incident.Status))
	if incident.History != nil {
		b.WriteString("Occurrence evidence source: " + incident.History.Source + "\n")
	}

	events := incident.Events
	if maxEvents > 0 && len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
		b.WriteString(fmt.Sprintf("Showing latest %d of %d recorded events.\n", len(events), len(incident.Events)))
	}
	for _, evt := range events {
		b.WriteString("- ")
		b.WriteString(formatIncidentTime(evt.Timestamp))
		b.WriteString(": ")
		b.WriteString(evt.Summary)
		if evt.Type == IncidentEventNote {
			if note, ok := stringMetadata(evt.Details, "note"); ok {
				b.WriteString(": " + note)
			}
		}
		if evt.Evidence != nil {
			b.WriteString(fmt.Sprintf(" [record=%s observed=%s occurred=%s source=%s actor=%s]", evt.Evidence.ID, formatIncidentTime(evt.Evidence.ObservedAt), formatIncidentOccurredTime(evt.Evidence.OccurredAt), evt.Evidence.SourceAdapter, evt.Evidence.Actor))
		} else if evt.Source != "" {
			b.WriteString(" [source=" + evt.Source + "]")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FormatForResource returns a condensed incident summary for a resource.
func (s *IncidentStore) FormatForResource(resourceID string, limit int) string {
	page, err := s.QueryIncidents(IncidentQuery{ResourceID: resourceID, Limit: limit})
	if err != nil {
		return "\n\nIncident history unavailable: canonical evidence could not be read.\n"
	}
	return FormatIncidentPageForResource(page)
}

// FormatIncidentPageForResource formats the same snapshot returned to an API
// caller, so a second read cannot silently describe a different history.
func FormatIncidentPageForResource(page IncidentPage) string {
	incidents := page.Incidents
	if len(incidents) == 0 {
		return formatIncidentCoverage(page)
	}

	var b strings.Builder
	b.WriteString("\n\n## Incident Memory\n")
	b.WriteString(formatIncidentCoverage(page))
	b.WriteString("Recent incidents:\n")
	for _, incident := range incidents {
		status := string(incident.Status)
		if incident.Acknowledged && incident.Status == IncidentStatusOpen {
			status = "acknowledged"
		}
		b.WriteString("- ")
		b.WriteString(formatIncidentTime(incident.OpenedAt))
		b.WriteString(": ")
		b.WriteString(incident.AlertType)
		if incident.Level != "" {
			b.WriteString(" (")
			b.WriteString(incident.Level)
			b.WriteString(")")
		}
		b.WriteString(" - ")
		b.WriteString(status)
		if incident.History != nil {
			b.WriteString(" [source=" + incident.History.Source + "]")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FormatForPatrol returns a condensed incident summary for infrastructure-wide patrol analysis.
func (s *IncidentStore) FormatForPatrol(limit int) string {
	if limit <= 0 {
		limit = 8
	}

	page, err := s.QueryIncidents(IncidentQuery{Limit: limit})
	if err != nil {
		return "\n\nIncident history unavailable: canonical evidence could not be read.\n"
	}
	snapshot := page.Incidents
	if len(snapshot) == 0 {
		return formatIncidentCoverage(page)
	}

	var b strings.Builder
	b.WriteString("\n\n## Incident Memory\n")
	b.WriteString(formatIncidentCoverage(page))
	b.WriteString("Recent incidents across infrastructure:\n")

	for _, incident := range snapshot {
		status := string(incident.Status)
		if incident.Acknowledged && incident.Status == IncidentStatusOpen {
			status = "acknowledged"
		}

		lastSummary := ""
		if len(incident.Events) > 0 {
			lastSummary = incident.Events[len(incident.Events)-1].Summary
		}

		b.WriteString("- ")
		b.WriteString(formatIncidentTime(incident.OpenedAt))
		b.WriteString(": ")
		if incident.ResourceName != "" {
			b.WriteString(incident.ResourceName)
			b.WriteString(" - ")
		}
		if incident.AlertType != "" {
			b.WriteString(incident.AlertType)
		}
		if incident.Level != "" {
			b.WriteString(" (")
			b.WriteString(incident.Level)
			b.WriteString(")")
		}
		b.WriteString(" - ")
		b.WriteString(status)
		if incident.History != nil {
			b.WriteString(" [source=" + incident.History.Source + "]")
		}
		if lastSummary != "" {
			b.WriteString(" - last: ")
			b.WriteString(truncateOutput(lastSummary, 80))
		} else if incident.Message != "" {
			b.WriteString(" - ")
			b.WriteString(truncateOutput(incident.Message, 80))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (s *IncidentStore) projectsFromCanonicalLocked() bool {
	return s.resourceTimelineStore != nil
}

func hydrateIncidentFromCanonicalChange(incident *Incident, change unifiedresources.ResourceChange) {
	if incident == nil {
		return
	}
	// Alert evidence owns its target and risk context. A command may execute on
	// a related host, so its target must not rename the alert's resource.
	alertEvidence := false
	switch change.Kind {
	case unifiedresources.ChangeAlertFired, unifiedresources.ChangeAlertResolved,
		unifiedresources.ChangeAlertAcknowledged, unifiedresources.ChangeAlertUnacknowledged,
		unifiedresources.ChangeAlertSnoozed, unifiedresources.ChangeAlertUnsnoozed:
		alertEvidence = true
	}
	if resourceID := strings.TrimSpace(change.ResourceID); resourceID != "" && (incident.ResourceID == "" || alertEvidence) {
		incident.ResourceID = resourceID
	}
	if alertType, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertType); ok && (incident.AlertType == "" || alertEvidence) {
		incident.AlertType = alertType
	}
	if level, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertLevel); ok && (incident.Level == "" || alertEvidence) {
		incident.Level = level
	}
	if message, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertMessage); ok && (incident.Message == "" || alertEvidence) {
		incident.Message = message
	}
	if resourceType, ok := stringMetadata(change.Metadata, "resourceType"); ok && (incident.ResourceType == "" || alertEvidence) {
		incident.ResourceType = resourceType
	}
	if incident.OpenedAt.IsZero() {
		incident.OpenedAt = incidentEventTimestamp(change)
	}
}

func applyProjectedIncidentState(incident *Incident, events []IncidentEvent) {
	if incident == nil || len(events) == 0 {
		return
	}

	for _, event := range events {
		if incident.OpenedAt.IsZero() || (!event.Timestamp.IsZero() && event.Timestamp.Before(incident.OpenedAt)) {
			incident.OpenedAt = event.Timestamp
		}

		switch event.Type {
		case IncidentEventAlertFired:
			incident.Status = IncidentStatusOpen
			incident.ClosedAt = nil
		case IncidentEventAlertAcknowledged:
			incident.Acknowledged = true
			timestamp := event.Timestamp
			incident.AckTime = &timestamp
			incident.AckUser = eventActor(event)
		case IncidentEventAlertUnacknowledged:
			incident.Acknowledged = false
			incident.AckTime = nil
			incident.AckUser = ""
		case IncidentEventAlertResolved:
			incident.Status = IncidentStatusResolved
			timestamp := event.Timestamp
			incident.ClosedAt = &timestamp
		}
	}
}

func incidentEventFromResourceChange(change unifiedresources.ResourceChange) (IncidentEvent, bool) {
	eventType, ok := incidentEventTypeFromChangeKind(change.Kind)
	if !ok {
		return IncidentEvent{}, false
	}

	details := cloneIncidentEventDetails(change.Metadata)
	if user := strings.TrimSpace(change.Actor); user != "" {
		switch eventType {
		case IncidentEventAlertAcknowledged, IncidentEventAlertUnacknowledged,
			IncidentEventAlertSnoozed, IncidentEventAlertUnsnoozed:
			details["user"] = user
		}
	}

	evidence := change
	evidence.Metadata = cloneIncidentEventDetails(change.Metadata)
	evidence.RelatedResources = append([]string(nil), change.RelatedResources...)
	if change.OccurredAt != nil {
		occurred := *change.OccurredAt
		evidence.OccurredAt = &occurred
	}
	return IncidentEvent{
		Source:    "canonical_resource_history",
		Evidence:  &evidence,
		ID:        strings.TrimSpace(change.ID),
		Type:      eventType,
		Timestamp: incidentEventTimestamp(change),
		Summary:   incidentEventSummaryFromChange(change, eventType),
		Details:   details,
	}, true
}

func incidentEventTypeFromChangeKind(kind unifiedresources.ChangeKind) (IncidentEventType, bool) {
	switch kind {
	case unifiedresources.ChangeAlertFired:
		return IncidentEventAlertFired, true
	case unifiedresources.ChangeAlertAcknowledged:
		return IncidentEventAlertAcknowledged, true
	case unifiedresources.ChangeAlertUnacknowledged:
		return IncidentEventAlertUnacknowledged, true
	case unifiedresources.ChangeAlertSnoozed:
		return IncidentEventAlertSnoozed, true
	case unifiedresources.ChangeAlertUnsnoozed:
		return IncidentEventAlertUnsnoozed, true
	case unifiedresources.ChangeAlertResolved:
		return IncidentEventAlertResolved, true
	case unifiedresources.ChangeCommandExecuted:
		return IncidentEventCommand, true
	case unifiedresources.ChangeRunbookExecuted:
		return IncidentEventRunbook, true
	default:
		return "", false
	}
}

func incidentEventSummaryFromChange(change unifiedresources.ResourceChange, eventType IncidentEventType) string {
	switch eventType {
	case IncidentEventAlertFired:
		// The source owns the alert condition and comparison direction. Numeric
		// placeholders cannot establish either, including for legacy records.
		if message, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertMessage); ok {
			return message
		}
		if reason := strings.TrimSpace(change.Reason); reason != "" {
			return reason
		}
		if alertType, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertType); ok {
			if level, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertLevel); ok {
				return fmt.Sprintf("Alert triggered: %s (%s)", alertType, level)
			}
			return "Alert triggered: " + alertType
		}
		return "Alert triggered"
	case IncidentEventAlertAcknowledged:
		return "Alert acknowledged"
	case IncidentEventAlertUnacknowledged:
		return "Alert unacknowledged"
	case IncidentEventAlertSnoozed:
		return "Alert snoozed"
	case IncidentEventAlertUnsnoozed:
		return "Alert notifications resumed"
	case IncidentEventAlertResolved:
		return "Alert resolved"
	default:
		if summary := strings.TrimSpace(change.Reason); summary != "" {
			return summary
		}
		return unifiedresources.ChangeKindLabel(change.Kind)
	}
}

func incidentEventTimestamp(change unifiedresources.ResourceChange) time.Time {
	if change.OccurredAt != nil && !change.OccurredAt.IsZero() {
		return change.OccurredAt.UTC()
	}
	if !change.ObservedAt.IsZero() {
		return change.ObservedAt.UTC()
	}
	return time.Time{}
}

func projectedAlertIdentifier(change unifiedresources.ResourceChange) string {
	value, _ := stringMetadata(change.Metadata, unifiedresources.MetadataAlertIdentifier)
	return value
}

func stringMetadata(metadata map[string]any, key string) (string, bool) {
	if len(metadata) == 0 {
		return "", false
	}
	value, ok := metadata[key]
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	if !ok {
		return "", false
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return "", false
	}
	return str, true
}

func eventActor(event IncidentEvent) string {
	if event.Details == nil {
		return ""
	}
	value, ok := event.Details["user"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func isCanonicalProjectedIncidentEventType(eventType IncidentEventType) bool {
	switch eventType {
	case IncidentEventAlertFired,
		IncidentEventAlertAcknowledged,
		IncidentEventAlertUnacknowledged,
		IncidentEventAlertSnoozed,
		IncidentEventAlertUnsnoozed,
		IncidentEventAlertResolved,
		IncidentEventCommand,
		IncidentEventRunbook:
		return true
	default:
		return false
	}
}

func isSnapshotProjectionEvent(event IncidentEvent) bool {
	if event.Details == nil {
		return false
	}
	source, ok := event.Details["projection_source"].(string)
	return ok && source == incidentSnapshotSource
}

func sortIncidentEvents(events []IncidentEvent) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp.Equal(events[j].Timestamp) {
			iRank := incidentEventTimestampRank(events[i].Type)
			jRank := incidentEventTimestampRank(events[j].Type)
			if iRank != jRank {
				return iRank < jRank
			}
			return events[i].ID < events[j].ID
		}
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
}

func incidentEventTimestampRank(eventType IncidentEventType) int {
	switch eventType {
	case IncidentEventAlertFired:
		return 0
	case IncidentEventAlertResolved:
		return 2
	default:
		return 1
	}
}

func cloneIncidentEvent(event IncidentEvent) IncidentEvent {
	cloned := event
	cloned.Details = cloneIncidentEventDetails(event.Details)
	if event.Evidence != nil {
		evidence := *event.Evidence
		evidence.Metadata = cloneIncidentEventDetails(evidence.Metadata)
		evidence.RelatedResources = append([]string(nil), evidence.RelatedResources...)
		if evidence.OccurredAt != nil {
			occurred := *evidence.OccurredAt
			evidence.OccurredAt = &occurred
		}
		cloned.Evidence = &evidence
	}
	return cloned
}

func cloneIncidentEventDetails(details map[string]any) map[string]interface{} {
	if len(details) == 0 {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(details))
	for key, value := range details {
		cloned[key] = cloneIncidentDetailValue(value)
	}
	return cloned
}

func cloneIncidentDetailValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneIncidentEventDetails(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneIncidentDetailValue(item)
		}
		return out
	case []string:
		return append([]string(nil), v...)
	default:
		return value
	}
}

func newIncidentShellFromAlert(alert *alerts.Alert) *incidentShell {
	openedAt := alert.StartTime
	if openedAt.IsZero() {
		openedAt = time.Now()
	}

	return &incidentShell{
		ID:              generateIncidentID(),
		AlertIdentifier: alert.ID,
		AlertType:       alert.Type,
		Level:           string(alert.Level),
		ResourceID:      alert.ResourceID,
		ResourceName:    alert.ResourceName,
		Node:            alert.Node,
		Instance:        alert.Instance,
		Message:         alert.Message,
		OpenedAt:        openedAt,
		Events:          make([]IncidentEvent, 0),
	}
}

func updateIncidentShellFromAlert(shell *incidentShell, alert *alerts.Alert) {
	if shell == nil || alert == nil {
		return
	}
	shell.AlertType = alert.Type
	shell.Level = string(alert.Level)
	shell.ResourceID = alert.ResourceID
	shell.ResourceName = alert.ResourceName
	shell.Node = alert.Node
	shell.Instance = alert.Instance
	shell.Message = alert.Message
}

// Lifecycle snapshots carry the exact occurrence start. Unlike legacy read
// repair, they must not use a time tolerance or select an unrelated open/latest
// incident: delayed transitions and replay can arrive after a genuine recurrence.
func (s *IncidentStore) findLifecycleOccurrenceLocked(alert *alerts.Alert) *incidentShell {
	if alert.ID == "" {
		return nil
	}
	if alert.StartTime.IsZero() {
		// Legacy callers without an occurrence key retain best-effort matching.
		return s.findLatestIncidentByAlertIdentifierLocked(alert.ID)
	}
	for i := len(s.incidents) - 1; i >= 0; i-- {
		shell := s.incidents[i]
		if shell != nil && shell.AlertIdentifier == alert.ID && shell.OpenedAt.Equal(alert.StartTime) {
			return shell
		}
	}
	return nil
}

func (s *IncidentStore) ensureIncidentForAlertLocked(alert *alerts.Alert) *incidentShell {
	shell := s.findLifecycleOccurrenceLocked(alert)
	if shell == nil {
		shell = newIncidentShellFromAlert(alert)
		s.incidents = append(s.incidents, shell)
	}
	updateIncidentShellFromAlert(shell, alert)
	return shell
}

func resetDerivedIncidentState(incident *Incident) {
	if incident == nil {
		return
	}
	incident.Status = IncidentStatusOpen
	incident.ClosedAt = nil
	incident.Acknowledged = false
	incident.AckUser = ""
	incident.AckTime = nil
}

func (s *IncidentStore) addEventLocked(shell *incidentShell, eventType IncidentEventType, summary string, details map[string]interface{}) {
	s.addEventAtLocked(shell, eventType, time.Now(), summary, details)
}

func (s *IncidentStore) addEventAtLocked(shell *incidentShell, eventType IncidentEventType, occurredAt time.Time, summary string, details map[string]interface{}) {
	if shell == nil {
		return
	}
	if summary == "" {
		summary = string(eventType)
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	event := IncidentEvent{
		ID:        generateIncidentEventID(),
		Type:      eventType,
		Timestamp: occurredAt,
		Summary:   summary,
		Details:   details,
	}
	if eventType == IncidentEventNote {
		event.Source = "operator_note"
	}
	if isSnapshotProjectionEvent(event) {
		event.Source = incidentSnapshotSource
	}

	shell.Events = append(shell.Events, event)
	if s.maxEvents > 0 && len(shell.Events) > s.maxEvents {
		shell.Events = shell.Events[len(shell.Events)-s.maxEvents:]
	}
}

func (s *IncidentStore) findOpenIncidentByAlertIdentifierLocked(alertIdentifier string) *incidentShell {
	if alertIdentifier == "" {
		return nil
	}
	for i := len(s.incidents) - 1; i >= 0; i-- {
		shell := s.incidents[i]
		if shell != nil && shell.AlertIdentifier == alertIdentifier && incidentOccurrenceClosedAt(shell) == nil {
			return shell
		}
	}
	return nil
}

func (s *IncidentStore) findLatestIncidentByAlertIdentifierLocked(alertIdentifier string) *incidentShell {
	if alertIdentifier == "" {
		return nil
	}
	for i := len(s.incidents) - 1; i >= 0; i-- {
		shell := s.incidents[i]
		if shell != nil && shell.AlertIdentifier == alertIdentifier {
			return shell
		}
	}
	return nil
}

func (s *IncidentStore) findIncidentByIDLocked(incidentID string) *incidentShell {
	if incidentID == "" {
		return nil
	}
	for i := len(s.incidents) - 1; i >= 0; i-- {
		shell := s.incidents[i]
		if shell != nil && shell.ID == incidentID {
			return shell
		}
	}
	return nil
}

func (s *IncidentStore) trimLocked() {
	if s.maxAge > 0 {
		cutoff := time.Now().Add(-s.maxAge)
		filtered := make([]*incidentShell, 0, len(s.incidents))
		for _, shell := range s.incidents {
			if shell == nil {
				continue
			}
			compareTime := shell.OpenedAt
			if closedAt := incidentOccurrenceClosedAt(shell); closedAt != nil {
				compareTime = *closedAt
			}
			if compareTime.After(cutoff) {
				filtered = append(filtered, shell)
			}
		}
		s.incidents = filtered
	}

	if s.maxIncidents > 0 && len(s.incidents) > s.maxIncidents {
		sort.Slice(s.incidents, func(i, j int) bool {
			return s.incidents[i].OpenedAt.Before(s.incidents[j].OpenedAt)
		})
		if len(s.incidents) > s.maxIncidents {
			s.incidents = s.incidents[len(s.incidents)-s.maxIncidents:]
		}
	}
}

// trimLockedPreserving applies the ordinary retention policy while keeping the
// occurrence currently being read-repaired. This lets a user open an older
// retained alert-history row even when the bounded incident cache is already
// full; another least-recent occurrence becomes repairable on demand instead.
func (s *IncidentStore) trimLockedPreserving(protected *incidentShell) {
	s.trimLocked()
	if protected == nil {
		return
	}
	for _, shell := range s.incidents {
		if shell == protected {
			return
		}
	}

	if s.maxAge > 0 {
		compareTime := protected.OpenedAt
		if closedAt := incidentOccurrenceClosedAt(protected); closedAt != nil {
			compareTime = *closedAt
		}
		if compareTime.Before(time.Now().Add(-s.maxAge)) {
			return
		}
	}

	s.incidents = append(s.incidents, protected)
	if s.maxIncidents <= 0 || len(s.incidents) <= s.maxIncidents {
		return
	}
	sort.Slice(s.incidents, func(i, j int) bool {
		return s.incidents[i].OpenedAt.Before(s.incidents[j].OpenedAt)
	})
	for len(s.incidents) > s.maxIncidents {
		removeAt := -1
		for index, shell := range s.incidents {
			if shell != protected {
				removeAt = index
				break
			}
		}
		if removeAt < 0 {
			break
		}
		s.incidents = append(s.incidents[:removeAt], s.incidents[removeAt+1:]...)
	}
}

func (s *IncidentStore) saveAsync() {
	if s.dataDir == "" || s.filePath == "" {
		return
	}
	// Coalesce: each save marshals the whole store, so a burst of mutations
	// (lifecycle replay, alert storms) must not queue one full serialization
	// per call. One pending save is enough — it snapshots the store when it
	// runs, so it covers every mutation made before it started, and any
	// mutation after the pending flag clears queues exactly one more save.
	if !s.savePending.CompareAndSwap(false, true) {
		return
	}
	go func() {
		if err := s.saveToDisk(); err != nil {
			log.Warn().Err(err).Msg("failed to save incident history")
		}
	}()
}

func (s *IncidentStore) saveToDisk() error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	// Clear the coalescing flag only once this save actually starts: the
	// snapshot below covers every mutation made before this point, and a
	// mutation after it queues exactly one follow-up save.
	s.savePending.Store(false)

	if s.dataDir == "" || s.filePath == "" {
		return nil
	}
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return err
	}

	s.mu.RLock()
	snapshot := make([]*incidentShell, 0, len(s.incidents))
	for _, shell := range s.incidents {
		snapshot = append(snapshot, cloneIncidentShell(shell))
	}
	s.mu.RUnlock()

	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	// Coalescing handles concurrent bursts, but sequential lifecycle replays
	// can also produce identical snapshots. Compare the actual recovery file
	// rather than caching a hash: restarts, failed writes and missing files
	// must not prevent the next checkpoint from restoring durable state.
	if file, err := os.Open(s.filePath); err == nil {
		// One extra byte detects a longer file without an unbounded read.
		existing, readErr := io.ReadAll(io.LimitReader(file, int64(len(data))+1))
		file.Close()
		if readErr == nil && bytes.Equal(existing, data) {
			return nil
		}
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpFile, s.filePath); err != nil {
		return err
	}
	s.savesCompleted.Add(1)
	return nil
}

func (s *IncidentStore) loadFromDisk() error {
	if s.filePath == "" {
		return nil
	}

	info, err := os.Stat(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() > maxIncidentFileSize {
		return fmt.Errorf("incident history file too large (%d bytes)", info.Size())
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var incidents []*incidentShell
	if err := json.Unmarshal(data, &incidents); err != nil {
		return err
	}

	for _, shell := range incidents {
		normalizeIncidentShellState(shell)
	}
	s.incidents = incidents
	s.trimLocked()
	return nil
}

func incidentFromShell(shell *incidentShell) *Incident {
	if shell == nil {
		return nil
	}
	incident := &Incident{
		ID:              shell.ID,
		AlertIdentifier: shell.AlertIdentifier,
		AlertType:       shell.AlertType,
		Level:           shell.Level,
		ResourceID:      shell.ResourceID,
		ResourceName:    shell.ResourceName,
		ResourceType:    shell.ResourceType,
		Node:            shell.Node,
		Instance:        shell.Instance,
		Message:         shell.Message,
		Status:          IncidentStatusOpen,
		OpenedAt:        shell.OpenedAt,
		Events:          cloneIncidentEvents(shell.Events),
	}
	if shell.OpenedAt.IsZero() {
		incident.Status = IncidentStatusUnknown
	}
	if shell.OccurrenceClosedAt != nil {
		closed := *shell.OccurrenceClosedAt
		incident.ClosedAt = &closed
		incident.Status = IncidentStatusResolved
	}
	applyProjectedIncidentState(incident, incident.Events)
	incident.OpenedAt = shell.OpenedAt
	return incident
}

func cloneIncident(src *Incident) *Incident {
	if src == nil {
		return nil
	}
	clone := *src
	if src.AckTime != nil {
		t := *src.AckTime
		clone.AckTime = &t
	}
	if src.ClosedAt != nil {
		t := *src.ClosedAt
		clone.ClosedAt = &t
	}
	clone.Events = cloneIncidentEvents(src.Events)
	if src.History != nil {
		history := *src.History
		clone.History = &history
	}

	return &clone
}

func cloneIncidentShell(src *incidentShell) *incidentShell {
	if src == nil {
		return nil
	}
	clone := *src
	if src.OccurrenceClosedAt != nil {
		t := *src.OccurrenceClosedAt
		clone.OccurrenceClosedAt = &t
	}
	clone.Events = cloneIncidentEvents(src.Events)
	return &clone
}

func cloneIncidentEvents(src []IncidentEvent) []IncidentEvent {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]IncidentEvent, len(src))
	for i, event := range src {
		cloned[i] = cloneIncidentEvent(event)
	}
	return cloned
}

func incidentOccurrenceClosedAt(shell *incidentShell) *time.Time {
	if shell == nil {
		return nil
	}
	return shell.OccurrenceClosedAt
}

func normalizeIncidentShellState(shell *incidentShell) {
	if shell == nil {
		return
	}
	sortIncidentEvents(shell.Events)
	if shell.OccurrenceClosedAt == nil {
		for i := len(shell.Events) - 1; i >= 0; i-- {
			if shell.Events[i].Type == IncidentEventAlertResolved {
				shell.OccurrenceClosedAt = cloneTime(shell.Events[i].Timestamp)
				break
			}
		}
	}
}

func hasIncidentEventType(events []IncidentEvent, types ...IncidentEventType) bool {
	if len(events) == 0 || len(types) == 0 {
		return false
	}
	wanted := make(map[IncidentEventType]struct{}, len(types))
	for _, eventType := range types {
		wanted[eventType] = struct{}{}
	}
	for _, event := range events {
		if _, ok := wanted[event.Type]; ok {
			return true
		}
	}
	return false
}

func cloneTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	cloned := value
	return &cloned
}

var incidentCounter int64

func generateIncidentID() string {
	incidentCounter++
	return "inc-" + time.Now().Format("20060102150405") + "-" + intToString(int(incidentCounter%1000))
}

var incidentEventCounter int64

func generateIncidentEventID() string {
	incidentEventCounter++
	return "inc-evt-" + time.Now().Format("20060102150405") + "-" + intToString(int(incidentEventCounter%1000))
}

func formatAlertSummary(alert *alerts.Alert) string {
	if alert == nil {
		return "Alert triggered"
	}
	summary := fmt.Sprintf("Alert triggered: %s (%s)", alert.Type, alert.Level)
	if alert.Value > 0 || alert.Threshold > 0 {
		summary = fmt.Sprintf("Alert triggered: %s (%s %.1f >= %.1f)", alert.Type, alert.Level, alert.Value, alert.Threshold)
	}
	return summary
}
