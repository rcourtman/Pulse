package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type stubForecastProvider struct {
	points []forecast.MetricDataPoint
	err    error
}

func (s stubForecastProvider) GetMetricHistory(_ string, _ string, _, _ time.Time) ([]forecast.MetricDataPoint, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.points, nil
}

type stubForecastResourceIterator struct {
	vms      []forecast.ResourceInfo
	cts      []forecast.ResourceInfo
	nodes    []forecast.ResourceInfo
	storages []forecast.ResourceInfo
}

func (s stubForecastResourceIterator) ForecastVMs() []forecast.ResourceInfo        { return s.vms }
func (s stubForecastResourceIterator) ForecastContainers() []forecast.ResourceInfo { return s.cts }
func (s stubForecastResourceIterator) ForecastNodes() []forecast.ResourceInfo      { return s.nodes }
func (s stubForecastResourceIterator) ForecastStoragePools() []forecast.ResourceInfo {
	return s.storages
}

func makeForecastPoints(count int, start time.Time, startValue, step float64) []forecast.MetricDataPoint {
	points := make([]forecast.MetricDataPoint, 0, count)
	for i := 0; i < count; i++ {
		points = append(points, forecast.MetricDataPoint{
			Timestamp: start.Add(time.Duration(i) * time.Hour),
			Value:     startValue + float64(i)*step,
		})
	}
	return points
}

func addBaseline(t *testing.T, store *ai.BaselineStore, resourceID, resourceType, metric string, value float64) {
	t.Helper()
	points := []ai.BaselineMetricPoint{{Value: value, Timestamp: time.Now()}}
	if err := store.Learn(resourceID, resourceType, metric, points); err != nil {
		t.Fatalf("baseline Learn error: %v", err)
	}
}

func TestHandleGetAnomalies_NoStateProvider(t *testing.T) {
	svc := newEnabledAIService(t)
	store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})
	svc.SetBaselineStore(store)

	handler := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/anomalies", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetAnomalies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "ReadState not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetAnomalies_MixedResources(t *testing.T) {
	svc := newEnabledAIService(t)
	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-1", Name: "vm-one", Status: "running", CPU: 0.9, Memory: models.Memory{Usage: 85}, Disk: models.Disk{Usage: 90}},
			{ID: "vm-template", Template: true, Status: "running", CPU: 0.9, Memory: models.Memory{Usage: 90}},
			{ID: "vm-stopped", Status: "stopped", CPU: 0.9, Memory: models.Memory{Usage: 90}},
		},
		Containers: []models.Container{
			{ID: "ct-1", Name: "ct-one", Status: "running", CPU: 0.9, Memory: models.Memory{Usage: 85}, Disk: models.Disk{Usage: 90}},
			{ID: "ct-template", Template: true, Status: "running", CPU: 0.9, Memory: models.Memory{Usage: 90}},
			{ID: "ct-stopped", Status: "stopped", CPU: 0.9, Memory: models.Memory{Usage: 90}},
		},
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-one", CPU: 0.9, Memory: models.Memory{Usage: 85}},
		},
	}
	svc.SetStateProvider(snapshotStateProvider{state: state})

	rs := newTestReadState(state)
	vmID := ""
	for _, v := range rs.VMs() {
		if v != nil && v.Name() == "vm-one" {
			vmID = v.ID()
			break
		}
	}
	ctID := ""
	for _, c := range rs.Containers() {
		if c != nil && c.Name() == "ct-one" {
			ctID = c.ID()
			break
		}
	}
	nodeID := ""
	for _, n := range rs.Nodes() {
		if n != nil && n.Name() == "node-one" {
			nodeID = n.ID()
			break
		}
	}
	if vmID == "" || ctID == "" || nodeID == "" {
		t.Fatalf("expected ReadState to contain test resources (vm=%q ct=%q node=%q)", vmID, ctID, nodeID)
	}

	store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})
	addBaseline(t, store, vmID, "vm", "cpu", 10)
	addBaseline(t, store, vmID, "vm", "memory", 10)
	addBaseline(t, store, vmID, "vm", "disk", 10)
	addBaseline(t, store, ctID, "container", "cpu", 10)
	addBaseline(t, store, ctID, "container", "memory", 10)
	addBaseline(t, store, ctID, "container", "disk", 10)
	addBaseline(t, store, nodeID, "node", "cpu", 10)
	addBaseline(t, store, nodeID, "node", "memory", 10)
	svc.SetBaselineStore(store)

	handler := &AISettingsHandler{defaultAIService: svc}
	handler.SetReadState(rs)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/anomalies", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetAnomalies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	anomalies, ok := payload["anomalies"].([]interface{})
	if !ok || len(anomalies) == 0 {
		t.Fatalf("expected anomalies, got %#v", payload["anomalies"])
	}

	types := map[string]bool{}
	for _, item := range anomalies {
		row, _ := item.(map[string]interface{})
		if rtype, ok := row["resource_type"].(string); ok {
			types[rtype] = true
		}
	}
	if !types["vm"] || !types["container"] || !types["node"] {
		t.Fatalf("expected vm, container, node anomalies, got %#v", types)
	}
}

