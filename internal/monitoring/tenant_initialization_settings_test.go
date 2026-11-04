package monitoring

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A policy read or validation failure during reconciliation must not publish a
// constructor carrying old grants, leak its lifetime, or require deleting data.
func TestTenantInitializationRejectsUnreadableChangedPolicy(t *testing.T) {
	for name, invalid := range map[string]string{
		"unreadable-json": `{`,
		"invalid-policy":  `{"webhookAllowedPrivateCIDRs":"not-a-cidr"}`,
	} {
		t.Run(name, func(t *testing.T) {
			mtp, p := newTestTenantPersistence(t)
			if err := mtp.SaveOrganization(&models.Organization{ID: "starting", DisplayName: "Starting"}); err != nil {
				t.Fatal(err)
			}
			tenant, err := mtp.GetPersistence("starting")
			if err != nil {
				t.Fatal(err)
			}
			if err := tenant.SaveNodesConfig([]config.PVEInstance{{Name: "saved", Host: "https://saved.invalid:8006", Disabled: true, VerifySSL: true, Fingerprint: "saved-pin"}}, nil, nil); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(tenant.GetConfigDir(), "nodes.enc")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			initial := config.DefaultSystemSettings()
			initial.WebhookAllowedPrivateCIDRs = "192.0.2.0/24"
			if err := p.SaveSystemSettings(*initial); err != nil {
				t.Fatal(err)
			}
			manager := NewMultiTenantMonitor(&config.Config{DataPath: p.GetConfigDir()}, mtp, nil)
			t.Cleanup(manager.Stop)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			manager.SetMonitorInitializer(func(*Monitor) { close(entered); <-release })
			var got *Monitor
			var initErr error
			done := make(chan struct{})
			go func() { got, initErr = manager.GetMonitor("starting"); close(done) }()
			t.Cleanup(func() { once.Do(func() { close(release) }); <-done })
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				t.Fatal("constructor did not finish loading its old policy")
			}
			// This is a corrupt fixture, not a settings API success. The visitor
			// marks an instance-settings change while the old policy is loaded.
			if err := os.WriteFile(filepath.Join(p.GetConfigDir(), "system.json"), []byte(invalid), 0o600); err != nil {
				t.Fatal(err)
			}
			manager.ForEachMonitor(func(*Monitor) {})
			once.Do(func() { close(release) })
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("failed settings admission did not finish")
			}
			if got != nil || initErr == nil {
				t.Fatal("unreadable changed policy admitted a monitor carrying old grants")
			}
			if m, ok := manager.PeekMonitor("starting"); ok || m != nil {
				t.Fatal("failed settings admission remained live")
			}
			manager.mu.RLock()
			_, owned := manager.tenantDone["starting"]
			manager.mu.RUnlock()
			if owned {
				t.Fatal("fully cleaned constructor retained its lifetime")
			}
			if err := p.SaveSystemSettings(*initial); err != nil {
				t.Fatal(err)
			}
			manager.SetMonitorInitializer(nil)
			recovered, err := manager.GetMonitor("starting")
			if err != nil {
				t.Fatalf("corrected policy could not be admitted normally: %v", err)
			}
			if err := recovered.GetNotificationManager().ValidateWebhookURL("https://192.0.2.10/hook"); err != nil {
				t.Fatalf("corrected valid policy was not restored: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("failed admission changed saved encrypted inventory or TLS trust")
			}
		})
	}
}
