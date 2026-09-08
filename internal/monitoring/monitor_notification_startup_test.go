package monitoring

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// Cover the real monitor constructor, not a fixture which reapplies manager
// setters after restart. Saving here uses persistence directly: this is not
// browser/API save acceptance, a process restart, or destination receipt proof.
func TestNewRestoresSavedNotificationChoices(t *testing.T) {
	for _, tc := range []struct {
		name, target     string
		resolve, enabled bool
		activation       alerts.ActivationState
	}{
		{"webhook_without_recovery", "webhook", false, true, alerts.ActivationActive},
		{"apprise_with_recovery", "apprise", true, true, alerts.ActivationActive},
		{"email_disabled", "email", true, false, alerts.ActivationActive},
		{"all_pending", "all", false, true, alerts.ActivationPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PULSE_DATA_DIR", dir)
			persistence := config.NewConfigPersistence(dir)
			saved := alerts.AlertConfig{Enabled: tc.enabled, ActivationState: tc.activation,
				Schedule: alerts.ScheduleConfig{InitialNotify: tc.target, NotifyOnResolve: tc.resolve}}
			if err := persistence.SaveAlertConfig(saved); err != nil {
				t.Fatal(err)
			}
			webhooks := []notifications.WebhookConfig{{ID: "saved-ops", Name: "saved-ops", URL: "https://example.invalid/alerts", Enabled: true, Service: "generic", MinimumSeverity: "warning", TagFilter: []string{"ops"}, TagMode: "any"}}
			if err := persistence.SaveWebhooks(webhooks); err != nil {
				t.Fatal(err)
			}
			// Synthetic credentials only; no notification is sent by this test.
			email := notifications.EmailConfig{
				Enabled: tc.enabled, Provider: "custom", SMTPHost: "smtp.example.invalid", SMTPPort: 587,
				Username: "fixture-user", Password: "synthetic-email-password", From: "pulse@example.invalid",
				To: []string{"ops@example.invalid", "backup@example.invalid"}, StartTLS: true, RateLimit: 17,
				MinimumSeverity: "critical", TagFilter: []string{"ops"}, TagMode: "all",
			}
			if err := persistence.SaveEmailConfig(email); err != nil {
				t.Fatal(err)
			}
			apprise := notifications.AppriseConfig{
				Enabled: tc.enabled, Mode: notifications.AppriseModeHTTP, CLIPath: "apprise",
				Targets: []string{"json://example.invalid/alerts"}, TimeoutSeconds: 23,
				ServerURL: "https://apprise.example.invalid", ConfigKey: "fixture-config",
				APIKey: "synthetic-apprise-key", APIKeyHeader: "X-Fixture-Key", MinimumSeverity: "warning",
			}
			if err := persistence.SaveAppriseConfig(apprise); err != nil {
				t.Fatal(err)
			}
			for startup := 0; startup < 2; startup++ {
				func() {
					m, err := New(&config.Config{DataPath: dir})
					if err != nil {
						t.Fatal(err)
					}
					defer m.Stop()
					n := m.GetNotificationManager()
					if got := n.GetInitialNotifyTarget(); got != tc.target {
						t.Errorf("startup %d target=%q, want %q", startup, got, tc.target)
					}
					if got := n.GetNotifyOnResolve(); got != tc.resolve {
						t.Errorf("startup %d resolve=%t, want %t", startup, got, tc.resolve)
					}
					wantEnabled := tc.enabled && tc.activation == alerts.ActivationActive
					if got := n.IsEnabled(); got != wantEnabled {
						t.Errorf("startup %d enabled=%t, want %t", startup, got, wantEnabled)
					}
					if got := n.GetEmailConfig(); !reflect.DeepEqual(got, email) {
						t.Errorf("startup %d did not restore saved email configuration", startup)
					}
					if got := n.GetAppriseConfig(); !reflect.DeepEqual(got, apprise) {
						t.Errorf("startup %d did not restore saved Apprise configuration", startup)
					}
					if got := n.GetWebhooks(); !reflect.DeepEqual(got, webhooks) {
						t.Errorf("startup %d did not restore saved webhook configuration", startup)
					}
				}()
			}
		})
	}
}