func TestHandleGetLearningStatus_WaitingAndActive(t *testing.T) {
	t.Run("waiting", func(t *testing.T) {
		svc := newEnabledAIService(t)
		store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})
		svc.SetBaselineStore(store)

		handler := &AISettingsHandler{defaultAIService: svc}
		req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/learning", nil)
		rec := httptest.NewRecorder()

		handler.HandleGetLearningStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload["status"] != "waiting" {
			t.Fatalf("expected status waiting, got %#v", payload["status"])
		}
	})

	t.Run("active", func(t *testing.T) {
		svc := newEnabledAIService(t)
		store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})
		for i := 0; i < 5; i++ {
			id := fmt.Sprintf("res-%d", i)
			addBaseline(t, store, id, "vm", "cpu", 10)
		}
		svc.SetBaselineStore(store)

		handler := &AISettingsHandler{defaultAIService: svc}
		req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/learning", nil)
		rec := httptest.NewRecorder()

		handler.HandleGetLearningStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload["status"] != "active" {
			t.Fatalf("expected status active, got %#v", payload["status"])
		}
	})
}

func TestHandleGetLearningPreferences_Stats(t *testing.T) {
	store := learning.NewLearningStore(learning.LearningStoreConfig{})
	store.RecordFeedback(learning.FeedbackRecord{
		FindingID:  "finding-1",
		ResourceID: "res-1",
		Category:   "performance",
		Severity:   "warning",
		Action:     learning.ActionAcknowledge,
	})

	handler := &AISettingsHandler{}
	handler.SetLearningStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/learning/preferences", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetLearningPreferences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["statistics"]; !ok {
		t.Fatalf("expected statistics in response")
	}
}

func TestHandleGetUnifiedFindings_Statuses(t *testing.T) {
	store := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	now := time.Now()
	resolvedAt := now.Add(-2 * time.Hour)
	snoozedUntil := now.Add(2 * time.Hour)

	store.AddFromAI(&unified.UnifiedFinding{
		ID:           "finding-active",
		Source:       unified.SourceAIPatrol,
		Severity:     unified.SeverityWarning,
		Category:     unified.CategoryPerformance,
		ResourceID:   "res-1",
		ResourceName: "res-1",
		ResourceType: "vm",
		Title:        "Active",
		Description:  "active",
		DetectedAt:   now,
		LastSeenAt:   now,
	})
	store.AddFromAI(&unified.UnifiedFinding{
		ID:           "finding-resolved",
		Source:       unified.SourceThreshold,
		Severity:     unified.SeverityCritical,
		Category:     unified.CategoryCapacity,
		ResourceID:   "res-1",
		ResourceName: "res-1",
		ResourceType: "vm",
		Title:        "Resolved",
		Description:  "resolved",
		ResolvedAt:   &resolvedAt,
		DetectedAt:   now,
		LastSeenAt:   now,
	})
	store.AddFromAI(&unified.UnifiedFinding{
		ID:           "finding-snoozed",
		Source:       unified.SourceAIPatrol,
		Severity:     unified.SeverityWarning,
		Category:     unified.CategoryPerformance,
		ResourceID:   "res-1",
		ResourceName: "res-1",
		ResourceType: "vm",
		Title:        "Snoozed",
		Description:  "snoozed",
		SnoozedUntil: &snoozedUntil,
		DetectedAt:   now,
		LastSeenAt:   now,
	})
	store.AddFromAI(&unified.UnifiedFinding{
		ID:              "finding-dismissed",
		Source:          unified.SourceAIPatrol,
		Severity:        unified.SeverityInfo,
		Category:        unified.CategoryGeneral,
		ResourceID:      "res-1",
		ResourceName:    "res-1",
		ResourceType:    "vm",
		Title:           "Dismissed",
		Description:     "dismissed",
		DismissedReason: "noise",
		Suppressed:      true,
		DetectedAt:      now,
		LastSeenAt:      now,
	})

	handler := &AISettingsHandler{}
	handler.SetUnifiedStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/unified/findings?resource_id=res-1&include_resolved=true", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetUnifiedFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["count"] != float64(4) {
		t.Fatalf("expected count 4, got %#v", payload["count"])
	}
	if payload["active_count"] != float64(1) {
		t.Fatalf("expected active_count 1, got %#v", payload["active_count"])
	}
}

