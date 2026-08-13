package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
)

func TestPatrolObjectivesHTTPContractAndOptimisticRevision(t *testing.T) {
	handler := NewAISettingsHandler(nil, nil, nil)

	createRequest := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/objectives", strings.NewReader(`{
		"brief":"Keep Jellyfin playback smooth",
		"optional_context":"Avoid polling the model continuously",
		"resource_ids":["jellyfin-1"]
	}`))
	createResponse := httptest.NewRecorder()
	handler.HandlePatrolObjectives(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", createResponse.Code, createResponse.Body.String())
	}
	if createResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create cache control = %q", createResponse.Header().Get("Cache-Control"))
	}
	var created ai.PatrolObjective
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created objective: %v", err)
	}
	if created.ID == "" || created.Revision != 1 || created.Coverage.State != ai.PatrolObjectiveUncovered || created.Coverage.ReasonCode != "observer_missing" {
		t.Fatalf("created objective = %+v", created)
	}
	if created.Observer != nil {
		t.Fatalf("public create unexpectedly accepted an observer: %+v", created.Observer)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/objectives", nil)
	listResponse := httptest.NewRecorder()
	handler.HandlePatrolObjectives(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listResponse.Code, listResponse.Body.String())
	}
	var listed struct {
		Objectives []ai.PatrolObjective `json:"objectives"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode objective list: %v", err)
	}
	if len(listed.Objectives) != 1 || listed.Objectives[0].ID != created.ID {
		t.Fatalf("listed objectives = %+v", listed.Objectives)
	}

	patchBody := fmt.Sprintf(`{"revision":%d,"status":"paused"}`, created.Revision)
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/ai/patrol/objectives/"+created.ID, strings.NewReader(patchBody))
	patchResponse := httptest.NewRecorder()
	handler.HandlePatrolObjective(patchResponse, patchRequest)
	if patchResponse.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body=%s", patchResponse.Code, patchResponse.Body.String())
	}
	var updated ai.PatrolObjective
	if err := json.Unmarshal(patchResponse.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated objective: %v", err)
	}
	if updated.Revision != 2 || updated.Status != ai.PatrolObjectivePaused || updated.Coverage.ReasonCode != "objective_paused" {
		t.Fatalf("updated objective = %+v", updated)
	}

	staleRequest := httptest.NewRequest(http.MethodPatch, "/api/ai/patrol/objectives/"+created.ID, strings.NewReader(patchBody))
	staleResponse := httptest.NewRecorder()
	handler.HandlePatrolObjective(staleResponse, staleRequest)
	if staleResponse.Code != http.StatusConflict || !strings.Contains(staleResponse.Body.String(), "patrol_objective_revision_conflict") {
		t.Fatalf("stale patch status = %d, body=%s", staleResponse.Code, staleResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/ai/patrol/objectives/%s?revision=%d", created.ID, updated.Revision), nil)
	deleteResponse := httptest.NewRecorder()
	handler.HandlePatrolObjective(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
}

func TestPatrolObjectivesHTTPRejectsClientAuthoredCoverageAndObservers(t *testing.T) {
	handler := NewAISettingsHandler(nil, nil, nil)
	requests := []string{
		`{"brief":"Keep cameras online","coverage":{"state":"covered"}}`,
		`{"brief":"Keep cameras online","observer":{"state":"installed"}}`,
	}
	for _, body := range requests {
		request := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/objectives", strings.NewReader(body))
		response := httptest.NewRecorder()
		handler.HandlePatrolObjectives(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "patrol_objective_invalid_request") {
			t.Fatalf("body %s: status = %d, response=%s", body, response.Code, response.Body.String())
		}
	}
}

func TestPatrolObjectivesHTTPRejectsOversizedAndAmbiguousBodies(t *testing.T) {
	handler := NewAISettingsHandler(nil, nil, nil)
	bodies := []string{
		`{"brief":"one"}{"brief":"two"}`,
		`{"brief":"` + strings.Repeat("x", maxPatrolObjectiveRequestBytes) + `"}`,
	}
	for _, body := range bodies {
		request := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/objectives", strings.NewReader(body))
		response := httptest.NewRecorder()
		handler.HandlePatrolObjectives(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
		}
	}
}
