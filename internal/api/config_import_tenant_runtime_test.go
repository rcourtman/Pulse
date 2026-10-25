package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Exercise a complete valid archive through the consuming router, not only the
// authorization denial or persistence transaction. Every credential is synthetic
// and every node is disabled: this test must not contact a monitored workload.
func TestConfigImportAppliesOnlySelectedTenant(t *testing.T) {
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(false) })
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_HOSTED_MODE", "false")
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	mtp := config.NewMultiTenantPersistence(dataDir)
	persistences := make(map[string]*config.ConfigPersistence)
	for _, orgID := range []string{"default", "tenant-a", "tenant-b"} {
		if err := mtp.SaveOrganization(&models.Organization{ID: orgID, DisplayName: orgID, OwnerUserID: "import-owner"}); err != nil {
			t.Fatal(err)
		}
		p, err := mtp.GetPersistence(orgID)
		if err != nil {
			t.Fatal(err)
		}
		persistences[orgID] = p
		seedConfigImportRuntime(t, p, orgID, orgID, 71)
	}
	cfg, err := config.LoadWithoutLoggingInit()
	if err != nil {
		t.Fatal(err)
	}
	// Local instance identity is outside the archive and must remain intact.
	cfg.AuthUser, cfg.AuthPass = "instance-admin", "synthetic-instance-password-hash"
	cfg.ProxyAuthSecret = "synthetic-instance-proxy-secret"
	mtm := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(mtm.Stop)
	monitors := make(map[string]*monitoring.Monitor)
	for _, orgID := range []string{"default", "tenant-a", "tenant-b"} {
		monitors[orgID], err = mtm.GetMonitor(orgID)
		if err != nil {
			t.Fatal(err)
		}
	}
	reloads := 0
	router := NewRouter(cfg, monitors["default"], mtm, nil, func() error {
		reloads++
		return nil
	}, "test")
	cleanupTestRouter(t, router)
	beforeDefault := cfg.DeepCopy()
	beforeOther := monitors["tenant-b"].GetConfig().DeepCopy()
	filesBefore := configImportFileDigests(t, persistences["default"], persistences["tenant-b"])
	for _, orgID := range []string{"default", "tenant-a", "tenant-b"} {
		logConfigImportSettings(t, router, orgID, "before")
	}

	source := config.NewConfigPersistence(t.TempDir())
	seedConfigImportRuntime(t, source, "restored", "tenant-a", 137)
	archive, err := source.ExportConfig("synthetic-import-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(ImportConfigRequest{Data: archive, Passphrase: "synthetic-import-passphrase"})
	if err != nil {
		t.Fatal(err)
	}
	const session = "synthetic-tenant-import-owner-session"
	GetSessionStore().CreateSession(session, time.Hour, "browser", "127.0.0.1", "import-owner")
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	req.Header.Set("X-CSRF-Token", generateCSRFToken(session))
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid tenant import status = %d: %s", rec.Code, rec.Body.String())
	}
	if reloads != 0 {
		t.Errorf("tenant import invoked instance-wide reload %d times", reloads)
	}
	after, err := mtm.GetMonitor("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	afterCfg := after.GetConfig()
	if len(afterCfg.PVEInstances) != 1 || afterCfg.PVEInstances[0].Name != "restored" || afterCfg.PVEInstances[0].TokenValue != "synthetic-restored-node-secret" {
		t.Error("selected tenant runtime did not apply only its imported nodes/credentials")
	}
	if afterCfg.PublicURL != "https://restored.invalid" || afterCfg.PBSPollingInterval != 137*time.Second {
		t.Errorf("selected tenant runtime settings = URL %q, PBS %s; want imported values", afterCfg.PublicURL, afterCfg.PBSPollingInterval)
	}
	if afterCfg.DataPath != persistences["tenant-a"].GetConfigDir() || afterCfg.ConfigPath != persistences["tenant-a"].GetConfigDir() {
		t.Error("selected tenant runtime adopted a non-target storage path")
	}
	if len(afterCfg.APITokens) != 1 || afterCfg.APITokens[0].ID != "restored-token" || afterCfg.APITokens[0].OrgID != "tenant-a" {
		t.Error("selected tenant runtime did not load only its imported token inventory")
	}
	for _, orgID := range []string{"default", "tenant-b"} {
		current, getErr := mtm.GetMonitor(orgID)
		if getErr != nil || current != monitors[orgID] {
			t.Errorf("non-target %s monitor replaced or unavailable", orgID)
		}
	}
	if !reflect.DeepEqual(cfg, beforeDefault) || !reflect.DeepEqual(monitors["tenant-b"].GetConfig(), beforeOther) {
		t.Error("non-target runtime configuration or credentials changed")
	}
	if !reflect.DeepEqual(configImportFileDigests(t, persistences["default"], persistences["tenant-b"]), filesBefore) {
		t.Error("non-target persisted settings/credentials changed")
	}
	persistedNodes, err := persistences["tenant-a"].LoadNodesConfig()
	if err != nil || len(persistedNodes.PVEInstances) != 1 || persistedNodes.PVEInstances[0].Name != "restored" {
		t.Error("selected tenant archive was not persisted")
	}
	// Imported credentials must be immediately usable in the selected scope,
	// not only present in a struct or saved for the next process restart.
	for _, check := range []struct {
		orgID, token string
		allowed      bool
	}{
		{"tenant-a", "synthetic-restored-api-token.12345678", true},
		{"tenant-a", "synthetic-tenant-a-api-token.12345678", false},
		{"tenant-b", "synthetic-restored-api-token.12345678", false},
		{"default", "synthetic-restored-api-token.12345678", false},
		{"tenant-b", "synthetic-tenant-b-api-token.12345678", true},
		{"default", "synthetic-default-api-token.12345678", true},
	} {
		r := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
		r.Header.Set("X-Pulse-Org-ID", check.orgID)
		r.Header.Set("X-API-Token", check.token)
		response := httptest.NewRecorder()
		router.Handler().ServeHTTP(response, r)
		if (response.Code == http.StatusOK) != check.allowed {
			t.Errorf("post-import token access for %s = %d; allowed=%v", check.orgID, response.Code, check.allowed)
		}
	}
	for _, orgID := range []string{"default", "tenant-a", "tenant-b"} {
		logConfigImportSettings(t, router, orgID, "after")
	}
	// These are the same producers the existing settings page consumes.
	ctx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, "tenant-a")
	settingsRec := httptest.NewRecorder()
	router.configHandlers.HandleGetSystemSettings(settingsRec, httptest.NewRequest(http.MethodGet, "/api/system/settings", nil).WithContext(ctx))
	var settings config.SystemSettings
	if err := json.Unmarshal(settingsRec.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.PublicURL != "https://restored.invalid" || settings.PBSPollingInterval != 137 {
		t.Error("consuming settings handler did not read back the imported tenant values")
	}
}

