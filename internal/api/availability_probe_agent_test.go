package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

type stubFeatureChecker struct {
	features map[string]bool
}

func (s stubFeatureChecker) HasFeature(feature string) bool {
	return s.features[feature]
}

func (s stubFeatureChecker) RequireFeature(feature string) error {
	if s.features[feature] {
		return nil
	}
	return fmt.Errorf("feature %s is not licensed", feature)
}

func stubFeatureResolver(features ...string) licenseFeatureServiceResolver {
	allowed := map[string]bool{}
	for _, feature := range features {
		allowed[feature] = true
	}
	return availabilityFeatureResolverFunc(func(context.Context) licenseFeatureChecker {
		return stubFeatureChecker{features: allowed}
	})
}

// monitorWithHostAgent registers one host agent so probe assignment can be
// validated against real monitor state.
func monitorWithHostAgent(t *testing.T, hostID string) *monitoring.Monitor {
	t.Helper()
	monitor, err := monitoring.New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatalf("monitoring.New() error = %v", err)
	}
	t.Cleanup(monitor.Stop)

	if _, err := monitor.ApplyHostReport(agentshost.Report{
		Agent:     agentshost.AgentInfo{ID: hostID + "-agent", Version: "6.1.1", IntervalSeconds: 30},
		Host:      agentshost.HostInfo{ID: hostID, MachineID: hostID, Hostname: hostID, Platform: "linux"},
		Timestamp: time.Now().UTC(),
	}, &config.APITokenRecord{ID: "token-" + hostID, Name: hostID}); err != nil {
		t.Fatalf("ApplyHostReport() error = %v", err)
	}
	return monitor
}

func probeAssignedTargetBody(t *testing.T, agentID string) *bytes.Reader {
	t.Helper()
	return availabilityRequestBody(t, config.AvailabilityTarget{
		Name:             "Branch gateway",
		Address:          "gateway.branch.local",
		Protocol:         config.AvailabilityProbeICMP,
		Enabled:          true,
		PollIntervalSecs: 30,
		TimeoutMillis:    1000,
		FailureThreshold: 2,
		ProbeAgentID:     agentID,
	})
}

func TestAvailabilityHandlersRejectProbeAssignmentWithoutLicense(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	monitor := monitorWithHostAgent(t, "probe-host")
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return monitor },
		stubFeatureResolver(),
	)

	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets", probeAssignedTargetBody(t, "probe-host")))
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("HandleAdd status = %d, want 402; body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode 402 payload: %v", err)
	}
	if payload["error"] != "license_required" || payload["feature"] != featureExternalProbeValue {
		t.Fatalf("402 payload = %+v, want the canonical license_required shape", payload)
	}

	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("persisted targets = %+v, want none", targets)
	}
}

func TestAvailabilityHandlersAcceptProbeAssignmentWithLicense(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	monitor := monitorWithHostAgent(t, "probe-host")
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return monitor },
		stubFeatureResolver(featureExternalProbeValue),
	)

	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets", probeAssignedTargetBody(t, "probe-host")))
	if rec.Code != http.StatusCreated {
		t.Fatalf("HandleAdd status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var created config.AvailabilityTarget
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created target: %v", err)
	}
	if created.ProbeAgentID != "probe-host" {
		t.Fatalf("created ProbeAgentID = %q, want probe-host", created.ProbeAgentID)
	}

	// Reassignment to an unknown agent is rejected even while licensed.
	updated := created
	updated.ProbeAgentID = "ghost-agent"
	updateRec := httptest.NewRecorder()
	handler.HandleUpdate(updateRec, httptest.NewRequest(http.MethodPut, "/api/availability-targets/"+created.ID, availabilityRequestBody(t, updated)))
	if updateRec.Code != http.StatusBadRequest {
		t.Fatalf("HandleUpdate status = %d, want 400; body=%s", updateRec.Code, updateRec.Body.String())
	}

	// Clearing the assignment returns the target to local execution. The wire
	// field is omitempty, so the client has to send it explicitly.
	clearBody := bytes.NewReader([]byte(`{"probeAgentId":""}`))
	clearRec := httptest.NewRecorder()
	handler.HandleUpdate(clearRec, httptest.NewRequest(http.MethodPut, "/api/availability-targets/"+created.ID, clearBody))
	if clearRec.Code != http.StatusOK {
		t.Fatalf("HandleUpdate status = %d, want 200; body=%s", clearRec.Code, clearRec.Body.String())
	}
	stored, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets() error = %v", err)
	}
	if len(stored) != 1 || stored[0].ProbeAgentID != "" {
		t.Fatalf("stored targets = %+v, want a locally executed target", stored)
	}
}

