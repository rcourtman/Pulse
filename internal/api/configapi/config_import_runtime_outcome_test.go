package configapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

type importMetadataReloadFunc func(context.Context) error

func (f importMetadataReloadFunc) Reload(ctx context.Context) error { return f(ctx) }

func TestHandleImportConfigRestoresHeldNotificationInventoryForRestart(t *testing.T) {
	h, calls := replayTestHandler(t)
	dir := h.defaultPersistence.GetConfigDir()
	if err := os.WriteFile(filepath.Join(dir, "webhooks.enc"), []byte("unreadable synthetic inventory"), 0600); err != nil {
		t.Fatal(err)
	}
	notifier := &notifications.NotificationManager{}
	notifier.HoldIncompleteConfiguration(errors.New("synthetic unreadable inventory"))
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "notificationMgr", notifier)
	manager := alerts.NewManager()
	t.Cleanup(manager.Stop)
	setUnexportedField(t, monitor, "alertManager", manager)
	h.defaultMonitor = monitor
	archive := config.ExportData{Version: "4.4", System: *config.DefaultSystemSettings(), Alerts: alerts.DefaultAlertConfig(), Webhooks: []notifications.WebhookConfig{{ID: "restored", URL: "https://restored.invalid", Enabled: false}}}
	archive.Alerts.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 73, Clear: 68}
	archive.Nodes.PVEInstances = []config.PVEInstance{{Name: "imported-node", Host: "https://imported.invalid:8006", TokenName: "synthetic@pve!monitor", TokenValue: "synthetic-test-token", Disabled: true, VerifySSL: true}}
	metadataReloads := 0
	h.guestMetadataHandler = importMetadataReloadFunc(func(context.Context) error { metadataReloads++; return nil })
	plain, err := json.Marshal(archive)
	if err != nil {
		t.Fatal(err)
	}
	request := ImportConfigRequest{Data: encryptedImportValidationFixture(t, plain, replayTestPassphrase), Passphrase: replayTestPassphrase}
	rec := replayTestSubmit(h, replayTestBody(t, request))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "Configuration saved. Restart Pulse") {
		t.Fatalf("held import did not report its committed restart boundary: status=%d body=%s", rec.Code, rec.Body.String())
	}
	stored, err := h.defaultPersistence.LoadWebhooks()
	if err != nil || len(stored) != 1 || stored[0].ID != "restored" {
		t.Fatalf("explicit backup import did not restore unreadable destinations: %v %v", stored, err)
	}
	if calls.Load() != 1 || metadataReloads != 1 || manager.GetConfig().GuestDefaults.CPU.Trigger != 73 || notifier.ConfigurationError() == nil || len(notifier.GetWebhooks()) != 0 {
		t.Fatal("held import did not apply independent settings while preserving notification hold")
	}
	if len(h.defaultConfig.PVEInstances) != 1 || h.defaultConfig.PVEInstances[0].Name != "imported-node" {
		t.Fatal("held import left stale node state live")
	}
	// An ordinary live node edit before restart must preserve the imported host
	// and credentials instead of writing the pre-import inventory over the backup.
	h.defaultConfig.PVEInstances[0].Name = "edited-before-restart"
	if err := h.defaultPersistence.SaveNodesConfig(h.defaultConfig.PVEInstances, nil, nil); err != nil {
		t.Fatal(err)
	}
	reloaded, err := config.LoadWithoutLoggingInit()
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.PVEInstances) != 1 || reloaded.PVEInstances[0].Name != "edited-before-restart" || reloaded.PVEInstances[0].Host != "https://imported.invalid:8006" {
		t.Fatal("live edit reverted imported node state before restart")
	}
	restarted, err := monitoring.New(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Stop()
	if restarted.GetNotificationManager().ConfigurationError() != nil || len(restarted.GetNotificationManager().GetWebhooks()) != 1 {
		t.Fatal("restart did not apply restored notification inventory")
	}
	if strings.Contains(rec.Body.String(), request.Data) || strings.Contains(rec.Body.String(), dir) {
		t.Fatal("restart response exposed private import or path data")
	}
}

