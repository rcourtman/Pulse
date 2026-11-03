package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func monitorWithNode(t *testing.T, nodeID string, cpu float64) *monitoring.Monitor {
	t.Helper()
	monitor, _, _ := newTestMonitor(t)
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{Nodes: []models.Node{{ID: nodeID, Name: nodeID, Status: "online", CPU: cpu}}})
	setUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))
	return monitor
}

func readStateNodeID(t *testing.T, monitor *monitoring.Monitor) string {
	t.Helper()
	nodes := monitor.GetUnifiedReadState().Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected one node, got %d", len(nodes))
	}
	return nodes[0].ID()
}

func patrolServiceWithFindings(t *testing.T, orgID string, resourceIDs ...string) *ai.Service {
	t.Helper()
	svc := ai.NewService(nil, nil)
	svc.SetOrgID(orgID)
	svc.SetStateProvider(&MockStateProvider{})
	setUnexportedField(t, svc, "cfg", &config.AIConfig{Enabled: true})
	setUnexportedField(t, svc, "provider", &stubProvider{})
	for _, resourceID := range resourceIDs {
		svc.GetPatrolService().GetFindings().Add(&ai.Finding{
			ID:         orgID + "-" + resourceID,
			ResourceID: resourceID,
			Title:      orgID + " finding",
			Severity:   ai.FindingSeverityWarning,
			DetectedAt: time.Now(),
			LastSeenAt: time.Now(),
		})
	}
	return svc
}

type tenantPatrolRig struct {
	router         *Router
	server         *httptest.Server
	token          string
	defaultMonitor *monitoring.Monitor
	tenantMonitor  *monitoring.Monitor
	defaultService *ai.Service
	tenantService  *ai.Service
}

// newTenantPatrolRig serves a real router for two organizations, each with
// its own monitor and Patrol, behind the real auth and tenant middleware.
func newTenantPatrolRig(t *testing.T) tenantPatrolRig {
	t.Helper()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(false) })
	t.Setenv("PULSE_DEV", "true")

	const rawToken = "patrol-tenant-scope-token-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "two-org-token", []string{config.ScopeMonitoringRead, config.ScopeAIExecute})
	if err != nil {
		t.Fatalf("create api token record: %v", err)
	}
	record.OrgIDs = []string{"default", "org-a"}

	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, APITokens: []config.APITokenRecord{*record}}
	cfg.SortAPITokens()
	if err := config.NewMultiTenantPersistence(dataDir).SaveOrganization(&models.Organization{ID: "org-a", DisplayName: "ORG-A"}); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	router := newMultiTenantRouter(t, cfg)
	defaultMonitor := monitorWithNode(t, "default-node", 0.9)
	tenantMonitor := monitorWithNode(t, "org-a-node", 0.9)
	router.monitor = defaultMonitor
	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"default": defaultMonitor, "org-a": tenantMonitor})
	router.mtMonitor = mtm
	router.aiSettingsHandler.SetMultiTenantMonitor(mtm)
	router.aiSettingsHandler.SetReadState(defaultMonitor.GetUnifiedReadState())
	router.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(mtm, defaultMonitor))

	defaultService := patrolServiceWithFindings(t, "default", "default-1", "default-2")
	tenantService := patrolServiceWithFindings(t, "org-a", "org-a-1")
	router.aiSettingsHandler.stateMu.Lock()
	router.aiSettingsHandler.defaultAIService = defaultService
	router.aiSettingsHandler.stateMu.Unlock()
	router.aiSettingsHandler.aiServicesMu.Lock()
	router.aiSettingsHandler.aiServices["org-a"] = tenantService
	router.aiSettingsHandler.aiServicesMu.Unlock()

	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)
	return tenantPatrolRig{
		router:         router,
		server:         server,
		token:          rawToken,
		defaultMonitor: defaultMonitor,
		tenantMonitor:  tenantMonitor,
		defaultService: defaultService,
		tenantService:  tenantService,
	}
}

func (rig tenantPatrolRig) get(t *testing.T, path, orgID string, out any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rig.server.URL+path, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("X-Pulse-Org-ID", orgID)
	req.Header.Set("Authorization", "Bearer "+rig.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("org %s %s: request failed: %v", orgID, path, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("org %s %s: status %d", orgID, path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		t.Fatalf("org %s %s: decode: %v", orgID, path, err)
	}
}

// Agent Patrol-control status counts only the request organization's
// findings, whichever organization's Patrol was wired last.
func TestRoutedAgentPatrolStatusCountsRequestOrganizationFindings(t *testing.T) {
	rig := newTenantPatrolRig(t)

	for orgID, want := range map[string]int{"default": 2, "org-a": 1} {
		var status AgentOperationsLoopStatus
		rig.get(t, agentcapabilities.PatrolControlStatusCapabilityPath, orgID, &status)
		if status.ActiveFindingCount != want {
			t.Fatalf("org %s: expected %d active findings, got %d", orgID, want, status.ActiveFindingCount)
		}
	}
}

func TestPatrolFindingNotificationManagerNeverFallsBackToDefault(t *testing.T) {
	defaultMonitor, _, _ := newTestMonitor(t)
	tenantMonitor, _, _ := newTestMonitor(t)
	defaultManager := notifications.NewNotificationManagerWithDataDir("", t.TempDir())
	tenantManager := notifications.NewNotificationManagerWithDataDir("", t.TempDir())
	t.Cleanup(defaultManager.Stop)
	t.Cleanup(tenantManager.Stop)
	setUnexportedField(t, defaultMonitor, "notificationMgr", defaultManager)
	setUnexportedField(t, tenantMonitor, "notificationMgr", tenantManager)
	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"default": defaultMonitor, "org-a": tenantMonitor})
	router := &Router{monitor: defaultMonitor, mtMonitor: mtm}

	if got := router.patrolFindingNotificationManager("org-a"); got != tenantManager {
		t.Fatalf("expected org-a findings to use org-a's notification manager")
	}
	if got := router.patrolFindingNotificationManager("default"); got != defaultManager {
		t.Fatalf("expected default findings to use the default notification manager")
	}
	if got := (&Router{monitor: defaultMonitor}).patrolFindingNotificationManager("org-a"); got != nil {
		t.Fatalf("expected no notification manager for an unresolved organization")
	}
}

