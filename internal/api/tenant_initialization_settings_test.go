package api

import (
	"context"
	"fmt"
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
	router := &Router{config: cfg, persistence: persistence, mtMonitor: manager, mux: http.NewServeMux()}
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
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "live"))
	rec := httptest.NewRecorder()
	saved := make(chan struct{})
	go func() { handler.HandleUpdateSystemSettings(rec, req); close(saved) }()
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); <-saved })
	select {
	case <-observer.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("settings request did not reach the real iterator")
	}
	// The save must complete while initialization is held. Router settings
	// reload also visits every monitor while holding its settings-cache lock;
	// waiting for construction there would block unrelated HTTP requests.
	select {
	case <-saved:
	case <-time.After(2 * time.Second):
		t.Fatal("instance settings propagation waited for another tenant's initializer")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body.String())
	}

	// Recording deferred settings must not reacquire the live-reader lock.
	connections := NewConnectionsHandlers(
		func(context.Context) *config.Config { return live.GetConfig() },
		func(context.Context) *config.ConfigPersistence { return live.GetConfigPersistence() },
		router.getTenantMonitor,
	)
	guarded := router.tenantMonitorGuardMiddleware(http.HandlerFunc(connections.HandleList))
	router.mux.Handle("/api/connections", guarded)
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
		r.Header.Set("X-API-Token", token)
		router.ServeHTTP(read, r.WithContext(context.WithValue(r.Context(), OrgIDContextKey, "live")))
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

// An operator-state visitor is independent of settings saves. It must not read
// the mutable base config while the real handler writes it, or turn a partial
// update into an admitted tenant's runtime snapshot. Concurrent partial saves
// must also preserve each other's saved fields.
func TestTenantInitializationConcurrentSettingsPublication(t *testing.T) {
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
	}
	initial := config.DefaultSystemSettings()
	initial.WebhookAllowedPrivateCIDRs = "192.0.2.0/24"
	initial.PublicURL = "https://old.pulse.invalid"
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}
	const token = "concurrent-settings-test-token"
	cfg := &config.Config{DataPath: dir, ConfigPath: dir,
		PublicURL: initial.PublicURL, PBSPollingInterval: time.Minute, PMGPollingInterval: time.Minute,
		APITokens: []config.APITokenRecord{{ID: "concurrent-test", Hash: auth.HashAPIToken(token)}},
	}
	manager := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(manager.Stop)
	live, err := manager.GetMonitor("live")
	if err != nil {
		t.Fatal(err)
	}
	entered, release, initialized := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	manager.SetMonitorInitializer(func(m *monitoring.Monitor) {
		if m.GetOrgID() == "starting" {
			close(entered)
			<-release
		}
	})
	var starting *monitoring.Monitor
	var initErr error
	go func() { starting, initErr = manager.GetMonitor("starting"); close(initialized) }()
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); <-initialized })
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("constructor did not reach held phase")
	}
	handler := NewSystemSettingsHandler(cfg, persistence, nil, manager, live, nil, nil)
	save := func(body string) {
		req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", strings.NewReader(body))
		req.Header.Set("X-API-Token", token)
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "live"))
		rec := httptest.NewRecorder()
		handler.HandleUpdateSystemSettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("save = %d %s", rec.Code, rec.Body.String())
		}
	}
	// Start an unrelated synchronous iterator first, and keep it active until
	// every actual config mutation completes. It never reads config itself.
	stopVisits, visitsDone, visitsStarted := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(visitsDone)
		close(visitsStarted)
		for {
			select {
			case <-stopVisits:
				return
			default:
			}
			manager.ForEachMonitor(func(m *monitoring.Monitor) {
				if m != live {
					t.Error("unrelated visitor observed an unfinished monitor")
				}
			})
		}
	}()
	<-visitsStarted
	var writers sync.WaitGroup
	writers.Add(2)
	go func() {
		defer writers.Done()
		for i := 0; i < 16; i++ {
			save(fmt.Sprintf(`{"pbsPollingInterval":%d,"publicURL":"https://saved-%d.pulse.invalid"}`, 300+i, i))
		}
	}()
	go func() {
		defer writers.Done()
		for i := 0; i < 16; i++ {
			save(fmt.Sprintf(`{"pmgPollingInterval":%d,"backupPollingInterval":%d,"backupPollingEnabled":true,"webhookAllowedPrivateCIDRs":""}`, 420+i, 600+i))
		}
	}()
	writersDone := make(chan struct{})
	go func() { writers.Wait(); close(writersDone) }()
	select {
	case <-writersDone:
	case <-time.After(20 * time.Second):
		// Joining still matters on failure; releasing the fixture breaks an old
		// constructor wait but does not excuse a blocked settings handler.
		releaseOnce.Do(func() { close(release) })
		t.Error("settings saves waited for a held constructor")
		<-writersDone
	}
	close(stopVisits)
	<-visitsDone
	persisted, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.PBSPollingInterval != 315 || persisted.PMGPollingInterval != 435 ||
		persisted.BackupPollingInterval != 615 || persisted.PublicURL != "https://saved-15.pulse.invalid" ||
		persisted.WebhookAllowedPrivateCIDRs != "" {
		t.Errorf("concurrent partial saves lost committed intent: %+v", persisted)
	}
	releaseOnce.Do(func() { close(release) })
	<-initialized
	if initErr != nil {
		t.Fatal(initErr)
	}
	later, err := manager.GetMonitor("later")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []*monitoring.Monitor{live, starting, later} {
		if got := m.BasePollInterval(monitoring.InstanceTypePBS); got != 315*time.Second {
			t.Errorf("%s: PBS = %v, want 315s", m.GetOrgID(), got)
		}
		if got := m.BasePollInterval(monitoring.InstanceTypePMG); got != 435*time.Second {
			t.Errorf("%s: PMG = %v, want 435s", m.GetOrgID(), got)
		}
		if got := m.GetNotificationManager().GetPublicURL(); got != "https://saved-15.pulse.invalid" {
			t.Errorf("%s: public URL = %q", m.GetOrgID(), got)
		}
		if m.GetNotificationManager().ValidateWebhookURL("https://192.0.2.10/hook") == nil {
			t.Errorf("%s: revoked grant survived settings publication", m.GetOrgID())
		}
	}
	t.Log("CONCURRENT_SETTINGS: 32 actual partial saves, independent visitor, live/starting/later runtime and persisted readback")
}
