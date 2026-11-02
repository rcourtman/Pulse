package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// A stale router/handler reference can outlive a manager swap. It must report
// unavailability, not recreate the retired org's runtime or fall back to the
// default org. A separately owned replacement manager can still serve it.
func TestTenantMonitorRetirementFailsClosedAndReplacementRecovers(t *testing.T) {
	dir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(dir)
	if err := mtp.SaveOrganization(&models.Organization{ID: "tenant-a", DisplayName: "Tenant A"}); err != nil {
		t.Fatal(err)
	}
	persistence, err := mtp.GetPersistence("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveNodesConfig([]config.PVEInstance{{Name: "Tenant saved PVE", Host: "https://tenant.invalid:8006", Disabled: true}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir}
	retired := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(retired.Stop)
	var retiredInitializations atomic.Int32
	retired.SetMonitorInitializer(func(*monitoring.Monitor) { retiredInitializations.Add(1) })
	retired.Stop()

	router := &Router{mtMonitor: retired, monitor: &monitoring.Monitor{}}
	handlers := NewConfigHandlers(mtp, retired, nil, nil, nil, nil, false)
	guarded := router.tenantMonitorGuardMiddleware(http.HandlerFunc(handlers.HandleGetNodes))
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil)
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
		rec := httptest.NewRecorder()
		guarded.ServeHTTP(rec, req)
		return rec
	}
	rec := request()
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"tenant_unavailable"`) {
		t.Fatalf("retired manager returned %d: %s", rec.Code, rec.Body.String())
	}
	if retiredInitializations.Load() != 0 {
		t.Fatal("HTTP guard reinitialized the retired tenant")
	}
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-a")
	if cfg, persistence, monitor := handlers.ContextState(ctx); cfg != nil || persistence != nil || monitor != nil {
		t.Fatal("stale config handler exposed tenant or default state")
	}
	if router.getTenantMonitor(ctx) != nil {
		t.Fatal("stale router fell back to the default runtime")
	}

	replacement := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(replacement.Stop)
	// Rebind only these consumers, without altering production swap order.
	router.mtMonitor = replacement
	handlers.SetMultiTenantMonitor(replacement)
	rec = request()
	if rec.Code != http.StatusOK {
		t.Fatalf("replacement manager returned %d: %s", rec.Code, rec.Body.String())
	}
	var nodes []NodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &nodes); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "Tenant saved PVE" || nodes[0].Host != "https://tenant.invalid:8006" {
		t.Fatal("replacement did not recover the saved tenant inventory")
	}
	if retiredInitializations.Load() != 0 {
		t.Fatal("replacement read consulted the retired owner")
	}
}
