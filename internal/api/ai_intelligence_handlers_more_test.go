package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type snapshotStateProvider struct {
	state models.StateSnapshot
}

func (s snapshotStateProvider) GetState() models.StateSnapshot {
	return s.state
}

func newTestReadState(snapshot models.StateSnapshot) unifiedresources.ReadState {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(snapshot)
	return rr
}

func buildBaselineStore(t *testing.T) *ai.BaselineStore {
	t.Helper()
	store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})
	points := []ai.BaselineMetricPoint{{Value: 10, Timestamp: time.Now()}}
	if err := store.Learn("vm-1", "vm", "cpu", points); err != nil {
		t.Fatalf("baseline Learn error: %v", err)
	}
	return store
}

func TestHandleGetRecentChanges_WithDetector(t *testing.T) {
	svc := newEnabledAIService(t)
	detector := ai.NewChangeDetector(ai.ChangeDetectorConfig{MaxChanges: 10})
	change := ai.Change{
		ID:           "change-1",
		ResourceID:   "vm-1",
		ResourceName: "vm-one",
		ResourceType: "vm",
		ChangeType:   ai.ChangeConfig,
		Before:       "old",
		After:        "new",
		DetectedAt:   time.Now().Add(-30 * time.Minute),
		Description:  "updated config",
	}
	setUnexportedField(t, detector, "changes", []ai.Change{change})
	svc.SetChangeDetector(detector)

	handler := &AISettingsHandler{legacyAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/changes?hours=1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRecentChanges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["count"] != float64(1) {
		t.Fatalf("expected count 1, got %#v", payload["count"])
	}
	changes, _ := payload["changes"].([]interface{})
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
}

func TestHandleGetBaselines_WithStore(t *testing.T) {
	svc := newEnabledAIService(t)
	store := buildBaselineStore(t)
	svc.SetBaselineStore(store)

	handler := &AISettingsHandler{legacyAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/baselines?resource_id=vm-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetBaselines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["count"] != float64(1) {
		t.Fatalf("expected count 1, got %#v", payload["count"])
	}
}

func TestHandleGetLearningStatus_WithBaselines(t *testing.T) {
	svc := newEnabledAIService(t)
	store := buildBaselineStore(t)
	svc.SetBaselineStore(store)

	handler := &AISettingsHandler{legacyAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/learning", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetLearningStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["resources_baselined"] != float64(1) {
		t.Fatalf("expected resources_baselined 1, got %#v", payload["resources_baselined"])
	}
	if payload["status"] != "learning" {
		t.Fatalf("expected status learning, got %#v", payload["status"])
	}
}

func TestHandleGetAnomalies_WithBaseline(t *testing.T) {
	svc := newEnabledAIService(t)

	state := models.StateSnapshot{
		VMs: []models.VM{{
			ID:     "vm-1",
			Name:   "vm-one",
			Status: "running",
			CPU:    0.8,
			Memory: models.Memory{Usage: 50},
		}},
	}
	svc.SetStateProvider(snapshotStateProvider{state: state})

	rs := newTestReadState(state)
	vmID := ""
	if vms := rs.VMs(); len(vms) > 0 && vms[0] != nil {
		vmID = vms[0].ID()
	}
	if vmID == "" {
		t.Fatalf("expected ReadState to contain the test VM")
	}

	store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})
	points := []ai.BaselineMetricPoint{{Value: 10, Timestamp: time.Now()}}
	if err := store.Learn(vmID, "vm", "cpu", points); err != nil {
		t.Fatalf("baseline Learn error: %v", err)
	}
	svc.SetBaselineStore(store)

	handler := &AISettingsHandler{legacyAIService: svc}
	handler.SetReadState(rs)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/anomalies?resource_id="+vmID, nil)
	rec := httptest.NewRecorder()

	handler.HandleGetAnomalies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["count"] == float64(0) {
		t.Fatalf("expected anomalies to be returned")
	}
}

func TestHandleGetLearningPreferences_WithStore(t *testing.T) {
	store := learning.NewLearningStore(learning.LearningStoreConfig{})
	handler := &AISettingsHandler{}
	handler.SetLearningStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/learning/preferences?resource_id=vm-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetLearningPreferences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["resource_id"] != "vm-1" {
		t.Fatalf("expected resource_id in response, got %#v", payload["resource_id"])
	}
}

func TestHandleGetUnifiedFindings_WithStore(t *testing.T) {
	store := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	store.AddFromAI(&unified.UnifiedFinding{
		ID:           "finding-1",
		Source:       unified.SourceAIPatrol,
		Severity:     unified.SeverityCritical,
		Category:     unified.CategoryPerformance,
		ResourceID:   "vm-1",
		ResourceName: "vm-one",
		ResourceType: "vm",
		Title:        "CPU high",
		Description:  "cpu usage high",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
	})

	handler := &AISettingsHandler{}
	handler.SetUnifiedStore(store)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/unified/findings?resource_id=vm-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetUnifiedFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["count"] == float64(0) {
		t.Fatalf("expected findings in response")
	}
}
