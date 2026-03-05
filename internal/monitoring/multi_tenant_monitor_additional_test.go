package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestMultiTenantMonitorRemoveTenant(t *testing.T) {
	monitor := &Monitor{}
	mtm := &MultiTenantMonitor{
		monitors: map[string]*Monitor{
			"org-1": monitor,
		},
	}

	mtm.RemoveTenant("org-1")
	if _, ok := mtm.monitors["org-1"]; ok {
		t.Fatalf("expected org-1 to be removed")
	}

	// Ensure removal of missing orgs is a no-op.
	mtm.RemoveTenant("missing")
}

func TestMultiTenantMonitorLoadsTenantAPITokens(t *testing.T) {
	baseDir := t.TempDir()
	baseCfg := &config.Config{
		DataPath:   baseDir,
		ConfigPath: baseDir,
	}

	globalRecord, err := config.NewAPITokenRecord("global-token-123.12345678", "global", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("new global token: %v", err)
	}
	baseCfg.APITokens = []config.APITokenRecord{*globalRecord}
	baseCfg.SortAPITokens()

	mtp := config.NewMultiTenantPersistence(baseDir)
	tenantPersistence, err := mtp.GetPersistence("org-1")
	if err != nil {
		t.Fatalf("get tenant persistence: %v", err)
	}

	tenantRecord, err := config.NewAPITokenRecord("tenant-token-123.12345678", "tenant", []string{config.ScopeHostReport})
	if err != nil {
		t.Fatalf("new tenant token: %v", err)
	}
	tenantRecord.OrgID = "org-1"
	if err := tenantPersistence.SaveAPITokens([]config.APITokenRecord{*tenantRecord}); err != nil {
		t.Fatalf("save tenant tokens: %v", err)
	}

	mtm := NewMultiTenantMonitor(baseCfg, mtp, nil)
	t.Cleanup(mtm.Stop)

	monitor, err := mtm.GetMonitor("org-1")
	if err != nil {
		t.Fatalf("get tenant monitor: %v", err)
	}

	cfg := monitor.GetConfig()
	if cfg == nil {
		t.Fatalf("expected tenant config")
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("expected 2 merged api tokens, got %d", len(cfg.APITokens))
	}
	if !cfg.IsValidAPIToken("global-token-123.12345678") {
		t.Fatalf("expected global token to remain valid")
	}
	if !cfg.IsValidAPIToken("tenant-token-123.12345678") {
		t.Fatalf("expected tenant token to be loaded")
	}
}
