package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// IncidentStatus represents the current state of an incident.
type IncidentStatus string

const (
	IncidentStatusOpen     IncidentStatus = "open"
	IncidentStatusResolved IncidentStatus = "resolved"
)

// IncidentEventType describes a timeline event type.
type IncidentEventType string

const (
	IncidentEventAlertFired          IncidentEventType = "alert_fired"
	IncidentEventAlertAcknowledged   IncidentEventType = "alert_acknowledged"
	IncidentEventAlertUnacknowledged IncidentEventType = "alert_unacknowledged"
	IncidentEventAlertResolved       IncidentEventType = "alert_resolved"
	IncidentEventAnalysis            IncidentEventType = "ai_analysis"
	IncidentEventCommand             IncidentEventType = "command"
	IncidentEventRunbook             IncidentEventType = "runbook"
	IncidentEventNote                IncidentEventType = "note"
)

// IncidentEvent represents a single timeline entry for an incident.
type IncidentEvent struct {
	ID        string                 `json:"id"`
	Type      IncidentEventType      `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Summary   string                 `json:"summary"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// Incident captures an alert occurrence and its investigation timeline.
// It is an alert-scoped memory/projection for investigation support rather than
// the canonical durable resource-change history.
type Incident struct {
	ID              string          `json:"id"`
	AlertIdentifier string          `json:"alertIdentifier"`
	AlertType       string          `json:"alertType"`
	Level           string          `json:"level"`
	ResourceID      string          `json:"resourceId"`
	ResourceName    string          `json:"resourceName"`
	ResourceType    string          `json:"resourceType,omitempty"`
	Node            string          `json:"node,omitempty"`
	Instance        string          `json:"instance,omitempty"`
	Message         string          `json:"message,omitempty"`
	Status          IncidentStatus  `json:"status"`
	OpenedAt        time.Time       `json:"openedAt"`
	ClosedAt        *time.Time      `json:"closedAt,omitempty"`
	Acknowledged    bool            `json:"acknowledged"`
	AckUser         string          `json:"ackUser,omitempty"`
	AckTime         *time.Time      `json:"ackTime,omitempty"`
	Events          []IncidentEvent `json:"events,omitempty"`
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
	ID              string          `json:"id"`
	AlertIdentifier string          `json:"alertIdentifier"`
	AlertType       string          `json:"alertType"`
	Level           string          `json:"level"`
	ResourceID      string          `json:"resourceId"`
	ResourceName    string          `json:"resourceName"`
	ResourceType    string          `json:"resourceType,omitempty"`
	Node            string          `json:"node,omitempty"`
	Instance        string          `json:"instance,omitempty"`
	Message         string          `json:"message,omitempty"`
	Status          IncidentStatus  `json:"status"`
	OpenedAt        time.Time       `json:"openedAt"`
	ClosedAt        *time.Time      `json:"closedAt,omitempty"`
	Acknowledged    bool            `json:"acknowledged"`
	AckUser         string          `json:"ackUser,omitempty"`
	AckTime         *time.Time      `json:"ackTime,omitempty"`
	Events          []IncidentEvent `json:"events,omitempty"`
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
	incidents             []*incidentShell
	maxIncidents          int
	maxEvents             int
	maxAge                time.Duration
	dataDir               string
	filePath              string
	resourceTimelineStore IncidentTimelineStore
}

const (
	defaultIncidentMaxIncidents  = 500
	defaultIncidentMaxEvents     = 120
	defaultIncidentMaxAgeDays    = 90
	incidentFileName             = "ai_incidents.json"
	maxIncidentFileSize          = 20 * 1024 * 1024 // 20MB
	incidentStartMatchTolerance  = 10 * time.Minute
	projectedIncidentChangeLimit = 256
)

// IncidentTimelineStore exposes the canonical resource timeline used to derive
// incident lifecycle and remediation history.
type IncidentTimelineStore interface {
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
		dataDir:      cfg.DataDir,
	}

	if store.dataDir != "" {
		store.filePath = filepath.Join(store.dataDir, incidentFileName)
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

	shell := s.findOpenIncidentByAlertIdentifierLocked(alert.ID)
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

	shell := s.findOpenIncidentByAlertIdentifierLocked(alert.ID)
	if shell == nil {
		shell = newIncidentShellFromAlert(alert)
		s.incidents = append(s.incidents, shell)
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

	s.mu.Lock()
	defer s.mu.Unlock()

	var shell *incidentShell
	if incidentID != "" {
		shell = s.findIncidentByIDLocked(incidentID)
	} else if alertIdentifier != "" {
		shell = s.findLatestIncidentByAlertIdentifierLocked(alertIdentifier)
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
	if alertIdentifier == "" {
		return nil
	}

	s.mu.RLock()
	shell := cloneIncidentShell(s.findLatestIncidentByAlertIdentifierLocked(alertIdentifier))
	timelineStore := s.resourceTimelineStore
	maxAge := s.maxAge
	s.mu.RUnlock()

	if shell == nil {
		return s.projectIncidentFromCanonical(alertIdentifier, time.Time{}, timelineStore, maxAge)
	}
	return s.projectIncident(incidentFromShell(shell), timelineStore)
}

// GetTimelineByAlertAt returns the incident closest to the provided start time for an alert.
func (s *IncidentStore) GetTimelineByAlertAt(alertIdentifier string, startedAt time.Time) *Incident {
	if alertIdentifier == "" {
		return nil
	}
	if startedAt.IsZero() {
		return s.GetTimelineByAlertIdentifier(alertIdentifier)
	}

	s.mu.RLock()
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
	timelineStore := s.resourceTimelineStore
	maxAge := s.maxAge
	s.mu.RUnlock()

	if best == nil || bestDelta > incidentStartMatchTolerance {
		return s.projectIncidentFromCanonical(alertIdentifier, startedAt, timelineStore, maxAge)
	}
	return s.projectIncident(incidentFromShell(cloneIncidentShell(best)), timelineStore)
}

// ListIncidentsByResource returns recent incidents for a resource.
func (s *IncidentStore) ListIncidentsByResource(resourceID string, limit int) []*Incident {
	if resourceID == "" {
		return nil
	}

	s.mu.RLock()
	var matches []*Incident
	for i := len(s.incidents) - 1; i >= 0; i-- {
		shell := s.incidents[i]
		if shell != nil && shell.ResourceID == resourceID {
			matches = append(matches, incidentFromShell(cloneIncidentShell(shell)))
			if limit > 0 && len(matches) >= limit {
				break
			}
		}
	}
	timelineStore := s.resourceTimelineStore
	s.mu.RUnlock()

	if timelineStore == nil {
		return matches
	}
	projected := make([]*Incident, 0, len(matches))
	for _, incident := range matches {
		projected = append(projected, s.projectIncident(incident, timelineStore))
	}
	return projected
}

// FormatForAlert returns a condensed incident timeline for prompt injection.
func (s *IncidentStore) FormatForAlert(alertIdentifier string, maxEvents int) string {
	incident := s.GetTimelineByAlertIdentifier(alertIdentifier)
	if incident == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Incident Memory\n")
	b.WriteString(fmt.Sprintf("Alert incident for %s (%s, %s)\n",
		incident.ResourceName, incident.AlertType, incident.Level))
	b.WriteString(fmt.Sprintf("Status: %s\n", incident.Status))

	events := incident.Events
	if maxEvents > 0 && len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
	}
	for _, evt := range events {
		b.WriteString("- ")
		b.WriteString(evt.Timestamp.Format(time.RFC3339))
		b.WriteString(": ")
		b.WriteString(evt.Summary)
		b.WriteString("\n")
	}
	return b.String()
}

// FormatForResource returns a condensed incident summary for a resource.
func (s *IncidentStore) FormatForResource(resourceID string, limit int) string {
	incidents := s.ListIncidentsByResource(resourceID, limit)
	if len(incidents) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Incident Memory\n")
	b.WriteString("Recent incidents for this resource:\n")
	for _, incident := range incidents {
		status := string(incident.Status)
		if incident.Acknowledged && incident.Status == IncidentStatusOpen {
			status = "acknowledged"
		}
		b.WriteString("- ")
		b.WriteString(incident.OpenedAt.Format(time.RFC3339))
		b.WriteString(": ")
		b.WriteString(incident.AlertType)
		if incident.Level != "" {
			b.WriteString(" (")
			b.WriteString(incident.Level)
			b.WriteString(")")
		}
		b.WriteString(" - ")
		b.WriteString(status)
		b.WriteString("\n")
	}
	return b.String()
}

// FormatForPatrol returns a condensed incident summary for infrastructure-wide patrol analysis.
func (s *IncidentStore) FormatForPatrol(limit int) string {
	if limit <= 0 {
		limit = 8
	}

	s.mu.RLock()
	snapshot := make([]*Incident, 0, len(s.incidents))
	for i := len(s.incidents) - 1; i >= 0 && len(snapshot) < limit; i-- {
		if shell := s.incidents[i]; shell != nil {
			snapshot = append(snapshot, incidentFromShell(cloneIncidentShell(shell)))
		}
	}
	timelineStore := s.resourceTimelineStore
	s.mu.RUnlock()

	if len(snapshot) == 0 {
		return ""
	}

	if timelineStore != nil {
		for i := range snapshot {
			snapshot[i] = s.projectIncident(snapshot[i], timelineStore)
		}
	}

	var b strings.Builder
	b.WriteString("\n\n## Incident Memory\n")
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
		b.WriteString(incident.OpenedAt.Format(time.RFC3339))
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

func (s *IncidentStore) projectIncident(incident *Incident, timelineStore IncidentTimelineStore) *Incident {
	if incident == nil || timelineStore == nil {
		return incident
	}

	projectedEvents := s.loadProjectedIncidentEvents(incident, timelineStore)
	if len(projectedEvents) == 0 {
		return incident
	}

	projected := cloneIncident(incident)
	resetDerivedIncidentState(projected)
	filtered := make([]IncidentEvent, 0, len(projected.Events)+len(projectedEvents))
	for _, event := range projected.Events {
		if isCanonicalProjectedIncidentEventType(event.Type) {
			continue
		}
		filtered = append(filtered, cloneIncidentEvent(event))
	}
	filtered = append(filtered, projectedEvents...)
	sortIncidentEvents(filtered)
	projected.Events = filtered
	applyProjectedIncidentState(projected, projectedEvents)
	return projected
}

func (s *IncidentStore) projectIncidentFromCanonical(alertIdentifier string, startedAt time.Time, timelineStore IncidentTimelineStore, maxAge time.Duration) *Incident {
	if timelineStore == nil || strings.TrimSpace(alertIdentifier) == "" {
		return nil
	}

	since := time.Now().Add(-defaultIncidentMaxAgeDays * 24 * time.Hour)
	if maxAge > 0 {
		since = time.Now().Add(-maxAge)
	}

	changes, err := timelineStore.GetRecentChanges("", since, projectedIncidentChangeLimit)
	if err != nil || len(changes) == 0 {
		return nil
	}

	events := make([]IncidentEvent, 0, len(changes))
	projected := &Incident{
		ID:              "projected-" + strings.TrimSpace(alertIdentifier),
		AlertIdentifier: strings.TrimSpace(alertIdentifier),
		Status:          IncidentStatusOpen,
	}
	for _, change := range changes {
		if projectedAlertIdentifier(change) != projected.AlertIdentifier {
			continue
		}
		event, ok := incidentEventFromResourceChange(change)
		if !ok {
			continue
		}
		events = append(events, event)
		hydrateIncidentFromCanonicalChange(projected, change)
	}
	if len(events) == 0 {
		return nil
	}

	sortIncidentEvents(events)
	projected.Events = events
	if !startedAt.IsZero() {
		openAt := projected.OpenedAt
		if openAt.IsZero() {
			openAt = events[0].Timestamp
		}
		delta := openAt.Sub(startedAt)
		if delta < 0 {
			delta = -delta
		}
		if delta > incidentStartMatchTolerance {
			return nil
		}
	}
	applyProjectedIncidentState(projected, events)
	return projected
}

func (s *IncidentStore) loadProjectedIncidentEvents(incident *Incident, timelineStore IncidentTimelineStore) []IncidentEvent {
	if incident == nil || timelineStore == nil {
		return nil
	}

	resourceID := strings.TrimSpace(incident.ResourceID)
	alertIdentifier := strings.TrimSpace(incident.AlertIdentifier)
	if resourceID == "" || alertIdentifier == "" {
		return nil
	}

	since := incident.OpenedAt
	if since.IsZero() {
		since = time.Now().Add(-defaultIncidentMaxAgeDays * 24 * time.Hour)
	} else {
		since = since.Add(-incidentStartMatchTolerance)
	}

	changes, err := timelineStore.GetRecentChanges(resourceID, since, projectedIncidentChangeLimit)
	if err != nil || len(changes) == 0 {
		return nil
	}

	events := make([]IncidentEvent, 0, len(changes))
	for _, change := range changes {
		if projectedAlertIdentifier(change) != alertIdentifier {
			continue
		}
		event, ok := incidentEventFromResourceChange(change)
		if !ok {
			continue
		}
		events = append(events, event)
		hydrateIncidentFromCanonicalChange(incident, change)
	}
	sortIncidentEvents(events)
	return events
}

func hydrateIncidentFromCanonicalChange(incident *Incident, change unifiedresources.ResourceChange) {
	if incident == nil {
		return
	}
	if resourceID := strings.TrimSpace(change.ResourceID); resourceID != "" && incident.ResourceID == "" {
		incident.ResourceID = resourceID
	}
	if alertType, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertType); ok && incident.AlertType == "" {
		incident.AlertType = alertType
	}
	if level, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertLevel); ok && incident.Level == "" {
		incident.Level = level
	}
	if message, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertMessage); ok && incident.Message == "" {
		incident.Message = message
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
		case IncidentEventAlertAcknowledged, IncidentEventAlertUnacknowledged:
			details["user"] = user
		}
	}

	return IncidentEvent{
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
		if alertType, ok := stringMetadata(change.Metadata, unifiedresources.MetadataAlertType); ok {
			level, _ := stringMetadata(change.Metadata, unifiedresources.MetadataAlertLevel)
			value, hasValue := floatMetadata(change.Metadata, unifiedresources.MetadataAlertValue)
			threshold, hasThreshold := floatMetadata(change.Metadata, unifiedresources.MetadataAlertThreshold)
			if hasValue || hasThreshold {
				return fmt.Sprintf("Alert triggered: %s (%s %.1f >= %.1f)", alertType, level, value, threshold)
			}
			if level != "" {
				return fmt.Sprintf("Alert triggered: %s (%s)", alertType, level)
			}
		}
		return "Alert triggered"
	case IncidentEventAlertAcknowledged:
		return "Alert acknowledged"
	case IncidentEventAlertUnacknowledged:
		return "Alert unacknowledged"
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
	return time.Now().UTC()
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

func floatMetadata(metadata map[string]any, key string) (float64, bool) {
	if len(metadata) == 0 {
		return 0, false
	}
	value, ok := metadata[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
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
		IncidentEventAlertResolved,
		IncidentEventCommand,
		IncidentEventRunbook:
		return true
	default:
		return false
	}
}

func sortIncidentEvents(events []IncidentEvent) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp.Equal(events[j].Timestamp) {
			return events[i].ID < events[j].ID
		}
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
}

func cloneIncidentEvent(event IncidentEvent) IncidentEvent {
	cloned := event
	cloned.Details = cloneIncidentEventDetails(event.Details)
	return cloned
}

func cloneIncidentEventDetails(details map[string]any) map[string]interface{} {
	if len(details) == 0 {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(details))
	for key, value := range details {
		cloned[key] = value
	}
	return cloned
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

func (s *IncidentStore) ensureIncidentForAlertLocked(alert *alerts.Alert) *incidentShell {
	shell := s.findLatestIncidentByAlertIdentifierLocked(alert.ID)
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

func (s *IncidentStore) saveAsync() {
	if s.dataDir == "" || s.filePath == "" {
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

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpFile, s.filePath); err != nil {
		return err
	}
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
	applyProjectedIncidentState(incident, incident.Events)
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
	if len(src.Events) > 0 {
		clone.Events = make([]IncidentEvent, len(src.Events))
		for i, event := range src.Events {
			cloneEvent := event
			if event.Details != nil {
				detailsCopy := make(map[string]interface{}, len(event.Details))
				for key, value := range event.Details {
					detailsCopy[key] = value
				}
				cloneEvent.Details = detailsCopy
			}
			clone.Events[i] = cloneEvent
		}
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
