package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A helper's default fallback is not evidence that a tenant can read it. Pin
// the real auth -> tenant selection -> monitor guard -> anomalies route, with
// enabled Patrol and colliding baselines so an accidental fallback is visible.
func TestRoutedAnomaliesNilMonitorBoundary(t *testing.T) {
	t.Setenv("PULSE_AUTH_BYPASS", "false")
	rig := newTenantPatrolRig(t)
	// A known but ungranted org tests authorization, not the earlier
	// invalid-org guard for an unprovisioned selector.
	if err := rig.router.multiTenant.SaveOrganization(&models.Organization{ID: "other-org", DisplayName: "OTHER"}); err != nil {
		t.Fatal(err)
	}
	rig.server.Client().Timeout = 10 * time.Second
	manager := rig.router.mtMonitor
	defaultID := readStateNodeID(t, rig.defaultMonitor)
	tenantID := readStateNodeID(t, rig.tenantMonitor)
	// Reload legitimately replaces the service's in-memory baseline store with
	// the persisted org-owned store. Seed that real owner, with the normal
	// minimum sample count, instead of treating a temporary injected store as
	// saved state that must survive initialization.
	for _, svc := range []*ai.Service{rig.defaultService, rig.tenantService} {
		cfg := ai.DefaultBaselineConfig()
		cfg.DataDir = rig.router.persistenceForOrg(tenantContext(svc.GetOrgID())).DataDir()
		store := ai.NewBaselineStore(cfg)
		points := make([]ai.BaselineMetricPoint, cfg.MinSamples)
		for i := range points {
			points[i] = ai.BaselineMetricPoint{Value: 10, Timestamp: time.Now().Add(-time.Duration(i) * time.Minute)}
		}
		for _, id := range []string{defaultID, tenantID} {
			if err := store.Learn(id, "node", "cpu", points); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.Save(); err != nil {
			t.Fatal(err)
		}
		svc.SetBaselineStore(store)
	}
	t.Cleanup(func() {
		rig.server.Close()
		rig.router.ShutdownAIIntelligence()
		rig.router.ShutdownBackgroundWorkers()
		rig.router.ShutdownResourceStores()
		rig.router.ShutdownRBAC()
	})

	read := func(stage, org, token string, want int, expectedID string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, rig.server.URL+"/api/ai/intelligence/anomalies", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Pulse-Org-ID", org)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := rig.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != want {
			t.Fatalf("%s/%s: status %d, want %d: %s", stage, org, res.StatusCode, want, body)
		}
		if want == http.StatusOK {
			var payload struct {
				Anomalies []struct {
					ID   string `json:"resource_id"`
					Name string `json:"resource_name"`
				} `json:"anomalies"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Anomalies) == 0 {
				t.Fatalf("%s/%s: no positive anomaly witness: %s", stage, org, body)
			}
			for _, a := range payload.Anomalies {
				if a.ID != expectedID || a.Name != org+"-node" {
					t.Fatalf("%s/%s: wrong-org anomaly: %s", stage, org, body)
				}
			}
		} else {
			if strings.Contains(string(body), defaultID) || strings.Contains(string(body), tenantID) {
				t.Fatalf("%s/%s: denial disclosed resource evidence: %s", stage, org, body)
			}
			if want == http.StatusServiceUnavailable {
				var failure struct {
					Code string `json:"code"`
				}
				if json.Unmarshal(body, &failure) != nil || failure.Code != "tenant_unavailable" {
					t.Fatalf("%s: no explicit unavailable outcome: %s", stage, body)
				}
			}
		}
		wire, err := json.Marshal(map[string]any{"stage": stage, "org": org, "status": res.StatusCode, "content_type": res.Header.Get("Content-Type"), "body": string(body)})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("NIL_MONITOR_ROUTE_WIRE %s", wire)
	}

	// Control: the tenant's real anomaly route is admitted and useful, not an
	// empty/disabled-AI response, and ignores its colliding default baseline.
	read("available", "org-a", rig.token, http.StatusOK, tenantID)
	read("available", "default", rig.token, http.StatusOK, defaultID)
	read("anonymous", "default", "", http.StatusUnauthorized, "")
	read("unauthorised-org", "other-org", rig.token, http.StatusForbidden, "")

	// This is the supported reload setter, not a replacement route, fake auth
	// context or direct handler call. Cold-start nil ownership has the same
	// router/handler binding; the previously admitted tenant service stays
	// enabled with the same baselines while the manager is unavailable.
	rig.router.SetMultiTenantMonitor(nil)
	t.Logf("NIL_MONITOR_HELPER_HAS_FALLBACK %t", rig.router.aiSettingsHandler.readStateForOrg("org-a") != nil)
	read("nil-monitor", "org-a", rig.token, http.StatusServiceUnavailable, "")
	read("nil-monitor", "default", rig.token, http.StatusOK, defaultID)

	// Restoration reuses the saved organizations, exact token and same live
	// monitors; no wider grant, recreation or state deletion establishes recovery.
	rig.router.SetMultiTenantMonitor(manager)
	read("restored", "org-a", rig.token, http.StatusOK, tenantID)
	read("restored", "default", rig.token, http.StatusOK, defaultID)
}
