package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestHostAgentHandlers_LegacyV5ReportUpgradesToSingleCanonicalV6Agent(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)

	decodeAgentID := func(t *testing.T, rec *httptest.ResponseRecorder) string {
		t.Helper()

		var resp struct {
			AgentID string `json:"agentId"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.AgentID == "" {
			t.Fatalf("expected non-empty agentId in response")
		}
		return resp.AgentID
	}

	postReport := func(t *testing.T, path string, report agentshost.Report) string {
		t.Helper()

		body, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleReport(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200: %s", path, rec.Code, rec.Body.String())
		}
		return decodeAgentID(t, rec)
	}

	now := time.Now().UTC()
	legacyReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "legacy-agent-1",
			Version:         "5.1.14",
			IntervalSeconds: 30,
			// Empty type simulates the pre-v6 legacy agent path.
		},
		Host: agentshost.HostInfo{
			ID:        "machine-upgrade-1",
			MachineID: "machine-upgrade-1",
			Hostname:  "tower.local",
			Platform:  "linux",
			OSName:    "Debian",
			OSVersion: "12",
		},
		Timestamp: now,
	}

	legacyID := postReport(t, "/api/agents/host/report", legacyReport)

	liveHosts := monitor.GetLiveHostsSnapshot()
	if len(liveHosts) != 1 {
		t.Fatalf("expected 1 live host after legacy report, got %d", len(liveHosts))
	}
	if liveHosts[0].ID != legacyID {
		t.Fatalf("legacy host ID = %q, want %q", liveHosts[0].ID, legacyID)
	}
	if !liveHosts[0].IsLegacy {
		t.Fatalf("expected legacy report to be marked legacy")
	}

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		t.Fatalf("expected unified read state")
	}
	if got := len(readState.Hosts()); got != 1 {
		t.Fatalf("expected 1 canonical host view after legacy report, got %d", got)
	}
	legacyCanonicalID := readState.Hosts()[0].ID()
	if legacyCanonicalID == "" {
		t.Fatalf("expected non-empty canonical host view ID")
	}
	if got := readState.Hosts()[0].AgentVersion(); got != "5.1.14" {
		t.Fatalf("legacy canonical host version = %q, want %q", got, "5.1.14")
	}
	legacyCanonicalAgentID := readState.Hosts()[0].AgentID()
	if legacyCanonicalAgentID == "" {
		t.Fatalf("expected non-empty canonical agent source ID")
	}

	upgradedReport := legacyReport
	upgradedReport.Agent.Type = "unified"
	upgradedReport.Agent.Version = "6.0.0-rc.1"
	upgradedReport.Agent.UpdatedFrom = "5.1.14"
	upgradedReport.Timestamp = now.Add(30 * time.Second)

	upgradedID := postReport(t, "/api/agents/agent/report", upgradedReport)
	if upgradedID != legacyID {
		t.Fatalf("upgraded agent ID = %q, want stable %q", upgradedID, legacyID)
	}

	liveHosts = monitor.GetLiveHostsSnapshot()
	if len(liveHosts) != 1 {
		t.Fatalf("expected 1 live host after v6 upgrade report, got %d", len(liveHosts))
	}
	if liveHosts[0].ID != legacyID {
		t.Fatalf("upgraded host ID = %q, want %q", liveHosts[0].ID, legacyID)
	}
	if liveHosts[0].IsLegacy {
		t.Fatalf("expected upgraded host to clear legacy flag")
	}
	if liveHosts[0].AgentVersion != "6.0.0-rc.1" {
		t.Fatalf("upgraded host version = %q, want %q", liveHosts[0].AgentVersion, "6.0.0-rc.1")
	}

	readState = monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		t.Fatalf("expected unified read state after upgrade report")
	}
	if got := len(readState.Hosts()); got != 1 {
		t.Fatalf("expected 1 canonical host view after upgrade report, got %d", got)
	}

	hostView := readState.Hosts()[0]
	if got := hostView.ID(); got != legacyCanonicalID {
		t.Fatalf("upgraded canonical host view ID = %q, want stable %q", got, legacyCanonicalID)
	}
	if got := hostView.AgentID(); got != legacyCanonicalAgentID {
		t.Fatalf("upgraded canonical agent source ID = %q, want stable %q", got, legacyCanonicalAgentID)
	}
	if got := hostView.AgentVersion(); got != "6.0.0-rc.1" {
		t.Fatalf("upgraded canonical host version = %q, want %q", got, "6.0.0-rc.1")
	}
}
