package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agentbinding"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// Docker analog of issue #1753: two standalone sites reuse one short hostname
// and one pasted unified install token. The host-agent side of this estate was
// fixed by machine-qualifying the host token bindings; these tests walk the
// same estate through /api/agents/docker/report and fail if the Docker records
// collapse into one identity, or if removing one Docker host revokes the
// shared token while a sibling record still authenticates with it - the same
// 401 "Unauthorized access attempt" chain the reporter hit on the host path.

func issue1753DockerInstallTokenRecord(t *testing.T, raw, site string) config.APITokenRecord {
	t.Helper()
	record, err := config.NewAPITokenRecord(raw, "Unified agent install ("+site+")", agenttokens.HostScopes(false))
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	record.Metadata = map[string]string{
		"install_type":                     "host",
		"issued_via":                       agentbinding.IssuedViaConfig,
		agenttokens.RuntimeRoleMetadataKey: agenttokens.CredentialKindMonitoringCollector,
	}
	return *record
}

func issue1753DockerReport(hostname, machineID string) agentsdocker.Report {
	return agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			// The unified agent's Docker module reports the machine ID as its
			// agent ID (the hostagent fallback chain, #985/#986).
			ID:              machineID,
			Version:         "6.4.2",
			Type:            "unified",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  hostname,
			MachineID: machineID,
			Runtime:   "docker",
		},
		Containers: []agentsdocker.Container{
			{ID: "container-" + machineID, Name: "app", State: "running"},
		},
		Timestamp: time.Now().UTC(),
	}
}

func issue1753PostDockerReport(t *testing.T, router *Router, token string, report agentsdocker.Report) (int, map[string]any) {
	t.Helper()
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal docker report: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/report", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	return rec.Code, payload
}

func issue1753TokenNames(cfg *config.Config) map[string]bool {
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	names := make(map[string]bool, len(cfg.APITokens))
	for _, record := range cfg.APITokens {
		names[record.Name] = true
	}
	return names
}

