package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const snapshotAuthorityToken = "snapshot-authority-reader.12345678"
const snapshotAuthorityControl = "snapshot-authority-control.12345678"
const snapshotAuthorityCommand = "synthetic-private-snapshot-command"

var snapshotAuthorityPaths = []string{
	"/api/agent/resource-context/vm:101",
	"/api/agent/resource-capabilities/vm:101",
	"/api/agent/fleet-context",
	"/api/agent/patrol-control/status",
	"/api/agent/operations-loop/status",
}

// Pause the normal inventory provider after route admission. Both admitted
// readers reach this boundary; no auth stub, database lock or runtime test
// hook is needed. Subsequent reads use the same retained inventory normally.
type snapshotAuthorityProvider struct {
	resourceUnifiedSeedProvider
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	calls   atomic.Int32
}

func (p *snapshotAuthorityProvider) UnifiedResourceSnapshot() ([]unified.Resource, time.Time) {
	if p.calls.Add(1) <= 2 {
		p.entered <- struct{}{}
		<-p.release
	}
	return p.resourceUnifiedSeedProvider.UnifiedResourceSnapshot()
}

func (p *snapshotAuthorityProvider) unblock() { p.once.Do(func() { close(p.release) }) }

type snapshotAuthorityApprovals struct{ staticApprovalsProvider }

func (p snapshotAuthorityApprovals) PendingApprovalsForResource(resourceID, orgID string) []AgentResourceApprovalSummary {
	// Match the production adapter's detached per-request projection. A read's
	// redaction must not mutate the fixture evidence used by another reader.
	return append([]AgentResourceApprovalSummary(nil), p.staticApprovalsProvider.PendingApprovalsForResource(resourceID, orgID)...)
}

type snapshotAuthorityFixture struct {
	router   *Router
	server   *httptest.Server
	provider *snapshotAuthorityProvider
	path     string
}

func newSnapshotAuthorityFixture(t *testing.T, path string, scopes []string) *snapshotAuthorityFixture {
	t.Helper()
	setMockModeForTest(t, false)
	t.Setenv("PULSE_AUTH_BYPASS", "false")
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, snapshotAuthorityToken, scopes, nil),
		newTokenRecord(t, snapshotAuthorityControl, []string{config.ScopeWildcard}, nil),
	)
	cfg.AuthUser, cfg.AuthPass = "operator", "fixture-nonempty-hash"
	cfg.ProxyAuthSecret, cfg.ProxyAuthUserHeader = "snapshot-proxy-fixture", "X-Remote-User"
	InitSessionStore(cfg.DataPath)
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownResourceStores)
	now := time.Now().UTC()
	p := &snapshotAuthorityProvider{
		resourceUnifiedSeedProvider: resourceUnifiedSeedProvider{
			snapshot: models.StateSnapshot{LastUpdate: now},
			resources: []unified.Resource{{
				ID: "vm:101", Type: unified.ResourceTypeVM, Name: "synthetic-database",
				Status: unified.StatusOnline, LastSeen: now, UpdatedAt: now,
				Capabilities: []unified.ResourceCapability{{Name: "synthetic_restart"}},
			}},
		},
		entered: make(chan struct{}, 2), release: make(chan struct{}),
	}
	router.resourceHandlers.SetStateProvider(p)
	router.agentContextHandler.SetFindingsProvider(staticFindingsProvider{byResource: map[string][]AgentResourceFindingSnapshot{
		"vm:101": {{ID: "synthetic-finding", Title: "Synthetic pressure", Severity: "warning"}},
	}})
	router.agentContextHandler.SetApprovalsProvider(snapshotAuthorityApprovals{staticApprovalsProvider{byResource: map[string][]AgentResourceApprovalSummary{
		"vm:101": {{ID: "synthetic-approval", Command: snapshotAuthorityCommand, RequestedAt: now, ExpiresAt: now.Add(time.Hour)}},
	}}})
	f := &snapshotAuthorityFixture{router: router, provider: p, path: path}
	f.server = httptest.NewServer(router.Handler())
	t.Cleanup(f.server.Close)
	t.Cleanup(p.unblock)
	return f
}

