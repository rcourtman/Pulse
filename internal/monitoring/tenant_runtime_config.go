package monitoring

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// loadTenantRuntimeConfig deliberately does not call config.Load: that loader
// owns the process environment, logger and default-org persistence. A tenant
// starts (including after import) from its own persisted resources,
// retaining the effective instance settings and environment locks in base.
func loadTenantRuntimeConfig(base *config.Config, p *config.ConfigPersistence) (*config.Config, error) {
	if base == nil || p == nil {
		return nil, fmt.Errorf("tenant configuration is unavailable")
	}
	cfg := base.DeepCopy()
	cfg.DataPath, cfg.ConfigPath = p.GetConfigDir(), p.GetConfigDir()
	// A process-level override names one database file, not a tenant template.
	cfg.MetricsDBPath = ""
	cfg.PVEInstances, cfg.PBSInstances, cfg.PMGInstances = nil, nil, nil
	cfg.APITokens, cfg.APIToken = nil, ""
	nodes, err := p.LoadNodesConfig()
	if err != nil {
		return nil, fmt.Errorf("load tenant nodes: %w", err)
	}
	if nodes != nil {
		cfg.PVEInstances, cfg.PBSInstances, cfg.PMGInstances = nodes.PVEInstances, nodes.PBSInstances, nodes.PMGInstances
	}
	tokens, err := p.LoadAPITokens()
	if err != nil {
		return nil, fmt.Errorf("load tenant API tokens: %w", err)
	}
	cfg.APITokens = tokens
	cfg.SortAPITokens()
	// System settings remain instance-wide: their existing API updates the
	// base config and all live tenant monitors. Importing a tenant archive
	// must not invent per-tenant overrides or re-read process .env/logging.

	return cfg, nil
}

// ReloadTenant rebuilds only the selected organization's runtime from its own
// persisted configuration. Never use the instance-wide reload for this path.
// removeTenant retains its availability guard when shutdown is incomplete.
func (mtm *MultiTenantMonitor) ReloadTenant(orgID string) (*Monitor, error) {
	orgID = strings.TrimSpace(orgID)
	if mtm == nil || orgID == "" || orgID == "default" {
		return nil, fmt.Errorf("tenant reload requires a non-default organization")
	}
	if err := mtm.removeTenant(orgID, false); err != nil {
		return nil, err
	}
	return mtm.GetMonitor(orgID)
}
