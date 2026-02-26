package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

type testAIAlertAnalysisEndpoints struct {
	investigateAlertCalls int
	analyzeK8sCalls       int
}

func (t *testAIAlertAnalysisEndpoints) HandleInvestigateAlert(http.ResponseWriter, *http.Request) {
	t.investigateAlertCalls++
}

func (t *testAIAlertAnalysisEndpoints) HandleAnalyzeKubernetesCluster(http.ResponseWriter, *http.Request) {
	t.analyzeK8sCalls++
}

func TestResolveAIAlertAnalysisEndpoints_DefaultWhenBinderNil(t *testing.T) {
	SetAIAlertAnalysisEndpointsBinder(nil)
	t.Cleanup(func() {
		SetAIAlertAnalysisEndpointsBinder(nil)
	})

	defaults := &testAIAlertAnalysisEndpoints{}
	resolved := resolveAIAlertAnalysisEndpoints(defaults, extensions.AIAlertAnalysisRuntime{})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/investigate-alert", nil)
	rec := httptest.NewRecorder()
	resolved.HandleInvestigateAlert(rec, req)
	if defaults.investigateAlertCalls != 1 {
		t.Fatalf("expected default investigate alert handler call, got %d", defaults.investigateAlertCalls)
	}
}

func TestResolveAIAlertAnalysisEndpoints_UsesBinderOverride(t *testing.T) {
	SetAIAlertAnalysisEndpointsBinder(nil)
	t.Cleanup(func() {
		SetAIAlertAnalysisEndpointsBinder(nil)
	})

	defaults := &testAIAlertAnalysisEndpoints{}
	override := &testAIAlertAnalysisEndpoints{}

	SetAIAlertAnalysisEndpointsBinder(func(_ extensions.AIAlertAnalysisEndpoints, _ extensions.AIAlertAnalysisRuntime) extensions.AIAlertAnalysisEndpoints {
		return override
	})

	resolved := resolveAIAlertAnalysisEndpoints(defaults, extensions.AIAlertAnalysisRuntime{})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/kubernetes/analyze", nil)
	rec := httptest.NewRecorder()
	resolved.HandleAnalyzeKubernetesCluster(rec, req)

	if defaults.analyzeK8sCalls != 0 {
		t.Fatalf("expected default k8s analyzer to be bypassed, got %d calls", defaults.analyzeK8sCalls)
	}
	if override.analyzeK8sCalls != 1 {
		t.Fatalf("expected override k8s analyzer call, got %d", override.analyzeK8sCalls)
	}
}
