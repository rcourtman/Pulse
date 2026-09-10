package alertlifecycle

import (
	"bytes"
	"encoding/json"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/apihttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"text/template"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// This proves driver fixtures against production wire types, not installed delivery.
func TestExternalDriverWireContract(t *testing.T) {
	cmd := exec.Command("python3", "-c", "import acceptance,json; print(json.dumps({'report':acceptance.report('acceptance-contract',95),'template':acceptance.TEMPLATE}))")
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Report   json.RawMessage
		Template string
	}
	if err := json.Unmarshal(output, &fixture); err != nil {
		t.Fatal(err)
	}
	var report agentshost.Report
	decoder := json.NewDecoder(bytes.NewReader(fixture.Report))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		t.Fatal(err)
	}
	if report.Agent.ID != "acceptance-contract" || report.Agent.CommandsEnabled ||
		report.Host.Hostname != report.Agent.ID || report.Metrics.CPUUsagePercent != 95 ||
		report.Timestamp.IsZero() {
		t.Fatalf("driver report lost required monitoring identity/metrics: %+v", report)
	}
	tmpl, err := template.New("receipt").Option("missingkey=error").Parse(fixture.Template)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, notifications.WebhookPayloadData{
		ID: "agent-cpu", ResourceName: report.Host.Hostname,
		StartTime: "2026-09-10T14:30:00Z", Event: "alert",
	}); err != nil {
		t.Fatal(err)
	}
	var receipt map[string]string
	if err := json.Unmarshal(body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt["id"] != "agent-cpu" || receipt["resource"] != report.Host.Hostname ||
		receipt["start"] != "2026-09-10T14:30:00Z" || receipt["event"] != "alert" {
		t.Fatalf("invalid receipt: %v", receipt)
	}
}

// Keep the provisioning contract aligned with the real scope guard. This is
// authorization proof only, not permission to replay a rejected installed run.
func TestDocumentedFixtureScopesAdmitAlertConfiguration(t *testing.T) {
	documentation, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		config.ScopeAgentReport, config.ScopeMonitoringRead,
		config.ScopeMonitoringWrite, config.ScopeSettingsRead, config.ScopeSettingsWrite,
	}
	for _, scope := range required {
		if !strings.Contains(string(documentation), scope) {
			t.Fatalf("fixture instructions omit required scope %q", scope)
		}
	}
	for _, tc := range []struct {
		name    string
		scopes  []string
		allowed bool
	}{
		{"documented", required, true},
		{"original-missing-monitoring-write", []string{config.ScopeAgentReport, config.ScopeMonitoringRead, config.ScopeSettingsRead, config.ScopeSettingsWrite}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record, err := config.NewAPITokenRecord("synthetic-fixture-contract-token", tc.name, tc.scopes)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPut, "/api/alerts/config", nil)
			req = req.WithContext(internalauth.WithAPIToken(req.Context(), record))
			response := httptest.NewRecorder()
			if got := apihttp.EnsureScope(response, req, config.ScopeMonitoringWrite); got != tc.allowed {
				t.Fatalf("admitted=%v, want %v", got, tc.allowed)
			}
			if !tc.allowed && response.Code != http.StatusForbidden {
				t.Fatalf("missing scope returned %d, want 403", response.Code)
			}
		})
	}
}
