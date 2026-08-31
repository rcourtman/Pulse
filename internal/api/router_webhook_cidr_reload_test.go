package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// TestReloadSystemSettings_AppliesWebhookCIDRsToNewMonitor verifies the fix
// for issue #1507: after a monitor reload (which recreates the notification
// manager), ReloadSystemSettings must re-apply the persisted webhook private
// CIDR allowlist to the new notification manager. Without this, webhooks to
// private IPs fail after any auto-registration-triggered monitor reload.
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

	// Simulate a monitor reload: create a fresh monitor with a brand-new
	// notification manager (empty CIDR allowlist).
	monitor2, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New (reload): %v", err)
	}
	t.Cleanup(func() { monitor2.Stop() })

	// Before the fix: the reload path in server.go never called
	// ReloadSystemSettings, so the new notification manager would have
	// an empty allowlist and reject private IPs.
	nm2 := monitor2.GetNotificationManager()
	if nm2 == nil {
		t.Fatal("reloaded monitor has no notification manager")
	}
	if err := nm2.ValidateWebhookURL("http://192.168.1.50/hook"); err == nil {
		t.Fatal("fresh notification manager should reject private IP before settings are re-applied")
	}

	// Now update the router's monitor reference (as SetMonitor does during reload).
	router.monitor = monitor2

	// Call ReloadSystemSettings — this is the fix.
	router.ReloadSystemSettings()

	// The new notification manager should now have the persisted CIDR allowlist.
	if err := nm2.ValidateWebhookURL("http://192.168.1.50/hook"); err != nil {
		t.Fatalf("after ReloadSystemSettings: expected CIDR to allow 192.168.1.50, got: %v", err)
	}
	if err := nm2.ValidateWebhookURL("http://10.5.5.5/hook"); err != nil {
		t.Fatalf("after ReloadSystemSettings: expected CIDR to allow 10.5.5.5, got: %v", err)
	}
	// IPs outside the allowlist should still be rejected.
	if err := nm2.ValidateWebhookURL("http://172.16.0.1/hook"); err == nil {
		t.Fatal("after ReloadSystemSettings: expected 172.16.0.1 to be rejected (not in allowlist)")
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
