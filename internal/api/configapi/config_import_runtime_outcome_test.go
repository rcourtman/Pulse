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
		wantFailures         []string
	}{
		{name: "complete"},
		{name: "alerts", brokenFiles: []string{"alerts.json"}, wantFailures: []string{"alert settings"}},
		{name: "webhooks", brokenFiles: []string{"webhooks.enc"}, wantFailures: []string{"webhook settings"}},
		{name: "email", brokenFiles: []string{"email.enc"}, wantFailures: []string{"email settings"}},
		{name: "metadata", metadataFailure: true, wantFailures: []string{"guest metadata"}},
		{name: "missing-alert-manager", missingAlerts: true, wantFailures: []string{"alert settings"}},
		{name: "missing-notification-manager", missingNotifications: true, wantFailures: []string{"webhook settings", "email settings"}},
		{name: "combined", brokenFiles: []string{"alerts.json", "webhooks.enc", "email.enc"}, metadataFailure: true, wantFailures: []string{"alert settings", "webhook settings", "email settings", "guest metadata"}},
		{name: "setup-without-monitor", withoutMonitor: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, calls := replayTestHandler(t)
			dir := h.defaultPersistence.GetConfigDir()
			manager := alerts.NewManager()
			t.Cleanup(manager.Stop)
			// Configuration setters need no running delivery worker.
			notifier := &notifications.NotificationManager{}
			notifier.SetEmailConfig(notifications.EmailConfig{SMTPHost: "before.invalid"})
			notifier.AddWebhook(notifications.WebhookConfig{ID: "before", URL: "https://before.invalid", Enabled: false})
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
			for _, secret := range []string{privateFailure, dir, request.Data, request.Passphrase, archive.Email.Password} {
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
