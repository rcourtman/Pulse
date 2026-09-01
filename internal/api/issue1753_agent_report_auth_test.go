package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agentbinding"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// Issue #1753 (2026-09-01 comment): two standalone Proxmox sites reuse the
// short node name pve01. After upgrading to v6.4.2 and reinstalling both
// Unified Agents, the reporter sees "Unauthorized access attempt" server log
// lines for /api/agents/agent/report. This test models that estate end to end
// through the real router (global auth middleware, RequireAuth, RequireScope,
// ApplyHostReport) and fails if either agent's report path ever loses
// authentication, including across the uninstall/reinstall cycle the reporter
// performed.

func issue1753InstallTokenRecord(t *testing.T, raw, site string) config.APITokenRecord {
	t.Helper()
	record, err := config.NewAPITokenRecord(raw, "Proxmox agent install ("+site+")", agenttokens.ProxmoxScopes(false))
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	record.Metadata = map[string]string{
		"install_type":                     "pve",
		"issued_via":                       agentbinding.IssuedViaConfig,
		agenttokens.RuntimeRoleMetadataKey: agenttokens.CredentialKindMonitoringCollector,
	}
	return *record
}

func issue1753Report(agentID, hostname, machineID, reportIP string) agentshost.Report {
	return agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              agentID,
			Version:         "6.4.2",
			Type:            "unified",
			IntervalSeconds: 30,
			Hostname:        hostname,
		},
		Host: agentshost.HostInfo{
			Hostname:  hostname,
			MachineID: machineID,
			Platform:  "proxmox",
			OSName:    "debian",
			ReportIP:  reportIP,
		},
		Timestamp: time.Now().UTC(),
	}
}

func issue1753PostReport(t *testing.T, router *Router, token string, report agentshost.Report) (int, map[string]any) {
	t.Helper()
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	return rec.Code, payload
}

func TestIssue1753TwoStandaloneSitesSameShortHostnameKeepReportAuth(t *testing.T) {
	const (
		tokenSiteA = "issue1753-site-a-token.12345678"
		tokenSiteB = "issue1753-site-b-token.87654321"
	)

	cfg := newTestConfigWithTokens(t,
		issue1753InstallTokenRecord(t, tokenSiteA, "site-a"),
		issue1753InstallTokenRecord(t, tokenSiteB, "site-b"),
	)
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	defer monitor.Stop()
	router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "6.4.2")

	var ackIDA, ackIDB string
	for round := 0; round < 4; round++ {
		code, payload := issue1753PostReport(t, router, tokenSiteA,
			issue1753Report("", "pve01", "machine-id-site-a", "10.1.0.10"))
		if code != http.StatusOK {
			t.Fatalf("round %d: site A report rejected with %d (%v)", round, code, payload)
		}
		ackIDA, _ = payload["agentId"].(string)

		code, payload = issue1753PostReport(t, router, tokenSiteB,
			issue1753Report("", "pve01", "machine-id-site-b", "10.2.0.10"))
		if code != http.StatusOK {
			t.Fatalf("round %d: site B report rejected with %d (%v)", round, code, payload)
		}
		ackIDB, _ = payload["agentId"].(string)
	}

	if ackIDA == "" || ackIDB == "" {
		t.Fatalf("expected both sites to receive acknowledged agent IDs, got %q and %q", ackIDA, ackIDB)
	}
	if ackIDA == ackIDB {
		t.Fatalf("both standalone sites were acknowledged with the same agent identity %q; uninstalling either agent would act on the other site's record", ackIDA)
	}

	// The reporter's cycle: site B uninstalls its agent (the uninstaller calls
	// the unregister endpoint), then reinstalls with a freshly minted token.
	uninstallBody, _ := json.Marshal(map[string]string{"agentId": ackIDB})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/uninstall", bytes.NewReader(uninstallBody))
	req.Header.Set("X-API-Token", tokenSiteB)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("site B uninstall failed with %d: %s", rec.Code, rec.Body.String())
	}

	// Site A must keep reporting with its original token after site B's
	// uninstall: revoking site A's credential here is the cross-site failure.
	code, payload := issue1753PostReport(t, router, tokenSiteA,
		issue1753Report(ackIDA, "pve01", "machine-id-site-a", "10.1.0.10"))
	if code != http.StatusOK {
		t.Fatalf("site A report rejected with %d after site B uninstall (%v)", code, payload)
	}

	// Site B reinstalls: fresh token, same short hostname, same machine.
	const tokenSiteB2 = "issue1753-site-b-second.11112222"
	recordB2 := issue1753InstallTokenRecord(t, tokenSiteB2, "site-b-reinstall")
	config.Mu.Lock()
	cfg.APITokens = append(cfg.APITokens, recordB2)
	cfg.SortAPITokens()
	config.Mu.Unlock()

	for round := 0; round < 3; round++ {
		code, payload = issue1753PostReport(t, router, tokenSiteB2,
			issue1753Report("", "pve01", "machine-id-site-b", "10.2.0.10"))
		if code != http.StatusOK {
			t.Fatalf("round %d: reinstalled site B report rejected with %d (%v)", round, code, payload)
		}
		code, payload = issue1753PostReport(t, router, tokenSiteA,
			issue1753Report(ackIDA, "pve01", "machine-id-site-a", "10.1.0.10"))
		if code != http.StatusOK {
			t.Fatalf("round %d: site A report rejected with %d after site B reinstall (%v)", round, code, payload)
		}
	}

	// Both live tokens must still be present in the inventory: a silent
	// revocation here is what surfaces as 401 "Unauthorized access attempt".
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	remaining := map[string]bool{}
	for _, record := range cfg.APITokens {
		remaining[record.Name] = true
	}
	for _, name := range []string{"Proxmox agent install (site-a)", "Proxmox agent install (site-b-reinstall)"} {
		if !remaining[name] {
			t.Fatalf("token %q was removed from the inventory (have %v)", name, remaining)
		}
	}
}