func seedConfigImportRuntime(t *testing.T, p *config.ConfigPersistence, name, orgID string, interval int) {
	t.Helper()
	if err := p.SaveNodesConfig([]config.PVEInstance{{Name: name, Host: "https://" + name + ".invalid:8006", TokenName: "synthetic@pve!monitor", TokenValue: "synthetic-" + name + "-node-secret", VerifySSL: true, Disabled: true}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultSystemSettings()
	settings.PublicURL, settings.PBSPollingInterval = "https://"+name+".invalid", interval
	if err := p.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	token := newTokenRecord(t, "synthetic-"+name+"-api-token.12345678", []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	token.ID, token.OrgID = name+"-token", orgID
	if err := p.SaveAPITokens([]config.APITokenRecord{token}); err != nil {
		t.Fatal(err)
	}
}

func configImportFileDigests(t *testing.T, persistences ...*config.ConfigPersistence) map[string][32]byte {
	t.Helper()
	digests := make(map[string][32]byte)
	for _, p := range persistences {
		for _, name := range []string{"nodes.enc", "system.json", "api_tokens.json", "org.json", ".encryption.key"} {
			path := filepath.Join(p.GetConfigDir(), name)
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			digests[path] = sha256.Sum256(data)
		}
	}
	return digests
}

// These readbacks contain only synthetic public settings, never archive/token
// material. Verbose regression logs make runtime-versus-persistence drift clear.
func logConfigImportSettings(t *testing.T, router *Router, orgID, phase string) {
	t.Helper()
	ctx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, orgID)
	rec := httptest.NewRecorder()
	router.configHandlers.HandleGetSystemSettings(rec, httptest.NewRequest(http.MethodGet, "/api/system/settings", nil).WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s settings readback failed: %d", orgID, rec.Code)
	}
	t.Logf("SYNTHETIC_SETTINGS_READBACK %s %s %s", phase, orgID, bytes.TrimSpace(rec.Body.Bytes()))
}
