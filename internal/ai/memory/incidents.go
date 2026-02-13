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

// Incident captures an alert occurrence and its timeline.
type Incident struct {
	ID           string          `json:"id"`
	AlertID      string          `json:"alertId"`
	AlertType    string          `json:"alertType"`
	Level        string          `json:"level"`
	ResourceID   string          `json:"resourceId"`
	ResourceName string          `json:"resourceName"`
	ResourceType string          `json:"resourceType,omitempty"`
	Node         string          `json:"node,omitempty"`
	Instance     string          `json:"instance,omitempty"`
	Message      string          `json:"message,omitempty"`
	Status       IncidentStatus  `json:"status"`
	OpenedAt     time.Time       `json:"openedAt"`
	ClosedAt     *time.Time      `json:"closedAt,omitempty"`
	Acknowledged bool            `json:"acknowledged"`
	AckUser      string          `json:"ackUser,omitempty"`
	AckTime      *time.Time      `json:"ackTime,omitempty"`
	Events       []IncidentEvent `json:"events,omitempty"`
}

// IncidentStoreConfig configures incident retention and persistence.
type IncidentStoreConfig struct {
	DataDir              string
	MaxIncidents         int
	MaxEventsPerIncident int
	MaxAgeDays           int
}

// IncidentStore maintains incident timelines and persistence.
type IncidentStore struct {
	mu           sync.RWMutex
	saveMu       sync.Mutex
	incidents    []*Incident
	maxIncidents int
	maxEvents    int
	maxAge       time.Duration
	dataDir      string
	filePath     string
}

const (
	defaultIncidentMaxIncidents = 500
	defaultIncidentMaxEvents    = 120
	defaultIncidentMaxAgeDays   = 90
	incidentFileName            = "ai_incidents.json"
	maxIncidentFileSize         = 20 * 1024 * 1024 // 20MB
	incidentStartMatchTolerance = 10 * time.Minute
)

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
		incidents:    make([]*Incident, 0),
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

