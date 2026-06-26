package api

import (
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
