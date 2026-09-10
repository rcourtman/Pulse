package alertlifecycle

import (
	"bytes"
	"encoding/json"
	"os/exec"
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
