package monitoring

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestLoadTenantRuntimeConfigUsesScopedResourcesAndPreservesInstanceSettings(t *testing.T) {
	p := config.NewConfigPersistence(t.TempDir())
	settings := config.DefaultSystemSettings()
	settings.PublicURL = "https://tenant.invalid"
	settings.AllowedOrigins = "https://tenant-ui.invalid"
	settings.PVEPollingInterval, settings.PBSPollingInterval = 57, 137
	settings.HideLocalLogin = false
	settings.DisableDockerUpdateActions = true
	settings.DiscoveryConfig.MaxConcurrent = 3
	settings.DiscoveryConfig.SubnetAllowlist = []string{"192.0.2.0/24"}
	if err := p.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	if err := p.SaveNodesConfig([]config.PVEInstance{{Name: "tenant-node", TokenValue: "synthetic-tenant-secret", Disabled: true}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := p.SaveAPITokens([]config.APITokenRecord{{ID: "tenant-token", Hash: "synthetic-tenant-hash", OrgID: "tenant-a", Scopes: []string{config.ScopeSettingsRead}}}); err != nil {
		t.Fatal(err)
	}
	// A tenant .env is not process configuration and must never be loaded.
	t.Setenv("PULSE_TENANT_IMPORT_SENTINEL", "process")
	if err := os.WriteFile(filepath.Join(p.GetConfigDir(), ".env"), []byte("PULSE_TENANT_IMPORT_SENTINEL=tenant\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := &config.Config{
		DataPath: t.TempDir(), ConfigPath: t.TempDir(), MetricsDBPath: "synthetic-default-metrics-path",
		AuthUser: "instance-admin", AuthPass: "synthetic-instance-auth", ProxyAuthSecret: "synthetic-instance-proxy",
		TLSCertFile: "instance-cert", TLSKeyFile: "instance-key", FrontendPort: 7655,
		APITokens:    []config.APITokenRecord{{ID: "default-token", Hash: "synthetic-default-hash"}},
		PVEInstances: []config.PVEInstance{{Name: "default-node", TokenValue: "synthetic-default-secret"}},
		PublicURL:    "https://default.invalid", PVEPollingInterval: 42 * time.Second, PBSPollingInterval: 71 * time.Second, AllowedOrigins: "https://instance-ui.invalid",
		HideLocalLogin: true, Discovery: config.DefaultDiscoveryConfig(),
		EnvOverrides: map[string]bool{"PVE_POLLING_INTERVAL": true, "PULSE_AUTH_HIDE_LOCAL_LOGIN": true, "discoveryMaxConcurrent": true},
	}
	base.Discovery.MaxConcurrent = 11
	before := base.DeepCopy()
	cfg, err := loadTenantRuntimeConfig(base, p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataPath != p.GetConfigDir() || cfg.ConfigPath != p.GetConfigDir() || cfg.MetricsDBPath != "" {
		t.Error("tenant paths inherited a default-org storage location")
	}
	if len(cfg.PVEInstances) != 1 || cfg.PVEInstances[0].Name != "tenant-node" || cfg.PVEInstances[0].TokenValue != "synthetic-tenant-secret" {
		t.Error("tenant nodes/credentials did not replace inherited resources")
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != "tenant-token" || cfg.APIToken != "synthetic-tenant-hash" {
		t.Error("tenant API tokens did not replace inherited tokens and legacy alias")
	}
	if cfg.PublicURL != base.PublicURL || cfg.AllowedOrigins != base.AllowedOrigins || cfg.PBSPollingInterval != base.PBSPollingInterval || cfg.DisableDockerUpdateActions != base.DisableDockerUpdateActions {
		t.Error("tenant archive settings overrode instance-wide runtime policy")
	}
	if cfg.PVEPollingInterval != 42*time.Second || !cfg.HideLocalLogin || cfg.Discovery.MaxConcurrent != 11 {
		t.Error("tenant settings bypassed effective process environment locks")
	}
	if !reflect.DeepEqual(cfg.Discovery, base.Discovery) {
		t.Error("tenant archive settings overrode instance-wide discovery policy")
	}
	if cfg.AuthUser != base.AuthUser || cfg.AuthPass != base.AuthPass || cfg.ProxyAuthSecret != base.ProxyAuthSecret || cfg.TLSCertFile != base.TLSCertFile || cfg.TLSKeyFile != base.TLSKeyFile || cfg.FrontendPort != base.FrontendPort {
		t.Error("tenant loader changed process identity or listener settings")
	}
	if os.Getenv("PULSE_TENANT_IMPORT_SENTINEL") != "process" {
		t.Error("tenant loader changed the process environment")
	}
	if !reflect.DeepEqual(base.DeepCopy(), before) {
		t.Error("tenant loader mutated the default config")
	}
	cfg.EnvOverrides["synthetic-test-lock"] = true
	cfg.PVEInstances[0].TokenValue = "changed"
	if base.EnvOverrides["synthetic-test-lock"] || base.PVEInstances[0].TokenValue != "synthetic-default-secret" {
		t.Error("tenant config aliases default mutable state")
	}
}

func TestLoadTenantRuntimeConfigEmptyInventoryDoesNotInheritCredentials(t *testing.T) {
	p := config.NewConfigPersistence(t.TempDir())
	base := &config.Config{PVEInstances: []config.PVEInstance{{Name: "default"}}, APITokens: []config.APITokenRecord{{ID: "default", Hash: "synthetic-hash"}}, APIToken: "synthetic-hash"}
	cfg, err := loadTenantRuntimeConfig(base, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PVEInstances) != 0 || len(cfg.APITokens) != 0 || cfg.APIToken != "" {
		t.Error("empty tenant inherited another organization's resources or token alias")
	}
	if err := os.WriteFile(filepath.Join(p.GetConfigDir(), "system.json"), []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTenantRuntimeConfig(base, p); err != nil {
		t.Errorf("tenant-local system record incorrectly became instance policy: %v", err)
	}
	if _, err := loadTenantRuntimeConfig(nil, p); err == nil {
		t.Error("missing base config was accepted")
	}
	if _, err := loadTenantRuntimeConfig(base, nil); err == nil {
		t.Error("missing persistence was accepted")
	}
	if err := os.WriteFile(filepath.Join(p.GetConfigDir(), "api_tokens.json"), []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTenantRuntimeConfig(base, p); err == nil {
		t.Error("unreadable scoped credentials fell back to another token inventory")
	}
}

func TestReloadTenantRejectsDefaultAndRetainsIncompleteStopGuard(t *testing.T) {
	mtm := NewMultiTenantMonitor(&config.Config{}, nil, nil)
	// No real monitor or storage is started: control only the owned loop's
	// completion signal to verify the existing incomplete-stop contract.
	defer mtm.globalCancel()
	for _, orgID := range []string{"", "default", " default "} {
		if _, err := mtm.ReloadTenant(orgID); err == nil {
			t.Errorf("reload accepted default-org selector %q", orgID)
		}
	}
	previous := tenantMonitorShutdownTimeout
	tenantMonitorShutdownTimeout = time.Millisecond
	t.Cleanup(func() { tenantMonitorShutdownTimeout = previous })
	mtm.tenantDone["tenant-a"] = make(chan struct{})
	mtm.tenantCancel["tenant-a"] = func() {}
	if _, err := mtm.ReloadTenant("tenant-a"); err == nil {
		t.Error("incomplete tenant stop reported a successful reload")
	}
	if _, guarded := mtm.tenantDeleting["tenant-a"]; !guarded {
		t.Error("incomplete stop released its tenant availability guard")
	}
	if _, err := mtm.GetMonitor("tenant-a"); err == nil {
		t.Error("incomplete stop admitted a replacement runtime")
	}
	select {
	case <-mtm.globalCtx.Done():
		t.Error("tenant reload cancelled the shared monitoring context")
	default:
	}
}
