package memory

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// IncidentQuery selects historical evidence, not current resource health.
// Limits bound returned evidence rather than deciding diagnostic quality.
type IncidentQuery struct {
	IncidentID      string
	ResourceID      string
	AlertIdentifier string
	StartedAt       time.Time
	Limit           int
	ChangeLimit     int
}

type IncidentHistoryCoverage struct {
	Source           string    `json:"source"`
	ObservedSince    time.Time `json:"observedSince"`
	ObservedBefore   time.Time `json:"observedBefore"`
	ChangeLimit      int       `json:"changeLimit"`
	HasMoreChanges   bool      `json:"hasMoreChanges"`
	HasMoreIncidents bool      `json:"hasMoreIncidents"`
}

type IncidentPage struct {
	Incidents []*Incident             `json:"incidents"`
	History   IncidentHistoryCoverage `json:"history"`
}

// QueryIncidents reads canonical evidence before selecting incident rows. Shells
// contribute occurrence identity and attributed local notes, never a second
// authoritative lifecycle. A failed canonical read is not an empty history.
func (s *IncidentStore) QueryIncidents(query IncidentQuery) (IncidentPage, error) {
	query.ResourceID = strings.TrimSpace(query.ResourceID)
	query.AlertIdentifier = strings.TrimSpace(query.AlertIdentifier)
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		return IncidentPage{}, fmt.Errorf("incident limit exceeds 100")
	}
	if query.ChangeLimit <= 0 {
		query.ChangeLimit = 4096
	}
	if query.ChangeLimit > 8192 {
		return IncidentPage{}, fmt.Errorf("incident change limit exceeds 8192")
	}
	s.mu.RLock()
	store, maxAge := s.resourceTimelineStore, s.maxAge
	shells := make([]*incidentShell, 0, len(s.incidents))
	for _, shell := range s.incidents {
		if shell != nil {
			shells = append(shells, cloneIncidentShell(shell))
		}
	}
	s.mu.RUnlock()
	before := time.Now().UTC()
	page := IncidentPage{Incidents: []*Incident{}, History: IncidentHistoryCoverage{Source: "legacy_incident_memory", ChangeLimit: query.ChangeLimit}}
	var changes []unifiedresources.ResourceChange
	resourceIDs := map[string]bool{query.ResourceID: true}
	if store != nil {
		page.History.Source = "canonical_resource_history"
		page.History.ObservedSince = before.Add(-maxAge)
		page.History.ObservedBefore = before
		filters := unifiedresources.ResourceChangeFilters{ObservedBefore: &before, Kinds: []unifiedresources.ChangeKind{
			unifiedresources.ChangeAlertFired, unifiedresources.ChangeAlertResolved,
			unifiedresources.ChangeAlertAcknowledged, unifiedresources.ChangeAlertUnacknowledged,
			unifiedresources.ChangeAlertSnoozed, unifiedresources.ChangeAlertUnsnoozed,
			unifiedresources.ChangeCommandExecuted, unifiedresources.ChangeRunbookExecuted,
		}}
		if query.AlertIdentifier != "" {
			filters.AlertIdentifiers = []string{query.AlertIdentifier}
		}
		var err error
		changes, err = store.GetRecentChangesFiltered(query.ResourceID, page.History.ObservedSince, query.ChangeLimit+1, filters)
		if err != nil {
			return IncidentPage{}, fmt.Errorf("read canonical incident history: %w", err)
		}
		if len(changes) > query.ChangeLimit {
			page.History.HasMoreChanges = true
			changes = changes[:query.ChangeLimit]
		}
		if query.ResourceID != "" {
			ids, err := store.ResourceHistoryIDs(query.ResourceID)
			if err != nil {
				return IncidentPage{}, fmt.Errorf("read incident history identities: %w", err)
			}
			for _, id := range ids {
				resourceIDs[id] = true
			}
		}
	}
	// Resource aliases are read identities. A shell for another resource must
	// not become an occurrence boundary merely because its alert ID matches.
	resourceKeys := make(map[string]string)
	identityIDs := make([]string, 0, len(shells)+len(changes))
	for _, shell := range shells {
		identityIDs = append(identityIDs, shell.ResourceID)
	}
	for _, change := range changes {
		identityIDs = append(identityIDs, change.ResourceID)
	}
	for _, id := range identityIDs {
		if _, known := resourceKeys[id]; known {
			continue
		}
		ids := []string{id}
		if store != nil && id != "" {
			aliases, err := store.ResourceHistoryIDs(id)
			if err != nil {
				return IncidentPage{}, fmt.Errorf("read incident occurrence identities: %w", err)
			}
			ids = append(ids, aliases...)
		}
		sort.Strings(ids)
		for _, alias := range ids {
			resourceKeys[alias] = ids[0]
		}
	}
	sameResource := func(a, b string) bool { return resourceKeys[a] == resourceKeys[b] }
	byAlert := make(map[string][]*Incident)
	byOccurrence := make(map[string]*Incident)
	occurrenceIDs := make(map[*Incident]map[string]bool)
	// Historical imports can leave several shells for the same explicit firing.
	// Keep a stable existing ID and all local notes, not competing lifecycles.
	sort.Slice(shells, func(i, j int) bool { return shells[i].ID < shells[j].ID })
	for _, shell := range shells {
		if query.ResourceID != "" && !resourceIDs[shell.ResourceID] {
			continue
		}
		if query.AlertIdentifier != "" && shell.AlertIdentifier != query.AlertIdentifier {
			continue
		}
		incident := incidentFromShell(shell)
		for i := range incident.Events {
			if incident.Events[i].Source == "" {
				incident.Events[i].Source = "legacy_incident_memory"
			}
		}
		key := incident.AlertIdentifier + "\x00" + resourceKeys[incident.ResourceID] + "\x00" + incident.OpenedAt.UTC().Format(time.RFC3339Nano)
		if incident.OpenedAt.IsZero() {
			key += "\x00" + incident.ID
		}
		if existing := byOccurrence[key]; existing != nil {
			occurrenceIDs[existing][incident.ID] = true
			for _, fields := range [][2]*string{{&existing.ResourceID, &incident.ResourceID}, {&existing.ResourceName, &incident.ResourceName}, {&existing.ResourceType, &incident.ResourceType}, {&existing.AlertType, &incident.AlertType}, {&existing.Level, &incident.Level}, {&existing.Message, &incident.Message}, {&existing.Node, &incident.Node}, {&existing.Instance, &incident.Instance}} {
				if *fields[0] == "" {
					*fields[0] = *fields[1]
				}
			}
			seen := make(map[string]bool, len(existing.Events))
			for _, event := range existing.Events {
				seen[event.ID] = true
			}
			for _, event := range incident.Events {
				if !seen[event.ID] {
					existing.Events = append(existing.Events, event)
					seen[event.ID] = true
				}
			}
			if existing.ClosedAt == nil && incident.ClosedAt != nil {
				existing.ClosedAt = incident.ClosedAt
				existing.Status = incident.Status
			}
			continue
		}
		byOccurrence[key] = incident
		occurrenceIDs[incident] = map[string]bool{incident.ID: true}
		byAlert[incident.AlertIdentifier] = append(byAlert[incident.AlertIdentifier], incident)
	}
	// Explicit fired events establish occurrence boundaries even if no shell was
	// saved. Sort by occurrence time, retaining the distinct observation time.
	sort.Slice(changes, func(i, j int) bool {
		a, b := incidentEventTimestamp(changes[i]), incidentEventTimestamp(changes[j])
		if a.Equal(b) {
			if changes[i].Kind == unifiedresources.ChangeAlertFired && changes[j].Kind != unifiedresources.ChangeAlertFired {
				return true
			}
			if changes[j].Kind == unifiedresources.ChangeAlertFired && changes[i].Kind != unifiedresources.ChangeAlertFired {
				return false
			}
			return changes[i].ID < changes[j].ID
		}
		return a.Before(b)
	})
	boundaries := make(map[*Incident]time.Time)
	firedOwners := make(map[string]*Incident)
	canonicalBoundary := make(map[*Incident]bool)
	for _, occurrences := range byAlert {
		for _, occurrence := range occurrences {
			boundaries[occurrence] = occurrence.OpenedAt
		}
	}
	for _, change := range changes {
		identifier := projectedAlertIdentifier(change)
		if identifier == "" || change.Kind != unifiedresources.ChangeAlertFired {
			continue
		}
		started := incidentEventTimestamp(change)
		var occurrence *Incident
		for _, candidate := range byAlert[identifier] {
			if canonicalBoundary[candidate] {
				if sameResource(candidate.ResourceID, change.ResourceID) && boundaries[candidate].Equal(started) {
					occurrence = candidate
					break
				}
				continue
			}
			if incidentStartsMatch(candidate.OpenedAt, started) && (occurrence == nil || incidentStartDelta(candidate.OpenedAt, started) < incidentStartDelta(occurrence.OpenedAt, started)) {
				occurrence = candidate
			}
		}
		if occurrence == nil {
			occurrence = &Incident{ID: canonicalIncidentID(identifier, started), AlertIdentifier: identifier, OpenedAt: started, Status: IncidentStatusUnknown}
			byAlert[identifier] = append(byAlert[identifier], occurrence)
		}
		hydrateIncidentFromCanonicalChange(occurrence, change)
		boundaries[occurrence] = started
		firedOwners[change.ID] = occurrence
		canonicalBoundary[occurrence] = true
	}
	for _, occurrences := range byAlert {
		sort.Slice(occurrences, func(i, j int) bool { return boundaries[occurrences[i]].Before(boundaries[occurrences[j]]) })
	}
	canonicalEvents := make(map[*Incident][]IncidentEvent)
	for _, change := range changes {
		identifier := projectedAlertIdentifier(change)
		if identifier == "" {
			continue
		}
		event, ok := incidentEventFromResourceChange(change)
		if !ok {
			continue
		}
		occurrence := firedOwners[change.ID]
		if occurrence == nil {
			for _, candidate := range byAlert[identifier] {
				if change.Kind != unifiedresources.ChangeCommandExecuted && change.Kind != unifiedresources.ChangeRunbookExecuted && !sameResource(candidate.ResourceID, change.ResourceID) {
					continue
				}
				// A capped read may have omitted a later firing after a saved shell.
				// Never attach those ambiguous events to the earlier occurrence.
				if page.History.HasMoreChanges && !canonicalBoundary[candidate] && !candidate.OpenedAt.IsZero() {
					continue
				}
				if boundaries[candidate].After(event.Timestamp) {
					break
				}
				occurrence = candidate
			}
		}
		if occurrence == nil {
			// An event without a retained start cannot establish when the alert
			// opened. Preserve it as incomplete evidence with an unknown lifecycle.
			for _, candidate := range byAlert[identifier] {
				if candidate.OpenedAt.IsZero() && sameResource(candidate.ResourceID, change.ResourceID) {
					occurrence = candidate
					break
				}
			}
			if occurrence == nil {
				occurrence = &Incident{ID: "projected-partial-" + change.ID, AlertIdentifier: identifier, Status: IncidentStatusUnknown}
				byAlert[identifier] = append([]*Incident{occurrence}, byAlert[identifier]...)
			}
		}
		openedAt := occurrence.OpenedAt
		hydrateIncidentFromCanonicalChange(occurrence, change)
		occurrence.OpenedAt = openedAt
		canonicalEvents[occurrence] = append(canonicalEvents[occurrence], event)
	}
	for _, occurrences := range byAlert {
		for _, occurrence := range occurrences {
			events := canonicalEvents[occurrence]
			if len(events) > 0 {
				kept := make([]IncidentEvent, 0, len(occurrence.Events)+len(events))
				for _, event := range occurrence.Events {
					if isCanonicalProjectedIncidentEventType(event.Type) && (!isSnapshotProjectionEvent(event) || hasIncidentEventType(events, event.Type)) {
						continue
					}
					kept = append(kept, event)
				}
				occurrence.Events = append(kept, events...)
				sortIncidentEvents(occurrence.Events)
				openedAt := occurrence.OpenedAt
				resetDerivedIncidentState(occurrence)
				occurrence.Status = IncidentStatusUnknown
				applyProjectedIncidentState(occurrence, occurrence.Events)
				occurrence.OpenedAt = openedAt
			}
			if query.IncidentID != "" && occurrence.ID != query.IncidentID && !occurrenceIDs[occurrence][query.IncidentID] {
				continue
			}
			if !query.StartedAt.IsZero() && !incidentStartsMatch(occurrence.OpenedAt, query.StartedAt) {
				continue
			}
			coverage := page.History
			if len(events) == 0 {
				coverage.Source = "legacy_incident_memory"
			}
			occurrence.History = &coverage
			page.Incidents = append(page.Incidents, occurrence)
		}
	}
	if !query.StartedAt.IsZero() && len(page.Incidents) > 1 {
		closest := page.Incidents[0]

		for _, incident := range page.Incidents[1:] {
			if incidentStartDelta(incident.OpenedAt, query.StartedAt) < incidentStartDelta(closest.OpenedAt, query.StartedAt) {
				closest = incident
			}
		}
		page.Incidents = []*Incident{closest}
	}
	sort.Slice(page.Incidents, func(i, j int) bool {
		if page.Incidents[i].OpenedAt.Equal(page.Incidents[j].OpenedAt) {
			return page.Incidents[i].ID > page.Incidents[j].ID
		}
		return page.Incidents[i].OpenedAt.After(page.Incidents[j].OpenedAt)
	})
	if len(page.Incidents) > query.Limit {
		page.History.HasMoreIncidents = true
		page.Incidents = page.Incidents[:query.Limit]
	}
	for _, incident := range page.Incidents {
		incident.History.HasMoreIncidents = page.History.HasMoreIncidents
	}
	return page, nil
}