func TestAvailabilityHandlersAcceptSeveralObservationLocationsOnOneTarget(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	monitor := monitorWithHostAgent(t, "edge-a")
	if _, err := monitor.ApplyHostReport(agentshost.Report{
		Agent:     agentshost.AgentInfo{ID: "edge-b-agent", Version: "6.1.1", IntervalSeconds: 30},
		Host:      agentshost.HostInfo{ID: "edge-b", MachineID: "edge-b", Hostname: "edge-b", Platform: "linux"},
		Timestamp: time.Now().UTC(),
	}, &config.APITokenRecord{ID: "token-edge-b", Name: "edge-b"}); err != nil {
		t.Fatalf("ApplyHostReport() error = %v", err)
	}
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return monitor },
		stubFeatureResolver(featureExternalProbeValue),
	)
	target := config.AvailabilityTarget{
		Name:                   "Customer API",
		Address:                "api.service.local",
		Protocol:               config.AvailabilityProbeHTTPS,
		Enabled:                true,
		ObservationLocationIDs: []string{config.AvailabilityObservationLocationLocal, "agent:edge-a", "agent:edge-b"},
	}
	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets", availabilityRequestBody(t, target)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("HandleAdd status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var created config.AvailabilityTarget
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created target: %v", err)
	}
	if len(created.ObservationLocationIDs) != 3 || created.ProbeAgentID != "" {
		t.Fatalf("created target = %+v, want one logical target with three locations", created)
	}
}

func TestAvailabilityHandlersRejectUnknownProbeAgent(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	monitor := monitorWithHostAgent(t, "probe-host")
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return monitor },
		stubFeatureResolver(featureExternalProbeValue),
	)

	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, httptest.NewRequest(http.MethodPost, "/api/availability-targets", probeAssignedTargetBody(t, "not-registered")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("HandleAdd status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode 400 payload: %v", err)
	}
	if payload["code"] != "unknown_probe_agent" {
		t.Fatalf("400 payload = %+v, want unknown_probe_agent", payload)
	}
}

