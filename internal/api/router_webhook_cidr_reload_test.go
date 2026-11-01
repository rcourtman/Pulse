package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// A recreated monitor restores its bootstrap policy before any router exists.
// ReloadSystemSettings still owns later persisted changes, including revoking
// a previously allowed private destination (the live replacement path of #1507).
func TestReloadSystemSettings_AppliesWebhookCIDRsToNewMonitor(t *testing.T) {
	tempDir := t.TempDir()

	persistence := config.NewConfigPersistence(tempDir)

	// Persist system settings with a webhook CIDR allowlist.
	settings := config.DefaultSystemSettings()
	settings.WebhookAllowedPrivateCIDRs = "192.168.1.0/24,10.0.0.0/8"
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatalf("SaveSystemSettings: %v", err)
	}

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	// Create the first monitor — simulates the initial monitor.
	monitor1, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New (initial): %v", err)
	}
	t.Cleanup(func() { monitor1.Stop() })

	// Create a minimal Router with the persistence and monitor set.
	// reloadSystemSettings only reads r.persistence, r.monitor, r.mtMonitor,
	// and r.config — the other fields are nil-safe.
	router := &Router{
		config:      cfg,
		persistence: persistence,
		monitor:     monitor1,
	}

	// First call applies CIDRs to the initial monitor.
	router.ReloadSystemSettings()

	nm1 := monitor1.GetNotificationManager()
	if nm1 == nil {
		t.Fatal("initial monitor has no notification manager")
	}
	if err := nm1.ValidateWebhookURL("http://192.168.1.50/hook"); err != nil {
		t.Fatalf("initial monitor: expected CIDR to allow 192.168.1.50, got: %v", err)
	}

	// Recreated managers must already have saved policy before queued work runs.
	monitor2, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New (reload): %v", err)
	}
	t.Cleanup(func() { monitor2.Stop() })

	nm2 := monitor2.GetNotificationManager()
	if nm2 == nil {
		t.Fatal("reloaded monitor has no notification manager")
	}
	if err := nm2.ValidateWebhookURL("http://192.168.1.50/hook"); err != nil {
		t.Fatalf("fresh manager must restore policy before router refresh: %v", err)
	}

	// Persist a real policy replacement after construction. Saving alone must
	// not mutate the live manager; refreshing must replace, not merge, grants.
	settings.WebhookAllowedPrivateCIDRs = "172.16.0.0/16"
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatalf("SaveSystemSettings replacement: %v", err)
	}
	if err := nm2.ValidateWebhookURL("http://192.168.1.50/hook"); err != nil {
		t.Fatalf("persisting replacement unexpectedly changed live policy: %v", err)
	}
	if err := nm2.ValidateWebhookURL("http://172.16.0.1/hook"); err == nil {
		t.Fatal("replacement grant reached the live manager before refresh")
	}
	router.monitor = monitor2
	router.ReloadSystemSettings()
	if err := nm2.ValidateWebhookURL("http://172.16.0.1/hook"); err != nil {
		t.Fatalf("after refresh: replacement destination rejected: %v", err)
	}
	for _, revoked := range []string{"http://192.168.1.50/hook", "http://10.5.5.5/hook"} {
		if err := nm2.ValidateWebhookURL(revoked); err == nil {
			t.Fatalf("after refresh: old grant retained for %s", revoked)
		}
	}
	settings.WebhookAllowedPrivateCIDRs = ""
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatalf("SaveSystemSettings revocation: %v", err)
	}
	router.ReloadSystemSettings()
	if err := nm2.ValidateWebhookURL("http://172.16.0.1/hook"); err == nil {
		t.Fatal("clearing the installation policy did not revoke the final grant")
	}
}

// captureRouterSettingsLogs uses the synchronized process-wide test sink so
// tests can assert on warnings emitted by system-settings load failures.
func captureRouterSettingsLogs(t *testing.T) *lockedLogBuffer {
	t.Helper()
	return captureTestLogs(t)
}

// unreadableSystemSettings returns a ConfigPersistence whose LoadSystemSettings
// fails: a directory named system.json produces a read error that is not
// os.IsNotExist — the same shape as a transient read failure.
func unreadableSystemSettings(t *testing.T) *config.ConfigPersistence {
	t.Helper()

	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "system.json"), 0o700); err != nil {
		t.Fatalf("mkdir system.json: %v", err)
	}
	return config.NewConfigPersistence(dataDir)
}

// A failed settings read while wiring a tenant monitor means the webhook
// private CIDR allowlist silently never applies and private webhook targets
// fail SSRF validation with no org-side fix — the failure must be logged.
func TestConfigureMonitorDependencies_WarnsWhenSettingsLoadFails(t *testing.T) {
	logOutput := captureRouterSettingsLogs(t)

	router := &Router{persistence: unreadableSystemSettings(t)}
	router.configureMonitorDependencies(&monitoring.Monitor{})

	if !strings.Contains(logOutput.String(), "Failed to load system settings for tenant monitor") {
		t.Fatalf("expected settings-load failure warning, got logs: %s", logOutput.String())
	}
}

// A missing system.json is a normal fresh install, not a failure — the tenant
// monitor wiring must stay quiet so fresh setups don't log spurious warnings.
func TestConfigureMonitorDependencies_NoWarningWhenSettingsFileMissing(t *testing.T) {
	logOutput := captureRouterSettingsLogs(t)

	router := &Router{persistence: config.NewConfigPersistence(t.TempDir())}
	router.configureMonitorDependencies(&monitoring.Monitor{})

	if strings.Contains(logOutput.String(), "Failed to load system settings") {
		t.Fatalf("missing system.json must not warn, got logs: %s", logOutput.String())
	}
}

// reloadSystemSettings deliberately fails closed (embedding off) on a settings
// read error, but the error must be observable: without a log line a
// persistent read failure looks identical to embedding being switched off on
// purpose.
func TestReloadSystemSettings_WarnsWhenLoadFailsAndFailsClosed(t *testing.T) {
	logOutput := captureRouterSettingsLogs(t)

	router := &Router{
		config:      &config.Config{},
		persistence: unreadableSystemSettings(t),
	}
	router.cachedAllowEmbedding = true
	router.cachedAllowedOrigins = "https://example.com"

	router.ReloadSystemSettings()

	if router.cachedAllowEmbedding || router.cachedAllowedOrigins != "" {
		t.Fatal("a failed settings read must fail closed (embedding off, origins cleared)")
	}
	if !strings.Contains(logOutput.String(), "Failed to load system settings during reload") {
		t.Fatalf("expected settings-load failure warning, got logs: %s", logOutput.String())
	}
}
