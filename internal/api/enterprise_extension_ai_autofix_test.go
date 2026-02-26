package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

type testAIAutoFixEndpoints struct {
	reinvestigateCalls int
	reapproveCalls     int
	autonomyCalls      int
	approveFixCalls    int
}

func (t *testAIAutoFixEndpoints) HandleReinvestigateFinding(http.ResponseWriter, *http.Request) {
	t.reinvestigateCalls++
}

func (t *testAIAutoFixEndpoints) HandleReapproveInvestigationFix(http.ResponseWriter, *http.Request) {
	t.reapproveCalls++
}

func (t *testAIAutoFixEndpoints) HandleUpdatePatrolAutonomy(http.ResponseWriter, *http.Request) {
	t.autonomyCalls++
}

func (t *testAIAutoFixEndpoints) HandleGetRemediationPlans(http.ResponseWriter, *http.Request) {}
func (t *testAIAutoFixEndpoints) HandleGetRemediationPlan(http.ResponseWriter, *http.Request)  {}
func (t *testAIAutoFixEndpoints) HandleApproveRemediationPlan(http.ResponseWriter, *http.Request) {
}
func (t *testAIAutoFixEndpoints) HandleExecuteRemediationPlan(http.ResponseWriter, *http.Request) {
}
func (t *testAIAutoFixEndpoints) HandleRollbackRemediationPlan(http.ResponseWriter, *http.Request) {
}

func (t *testAIAutoFixEndpoints) HandleApproveInvestigationFix(http.ResponseWriter, *http.Request) {
	t.approveFixCalls++
}

func TestResolveAIAutoFixEndpoints_DefaultWhenBinderNil(t *testing.T) {
	SetAIAutoFixEndpointsBinder(nil)
	t.Cleanup(func() {
		SetAIAutoFixEndpointsBinder(nil)
	})

	defaults := &testAIAutoFixEndpoints{}
	resolved := resolveAIAutoFixEndpoints(defaults, extensions.AIAutoFixRuntime{})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/findings/abc/reinvestigate", nil)
	rec := httptest.NewRecorder()
	resolved.HandleReinvestigateFinding(rec, req)
	if defaults.reinvestigateCalls != 1 {
		t.Fatalf("expected default reinvestigate handler call, got %d", defaults.reinvestigateCalls)
	}
}

func TestResolveAIAutoFixEndpoints_UsesBinderOverride(t *testing.T) {
	SetAIAutoFixEndpointsBinder(nil)
	t.Cleanup(func() {
		SetAIAutoFixEndpointsBinder(nil)
	})

	defaults := &testAIAutoFixEndpoints{}
	override := &testAIAutoFixEndpoints{}

	SetAIAutoFixEndpointsBinder(func(_ extensions.AIAutoFixEndpoints, _ extensions.AIAutoFixRuntime) extensions.AIAutoFixEndpoints {
		return override
	})

	resolved := resolveAIAutoFixEndpoints(defaults, extensions.AIAutoFixRuntime{})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/findings/abc/reapprove", nil)
	rec := httptest.NewRecorder()
	resolved.HandleReapproveInvestigationFix(rec, req)

	if defaults.reapproveCalls != 0 {
		t.Fatalf("expected default reapprove handler to be bypassed, got %d calls", defaults.reapproveCalls)
	}
	if override.reapproveCalls != 1 {
		t.Fatalf("expected override reapprove handler call, got %d", override.reapproveCalls)
	}
}