func TestAvailabilityHandlersUpdateKeepsStoredHTTPSecretsFromNewProbeAgent(t *testing.T) {
	const password = "stored-password"
	const headerValue = "stored-header"
	persistence := config.NewConfigPersistence(t.TempDir())
	monitor := monitorWithHostAgent(t, "probe-host")
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return monitor },
		stubFeatureResolver(featureExternalProbeValue),
	)
	passwordValue, headerSecret := password, headerValue
	createRec := httptest.NewRecorder()
	handler.HandleAdd(createRec, httptest.NewRequest(http.MethodPost, "/api/availability-targets", availabilityRequestBody(t, config.AvailabilityTarget{
		ID: "nas", Name: "NAS", Address: "https://nas.service.local/health",
		Protocol: config.AvailabilityProbeHTTPS, Enabled: true,
		HTTP: &config.AvailabilityHTTPConfig{
			Method:         config.AvailabilityHTTPMethodGET,
			Headers:        []config.AvailabilityHTTPHeader{{ID: "key", Name: "X-Api-Key", Value: &headerSecret}},
			Authentication: config.AvailabilityHTTPAuthentication{Type: config.AvailabilityHTTPAuthBasic, Username: "monitor", Password: &passwordValue},
		},
	})))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("HandleAdd status = %d, want 201; body=%s", createRec.Code, createRec.Body.String())
	}
	update := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.HandleUpdate(rec, httptest.NewRequest(http.MethodPut, "/api/availability-targets/nas", bytes.NewBufferString(body)))
		return rec
	}
	stored := func() config.AvailabilityTarget {
		t.Helper()
		targets, err := persistence.LoadAvailabilityTargets()
		if err != nil || len(targets) != 1 {
			t.Fatalf("LoadAvailabilityTargets() = %+v, %v", targets, err)
		}
		return targets[0]
	}

	// The stored values never follow the check to an agent it gains, whichever
	// field names the agent and whichever of them the update leaves out.
	for _, body := range []string{
		`{"observationLocationIds":["pulse:local","agent:probe-host"]}`,
		`{"observationLocationIds":["agent:probe-host"]}`,
		`{"probeAgentId":"probe-host"}`,
		`{"observationLocationIds":["pulse:local","agent:probe-host"],"http":{"authentication":{"password":"entered-again"}}}`,
		`{"observationLocationIds":["pulse:local","agent:probe-host"],"http":{"headers":[{"id":"key","name":"X-Api-Key","value":"entered-again"}]}}`,
	} {
		rec := update(body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "http_secrets_reentry_required") {
			t.Fatalf("HandleUpdate(%s) = %d %s, want http_secrets_reentry_required", body, rec.Code, rec.Body.String())
		}
		got := stored()
		if len(got.AssignedProbeAgentIDs()) != 0 || got.HTTP.Authentication.Password == nil || *got.HTTP.Authentication.Password != password ||
			got.HTTP.Headers[0].Value == nil || *got.HTTP.Headers[0].Value != headerValue {
			t.Fatalf("stored target after refused update = %+v, want the local check unchanged", got)
		}
	}

	// Entering every stored value again assigns the agent.
	rec := update(`{"observationLocationIds":["pulse:local","agent:probe-host"],"http":{"headers":[{"id":"key","name":"X-Api-Key","value":"header-again"}],"authentication":{"password":"entered-again"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("HandleUpdate with re-entered values status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got := stored()
	if !got.IsAssignedToProbeAgent("probe-host") || got.HTTP.Authentication.Password == nil || *got.HTTP.Authentication.Password != "entered-again" ||
		got.HTTP.Headers[0].Value == nil || *got.HTTP.Headers[0].Value != "header-again" {
		t.Fatalf("stored target = %+v, want the agent assigned with the re-entered values", got)
	}

	// Edits that add no agent keep reusing the stored values.
	for _, body := range []string{`{"name":"NAS health"}`, `{"observationLocationIds":["agent:probe-host"]}`} {
		if rec := update(body); rec.Code != http.StatusOK {
			t.Fatalf("HandleUpdate(%s) status = %d, want 200; body=%s", body, rec.Code, rec.Body.String())
		}
		if got := stored(); got.HTTP.Authentication.Password == nil || *got.HTTP.Authentication.Password != "entered-again" {
			t.Fatalf("stored target after %s = %+v, want the stored password kept", body, got)
		}
	}
}

func TestAvailabilityHandlersLocalTargetsNeverConsultLicense(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	resolver := availabilityFeatureResolverFunc(func(context.Context) licenseFeatureChecker {
		t.Fatal("locally executed availability targets must not consult the license service")
		return nil
	})
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return nil },
		resolver,
	)

	createRec := httptest.NewRecorder()
	handler.HandleAdd(createRec, httptest.NewRequest(http.MethodPost, "/api/availability-targets", availabilityRequestBody(t, config.AvailabilityTarget{
		Name:     "Local gateway",
		Address:  "gateway.local",
		Protocol: config.AvailabilityProbeICMP,
		Enabled:  true,
	})))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("HandleAdd status = %d, want 201; body=%s", createRec.Code, createRec.Body.String())
	}
	var created config.AvailabilityTarget
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created target: %v", err)
	}

	listRec := httptest.NewRecorder()
	handler.HandleList(listRec, httptest.NewRequest(http.MethodGet, "/api/availability-targets", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("HandleList status = %d, want 200; body=%s", listRec.Code, listRec.Body.String())
	}

	created.Enabled = false
	updateRec := httptest.NewRecorder()
	handler.HandleUpdate(updateRec, httptest.NewRequest(http.MethodPut, "/api/availability-targets/"+created.ID, availabilityRequestBody(t, created)))
	if updateRec.Code != http.StatusOK {
		t.Fatalf("HandleUpdate status = %d, want 200; body=%s", updateRec.Code, updateRec.Body.String())
	}

	deleteRec := httptest.NewRecorder()
	handler.HandleDelete(deleteRec, httptest.NewRequest(http.MethodDelete, "/api/availability-targets/"+created.ID, nil))
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("HandleDelete status = %d, want 200; body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestSanitizeHostAgentConfigForTokenKeepsProbeTargetsForReportOnlyToken(t *testing.T) {
	targets := []map[string]interface{}{{"id": "remote", "address": "gateway.branch.local"}}
	commandsEnabled := true
	cfg := monitoring.HostAgentConfig{
		CommandsEnabled: &commandsEnabled,
		Settings:        map[string]interface{}{"availabilityTargets": targets},
	}
	record := &config.APITokenRecord{
		ID:       "report-only",
		Scopes:   []string{config.ScopeAgentReport},
		Metadata: map[string]string{"bound_agent_id": "probe-host"},
	}

	sanitized := sanitizeHostAgentConfigForToken(cfg, record, models.Host{ID: "probe-host", Hostname: "probe-host"})
	if sanitized.CommandsEnabled == nil || *sanitized.CommandsEnabled {
		t.Fatalf("commands enabled = %+v, want disabled for a report-only token", sanitized.CommandsEnabled)
	}
	if got, ok := sanitized.Settings["availabilityTargets"].([]map[string]interface{}); !ok || len(got) != 1 || got[0]["id"] != "remote" {
		t.Fatalf("sanitized settings = %+v, want the bound host's own probe targets", sanitized.Settings)
	}
}
