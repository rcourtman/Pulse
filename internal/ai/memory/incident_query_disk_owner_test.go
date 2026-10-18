package memory

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

// PVE disk alert lifecycle rows are written under the physical disk their
// recorded hardware identity names, recording the alert's own reference.
// Reads by the alert identifier and by the reference (the Alerts history
// Resource action) must project exactly what they projected when every row
// sat under the reference, while a read by a disk returns its own
// occurrences, with their saved shells.
func TestIncidentQueryKeepsOccurrencesOfHardwareOwnedDiskAlerts(t *testing.T) {
	// The occurrence two disks own may have a saved shell: one opened at the
	// alert's start, as lifecycle shells are, or one a little off it, as an
	// imported shell can be, which reads match within a tolerance.
	noShell := time.Duration(-1)
	for _, tc := range []struct {
		backend     string
		shellOffset time.Duration
	}{{"memory", noShell}, {"sqlite", noShell}, {"memory", 0}, {"sqlite", 0}, {"memory", 500 * time.Millisecond}, {"sqlite", 500 * time.Millisecond}} {
		backend, withSwappedShell, shellOffset := tc.backend, tc.shellOffset != noShell, tc.shellOffset
		shell := "none"
		if withSwappedShell {
			shell = shellOffset.String()
		}
		t.Run(fmt.Sprintf("%s/swapped-shell=%s", backend, shell), func(t *testing.T) {
			ref := unifiedresources.ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/dev/sda")
			identifier := unifiedresources.ProxmoxPhysicalDiskAlertIdentifiers(ref)[0]
			base := time.Now().UTC().Add(-6 * time.Hour).Truncate(time.Second)
			legacy, first, swapped, latest := base, base.Add(time.Hour), base.Add(2*time.Hour), base.Add(3*time.Hour)
			type row struct {
				id      string
				owner   string
				kind    unifiedresources.ChangeKind
				started time.Time
				at      time.Time
			}
			rows := []row{
				// Journaled under the reference before ownership existed.
				{"legacy-fired", "", unifiedresources.ChangeAlertFired, legacy, legacy},
				{"legacy-resolved", "", unifiedresources.ChangeAlertResolved, legacy, legacy.Add(time.Minute)},
				{"first-fired", "physical_disk-a", unifiedresources.ChangeAlertFired, first, first},
				{"first-resolved", "physical_disk-a", unifiedresources.ChangeAlertResolved, first, first.Add(time.Minute)},
				// One occurrence spans two disks: B took the path while failing.
				{"swapped-fired", "physical_disk-a", unifiedresources.ChangeAlertFired, swapped, swapped},
				{"swapped-resolved", "physical_disk-b", unifiedresources.ChangeAlertResolved, swapped, swapped.Add(5 * time.Minute)},
				{"latest-fired", "physical_disk-b", unifiedresources.ChangeAlertFired, latest, latest},
				{"latest-ack", "physical_disk-b", unifiedresources.ChangeAlertAcknowledged, latest, latest.Add(time.Minute)},
			}
			newStore := func(owned bool) *IncidentStore {
				var canonical unifiedresources.ResourceStore = unifiedresources.NewMemoryStore()
				if backend == "sqlite" {
					sqlite, err := unifiedresources.NewSQLiteResourceStore(t.TempDir(), "default")
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, sqlite.Close()) })
					canonical = sqlite
				}
				for _, r := range rows {
					change := unifiedresources.BuildAlertTimelineChange(ref, r.kind, r.at, "", unifiedresources.AlertTimelineChange{
						AlertIdentifier: identifier, AlertStartedAt: r.started, AlertType: "disk-health", AlertLevel: "critical",
					})
					change.ID = r.id
					if owned && r.owner != "" {
						change.ResourceID = r.owner
						change.Metadata[unifiedresources.MetadataAlertResourceID] = ref
					}
					require.NoError(t, canonical.RecordChange(*change))
				}
				// A command run on disk B for the latest occurrence, without a start.
				command := unifiedresources.BuildCommandExecutionChange("physical_disk-b", identifier, "operator", "smartctl -a /dev/sda", true, "", nil)
				command.ID = "latest-command"
				command.ObservedAt = latest.Add(2 * time.Minute)
				require.NoError(t, canonical.RecordChange(*command))
				store := NewIncidentStore(IncidentStoreConfig{})
				store.SetResourceTimelineStore(canonical)
				store.incidents = []*incidentShell{{
					ID: "saved-first", AlertIdentifier: identifier, ResourceID: ref, OpenedAt: first,
					Events: []IncidentEvent{{ID: "note", Type: IncidentEventNote, Timestamp: first.Add(30 * time.Second), Details: map[string]any{"note": "reseated the cable"}}},
				}}
				if withSwappedShell {
					store.incidents = append(store.incidents, &incidentShell{ID: "saved-swapped", AlertIdentifier: identifier, ResourceID: ref, OpenedAt: swapped.Add(shellOffset),
						Events: []IncidentEvent{{ID: "swapped-note", Type: IncidentEventNote, Timestamp: swapped.Add(time.Minute), Details: map[string]any{"note": "disk B went in failing"}}}})
				}
				return store
			}
			journaled, owned := newStore(false), newStore(true)

			type occurrence struct {
				ID, ResourceID string
				Status         IncidentStatus
				OpenedAt       time.Time
				Closed         bool
				Acknowledged   bool
				Events         int
			}
			project := func(store *IncidentStore, query IncidentQuery) []occurrence {
				page, err := store.QueryIncidents(query)
				require.NoError(t, err)
				out := make([]occurrence, 0, len(page.Incidents))
				for _, incident := range page.Incidents {
					out = append(out, occurrence{incident.ID, incident.ResourceID, incident.Status, incident.OpenedAt, incident.ClosedAt != nil, incident.Acknowledged, len(incident.Events)})
				}
				return out
			}
			for _, query := range []IncidentQuery{
				{AlertIdentifier: identifier, Limit: 10},
				{AlertIdentifier: identifier, StartedAt: swapped, Limit: 1},
				{ResourceID: ref, Limit: 10},
			} {
				want := project(journaled, query)
				require.Len(t, want, 1+3*boolInt(query.StartedAt.IsZero()))
				require.Equal(t, want, project(owned, query), "%+v", query)
			}

			starts := func(store *IncidentStore, resourceID string) map[time.Time]occurrence {
				out := make(map[time.Time]occurrence)
				for _, incident := range project(store, IncidentQuery{ResourceID: resourceID, Limit: 10}) {
					out[incident.OpenedAt] = incident
				}
				return out
			}
			diskA := starts(owned, "physical_disk-a")
			require.Len(t, diskA, 2)
			require.Equal(t, "saved-first", diskA[first].ID, "the disk reads the saved occurrence, not a second projection")
			require.Equal(t, IncidentStatusResolved, diskA[first].Status)
			require.Equal(t, 3, diskA[first].Events, "fired, note, resolved")
			require.Equal(t, ref, diskA[first].ResourceID)
			diskAIDs := map[string]bool{}
			for _, incident := range diskA {
				diskAIDs[incident.ID] = true
			}
			diskB := starts(owned, "physical_disk-b")
			require.Contains(t, diskB, latest)
			require.True(t, diskB[latest].Acknowledged)
			require.Equal(t, 3, diskB[latest].Events, "fired, acknowledged, and the command run after it")
			require.NotContains(t, diskB, first)
			require.NotContains(t, diskB, legacy)
			require.NotContains(t, diskA, legacy)

			// B owns only the resolution of the occurrence it took over. Its read
			// names that occurrence as the alert-centric read does, so a note
			// recorded from the disk reaches the same occurrence.
			swappedOccurrence := project(owned, IncidentQuery{AlertIdentifier: identifier, StartedAt: swapped, Limit: 1})[0]
			if withSwappedShell {
				require.Equal(t, "saved-swapped", swappedOccurrence.ID)
			}
			diskBSwapped := project(owned, IncidentQuery{ResourceID: "physical_disk-b", Limit: 10})
			require.Len(t, diskBSwapped, 2, "the latest occurrence and the swapped one, never a second copy of it")
			require.Equal(t, swappedOccurrence.ID, diskBSwapped[1].ID)
			require.Equal(t, 1+boolInt(withSwappedShell), diskBSwapped[1].Events, "B's resolution, with the saved note when there is a shell")
			require.True(t, diskAIDs[swappedOccurrence.ID], "disk A reads the swapped occurrence under the same ID")
			require.True(t, owned.RecordNote(identifier, diskBSwapped[1].ID, "replaced the failing disk", "operator"))
			noted, err := owned.QueryIncidents(IncidentQuery{AlertIdentifier: identifier, StartedAt: swapped, Limit: 1})
			require.NoError(t, err)
			require.Len(t, noted.Incidents, 1)
			owners := map[string]string{}
			for _, event := range noted.Incidents[0].Events {
				if event.Evidence != nil {
					owners[event.ID] = event.Evidence.ResourceID
				}
			}
			require.Equal(t, map[string]string{"swapped-fired": "physical_disk-a", "swapped-resolved": "physical_disk-b"}, owners,
				"evidence keeps the disk that owns each row")
			require.Len(t, noted.Incidents[0].Events, 3+boolInt(withSwappedShell), "fired, resolved, the new note, and the saved note")
		})
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// A truncated read keeps the guard that stops events without a start from
// joining an occurrence a later saved shell may own. An owned row naming an
// older occurrence must not give such an event a destination it lacked when
// the rows sat under the reference.
func TestIncidentQueryTruncatedReadKeepsOwnedOccurrencesGuarded(t *testing.T) {
	ref := unifiedresources.ProxmoxPhysicalDiskAlertResourceID("lab", "pve1", "/dev/sda")
	identifier := unifiedresources.ProxmoxPhysicalDiskAlertIdentifiers(ref)[0]
	older := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Second)
	later := older.Add(time.Hour)
	project := func(owned bool) []string {
		canonical := unifiedresources.NewMemoryStore()
		record := func(id, owner string, kind unifiedresources.ChangeKind, started, at time.Time) {
			change := unifiedresources.BuildAlertTimelineChange(ref, kind, at, "", unifiedresources.AlertTimelineChange{AlertIdentifier: identifier, AlertStartedAt: started})
			change.ID = id
			if owned {
				change.ResourceID = owner
				change.Metadata[unifiedresources.MetadataAlertResourceID] = ref
			}
			require.NoError(t, canonical.RecordChange(*change))
		}
		record("older-fired", "physical_disk-a", unifiedresources.ChangeAlertFired, older, older)
		record("later-fired", "physical_disk-a", unifiedresources.ChangeAlertFired, later, later)
		command := unifiedresources.BuildCommandExecutionChange("node-pve1", identifier, "operator", "smartctl -a /dev/sda", true, "", nil)
		command.ID = "command"
		command.ObservedAt = later.Add(time.Minute)
		require.NoError(t, canonical.RecordChange(*command))
		record("older-ack", "physical_disk-b", unifiedresources.ChangeAlertAcknowledged, older, later.Add(2*time.Minute))
		store := NewIncidentStore(IncidentStoreConfig{})
		store.SetResourceTimelineStore(canonical)
		store.incidents = []*incidentShell{
			{ID: "saved-older", AlertIdentifier: identifier, ResourceID: ref, OpenedAt: older},
			{ID: "saved-later", AlertIdentifier: identifier, ResourceID: ref, OpenedAt: later},
		}
		page, err := store.QueryIncidents(IncidentQuery{AlertIdentifier: identifier, ChangeLimit: 2, Limit: 10})
		require.NoError(t, err)
		require.True(t, page.History.HasMoreChanges)
		var out []string
		for _, incident := range page.Incidents {
			events := make([]string, 0, len(incident.Events))
			for _, event := range incident.Events {
				events = append(events, event.ID)
			}
			out = append(out, fmt.Sprintf("%s %v", incident.ID, events))
		}
		return out
	}
	journaled := project(false)
	require.Equal(t, journaled, project(true))
	for _, incident := range journaled {
		if strings.HasPrefix(incident, "saved-older") {
			require.NotContains(t, incident, "command")
		}
	}
}
