package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// tenantImportBody seals whatever seed saves into an import request, the way
// anyone holding a passphrase of their own choosing can.
func tenantImportBody(t *testing.T, seed func(source *config.ConfigPersistence) error) string {
	t.Helper()
	const passphrase = "synthetic-import-passphrase"
	source := config.NewConfigPersistence(t.TempDir())
	if err := seed(source); err != nil {
		t.Fatalf("seed archive: %v", err)
	}
	archive, err := source.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("export archive: %v", err)
	}
	body, err := json.Marshal(ImportConfigRequest{Data: archive, Passphrase: passphrase})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// Configuration import replaces an organization's connections and alert
// destinations in one request. An organization's owner reaches it, so it must
// not write what the settings routes refuse that owner: a connection to this
// server's own loopback, or Apprise targets for the server's command line.
// The instance administrator restores any backup.
func TestTenantAdministratorImportIsHeldToTheSettingsRouteLimits(t *testing.T) {
	router := notificationAuthorityRouter(t)
	port, accepted := loopbackListener(t)

	loopbackNode := tenantImportBody(t, func(source *config.ConfigPersistence) error {
		return source.SaveNodesConfig(nil, []config.PBSInstance{{
			Name: "loop", Host: fmt.Sprintf("http://127.0.0.1:%d", port), TokenName: "a@pbs!x", TokenValue: "synthetic-node-secret", MonitorDatastores: true,
		}}, nil)
	})
	commandLineTargets := tenantImportBody(t, func(source *config.ConfigPersistence) error {
		return source.SaveAppriseConfig(notifications.AppriseConfig{
			Enabled: true, Mode: notifications.AppriseModeCLI, Targets: []string{fmt.Sprintf("json://127.0.0.1:%d/chosen-by-tenant", port)},
		})
	})
	ownNetwork := tenantImportBody(t, func(source *config.ConfigPersistence) error {
		if err := source.SaveAppriseConfig(notifications.AppriseConfig{
			Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://apprise.example.com", ConfigKey: "pulse",
		}); err != nil {
			return err
		}
		return source.SaveNodesConfig([]config.PVEInstance{{
			Name: "site", Host: "https://192.168.77.20:8006", TokenName: "synthetic@pve!monitor", TokenValue: "synthetic-node-secret", VerifySSL: true, Disabled: true,
		}}, nil, nil)
	})

	for _, tc := range []struct {
		name, body, want string
	}{
		{"loopback node", loopbackNode, "loopback"},
		{"Apprise command-line targets", commandLineTargets, "Apprise command-line targets need an instance administrator"},
	} {
		rec := notificationAuthorityRequest(t, router, http.MethodPost, "/api/config/import", tc.body, "org-a-owner", "org-a")
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("tenant owner import of %s = %d, want 403 naming %q: %s", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}
	persistence, err := router.multiTenant.GetPersistence("org-a")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := persistence.LoadNodesConfig()
	if err != nil {
		t.Fatal(err)
	}
	apprise, err := persistence.LoadAppriseConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes.PBSInstances) != 0 || len(apprise.Targets) != 0 {
		t.Fatalf("refused imports were persisted: %d PBS nodes, Apprise targets %v", len(nodes.PBSInstances), apprise.Targets)
	}
	// Give any stray dial a moment to land before asserting none did.
	time.Sleep(50 * time.Millisecond)
	if got := accepted.Load(); got != 0 {
		t.Fatalf("refused imports reached the loopback listener %d times", got)
	}

	rec := notificationAuthorityRequest(t, router, http.MethodPost, "/api/config/import", ownNetwork, "org-a-owner", "org-a")
	if rec.Code != http.StatusOK {
		t.Fatalf("tenant owner import of its own network's connections = %d: %s", rec.Code, rec.Body.String())
	}

	rec = notificationAuthorityRequest(t, router, http.MethodPost, "/api/config/import", commandLineTargets, "platform-admin", "default")
	if rec.Code != http.StatusOK {
		t.Fatalf("instance administrator import = %d: %s", rec.Code, rec.Body.String())
	}
}
