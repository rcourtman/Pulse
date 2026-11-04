package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Observe entry to the real propagation path without replacing its iterator,
// monitor, settings handler, persistence or notification-policy implementation.
type settingsPropagationObserver struct {
	*monitoring.MultiTenantMonitor
	entered chan struct{}
	once    sync.Once
}

func (o *settingsPropagationObserver) ForEachMonitor(fn func(*monitoring.Monitor)) {
	o.once.Do(func() { close(o.entered) })
	o.MultiTenantMonitor.ForEachMonitor(fn)
}

// A successful instance settings save cannot miss a tenant that has already
// copied its config and loaded the old notification policy but is not yet live.
func TestTenantInitializationSettingsSaveReachesAdmittedMonitor(t *testing.T) {
	dir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(dir)
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"live", "starting", "later"} {
		if err := mtp.SaveOrganization(&models.Organization{ID: id, DisplayName: id}); err != nil {
			t.Fatal(err)
		}
		p, err := mtp.GetPersistence(id)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.SaveNodesConfig([]config.PVEInstance{{Name: id + "-pve", Host: "https://" + id + ".invalid:8006", Disabled: true, VerifySSL: true, Fingerprint: id + "-pin"}}, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	initial := config.DefaultSystemSettings()
	initial.PBSPollingInterval, initial.PMGPollingInterval = 60, 60
	initial.WebhookAllowedPrivateCIDRs = "192.0.2.0/24"
	initial.PublicURL = "https://old.pulse.invalid"
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}
	const token = "settings-admission-test-token"
	cfg := &config.Config{
		DataPath: dir, ConfigPath: dir,
		PBSPollingInterval: time.Minute, PMGPollingInterval: time.Minute,
		PublicURL: initial.PublicURL,
		APITokens: []config.APITokenRecord{{ID: "settings-test", Name: "Settings test", Hash: auth.HashAPIToken(token)}},
	}
	manager := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(manager.Stop)
	live, err := manager.GetMonitor("live")
	if err != nil {
		t.Fatal(err)
	}
	router := &Router{config: cfg, persistence: persistence, mtMonitor: manager}
	entered := make(chan *monitoring.Monitor, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	manager.SetMonitorInitializer(func(m *monitoring.Monitor) {
		// In production this loads the instance-wide policy before publication.
		router.configureMonitorDependencies(m)
		if m.GetOrgID() == "starting" {
			entered <- m
			<-release
		}
	})
	var initErr error
	initialized := make(chan struct{})
	go func() { _, initErr = manager.GetMonitor("starting"); close(initialized) }()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		<-initialized
		if initErr != nil {
			t.Errorf("initialization: %v", initErr)
		}
	})
	var starting *monitoring.Monitor
	select {
	case starting = <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("initializer did not finish loading the old settings")
	}
	const target = "https://192.0.2.10/hook"
	if err := starting.GetNotificationManager().ValidateWebhookURL(target); err != nil {
		t.Fatalf("old private-network grant was not loaded: %v", err)
	}
	if got := starting.BasePollInterval(monitoring.InstanceTypePBS); got != time.Minute {
		t.Fatalf("old PBS interval = %v, want 1m", got)
	}
	if m, ok := manager.PeekMonitor("starting"); ok || m != nil {
		t.Fatal("unfinished constructor was published")
	}
	observer := &settingsPropagationObserver{MultiTenantMonitor: manager, entered: make(chan struct{})}
	handler := NewSystemSettingsHandler(cfg, persistence, nil, manager, live, router.ReloadSystemSettings, nil)
	handler.mtMonitor = observer
	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", strings.NewReader(`{"pbsPollingInterval":300,"pmgPollingInterval":420,"publicURL":"https://new.pulse.invalid","webhookAllowedPrivateCIDRs":""}`))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	saved := make(chan struct{})
	go func() { handler.HandleUpdateSystemSettings(rec, req); close(saved) }()
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); <-saved })
	select {
	case <-observer.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("settings request did not reach the real iterator")
	}
	// The held constructor makes the vulnerable ordering explicit. Time bounds
	// detect lack of progress; they do not schedule or simulate the race.
	select {
	case <-saved:
		t.Error("settings save completed while an admitted monitor still held the old policy")
	case <-time.After(250 * time.Millisecond):
	}

	// Waiting to propagate settings must not reacquire the live-reader lock.
	connections := NewConnectionsHandlers(
		func(context.Context) *config.Config { return live.GetConfig() },
		func(context.Context) *config.ConfigPersistence { return live.GetConfigPersistence() },
		router.getTenantMonitor,
	)
	guarded := router.tenantMonitorGuardMiddleware(http.HandlerFunc(connections.HandleList))
	readDone := make(chan struct{})
	var read *httptest.ResponseRecorder
	go func() {
		defer close(readDone)
		if m, err := manager.GetMonitor("live"); err != nil || m != live {
			t.Errorf("cached live lookup failed: %v", err)
		}
		if m, ok := manager.PeekMonitor("live"); !ok || m != live {
			t.Error("cached live peek failed")
		}
		read = httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
		guarded.ServeHTTP(read, r.WithContext(context.WithValue(r.Context(), OrgIDContextKey, "live")))
	}()
	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		releaseOnce.Do(func() { close(release) })
		<-readDone
		t.Fatal("settings propagation blocked the unrelated live tenant's reads")
	}
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), "https://live.invalid:8006") || strings.Contains(read.Body.String(), "starting.invalid") {
		t.Fatalf("live connections lost saved intent or isolation: %d %s", read.Code, read.Body.String())
	}
	t.Logf("SETTINGS_ADMISSION_LIVE_WIRE %s", read.Body.String())
	releaseOnce.Do(func() { close(release) })
	select {
	case <-saved:
	case <-time.After(10 * time.Second):
		t.Fatal("settings save did not finish after releasing the constructor")
	}
	<-initialized
	if initErr != nil {
		t.Fatal(initErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body.String())
	}
	if got, err := manager.GetMonitor("starting"); err != nil || got != starting {
		t.Fatalf("settings save replaced the admitted monitor: %v", err)
	}
	if starting.GetConfig().PBSPollingInterval != time.Minute {
		t.Fatal("test did not retain the pre-save detached configuration")
	}
	later, err := manager.GetMonitor("later")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []*monitoring.Monitor{live, starting, later} {
		if got := m.BasePollInterval(monitoring.InstanceTypePBS); got != 5*time.Minute {
			t.Errorf("%s: effective PBS interval = %v, want 5m", m.GetOrgID(), got)
		}
		if got := m.BasePollInterval(monitoring.InstanceTypePMG); got != 7*time.Minute {
			t.Errorf("%s: effective PMG interval = %v, want 7m", m.GetOrgID(), got)
		}
		nm := m.GetNotificationManager()
		if nm.ValidateWebhookURL(target) == nil {
			t.Errorf("%s: revoked private-network target remained permitted", m.GetOrgID())
		}
		if got := nm.GetPublicURL(); got != "https://new.pulse.invalid" {
			t.Errorf("%s: public URL = %q, want saved URL", m.GetOrgID(), got)
		}
	}
	persisted, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.WebhookAllowedPrivateCIDRs != "" || persisted.PBSPollingInterval != 300 || persisted.PMGPollingInterval != 420 || persisted.PublicURL != "https://new.pulse.invalid" {
		t.Fatalf("saved settings not preserved: %+v", persisted)
	}
}
