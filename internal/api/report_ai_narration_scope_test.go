package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// Report narration through Pulse Assistant is a paid provider call recorded in
// the cost ledger. Reports are readable with settings:read, so an API token
// must also hold ai:execute to spend that budget, as on every other AI route.
// Browser sessions keep working as before.
func reportCaller(t *testing.T, scopes []string) *config.APITokenRecord {
	t.Helper()
	if scopes == nil {
		return nil
	}
	record, err := config.NewAPITokenRecord("report-narration-token.12345678", "report-caller", scopes)
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	return record
}

var reportNarrationCallers = []struct {
	name   string
	scopes []string
	wantAI bool
}{
	{"browser session (no token)", nil, true},
	{"settings:read token", []string{config.ScopeSettingsRead}, false},
	{"settings:read plus monitoring:read token", []string{config.ScopeSettingsRead, config.ScopeMonitoringRead}, false},
	{"settings:read plus ai:execute token", []string{config.ScopeSettingsRead, config.ScopeAIExecute}, true},
	{"full-access token", []string{config.ScopeWildcard}, true},
}

func TestSingleReportAINarrationNeedsAIExecuteForTokens(t *testing.T) {
	for _, tc := range reportNarrationCallers {
		t.Run(tc.name, func(t *testing.T) {
			store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			engine := &recordingReportEngine{ReportEngine: reporting.NewReportEngine(reporting.EngineConfig{MetricsStore: store})}
			original := reporting.GetEngine()
			reporting.SetEngine(engine)
			t.Cleanup(func() { reporting.SetEngine(original) })

			narrator := &countingReportNarrator{}
			var narratorResolves, findingsResolves atomic.Int32
			handler := NewReportingHandlers(nil, nil)
			handler.SetNarratorResolver(func(context.Context) (reporting.Narrator, reporting.FleetNarrator, reporting.FindingsProvider) {
				narratorResolves.Add(1)
				return narrator, nil, nil
			})
			handler.SetExistingFindingsResolver(func(context.Context) reporting.FindingsProvider {
				findingsResolves.Add(1)
				return nil
			})

			req := httptest.NewRequest(http.MethodPost, "/api/admin/reports/generate",
				strings.NewReader(`{"format":"pdf","resourceType":"node","resourceId":"node-1"}`))
			if caller := reportCaller(t, tc.scopes); caller != nil {
				attachAPITokenRecord(req, caller)
			}
			rec := httptest.NewRecorder()
			handler.HandleGenerateReport(rec, req)
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" {
				t.Fatalf("status %d, content type %q, body=%s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
			}

			if tc.wantAI {
				if narrator.calls.Load() != 1 || narratorResolves.Load() != 1 {
					t.Fatalf("AI narration calls=%d resolves=%d, want 1 and 1", narrator.calls.Load(), narratorResolves.Load())
				}
				return
			}
			if narrator.calls.Load() != 0 || narratorResolves.Load() != 0 {
				t.Fatalf("a caller without ai:execute spent AI budget: calls=%d resolves=%d", narrator.calls.Load(), narratorResolves.Load())
			}
			if findingsResolves.Load() != 1 {
				t.Fatalf("existing Patrol findings lookups = %d, want 1", findingsResolves.Load())
			}
			narrative, err := engine.NarrativeFor(engine.requests[len(engine.requests)-1])
			if err != nil {
				t.Fatalf("narrative: %v", err)
			}
			if narrative.Source != reporting.NarrativeSourceHeuristic || narrative.Disclaimer != reportScopeNarrativeNote {
				t.Fatalf("narrative = source %q disclaimer %q, want the deterministic summary with the scope note", narrative.Source, narrative.Disclaimer)
			}
		})
	}
}

type stubFleetNarrator struct{}

func (stubFleetNarrator) NarrateFleet(context.Context, reporting.FleetNarrativeInput) (reporting.FleetNarrative, error) {
	return reporting.FleetNarrative{}, nil
}

func TestFleetReportAINarrationNeedsAIExecuteForTokens(t *testing.T) {
	for _, tc := range reportNarrationCallers {
		t.Run(tc.name, func(t *testing.T) {
			engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
			original := reporting.GetEngine()
			reporting.SetEngine(engine)
			t.Cleanup(func() { reporting.SetEngine(original) })

			var narratorResolves, findingsResolves atomic.Int32
			handler := NewReportingHandlers(nil, nil)
			handler.SetNarratorResolver(func(context.Context) (reporting.Narrator, reporting.FleetNarrator, reporting.FindingsProvider) {
				narratorResolves.Add(1)
				return nil, stubFleetNarrator{}, nil
			})
			handler.SetExistingFindingsResolver(func(context.Context) reporting.FindingsProvider {
				findingsResolves.Add(1)
				return nil
			})

			req := httptest.NewRequest(http.MethodPost, "/api/admin/reports/generate-multi",
				strings.NewReader(`{"resources":[{"resourceType":"vm","resourceId":"vm-1"}],"format":"pdf"}`))
			if caller := reportCaller(t, tc.scopes); caller != nil {
				attachAPITokenRecord(req, caller)
			}
			rec := httptest.NewRecorder()
			handler.HandleGenerateMultiReport(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, body=%s", rec.Code, rec.Body.String())
			}

			gotAI := engine.lastMulti.FleetNarrator != nil
			if gotAI != tc.wantAI {
				t.Fatalf("fleet narrator passed to the engine = %v, want %v", gotAI, tc.wantAI)
			}
			if tc.wantAI && narratorResolves.Load() != 1 {
				t.Fatalf("AI service resolves = %d, want 1", narratorResolves.Load())
			}
			if !tc.wantAI && (narratorResolves.Load() != 0 || findingsResolves.Load() != 1) {
				t.Fatalf("a caller without ai:execute resolved the AI service %d times and looked up existing findings %d times, want 0 and 1", narratorResolves.Load(), findingsResolves.Load())
			}
		})
	}
}
