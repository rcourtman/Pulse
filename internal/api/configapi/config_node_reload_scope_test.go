package configapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// A node change in one organization must recreate only that organization's
// monitor. Every node is disabled and every credential synthetic: this test
// must not contact a monitored workload.
func TestNodeChangeReloadsOnlySelectedTenant(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	mtp := config.NewMultiTenantPersistence(dataDir)
	orgIDs := []string{"default", "tenant-a", "tenant-b"}
	for _, orgID := range orgIDs {
		if err := mtp.SaveOrganization(&models.Organization{ID: orgID, DisplayName: orgID}); err != nil {
			t.Fatal(err)
		}
		p, err := mtp.GetPersistence(orgID)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.SaveNodesConfig([]config.PVEInstance{
			{Name: "remove-me", Host: "https://removed-" + orgID + ".invalid:8006", TokenName: "synthetic@pve!monitor", TokenValue: "synthetic-fixture-token", VerifySSL: true, Disabled: true},
			{Name: "keep-me", Host: "https://kept-" + orgID + ".invalid:8006", TokenName: "synthetic@pve!monitor", TokenValue: "synthetic-fixture-token", VerifySSL: true, Disabled: true},
		}, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.LoadWithoutLoggingInit()
	if err != nil {
		t.Fatal(err)
	}
	mtm := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(mtm.Stop)
	monitors := make(map[string]*monitoring.Monitor)
	for _, orgID := range orgIDs {
		if monitors[orgID], err = mtm.GetMonitor(orgID); err != nil {
			t.Fatal(err)
		}
	}
	reloads := 0
	h := NewConfigHandlers(mtp, mtm, func() error { reloads++; return nil }, nil, nil, func() {}, false)
	t.Cleanup(h.Stop)

	removeNode := func(orgID string) {
		t.Helper()
		ctx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, orgID)
		rec := httptest.NewRecorder()
		h.HandleDeleteNode(rec, httptest.NewRequest(http.MethodDelete, "/api/config/nodes/pve:remove-me", nil).WithContext(ctx))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s node removal status = %d: %s", orgID, rec.Code, rec.Body.String())
		}
	}

	removeNode("tenant-a")
	if reloads != 0 {
		t.Errorf("tenant node change invoked instance-wide reload %d times", reloads)
	}
	for _, orgID := range []string{"default", "tenant-b"} {
		current, getErr := mtm.GetMonitor(orgID)
		if getErr != nil || current != monitors[orgID] {
			t.Errorf("non-target %s monitor replaced or unavailable", orgID)
		}
		if nodes := current.GetConfig().PVEInstances; len(nodes) != 2 {
			t.Errorf("non-target %s runtime has %d nodes, want 2", orgID, len(nodes))
		}
	}
	after, err := mtm.GetMonitor("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if after == monitors["tenant-a"] {
		t.Error("selected tenant runtime was not recreated")
	}
	if nodes := after.GetConfig().PVEInstances; len(nodes) != 1 || nodes[0].Name != "keep-me" {
		t.Error("selected tenant runtime did not apply its saved node change")
	}

	// The default organization retains its existing instance-wide reload.
	removeNode("default")
	if reloads != 1 {
		t.Errorf("default node change invoked instance-wide reload %d times, want 1", reloads)
	}
}