func TestLearnBaselinesUsesOrganizationInventory(t *testing.T) {
	defaultMonitor, defaultState, defaultHistory := newTestMonitor(t)
	tenantMonitor, tenantState, tenantHistory := newTestMonitor(t)
	defaultState.Nodes = []models.Node{{ID: "default-node", Name: "default-node"}}
	tenantState.Nodes = []models.Node{{ID: "org-a-node", Name: "org-a-node"}}
	syncTestResourceStore(t, defaultMonitor, defaultState)
	syncTestResourceStore(t, tenantMonitor, tenantState)
	now := time.Now()
	// The tenant's history also holds the default node's ID, and only the
	// tenant's history holds its own node: learning from the default inventory
	// or from the default history both show up below.
	defaultHistory.AddNodeMetric("default-node", "cpu", 0.5, now)
	tenantHistory.AddNodeMetric("default-node", "cpu", 0.5, now)
	tenantHistory.AddNodeMetric("org-a-node", "cpu", 0.5, now)
	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"default": defaultMonitor, "org-a": tenantMonitor})
	router := &Router{monitor: defaultMonitor, mtMonitor: mtm}

	store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})
	router.learnBaselines(store, router.resolveMonitorForOrg("org-a"))
	if _, ok := store.GetBaseline("org-a-node", "cpu"); !ok {
		t.Fatalf("expected org-a to learn its own node")
	}
	if _, ok := store.GetBaseline("default-node", "cpu"); ok {
		t.Fatalf("org-a learned a default-organization resource")
	}
}

func TestDeployCandidatesReadRequestOrganizationMonitor(t *testing.T) {
	defaultMonitor := monitorWithNode(t, "default-node", 0.1)
	handler := &DeployHandlers{monitor: defaultMonitor}
	handler.SetMonitorResolver(func(ctx context.Context) *monitoring.Monitor {
		if GetOrgID(ctx) == "default" {
			return defaultMonitor
		}
		return nil
	})

	rr := httptest.NewRecorder()
	handler.HandleCandidates(rr, withOrg(httptest.NewRequest(http.MethodGet, "/api/clusters/c1/agent-deploy/candidates", nil), "org-a"))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected an unresolved organization not to read the default cluster, got %d", rr.Code)
	}
}

// A tenant's Patrol notifies through its own channels: it gets the finding
// notification callback but never the instance relay's mobile push.
func TestStartPatrolForContextKeepsTenantFindingsOffInstanceRelay(t *testing.T) {
	setMockModeForTest(t, true)

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	defaultMonitor, _, _ := newTestMonitor(t)
	tenantMonitor, _, _ := newTestMonitor(t)
	tenantMonitor.SetResourceStore(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)))
	// Finding callbacks are wired only once the unified store exists, which
	// needs the organization's alert manager.
	alertManager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(alertManager.Stop)
	setUnexportedField(t, tenantMonitor, "alertManager", alertManager)
	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"default": defaultMonitor, "tenant-1": tenantMonitor})

	router := &Router{
		monitor:                 defaultMonitor,
		mtMonitor:               mtm,
		multiTenant:             mtp,
		aiSettingsHandler:       NewAISettingsHandler(mtp, mtm, nil),
		aiHandler:               &AIHandler{},
		startedPatrolOrgs:       make(map[string]bool),
		monitorResourceAdapters: make(map[string]*unifiedresources.MonitorAdapter),
	}
	router.aiSettingsHandler.SetStateProvider(&stubStateProvider{})
	defer router.ShutdownAIIntelligence()

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	if ok := router.startPatrolForContext(ctx, "tenant-1"); !ok {
		t.Fatal("expected tenant-1 patrol start to succeed")
	}
	patrol := reflect.ValueOf(router.aiSettingsHandler.GetAIService(ctx).GetPatrolService()).Elem()
	if patrol.FieldByName("findingNotifyCallback").IsNil() {
		t.Fatal("expected tenant-1 findings to be wired to notifications")
	}
	if !patrol.FieldByName("pushNotifyCallback").IsNil() {
		t.Fatal("tenant-1 findings must not be pushed through the instance relay")
	}
}