// RecordAlertFired opens or updates an incident for a fired alert.
func (s *IncidentStore) RecordAlertFired(alert *alerts.Alert) {
	if alert == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	incident := s.findOpenIncidentByAlertIDLocked(alert.ID)
	if incident == nil {
		incident = newIncidentFromAlert(alert)
		s.incidents = append(s.incidents, incident)
		s.addEventLocked(incident, IncidentEventAlertFired, formatAlertSummary(alert), map[string]interface{}{
			"type":      alert.Type,
			"level":     string(alert.Level),
			"value":     alert.Value,
			"threshold": alert.Threshold,
		})
	} else {
		updateIncidentFromAlert(incident, alert)
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

	incident := s.ensureIncidentForAlertLocked(alert)

	incident.Acknowledged = true
	if alert.AckTime != nil {
		incident.AckTime = alert.AckTime
	} else {
		now := time.Now()
		incident.AckTime = &now
	}
	incident.AckUser = user

	s.addEventLocked(incident, IncidentEventAlertAcknowledged, "Alert acknowledged", map[string]interface{}{
		"user": user,
	})

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

	incident := s.ensureIncidentForAlertLocked(alert)

	incident.Acknowledged = false
	incident.AckTime = nil
	incident.AckUser = ""

	s.addEventLocked(incident, IncidentEventAlertUnacknowledged, "Alert unacknowledged", map[string]interface{}{
		"user": user,
	})

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

	incident := s.findOpenIncidentByAlertIDLocked(alert.ID)
	if incident == nil {
		incident = newIncidentFromAlert(alert)
		s.incidents = append(s.incidents, incident)
	}

	incident.Status = IncidentStatusResolved
	if resolvedAt.IsZero() {
		now := time.Now()
		resolvedAt = now
	}
	incident.ClosedAt = &resolvedAt

	s.addEventLocked(incident, IncidentEventAlertResolved, "Alert resolved", map[string]interface{}{
		"resolved_at": resolvedAt.Format(time.RFC3339),
	})

	s.trimLocked()
	s.saveAsync()
}

// RecordAnalysis adds an AI analysis event to the incident for an alert.
func (s *IncidentStore) RecordAnalysis(alertID, summary string, details map[string]interface{}) {
	if alertID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	incident := s.findLatestIncidentByAlertIDLocked(alertID)
	if incident == nil {
		incident = &Incident{
			ID:       generateIncidentID(),
			AlertID:  alertID,
			Status:   IncidentStatusOpen,
			OpenedAt: time.Now(),
		}
		s.incidents = append(s.incidents, incident)
	}

	if summary == "" {
		summary = "Pulse Patrol analysis completed"
	}

	s.addEventLocked(incident, IncidentEventAnalysis, summary, details)
	s.trimLocked()
	s.saveAsync()
}

// RecordCommand adds a command execution event to the incident for an alert.
func (s *IncidentStore) RecordCommand(alertID, command string, success bool, output string, details map[string]interface{}) {
	if alertID == "" || command == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	incident := s.findLatestIncidentByAlertIDLocked(alertID)
	if incident == nil {
		incident = &Incident{
			ID:       generateIncidentID(),
			AlertID:  alertID,
			Status:   IncidentStatusOpen,
			OpenedAt: time.Now(),
		}
		s.incidents = append(s.incidents, incident)
	}

	if details == nil {
		details = make(map[string]interface{})
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

	s.addEventLocked(incident, IncidentEventCommand, summary, details)
	s.trimLocked()
	s.saveAsync()
}

// RecordRunbook adds a runbook execution event to the incident for an alert.
func (s *IncidentStore) RecordRunbook(alertID, runbookID, title string, outcome string, automatic bool, message string) {
	if alertID == "" || runbookID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	incident := s.findLatestIncidentByAlertIDLocked(alertID)
	if incident == nil {
		incident = &Incident{
			ID:       generateIncidentID(),
			AlertID:  alertID,
			Status:   IncidentStatusOpen,
			OpenedAt: time.Now(),
		}
		s.incidents = append(s.incidents, incident)
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

	s.addEventLocked(incident, IncidentEventRunbook, summary, details)
	s.trimLocked()
	s.saveAsync()
}

// RecordNote appends a user note to an incident identified by alert ID or incident ID.
func (s *IncidentStore) RecordNote(alertID, incidentID, note, user string) bool {
	note = strings.TrimSpace(note)
	if note == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var incident *Incident
	if incidentID != "" {
		incident = s.findIncidentByIDLocked(incidentID)
	} else if alertID != "" {
		incident = s.findLatestIncidentByAlertIDLocked(alertID)
	}
	if incident == nil {
		return false
	}

	summary := "Note added"
	if user != "" {
		summary = fmt.Sprintf("Note added by %s", user)
	}

	s.addEventLocked(incident, IncidentEventNote, summary, map[string]interface{}{
		"note": note,
		"user": user,
	})

	s.trimLocked()
	s.saveAsync()
	return true
}

// GetTimelineByAlertID returns the most recent incident for the alert.
func (s *IncidentStore) GetTimelineByAlertID(alertID string) *Incident {
	if alertID == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	incident := s.findLatestIncidentByAlertIDLocked(alertID)
	if incident == nil {
		return nil
	}
	return cloneIncident(incident)
}

// GetTimelineByAlertAt returns the incident closest to the provided start time for an alert.
func (s *IncidentStore) GetTimelineByAlertAt(alertID string, startedAt time.Time) *Incident {
	if alertID == "" {
		return nil
	}
	if startedAt.IsZero() {
		return s.GetTimelineByAlertID(alertID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var best *Incident
	var bestDelta time.Duration
	for _, incident := range s.incidents {
		if incident == nil || incident.AlertID != alertID {
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

	if best == nil || bestDelta > incidentStartMatchTolerance {
		return nil
	}
	return cloneIncident(best)
}

// ListIncidentsByResource returns recent incidents for a resource.
func (s *IncidentStore) ListIncidentsByResource(resourceID string, limit int) []*Incident {
	if resourceID == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []*Incident
	for i := len(s.incidents) - 1; i >= 0; i-- {
		incident := s.incidents[i]
		if incident != nil && incident.ResourceID == resourceID {
			matches = append(matches, cloneIncident(incident))
			if limit > 0 && len(matches) >= limit {
				break
			}
		}
	}
	return matches
}

// FormatForAlert returns a condensed incident timeline for prompt injection.
func (s *IncidentStore) FormatForAlert(alertID string, maxEvents int) string {
	incident := s.GetTimelineByAlertID(alertID)
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
	defer s.mu.RUnlock()

	if len(s.incidents) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Incident Memory\n")
	b.WriteString("Recent incidents across infrastructure:\n")

	count := 0
	for i := len(s.incidents) - 1; i >= 0 && count < limit; i-- {
		incident := s.incidents[i]
		if incident == nil {
			continue
		}
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
		count++
	}

	return b.String()
}

func newIncidentFromAlert(alert *alerts.Alert) *Incident {
	openedAt := alert.StartTime
	if openedAt.IsZero() {
		openedAt = time.Now()
	}

	return &Incident{
		ID:           generateIncidentID(),
		AlertID:      alert.ID,
		AlertType:    alert.Type,
		Level:        string(alert.Level),
		ResourceID:   alert.ResourceID,
		ResourceName: alert.ResourceName,
		Node:         alert.Node,
		Instance:     alert.Instance,
		Message:      alert.Message,
		Status:       IncidentStatusOpen,
		OpenedAt:     openedAt,
		Acknowledged: alert.Acknowledged,
		AckUser:      alert.AckUser,
		AckTime:      alert.AckTime,
		Events:       make([]IncidentEvent, 0),
	}
}

func updateIncidentFromAlert(incident *Incident, alert *alerts.Alert) {
	if incident == nil || alert == nil {
		return
	}
	incident.AlertType = alert.Type
	incident.Level = string(alert.Level)
	incident.ResourceID = alert.ResourceID
	incident.ResourceName = alert.ResourceName
	incident.Node = alert.Node
	incident.Instance = alert.Instance
	incident.Message = alert.Message
	incident.Acknowledged = alert.Acknowledged
	incident.AckUser = alert.AckUser
	incident.AckTime = alert.AckTime
}

func (s *IncidentStore) ensureIncidentForAlertLocked(alert *alerts.Alert) *Incident {
	incident := s.findLatestIncidentByAlertIDLocked(alert.ID)
	if incident == nil {
		incident = newIncidentFromAlert(alert)
		s.incidents = append(s.incidents, incident)
	}
	updateIncidentFromAlert(incident, alert)
	return incident
}

func (s *IncidentStore) addEventLocked(incident *Incident, eventType IncidentEventType, summary string, details map[string]interface{}) {
	if incident == nil {
		return
	}
	if summary == "" {
		summary = string(eventType)
	}

	event := IncidentEvent{
		ID:        generateIncidentEventID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Summary:   summary,
		Details:   details,
	}
	incident.Events = append(incident.Events, event)
	if s.maxEvents > 0 && len(incident.Events) > s.maxEvents {
		incident.Events = incident.Events[len(incident.Events)-s.maxEvents:]
	}
}

func (s *IncidentStore) findOpenIncidentByAlertIDLocked(alertID string) *Incident {
	if alertID == "" {
		return nil
	}
	for i := len(s.incidents) - 1; i >= 0; i-- {
		incident := s.incidents[i]
		if incident != nil && incident.AlertID == alertID && incident.Status == IncidentStatusOpen {
			return incident
		}
	}
	return nil
}

func (s *IncidentStore) findLatestIncidentByAlertIDLocked(alertID string) *Incident {
	if alertID == "" {
		return nil
	}
	for i := len(s.incidents) - 1; i >= 0; i-- {
		incident := s.incidents[i]
		if incident != nil && incident.AlertID == alertID {
			return incident
		}
	}
	return nil
}

func (s *IncidentStore) findIncidentByIDLocked(incidentID string) *Incident {
	if incidentID == "" {
		return nil
	}
	for i := len(s.incidents) - 1; i >= 0; i-- {
		incident := s.incidents[i]
		if incident != nil && incident.ID == incidentID {
			return incident
		}
	}
	return nil
}

func (s *IncidentStore) trimLocked() {
	if s.maxAge > 0 {
		cutoff := time.Now().Add(-s.maxAge)
		filtered := make([]*Incident, 0, len(s.incidents))
		for _, incident := range s.incidents {
			if incident == nil {
				continue
			}
			compareTime := incident.OpenedAt
			if incident.ClosedAt != nil {
				compareTime = *incident.ClosedAt
			}
			if compareTime.After(cutoff) {
				filtered = append(filtered, incident)
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
	snapshot := make([]*Incident, 0, len(s.incidents))
	for _, incident := range s.incidents {
		snapshot = append(snapshot, cloneIncident(incident))
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

	var incidents []*Incident
	if err := json.Unmarshal(data, &incidents); err != nil {
		return err
	}

	s.incidents = incidents
	s.trimLocked()
	return nil
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