func (f *snapshotAuthorityFixture) request(t *testing.T, raw, transport string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.server.URL+f.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	switch transport {
	case "header":
		req.Header.Set("X-API-Token", raw)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	case "proxy":
		req.Header.Set("X-Proxy-Secret", f.router.config.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", "operator")
	default:
		t.Fatalf("unknown snapshot transport %q", transport)
	}
	return req
}

func (f *snapshotAuthorityFixture) begin(t *testing.T, raw, transport string) <-chan historyAuthorityResult {
	t.Helper()
	req := f.request(t, raw, transport)
	result := make(chan historyAuthorityResult, 1)
	go func() { response, err := f.server.Client().Do(req); result <- historyAuthorityResult{response, err} }()
	select {
	case <-f.provider.entered:
	case got := <-result:
		if got.response != nil {
			got.response.Body.Close()
		}
		t.Fatalf("snapshot returned before inventory admission: %v", got.err)
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot did not reach admitted inventory read")
	}
	return result
}

type snapshotAuthorityWire struct {
	Status int         `json:"status"`
	Header http.Header `json:"header"`
	Body   string      `json:"body"`
}

func readSnapshotAuthorityResult(t *testing.T, result <-chan historyAuthorityResult, status int) snapshotAuthorityWire {
	t.Helper()
	return readSnapshotHTTPResult(t, result, status, true)
}

func readSnapshotHTTPResult(t *testing.T, result <-chan historyAuthorityResult, status int, withdrawn bool) snapshotAuthorityWire {
	t.Helper()
	select {
	case got := <-result:
		if got.err != nil {
			t.Fatal(got.err)
		}
		defer got.response.Body.Close()
		body, err := io.ReadAll(got.response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got.response.StatusCode != status {
			t.Errorf("snapshot status = %d, want %d: %s", got.response.StatusCode, status, body)
		}
		if withdrawn && status == http.StatusUnauthorized && (string(body) != "Authentication required\n" || strings.Contains(got.response.Header.Get("Content-Type"), "json")) {
			t.Error("withdrawn snapshot disclosed data or implied successful empty JSON")
		}
		if status == http.StatusOK && (!json.Valid(body) || got.response.Header.Get("Content-Type") != "application/json") {
			t.Error("current snapshot lost its JSON transport")
		}
		return snapshotAuthorityWire{got.response.StatusCode, got.response.Header, string(body)}
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot did not finish after inventory release")
	}
	return snapshotAuthorityWire{}
}

func (f *snapshotAuthorityFixture) current(t *testing.T, raw, transport string, status int) snapshotAuthorityWire {
	t.Helper()
	response, err := f.server.Client().Do(f.request(t, raw, transport))
	result := make(chan historyAuthorityResult, 1)
	result <- historyAuthorityResult{response, err}
	// Initial authentication errors retain their existing transport; only a
	// withdrawn admitted response is required to be the generic guard denial.
	return readSnapshotHTTPResult(t, result, status, false)
}

func (f *snapshotAuthorityFixture) checkCurrent(t *testing.T, wire snapshotAuthorityWire, private bool) {
	t.Helper()
	switch {
	case strings.Contains(f.path, "/resource-context/"):
		var bundle AgentResourceContext
		if json.Unmarshal([]byte(wire.Body), &bundle) != nil || bundle.CanonicalID != "vm:101" || bundle.ResourceName != "synthetic-database" || len(bundle.ActiveFindings) != 1 || len(bundle.PendingApprovals) != 1 {
			t.Fatalf("current reader lost resource context: %s", wire.Body)
		}
		if strings.Contains(wire.Body, snapshotAuthorityCommand) != private || bundle.PendingApprovals[0].CommandRedacted == private {
			t.Error("resource context changed admitted command visibility")
		}
	case strings.Contains(f.path, "/resource-capabilities/"):
		var bundle AgentResourceCapabilities
		if json.Unmarshal([]byte(wire.Body), &bundle) != nil || bundle.ResourceID != "vm:101" || len(bundle.Capabilities) != 1 || bundle.Capabilities[0].Name != "synthetic_restart" {
			t.Fatalf("current reader lost capabilities: %s", wire.Body)
		}
	case strings.HasSuffix(f.path, "/fleet-context"):
		var bundle AgentFleetContext
		if json.Unmarshal([]byte(wire.Body), &bundle) != nil || len(bundle.Resources) != 1 || bundle.Resources[0].ResourceName != "synthetic-database" || bundle.Resources[0].PendingApprovalCount != 1 {
			t.Fatalf("current reader lost fleet context: %s", wire.Body)
		}
	default:
		var bundle AgentOperationsLoopStatus
		if json.Unmarshal([]byte(wire.Body), &bundle) != nil || bundle.PendingApprovalCount != 1 || bundle.ActiveFindingCount != 1 || bundle.GeneratedAt.IsZero() {
			t.Fatalf("current reader lost Patrol status: %s", wire.Body)
		}
	}
}

func (f *snapshotAuthorityFixture) changeToken(t *testing.T, change string) {
	t.Helper()
	if change == "revoked" {
		id := f.router.config.APITokens[0].ID
		req, err := http.NewRequest(http.MethodDelete, f.server.URL+"/api/security/tokens/"+id, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-API-Token", snapshotAuthorityControl)
		response, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("token DELETE = %d", response.StatusCode)
		}
		tokens, err := f.router.persistence.LoadAPITokens()
		if err != nil || len(tokens) != 1 || tokens[0].ID == id {
			t.Fatalf("withdrawal was not persisted: %v", err)
		}
		return
	}
	config.Mu.Lock()
	defer config.Mu.Unlock()
	record := &f.router.config.APITokens[0]
	switch change {
	case "expired":
		past := time.Now().Add(-time.Second)
		record.ExpiresAt = &past
	case "scope":
		record.Scopes = []string{config.ScopeSettingsRead}
	case "command_scope":
		record.Scopes = []string{config.ScopeMonitoringRead}
	case "organization":
		record.OrgIDs = []string{"other-org"}
	case "credential":
		record.ID = "replacement-snapshot-credential"
	case "principal":
		record.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "replacement-snapshot-principal"}
	}
}

func TestAgentSnapshotsWithdrawnTokenCannotDisclose(t *testing.T) {
	for _, path := range snapshotAuthorityPaths {
		for _, transport := range []string{"header", "bearer"} {
			for _, change := range []string{"revoked", "expired", "scope", "organization", "credential", "principal"} {
				t.Run(path+"/"+transport+"/"+change, func(t *testing.T) {
					f := newSnapshotAuthorityFixture(t, path, []string{config.ScopeMonitoringRead, config.ScopeAIExecute})
					control := f.begin(t, snapshotAuthorityControl, "header")
					primary := f.begin(t, snapshotAuthorityToken, transport)
					f.changeToken(t, change)
					f.provider.unblock()
					readSnapshotAuthorityResult(t, primary, http.StatusUnauthorized)
					f.checkCurrent(t, readSnapshotAuthorityResult(t, control, http.StatusOK), true)
					if change == "credential" || change == "principal" {
						f.checkCurrent(t, f.current(t, snapshotAuthorityToken, transport, http.StatusOK), true)
					} else {
						response, err := f.server.Client().Do(f.request(t, snapshotAuthorityToken, transport))
						if err != nil {
							t.Fatal(err)
						}
						response.Body.Close()
						if response.StatusCode == http.StatusOK {
							t.Error("fresh snapshot reused withdrawn authority")
						}
					}
				})
			}
		}
	}
}

func TestAgentSnapshotsWithdrawnSessionCannotDisclose(t *testing.T) {
	for _, path := range snapshotAuthorityPaths {
		for _, change := range []string{"logout", "expired", "replaced", "principal", "store"} {
			t.Run(path+"/"+change, func(t *testing.T) {
				f := newSnapshotAuthorityFixture(t, path, []string{config.ScopeMonitoringRead})
				const raw = "synthetic-snapshot-session"
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
				t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
				control := f.begin(t, snapshotAuthorityControl, "header")
				primary := f.begin(t, raw, "session")
				if change == "logout" {
					logoutReadSession(t, f.server.URL, raw)
				} else if change == "store" {
					InitSessionStore(t.TempDir())
				} else {
					mutateReadSession(raw, change)
				}
				f.provider.unblock()
				readSnapshotAuthorityResult(t, primary, http.StatusUnauthorized)
				f.checkCurrent(t, readSnapshotAuthorityResult(t, control, http.StatusOK), true)
			})
		}
	}
}

func TestAgentSnapshotsCommandWithdrawalAndCurrentRedaction(t *testing.T) {
	for _, transport := range []string{"header", "bearer"} {
		t.Run(transport, func(t *testing.T) {
			f := newSnapshotAuthorityFixture(t, snapshotAuthorityPaths[0], []string{config.ScopeMonitoringRead, config.ScopeAIExecute})
			control := f.begin(t, snapshotAuthorityControl, "header")
			primary := f.begin(t, snapshotAuthorityToken, transport)
			f.changeToken(t, "command_scope")
			f.provider.unblock()
			readSnapshotAuthorityResult(t, primary, http.StatusUnauthorized)
			f.checkCurrent(t, readSnapshotAuthorityResult(t, control, http.StatusOK), true)
			f.checkCurrent(t, f.current(t, snapshotAuthorityToken, transport, http.StatusOK), false)
		})
	}
}

func TestAgentSnapshotsWithdrawalWithholdsMissingResourceError(t *testing.T) {
	for _, kind := range []string{"resource-context", "resource-capabilities"} {
		t.Run(kind, func(t *testing.T) {
			f := newSnapshotAuthorityFixture(t, "/api/agent/"+kind+"/missing", []string{config.ScopeMonitoringRead})
			primary := f.begin(t, snapshotAuthorityToken, "header")
			f.changeToken(t, "revoked")
			f.provider.unblock()
			readSnapshotAuthorityResult(t, primary, http.StatusUnauthorized)
			f.current(t, snapshotAuthorityControl, "header", http.StatusNotFound)
		})
	}
}

func TestAgentSnapshotsKeepAdmissionAndRecovery(t *testing.T) {
	for _, path := range snapshotAuthorityPaths {
		t.Run(path, func(t *testing.T) {
			f := newSnapshotAuthorityFixture(t, path, []string{config.ScopeSettingsRead})
			f.provider.unblock()
			f.current(t, "", "header", http.StatusUnauthorized)
			f.current(t, snapshotAuthorityToken, "header", http.StatusForbidden)
			f.checkCurrent(t, f.current(t, snapshotAuthorityControl, "bearer", http.StatusOK), true)
			config.Mu.Lock()
			f.router.config.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
			config.Mu.Unlock()
			f.checkCurrent(t, f.current(t, snapshotAuthorityToken, "header", http.StatusOK), false)
			f.checkCurrent(t, f.current(t, "", "proxy", http.StatusOK), true)
		})
	}
}
