package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

func TestMockActionInboxAndDetailUseCanonicalResponses(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })
	handler := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})

	for _, test := range []struct {
		view      string
		wantCount int
	}{
		{view: "pending", wantCount: 3},
		{view: "settled", wantCount: 3},
	} {
		recorder := httptest.NewRecorder()
		handler.HandleListActions(recorder, httptest.NewRequest(http.MethodGet, "/api/actions?view="+test.view, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("list %s status = %d, body=%s", test.view, recorder.Code, recorder.Body.String())
		}
		var response actionInboxResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode %s response: %v", test.view, err)
		}
		if response.Count != test.wantCount || len(response.Actions) != test.wantCount {
			t.Fatalf("list %s count = %d actions=%d, want %d", test.view, response.Count, len(response.Actions), test.wantCount)
		}
		if !response.ReadOnly {
			t.Fatalf("list %s must declare the mock projection read-only", test.view)
		}
		for _, action := range response.Actions {
			if action.Plan.PolicyDecision.DecisionID == "" || action.Plan.PlanHash == "" {
				t.Fatalf("list %s returned incomplete review identity: %#v", test.view, action.Plan)
			}
		}
	}

	fixtures := mock.ActionFixtures()
	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/actions/"+fixtures[0].Audit.ID, nil)
	detailRequest.SetPathValue("id", fixtures[0].Audit.ID)
	handler.HandleGetAction(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detail actionDetailResponse
	if err := json.NewDecoder(detailRecorder.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Audit.ID != fixtures[0].Audit.ID || len(detail.Events) == 0 {
		t.Fatalf("detail = %#v", detail)
	}
	if !detail.ReadOnly {
		t.Fatal("mock detail must declare itself read-only")
	}
}

func TestMockActionInboxHonorsLimitAndLegacyPendingProjection(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })
	handler := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})

	limited := httptest.NewRecorder()
	handler.HandleListActions(limited, httptest.NewRequest(http.MethodGet, "/api/actions?view=pending&limit=1", nil))
	var limitedResponse actionInboxResponse
	if err := json.NewDecoder(limited.Body).Decode(&limitedResponse); err != nil {
		t.Fatalf("decode limited response: %v", err)
	}
	if limitedResponse.Count != 1 || len(limitedResponse.Actions) != 1 {
		t.Fatalf("limited response count=%d actions=%d", limitedResponse.Count, len(limitedResponse.Actions))
	}
	if !limitedResponse.ReadOnly {
		t.Fatal("limited mock response must declare itself read-only")
	}

	pending := httptest.NewRecorder()
	handler.HandleListPendingActions(pending, httptest.NewRequest(http.MethodGet, "/api/actions/pending", nil))
	var pendingResponse pendingActionsResponse
	if err := json.NewDecoder(pending.Body).Decode(&pendingResponse); err != nil {
		t.Fatalf("decode pending response: %v", err)
	}
	if pendingResponse.Count != 1 || pendingResponse.Actions[0].State != "pending_approval" {
		t.Fatalf("legacy pending response = %#v", pendingResponse)
	}
	if !pendingResponse.ReadOnly {
		t.Fatal("legacy pending response must declare itself read-only")
	}
}

func TestMockActionMutationsFailReadOnly(t *testing.T) {
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previous) })
	handler := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})

	tests := []struct {
		name string
		path string
		body string
		run  func(http.ResponseWriter, *http.Request)
	}{
		{name: "plan", path: "/api/actions/plan", body: `{}`, run: handler.HandlePlanAction},
		{name: "decide", path: "/api/actions/demo-action-install-updates/decision", body: `{"outcome":"approved"}`, run: handler.HandleDecideAction},
		{name: "execute", path: "/api/actions/demo-action-restart-checkout/execute", body: `{}`, run: handler.HandleExecuteAction},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(test.body))
			test.run(recorder, request)
			if recorder.Code != http.StatusForbidden || !bytes.Contains(recorder.Body.Bytes(), []byte("mock_mode_enabled")) {
				t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
