package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestContract_DiagnosticsInfoInfrastructureOnboardingJSONSnapshot(t *testing.T) {
	payload := EmptyDiagnosticsInfo()
	infrastructureOnboarding := (&InfrastructureOnboardingDiagnostic{
		Enabled:    true,
		Status:     "idle",
		WindowDays: 30,
		Summary: conversionInfrastructureOnboardingSummary{
			Period: struct {
				From time.Time `json:"from"`
				To   time.Time `json:"to"`
			}{
				From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
				To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}).NormalizeCollections()
	payload.InfrastructureOnboarding = &infrastructureOnboarding

	got, err := json.Marshal(payload.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal diagnostics onboarding info: %v", err)
	}

	const want = `{
		"version":"",
		"runtime":"",
		"uptime":0,
		"nodes":[],
		"pbs":[],
		"system":{"os":"","arch":"","goVersion":"","numCPU":0,"numGoroutine":0,"memoryMB":0},
		"infrastructureOnboarding":{
			"enabled":true,
			"status":"idle",
			"windowDays":30,
			"summary":{
				"opened":0,
				"api_path_selected":0,
				"agent_path_selected":0,
				"probe_detected":0,
				"probe_no_match":0,
				"probe_error":0,
				"catalog_selected":0,
				"credentials_opened":0,
				"period":{"from":"2026-04-01T00:00:00Z","to":"2026-05-01T00:00:00Z"}
			},
			"daily":[],
			"paths":[],
			"platforms":[],
			"notes":[]
		},
		"errors":[],
		"nodeSnapshots":[],
		"guestSnapshots":[],
		"memorySources":[],
		"memorySourceBreakdown":[]
	}`

	assertJSONSnapshot(t, got, want)
}