func issue1753UninstallAgent(t *testing.T, router *Router, token, agentID string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"agentId": agentID})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/uninstall", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	return rec.Code
}

// TestIssue1753SharedInstallTokenTwoSites models the other common install
// pattern: the operator pastes one existing API token into both sites'
// install commands, so both agents authenticate with the same credential.
func TestIssue1753SharedInstallTokenTwoSites(t *testing.T) {
	const shared = "issue1753-shared-token.99990000"

	cfg := newTestConfigWithTokens(t, issue1753InstallTokenRecord(t, shared, "shared"))
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	defer monitor.Stop()
	router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "6.4.2")

	var ackIDA, ackIDB string
	for round := 0; round < 4; round++ {
		code, payload := issue1753PostReport(t, router, shared,
			issue1753Report("", "pve01", "machine-id-site-a", "10.1.0.10"))
		if code != http.StatusOK {
			t.Fatalf("round %d: site A report rejected with %d (%v)", round, code, payload)
		}
		ackIDA, _ = payload["agentId"].(string)

		code, payload = issue1753PostReport(t, router, shared,
			issue1753Report("", "pve01", "machine-id-site-b", "10.2.0.10"))
		if code != http.StatusOK {
			t.Fatalf("round %d: site B report rejected with %d (%v)", round, code, payload)
		}
		ackIDB, _ = payload["agentId"].(string)
	}

	if ackIDA == ackIDB {
		t.Fatalf("both sites sharing one token were acknowledged with the same agent identity %q", ackIDA)
	}

	// Site B uninstalls, then both keep going. The shared token must survive
	// the uninstall because site A still uses it.
	if code := issue1753UninstallAgent(t, router, shared, ackIDB); code != http.StatusOK {
		t.Fatalf("site B uninstall failed with %d", code)
	}
	code, payload := issue1753PostReport(t, router, shared,
		issue1753Report(ackIDA, "pve01", "machine-id-site-a", "10.1.0.10"))
	if code != http.StatusOK {
		t.Fatalf("site A report rejected with %d after shared-token uninstall (%v)", code, payload)
	}

	config.Mu.RLock()
	tokens := len(cfg.APITokens)
	config.Mu.RUnlock()
	if tokens != 1 {
		t.Fatalf("expected the shared token to survive site B's uninstall, inventory now holds %d tokens", tokens)
	}
}