// TestIssue1753DockerSharedTokenTwoSitesSameShortHostname models the reporter's
// estate on the Docker module: both sites' unified agents report host and
// Docker inventory with the same short hostname and one shared install token.
//
// The Docker path deliberately requires a unique token per module, so the
// second site's Docker reports must converge to the documented, actionable
// rejection ("generate a new token") - never to a silent fold that leaves one
// record flip-flopping between machines. The first site's Docker identity must
// stay stable throughout, and removing the Docker record must not revoke the
// shared token the host records still authenticate with (the 401 chain from
// the issue).
func TestIssue1753DockerSharedTokenTwoSitesSameShortHostname(t *testing.T) {
	const shared = "issue1753-docker-shared.13571357"

	cfg := newTestConfigWithTokens(t, issue1753DockerInstallTokenRecord(t, shared, "shared"))
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	defer monitor.Stop()
	router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "6.4.2")

	// Alternate reports the way two live 30s-interval agents do. Site B's
	// very first Docker report may legitimately fold (a changed machine ID is
	// indistinguishable from a recreated container until the old machine
	// reports again), but from then on it must be rejected with the
	// unique-token guidance rather than adopted.
	var ackIDA string
	var siteBRejected bool
	for round := 0; round < 4; round++ {
		code, payload := issue1753PostReport(t, router, shared,
			issue1753Report("", "docker01", "machine-id-site-a", "10.1.0.10"))
		if code != http.StatusOK {
			t.Fatalf("round %d: site A host report rejected with %d (%v)", round, code, payload)
		}
		code, payload = issue1753PostReport(t, router, shared,
			issue1753Report("", "docker01", "machine-id-site-b", "10.2.0.10"))
		if code != http.StatusOK {
			t.Fatalf("round %d: site B host report rejected with %d (%v)", round, code, payload)
		}

		code, payload = issue1753PostDockerReport(t, router, shared,
			issue1753DockerReport("docker01", "machine-id-site-a"))
		if code != http.StatusOK {
			t.Fatalf("round %d: site A docker report rejected with %d (%v)", round, code, payload)
		}
		gotID, _ := payload["agentId"].(string)
		if gotID == "" {
			t.Fatalf("round %d: site A docker report acknowledged without an identity", round)
		}
		if ackIDA == "" {
			ackIDA = gotID
		} else if gotID != ackIDA {
			t.Fatalf("round %d: site A docker identity flapped from %q to %q", round, ackIDA, gotID)
		}

		code, payload = issue1753PostDockerReport(t, router, shared,
			issue1753DockerReport("docker01", "machine-id-site-b"))
		switch {
		case code == http.StatusOK && round == 0:
			// The one ambiguous cycle: nothing distinguishes site B's first
			// report from a recreated container yet.
		case code == http.StatusOK:
			t.Fatalf("round %d: site B's Docker module silently adopted an identity (%v) instead of the unique-token rejection; the record would flip-flop between machines", round, payload)
		default:
			message, _ := payload["error"].(string)
			if !strings.Contains(message, "unique API token") {
				t.Fatalf("round %d: site B docker report rejected with %d but without the unique-token guidance: %v", round, code, payload)
			}
			siteBRejected = true
		}
	}
	if !siteBRejected {
		t.Fatal("site B's Docker module was never steered to a unique token")
	}

	// Site A's record must have settled on site A's machine, not the last
	// reporter's: the flip-flop is what let a later removal act on the wrong
	// site in the original estate.
	dockerHost, ok := monitor.GetDockerHost(ackIDA)
	if !ok {
		t.Fatalf("site A docker host %q missing from state", ackIDA)
	}
	if dockerHost.MachineID != "machine-id-site-a" {
		t.Fatalf("site A docker record carries machine ID %q, want %q", dockerHost.MachineID, "machine-id-site-a")
	}

	// The reporter's removal step: the Docker record is removed while both
	// sites' host records still authenticate with the shared token. The token
	// must survive, and host reporting must keep working - its revocation is
	// the 401 "Unauthorized access attempt" chain from the issue.
	if _, err := monitor.RemoveDockerHost(ackIDA); err != nil {
		t.Fatalf("RemoveDockerHost(%q): %v", ackIDA, err)
	}

	code, payload := issue1753PostReport(t, router, shared,
		issue1753Report("", "docker01", "machine-id-site-a", "10.1.0.10"))
	if code != http.StatusOK {
		t.Fatalf("site A host report rejected with %d after Docker host removal (%v)", code, payload)
	}
	code, payload = issue1753PostReport(t, router, shared,
		issue1753Report("", "docker01", "machine-id-site-b", "10.2.0.10"))
	if code != http.StatusOK {
		t.Fatalf("site B host report rejected with %d after Docker host removal (%v)", code, payload)
	}

	if names := issue1753TokenNames(cfg); !names["Unified agent install (shared)"] {
		t.Fatalf("shared install token was revoked while both host records still use it (have %v)", names)
	}
}

// TestIssue1753DockerHostRemovalKeepsUnifiedSiblingToken covers the same
// revocation chain without any hostname collision: a single unified install
// shares one token between the machine's host record and its Docker record.
// Removing only the Docker host (stop monitoring Docker, keep the host) must
// not revoke the token the host module still reports with.
func TestIssue1753DockerHostRemovalKeepsUnifiedSiblingToken(t *testing.T) {
	const token = "issue1753-unified-single.24682468"

	cfg := newTestConfigWithTokens(t, issue1753DockerInstallTokenRecord(t, token, "single"))
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	defer monitor.Stop()
	router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "6.4.2")

	code, payload := issue1753PostReport(t, router, token,
		issue1753Report("", "nas01", "machine-id-nas01", "10.3.0.10"))
	if code != http.StatusOK {
		t.Fatalf("host report rejected with %d (%v)", code, payload)
	}

	code, payload = issue1753PostDockerReport(t, router, token,
		issue1753DockerReport("nas01", "machine-id-nas01"))
	if code != http.StatusOK {
		t.Fatalf("docker report rejected with %d (%v)", code, payload)
	}
	dockerID, _ := payload["agentId"].(string)
	if dockerID == "" {
		t.Fatal("expected an acknowledged Docker host identity")
	}

	if _, err := monitor.RemoveDockerHost(dockerID); err != nil {
		t.Fatalf("RemoveDockerHost(%q): %v", dockerID, err)
	}

	code, payload = issue1753PostReport(t, router, token,
		issue1753Report("", "nas01", "machine-id-nas01", "10.3.0.10"))
	if code != http.StatusOK {
		t.Fatalf("host report rejected with %d after Docker host removal (%v); the shared unified token must survive", code, payload)
	}

	if names := issue1753TokenNames(cfg); !names["Unified agent install (single)"] {
		t.Fatalf("unified install token was revoked while the host record still uses it (have %v)", names)
	}
}