// These are real persistence imports followed by bounded, local read faults.
// No monitor, notification queue or external destination is started.
func TestHandleImportConfigRuntimeOutcome(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		brokenFiles          []string
		metadataFailure      bool
		missingAlerts        bool
		missingNotifications bool
		withoutMonitor       bool
		stoppedNotifications bool
		wantFailures         []string
	}{
		{name: "complete"},
		{name: "alerts", brokenFiles: []string{"alerts.json"}, wantFailures: []string{"alert settings"}},
		{name: "webhooks", brokenFiles: []string{"webhooks.enc"}, wantFailures: []string{"webhook settings"}},
		{name: "email", brokenFiles: []string{"email.enc"}, wantFailures: []string{"email settings"}},
		{name: "apprise", brokenFiles: []string{"apprise.enc"}, wantFailures: []string{"Apprise settings"}},
		{name: "metadata", metadataFailure: true, wantFailures: []string{"guest metadata"}},
		{name: "missing-alert-manager", missingAlerts: true, wantFailures: []string{"alert settings"}},
		{name: "missing-notification-manager", missingNotifications: true, wantFailures: []string{"webhook settings", "email settings", "Apprise settings"}},
		{name: "combined", brokenFiles: []string{"alerts.json", "webhooks.enc", "email.enc", "apprise.enc"}, metadataFailure: true, wantFailures: []string{"alert settings", "webhook settings", "email settings", "Apprise settings", "guest metadata"}},
		{name: "setup-without-monitor", withoutMonitor: true},
		{name: "email-publication-refused", stoppedNotifications: true, wantFailures: []string{"webhook settings", "email settings", "Apprise settings"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, calls := replayTestHandler(t)
			dir := h.defaultPersistence.GetConfigDir()
			manager := alerts.NewManager()
			t.Cleanup(manager.Stop)
			// Configuration setters need no running delivery worker.
			notifier := &notifications.NotificationManager{}
			if tc.stoppedNotifications {
				notifier = notifications.NewNotificationManagerWithDeferredQueue("", t.TempDir())
				if err := notifier.SetEmailConfig(notifications.EmailConfig{Enabled: true, To: []string{"previous@example.test"}}); err != nil {
					t.Fatal(err)
				}
				notifier.Stop()
			}
			if !tc.stoppedNotifications {
				notifier.SetEmailConfig(notifications.EmailConfig{SMTPHost: "before.invalid"})
				notifier.AddWebhook(notifications.WebhookConfig{ID: "before", URL: "https://before.invalid", Enabled: false})
			}
			monitor := &monitoring.Monitor{}
			if !tc.missingAlerts {
				setUnexportedField(t, monitor, "alertManager", manager)
			}
			if !tc.missingNotifications {
				setUnexportedField(t, monitor, "notificationMgr", notifier)
			}
			if !tc.withoutMonitor {
				h.defaultMonitor = monitor
			}

			const privateFailure = "synthetic-private-path-and-secret-must-not-reach-client"
			metadataCalls := 0
			h.guestMetadataHandler = importMetadataReloadFunc(func(context.Context) error {
				metadataCalls++
				if tc.metadataFailure {
					return errors.New(privateFailure)
				}
				return nil
			})
			h.reloadFunc = func() error {
				calls.Add(1)
				for _, name := range tc.brokenFiles {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(privateFailure), 0600); err != nil {
						return err
					}
				}
				return nil
			}
			var audits []bool
			var auditDetails []string
			h.SetRuntimeDependencies(RuntimeDependencies{AuditEvent: func(_, event, _, _, _ string, success bool, detail string) {
				if event != "config_imported" {
					t.Errorf("unexpected audit event %q", event)
				}
				audits = append(audits, success)
				auditDetails = append(auditDetails, detail)
			}})

			importedAlerts := alerts.DefaultAlertConfig()
			importedAlerts.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 73, Clear: 68}
			archive := config.ExportData{
				Version: "4.4", System: *config.DefaultSystemSettings(), Alerts: importedAlerts,
				Nodes:         config.NodesConfig{PVEInstances: []config.PVEInstance{{Name: "runtime-fixture", Host: "https://runtime.invalid:8006", TokenName: "synthetic@pve!monitor", TokenValue: "synthetic-runtime-token", Disabled: true, VerifySSL: true}}},
				Email:         notifications.EmailConfig{Enabled: false, SMTPHost: "after.invalid", Password: "synthetic-email-secret"},
				Apprise:       notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://imported-relay.invalid", ConfigKey: "imported", APIKey: "synthetic-import-apprise-key"},
				Webhooks:      []notifications.WebhookConfig{{ID: "after", URL: "https://after.invalid", Enabled: false}},
				GuestMetadata: map[string]*config.GuestMetadata{"synthetic:101": {Description: "imported metadata"}},
			}
			archive.System.PublicURL = "https://imported.invalid"
			plain, err := json.Marshal(archive)
			if err != nil {
				t.Fatal(err)
			}
			request := ImportConfigRequest{Data: encryptedImportValidationFixture(t, plain, replayTestPassphrase), Passphrase: replayTestPassphrase}
			body := replayTestBody(t, request)
			rec := replayTestSubmit(h, body)
			failed := len(tc.wantFailures) != 0
			wantStatus := http.StatusOK
			if failed {
				wantStatus = http.StatusInternalServerError
				want := "Configuration saved, but live reload failed: " + strings.Join(tc.wantFailures, ", ") + ". Check the destination before relying on it. Do not repeat the import.\n"
				if rec.Body.String() != want {
					t.Errorf("partial apply response = %q, want %q", rec.Body.String(), want)
				}
			}
			if rec.Code != wantStatus || calls.Load() != 1 || metadataCalls != 1 || !reflect.DeepEqual(audits, []bool{!failed}) {
				t.Errorf("status=%d body=%s reloads=%d metadata=%d audits=%v", rec.Code, rec.Body.String(), calls.Load(), metadataCalls, audits)
			}
			for _, secret := range []string{privateFailure, dir, request.Data, request.Passphrase, archive.Email.Password, archive.Apprise.APIKey} {
				if strings.Contains(rec.Body.String(), secret) || strings.Contains(strings.Join(auditDetails, " "), secret) {
					t.Error("client response or audit leaked private failure/input detail")
				}
			}
			if !tc.withoutMonitor && !tc.missingAlerts && !containsImportFailure(tc.wantFailures, "alert settings") && manager.GetConfig().GuestDefaults.CPU.Trigger != 73 {
				t.Error("an independent successful alert reload was abandoned")
			}
			if !tc.withoutMonitor && !tc.missingNotifications {
				if !containsImportFailure(tc.wantFailures, "email settings") && notifier.GetEmailConfig().SMTPHost != "after.invalid" {
					t.Error("an independent successful email reload was abandoned")
				}
				if !containsImportFailure(tc.wantFailures, "Apprise settings") && notifier.GetAppriseConfig().ServerURL != "https://imported-relay.invalid" {
					t.Error("an independent successful Apprise reload was abandoned")
				}
				if !containsImportFailure(tc.wantFailures, "webhook settings") && (len(notifier.GetWebhooks()) != 1 || notifier.GetWebhooks()[0].ID != "after") {
					t.Error("an independent successful webhook reload was abandoned")
				}
			}
			settings, err := h.defaultPersistence.LoadSystemSettings()
			if err != nil || settings.PublicURL != "https://imported.invalid" {
				t.Fatalf("committed settings lost after live apply: %#v, %v", settings, err)
			}
			readback := map[string]any{"case": tc.name, "request": request, "status": rec.Code, "contentType": rec.Header().Get("Content-Type"), "body": rec.Body.String()}

			// The failure is post-commit, not permission to overwrite newer work.
			settings.PublicURL = "https://newer-edit.invalid"
			if err := h.defaultPersistence.SaveSystemSettings(*settings); err != nil {
				t.Fatal(err)
			}
			before := metadataImportFiles(t, dir)
			replay := replayTestSubmit(h, body)
			if replay.Code != http.StatusConflict || calls.Load() != 1 || metadataCalls != 1 || len(audits) != 1 || !reflect.DeepEqual(before, metadataImportFiles(t, dir)) {
				t.Error("post-commit replay changed newer settings, metadata, backups or live state")
			}
			readback["replay"] = map[string]any{"status": replay.Code, "contentType": replay.Header().Get("Content-Type"), "body": replay.Body.String()}
			encoded, err := json.Marshal(readback)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("SYNTHETIC_IMPORT_RUNTIME_READBACK %s", encoded)
		})
	}
}

func containsImportFailure(failures []string, component string) bool {
	for _, failure := range failures {
		if failure == component {
			return true
		}
	}
	return false
}
