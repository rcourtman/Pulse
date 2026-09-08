package alerting

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func TestIncidentQueryHTTPPreservesOccurrenceAndEvidence(t *testing.T) {
	canonical, err := unifiedresources.NewSQLiteResourceStore(t.TempDir(), "default")
	require.NoError(t, err)
	defer canonical.Close()
	incidents := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	incidents.SetResourceTimelineStore(canonical)
	monitor := new(MockAlertMonitor)
	monitor.On("GetIncidentStore").Return(incidents)
	handler := NewAlertHandlers(nil, monitor, nil)
	first := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	for index, kind := range []unifiedresources.ChangeKind{unifiedresources.ChangeAlertFired, unifiedresources.ChangeAlertResolved, unifiedresources.ChangeAlertFired} {
		at := first.Add(time.Duration(index) * time.Minute)
		change := unifiedresources.BuildAlertTimelineChange("resource-a", kind, at, "operator", unifiedresources.AlertTimelineChange{AlertIdentifier: "repeated", AlertType: "disk", AlertLevel: "warning"})
		require.NoError(t, canonical.RecordChange(*change))
	}
	request := httptest.NewRequest(http.MethodGet, "/api/alerts/incidents?alertIdentifier=repeated&started_at="+url.QueryEscape(first.Format(time.RFC3339)), nil)
	response := httptest.NewRecorder()
	handler.GetAlertIncidentTimeline(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var timeline memory.Incident
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &timeline))
	require.Equal(t, memory.IncidentStatusResolved, timeline.Status)
	require.Len(t, timeline.Events, 2)
	require.NotNil(t, timeline.History)
	require.NotNil(t, timeline.Events[0].Evidence)
	require.Equal(t, "operator", timeline.Events[0].Evidence.Actor)
	require.Equal(t, "canonical_resource_history", timeline.Events[0].Source)
	// A failed read must not enter snapshot-repair fallback or return null/200.
	require.NoError(t, canonical.Close())
	response = httptest.NewRecorder()
	handler.GetAlertIncidentTimeline(response, request)
	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	monitor.AssertNotCalled(t, "GetAlertManager")
}