func TestHandleGetForecast_MissingParams(t *testing.T) {
	handler := &AISettingsHandler{forecastService: forecast.NewService(forecast.DefaultForecastConfig())}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/forecast?metric=cpu", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecast(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleGetForecast_Success(t *testing.T) {
	points := makeForecastPoints(60, time.Now().Add(-60*time.Hour), 50, 0.1)
	svc := forecast.NewService(forecast.DefaultForecastConfig())
	svc.SetDataProvider(stubForecastProvider{points: points})

	handler := &AISettingsHandler{forecastService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/forecast?resource_id=vm-1&resource_name=vm-one&metric=cpu&horizon_hours=2&threshold=60", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecast(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	forecastVal, ok := payload["forecast"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected forecast object")
	}
	if forecastVal["resource_id"] != "vm-1" {
		t.Fatalf("unexpected resource_id: %#v", forecastVal["resource_id"])
	}
	if forecastVal["metric"] != "cpu" {
		t.Fatalf("unexpected metric: %#v", forecastVal["metric"])
	}
}

func TestHandleGetForecastOverview_Success(t *testing.T) {
	points := makeForecastPoints(60, time.Now().Add(-60*time.Hour), 50, 0.1)
	svc := forecast.NewService(forecast.DefaultForecastConfig())
	svc.SetDataProvider(stubForecastProvider{points: points})
	svc.SetResourceIterator(stubForecastResourceIterator{
		vms:   []forecast.ResourceInfo{{ID: "vm-1", Name: "vm-one"}},
		cts:   []forecast.ResourceInfo{{ID: "ct-1", Name: "ct-one"}},
		nodes: []forecast.ResourceInfo{{ID: "node-1", Name: "node-one"}},
	})

	handler := &AISettingsHandler{forecastService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/forecasts/overview?metric=cpu&horizon_hours=24&threshold=60", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecastOverview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["metric"] != "cpu" {
		t.Fatalf("unexpected metric: %#v", payload["metric"])
	}
	if payload["threshold"] != float64(60) {
		t.Fatalf("unexpected threshold: %#v", payload["threshold"])
	}
	forecasts, ok := payload["forecasts"].([]interface{})
	if !ok || len(forecasts) == 0 {
		t.Fatalf("expected forecasts, got %#v", payload["forecasts"])
	}
}

func TestHandleGetProxmoxEvents_ResourceFilter(t *testing.T) {
	correlator := proxmox.NewEventCorrelator(proxmox.EventCorrelatorConfig{})
	correlator.RecordEvent(proxmox.ProxmoxEvent{
		ID:         "evt-1",
		Type:       proxmox.EventVMStart,
		ResourceID: "vm-1",
	})

	handler := &AISettingsHandler{proxmoxCorrelator: correlator}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/proxmox/events?resource_id=vm-1&limit=1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetProxmoxEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	events, ok := payload["events"].([]interface{})
	if !ok || len(events) != 1 {
		t.Fatalf("expected 1 event, got %#v", payload["events"])
	}
}