func incidentStartsMatch(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	delta := a.Sub(b)
	return delta >= -incidentStartMatchTolerance && delta <= incidentStartMatchTolerance
}

func formatIncidentCoverage(page IncidentPage) string {
	if len(page.Incidents) == 0 && page.History.Source == "legacy_incident_memory" {
		return ""
	}
	if page.History.Source == "legacy_incident_memory" {
		return "Saved incident memory only. Canonical history coverage was not read. This does not establish current health.\n"
	}
	text := fmt.Sprintf("Historical incident evidence: %s, observations [%s, %s). Retained history only. This does not establish current health.\n", page.History.Source, page.History.ObservedSince.Format(time.RFC3339), page.History.ObservedBefore.Format(time.RFC3339))
	if page.History.HasMoreChanges || page.History.HasMoreIncidents {
		text += "The query is truncated. Earlier occurrences or events may be absent.\n"
	}
	if len(page.Incidents) == 0 {
		text += "No incident evidence was returned in this window.\n"
	}
	return text
}

func formatIncidentTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Format(time.RFC3339)
}
func formatIncidentOccurredTime(value *time.Time) string {
	if value == nil {
		return "unknown"
	}
	return formatIncidentTime(*value)
}

// The alert identifier and explicit firing time name the occurrence. Event
// arrival order, duplicated observations and resource aliases do not rename it.
func canonicalIncidentID(alertIdentifier string, startedAt time.Time) string {
	return fmt.Sprintf("projected-%x", sha256.Sum256([]byte(alertIdentifier+"\x00"+startedAt.UTC().Format(time.RFC3339Nano))))
}

func incidentStartDelta(a, b time.Time) time.Duration {
	d := a.Sub(b)
	if d < 0 {
		return -d
	}
	return d
}
