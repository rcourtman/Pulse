package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

type failedIncidentHistoryStore struct{ unifiedresources.ResourceStore }

func (failedIncidentHistoryStore) GetRecentChanges(string, time.Time, int) ([]unifiedresources.ResourceChange, error) {
	return nil, errors.New("history store unavailable")
}

type incidentArchiveFixture struct{ window *IncidentWindow }

func (s incidentArchiveFixture) GetWindow(string) *IncidentWindow { return s.window }
func (s incidentArchiveFixture) GetWindowsForResource(string, int) []*IncidentWindow {
	panic("canonical incident reads must not enumerate legacy recordings")
}

func TestIncidentHistoryRetainsCanonicalEvidence(t *testing.T) {
	store, err := unifiedresources.NewSQLiteResourceStore(t.TempDir(), "incident-test")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	resourceID := "app-container-7020f37498275208"
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	occurred := start.Add(-time.Minute)
	changes := []unifiedresources.ResourceChange{
		{ID: "outside-window", ResourceID: resourceID, ObservedAt: start.Add(-time.Second), Kind: unifiedresources.ChangeRestart, SourceType: unifiedresources.SourcePlatformEvent},
		{ID: "alert-fired", ResourceID: resourceID, ObservedAt: start.Add(time.Minute), OccurredAt: &occurred, Kind: unifiedresources.ChangeAlertFired, SourceType: unifiedresources.SourcePulseDiff, From: "healthy", To: "unhealthy", Reason: "Health check failed", Metadata: map[string]any{"alert_id": "health-check", "value": 1.0}},
		{ID: "related-only", ResourceID: "agent-parent", RelatedResources: []string{resourceID}, ObservedAt: start.Add(2 * time.Minute), Kind: unifiedresources.ChangeRestart, SourceType: unifiedresources.SourcePlatformEvent},
		{ID: "alert-resolved", ResourceID: resourceID, ObservedAt: start.Add(3 * time.Minute), Kind: unifiedresources.ChangeAlertResolved, SourceType: unifiedresources.SourcePulseDiff, From: "unhealthy", To: "healthy"},
	}
	for _, change := range changes {
		require.NoError(t, store.RecordChange(change))
	}
	// No live inventory or incident recorder is necessary to read a retained
	// event for a container that has since been removed.
	exec := NewPulseToolExecutor(ExecutorConfig{ActionAuditStore: store, IncidentRecorderProvider: incidentArchiveFixture{}})
	require.True(t, exec.isToolAvailable(agentcapabilities.PulseKnowledgeToolName))
	for _, tc := range []struct {
		name      string
		id        string
		limit     int
		wantCount int
		wantMore  bool
	}{
		{"retained lifecycle", resourceID, 50, 2, false},
		{"bounded lifecycle", resourceID, 1, 1, true},
		{"empty history", "app-container-absent", 50, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := map[string]interface{}{"action": "incidents", "resource_id": tc.id, "since": start.Format(time.RFC3339), "limit": float64(tc.limit)}
			result, err := exec.registry.Execute(context.Background(), exec, agentcapabilities.PulseKnowledgeToolName, input)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Content)
			var got struct {
				Source    string                            `json:"source"`
				Events    []unifiedresources.ResourceChange `json:"events"`
				HasMore   bool                              `json:"has_more"`
				Coverage  string                            `json:"coverage"`
				TimeBasis string                            `json:"time_basis"`
			}
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &got))
			require.Equal(t, "canonical_resource_timeline", got.Source)
			require.Equal(t, "retained_records_only", got.Coverage)
			require.Equal(t, "observed_at", got.TimeBasis)
			require.NotNil(t, got.Events)
			require.Len(t, got.Events, tc.wantCount)
			require.Equal(t, tc.wantMore, got.HasMore)
			if tc.wantCount > 0 {
				require.Equal(t, "alert-resolved", got.Events[0].ID)
				require.Nil(t, got.Events[0].OccurredAt)
			}
			if tc.wantCount == 2 {
				require.Equal(t, changes[1], got.Events[1])
			}
			capture, err := json.Marshal(map[string]any{"case": tc.name, "input": input, "result": result})
			require.NoError(t, err)
			t.Logf("INCIDENT_EVIDENCE %s", capture)
		})
	}
}

func TestIncidentHistoryUnavailableAndInvalid(t *testing.T) {
	for _, tc := range []struct {
		name  string
		store unifiedresources.ResourceStore
		input map[string]interface{}
	}{
		{"unavailable", nil, map[string]interface{}{"resource_id": "app-container-1"}},
		{"failed", failedIncidentHistoryStore{}, map[string]interface{}{"resource_id": "app-container-1"}},
		{"empty resource", unifiedresources.NewMemoryStore(), map[string]interface{}{"resource_id": "  "}},
		{"invalid time", unifiedresources.NewMemoryStore(), map[string]interface{}{"resource_id": "app-container-1", "since": "yesterday"}},
		{"future time", unifiedresources.NewMemoryStore(), map[string]interface{}{"resource_id": "app-container-1", "since": time.Now().Add(time.Hour).Format(time.RFC3339)}},
		{"negative limit", unifiedresources.NewMemoryStore(), map[string]interface{}{"resource_id": "app-container-1", "limit": float64(-1)}},
		{"large limit", unifiedresources.NewMemoryStore(), map[string]interface{}{"resource_id": "app-container-1", "limit": float64(201)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := NewPulseToolExecutor(ExecutorConfig{ActionAuditStore: tc.store, IncidentRecorderProvider: incidentArchiveFixture{}})
			tc.input["action"] = "incidents"
			result, err := exec.registry.Execute(context.Background(), exec, agentcapabilities.PulseKnowledgeToolName, tc.input)
			require.NoError(t, err)
			require.True(t, result.IsError, result.Content)
			capture, err := json.Marshal(map[string]any{"case": tc.name, "input": tc.input, "result": result})
			require.NoError(t, err)
			t.Logf("INCIDENT_EVIDENCE %s", capture)
		})
	}
}

func TestIncidentHistoryLegacyArchiveIsResourceBound(t *testing.T) {
	window := &IncidentWindow{ID: "archive-1", ResourceID: "vm-1"}
	exec := NewPulseToolExecutor(ExecutorConfig{IncidentRecorderProvider: incidentArchiveFixture{window}})
	for _, id := range []string{"vm-1", "vm-2"} {
		result, err := exec.executeGetIncidentWindow(context.Background(), map[string]interface{}{"resource_id": id, "window_id": window.ID})
		require.NoError(t, err)
		if id == window.ResourceID {
			require.False(t, result.IsError)
			require.Contains(t, result.Content[0].Text, "legacy_incident_recording")
			require.Contains(t, result.Content[0].Text, "cached observations")
		} else {
			require.True(t, result.IsError)
			require.NotContains(t, result.Content[0].Text, "vm-1")
		}
	}
}