func TestHandleGetProxmoxCorrelations_ResourceFilter(t *testing.T) {
	correlator := proxmox.NewEventCorrelator(proxmox.EventCorrelatorConfig{})
	corr := proxmox.EventCorrelation{
		ID: "corr-1",
		Event: proxmox.ProxmoxEvent{
			ID:         "evt-1",
			Type:       proxmox.EventVMStart,
			ResourceID: "vm-1",
		},
		ImpactedResources: []string{"vm-1"},
		CreatedAt:         time.Now(),
	}
	setUnexportedField(t, correlator, "correlations", []proxmox.EventCorrelation{corr})

	handler := &AISettingsHandler{proxmoxCorrelator: correlator}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/proxmox/correlations?resource_id=vm-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetProxmoxCorrelations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	correlations, ok := payload["correlations"].([]interface{})
	if !ok || len(correlations) != 1 {
		t.Fatalf("expected 1 correlation, got %#v", payload["correlations"])
	}
}

func TestHandleGetIncidentData_Errors(t *testing.T) {
	t.Run("invalid_path", func(t *testing.T) {
		handler := &AISettingsHandler{}
		req := httptest.NewRequest(http.MethodGet, "/api/ai/incident/abc", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetIncidentData(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("missing_resource", func(t *testing.T) {
		handler := &AISettingsHandler{}
		req := httptest.NewRequest(http.MethodGet, "/api/ai/incidents/", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetIncidentData(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("no_service", func(t *testing.T) {
		handler := &AISettingsHandler{}
		req := httptest.NewRequest(http.MethodGet, "/api/ai/incidents/res-1", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetIncidentData(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload["resource_id"] != "res-1" {
			t.Fatalf("unexpected resource_id: %#v", payload["resource_id"])
		}
		if payload["message"] != "Pulse Patrol service not available" {
			t.Fatalf("unexpected message: %#v", payload["message"])
		}
		if incidents, ok := payload["incidents"].([]interface{}); !ok || len(incidents) != 0 {
			t.Fatalf("expected empty incidents, got %#v", payload["incidents"])
		}
		if payload["formatted_context"] != "" {
			t.Fatalf("expected empty formatted_context, got %#v", payload["formatted_context"])
		}
	})

	t.Run("no_patrol", func(t *testing.T) {
		svc := newEnabledAIService(t)
		setUnexportedField(t, svc, "patrolService", (*ai.PatrolService)(nil))
		handler := &AISettingsHandler{defaultAIService: svc}
		req := httptest.NewRequest(http.MethodGet, "/api/ai/incidents/res-1", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetIncidentData(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload["message"] != "Patrol service not available" {
			t.Fatalf("unexpected message: %#v", payload["message"])
		}
	})

	t.Run("no_incident_store", func(t *testing.T) {
		svc := newEnabledAIService(t)
		handler := &AISettingsHandler{defaultAIService: svc}
		req := httptest.NewRequest(http.MethodGet, "/api/ai/incidents/res-1", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetIncidentData(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload["message"] != "Incident store not available" {
			t.Fatalf("unexpected message: %#v", payload["message"])
		}
		if payload["resource_id"] != "res-1" {
			t.Fatalf("unexpected resource_id: %#v", payload["resource_id"])
		}
	})
}

func TestHandleGetRecentIncidents_NoService(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/incidents", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRecentIncidents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Pulse Patrol service not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetCircuitBreakerStatus_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/circuit/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetCircuitBreakerStatus(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetForecastOverview_NoStateProvider(t *testing.T) {
	points := makeForecastPoints(40, time.Now().Add(-40*time.Hour), 20, 1)
	svc := forecast.NewService(forecast.DefaultForecastConfig())
	svc.SetDataProvider(stubForecastProvider{points: points})

	handler := &AISettingsHandler{forecastService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/forecasts/overview?metric=cpu", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecastOverview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] == nil {
		t.Fatalf("expected error in response")
	}
}

func TestHandleGetForecast_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{forecastService: forecast.NewService(forecast.DefaultForecastConfig())}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/forecast", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecast(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetIncidentData_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/incidents/res-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetIncidentData(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetRecentIncidents_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/incidents", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRecentIncidents(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetAnomalies_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence/anomalies", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetAnomalies(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetForecastOverview_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/forecasts/overview", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecastOverview(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetLearningPreferences_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/learning/preferences", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetLearningPreferences(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetUnifiedFindings_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/unified/findings", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetUnifiedFindings(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetProxmoxEvents_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/proxmox/events", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetProxmoxEvents(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetProxmoxCorrelations_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/proxmox/correlations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetProxmoxCorrelations(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
