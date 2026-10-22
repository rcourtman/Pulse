package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Discussion #1699: exercise the alert producer, the exact alert-button body,
// HTTP admission and the accepted scoped run, not just the resolver helper.
// No ChatService is installed: the accepted run must retain its identity and
// finish as an explicit analysis error without calling a model or any tools.
func TestPatrolRunAPIDockerAlertIdentityContract(t *testing.T) {
	for _, unified := range []bool{false, true} {
		path := "snapshot"
		if unified {
			path = "unified"
		}
		for _, target := range []struct {
			name         string
			hostAlert    bool
			withoutID    bool
			acknowledged bool
		}{
			{name: "exited-container"},
			{name: "acknowledged-exited-container", acknowledged: true},
			{name: "offline-host", hostAlert: true},
			{name: "name-only-container", withoutID: true, acknowledged: true},
		} {
			t.Run(path+"/"+target.name, func(t *testing.T) {
				container := models.DockerContainer{ID: "abc123def456", Name: "canary-app", State: "exited", Status: "Exited (0)"}
				if target.withoutID {
					container.ID = ""
				}
				host := models.DockerHost{ID: "fixture-nas", Hostname: "fixture-nas", DisplayName: "Fixture NAS", LastSeen: time.Now(), Containers: []models.DockerContainer{container}}
				manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				t.Cleanup(manager.Stop)
				setUnexportedField(t, manager, "config", alerts.AlertConfig{
					Enabled: true, ActivationState: alerts.ActivationActive,
					DockerDefaults: alerts.DockerThresholdConfig{StatePoweredOffSeverity: alerts.AlertLevelWarning},
					Overrides:      map[string]alerts.ThresholdConfig{}, TimeThresholds: map[string]int{},
				})
				// Preserve production confirmation counts (two for a stopped
				// container, three for an offline host); no timers or policies
				// in the producer are weakened to manufacture the alert.
				for range 3 {
					if target.hostAlert {
						manager.HandleDockerHostOffline(host)
					} else {
						manager.CheckDockerHost(host)
					}
				}
				active := manager.GetActiveAlerts()
				if len(active) != 1 {
					t.Fatalf("producer alerts = %+v, want exactly one", active)
				}
				if target.acknowledged {
					if err := manager.AcknowledgeAlert(active[0].ID, "fixture-operator"); err != nil {
						t.Fatal(err)
					}
					active = manager.GetActiveAlerts()
				}
				alert := active[0]
				wantID := alerts.DockerContainerResourceID(host.ID, container.ID, container.Name)
				wantLevel := alerts.AlertLevelWarning
				wantType := "docker-container-state"
				if target.hostAlert {
					wantID, wantLevel, wantType = alerts.DockerHostResourceID(host.ID), alerts.AlertLevelCritical, "docker-host-offline"
				}
				if alert.ResourceID != wantID || alert.Type != wantType || alert.Level != wantLevel || alert.Acknowledged != target.acknowledged {
					t.Fatalf("unexpected producer identity/state: %+v", alert)
				}

				// A second host has the same container name. An exact alert ID
				// must not fall back to that name or broaden to that host.
				snapshot := models.StateSnapshot{DockerHosts: []models.DockerHost{host, {
					ID: "other-nas", Hostname: "other-nas", LastSeen: time.Now(),
					Containers: []models.DockerContainer{{ID: "other-container", Name: container.Name, State: "running"}},
				}}, ActiveAlerts: []models.Alert{{
					ID: alert.ID, ResourceID: alert.ResourceID, ResourceName: alert.ResourceName,
					Type: alert.Type, Level: string(alert.Level), Message: alert.Message, Metadata: alert.Metadata,
				}}}
				handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
				t.Cleanup(handler.defaultAIService.Stop)
				seedReadyAnthropicPatrolRuntime(t, handler)
				handler.defaultAIService.SetChatService(nil)
				handler.defaultAIService.SetStateProvider(&scopedPatrolStateProvider{state: snapshot})
				patrol.SetConfig(ai.PatrolConfig{Enabled: true, AnalyzeDocker: true})
				if unified {
					registry := unifiedresources.NewRegistry(nil)
					registry.IngestSnapshot(snapshot)
					handler.defaultAIService.SetReadState(registry)
					patrol.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
				}
				request := func(resourceID string) *httptest.ResponseRecorder {
					// Match InvestigateAlertButton; never include a client-authored
					// context, resource type or rewritten resource identity.
					body, err := json.Marshal(map[string]any{
						"resource_ids": []string{resourceID}, "alert_identifier": alert.ID, "alert_type": alert.Type,
					})
					if err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					handler.HandleForcePatrol(rec, newLoopbackRequest(http.MethodPost, "/api/ai/patrol/run", bytes.NewReader(body)))
					return rec
				}
				for _, invalid := range []struct{ id, detail string }{
					{alerts.DockerContainerResourceID("missing-nas", container.ID, container.Name), "unmatched_resource_ids"},
					{container.Name, "ambiguous_resource_ids"},
				} {
					rec := request(invalid.id)
					var payload APIError
					if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusUnprocessableEntity || payload.Code != "patrol_scope_unresolved" || payload.Details[invalid.detail] != invalid.id {
						t.Fatalf("invalid target %q: %d %s", invalid.id, rec.Code, rec.Body.String())
					}
				}
				if patrol.GetStatus().Running || len(patrol.GetRunHistory(10)) != 0 {
					t.Fatal("unresolved request started a run")
				}
				setUnexportedField(t, patrol, "runInProgress", true)
				setUnexportedField(t, patrol, "currentRunID", "fixture-busy")
				busy := request(alert.ResourceID)
				var busyError APIError
				if err := json.Unmarshal(busy.Body.Bytes(), &busyError); err != nil {
					t.Fatal(err)
				}
				if busy.Code != http.StatusConflict || busyError.Code != "patrol_already_running" || busyError.Details["current_run_id"] != "fixture-busy" {
					t.Fatalf("resolved target did not reach busy admission: %d %s", busy.Code, busy.Body.String())
				}
				setUnexportedField(t, patrol, "runInProgress", false)
				setUnexportedField(t, patrol, "currentRunID", "")

				rec := request(alert.ResourceID)
				var accepted struct {
					Success    bool                     `json:"success"`
					RunID      string                   `json:"run_id"`
					StartedAt  time.Time                `json:"started_at"`
					Resolution ai.PatrolScopeResolution `json:"scope_resolution"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusOK || !accepted.Success || accepted.RunID == "" || accepted.StartedAt.IsZero() {
					t.Fatalf("exact alert target not accepted: %d %s", rec.Code, rec.Body.String())
				}
				resolution := accepted.Resolution
				if !slices.Equal(resolution.RequestedResourceIDs, []string{alert.ResourceID}) || len(resolution.EffectiveResourceIDs) == 0 || len(resolution.UnmatchedResourceIDs)+len(resolution.AmbiguousResourceIDs) != 0 {
					t.Fatalf("unexpected accepted resolution: %+v", resolution)
				}
				for _, other := range []string{"other-nas", "other-container"} {
					if slices.Contains(resolution.EffectiveResourceIDs, other) {
						t.Fatalf("target scope leaked to %q: %+v", other, resolution)
					}
				}
				deadline := time.Now().Add(3 * time.Second)
				for patrol.GetStatus().Running && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if patrol.GetStatus().Running {
					t.Fatal("provider-free accepted run did not finish")
				}
				history := patrol.GetRunHistory(10)
				if len(history) != 1 {
					t.Fatalf("accepted run history = %+v, want exactly one", history)
				}
				run := history[0]
				if run.ID != accepted.RunID || !run.StartedAt.Equal(accepted.StartedAt) || run.Type != "scoped" || run.TriggerReason != string(ai.TriggerReasonManual) || run.AlertIdentifier != alert.ID || !slices.Equal(run.ScopeResourceIDs, resolution.RequestedResourceIDs) || !slices.Equal(run.EffectiveScopeResourceIDs, resolution.EffectiveResourceIDs) {
					t.Fatalf("accepted request identity lost at scoped execution: %+v", run)
				}
				if run.Status != "error" || run.ErrorCount != 1 || !strings.Contains(run.ErrorDetail, "chat service not available") || run.FindingsSummary == "All healthy" || run.ResourcesChecked == 0 || run.InputTokens+run.OutputTokens != 0 {
					t.Fatalf("fixture must retain explicit no-chat failure, not successful analysis: %+v", run)
				}
				t.Logf("producer=%q level=%q acknowledged=%v; HTTP=200; scope=%v; accepted run=%q; history=%s (%s)", alert.ResourceID, alert.Level, alert.Acknowledged, run.EffectiveScopeResourceIDs, run.ID, run.Status, run.ErrorDetail)
				// Export only this synthetic producer/handler/history chain so
				// browser proof can consume the actual JSON without replacing
				// backend outcomes with hand-authored success fixtures.
				proof, err := json.Marshal(map[string]any{
					"case": path + "/" + target.name, "alert": alert, "accepted": json.RawMessage(rec.Body.Bytes()),
					"busy": json.RawMessage(busy.Body.Bytes()), "run": run,
				})
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("PATROL_DOCKER_PROOF=%s", proof)
			})
		}
	}
}
