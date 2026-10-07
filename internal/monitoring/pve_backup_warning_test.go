package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type backupWarningClient struct {
	stubPVEClient
	contentErr error
	storageErr error
}

func (c *backupWarningClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	if c.storageErr != nil {
		return nil, c.storageErr
	}
	return []proxmox.Storage{{Storage: "backup-store", Type: "dir", Content: "backup", Enabled: 1, Active: 1}}, nil
}
func (c *backupWarningClient) GetStorageContent(context.Context, string, string) ([]proxmox.StorageContent, error) {
	return nil, c.contentErr
}

func TestPVEBackupWarningProducerUsesActualStatusAndEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		list           bool
		body, expected string
	}{
		{"custom-token-auth", 401, false, "authentication failed; private-synthetic-body", "authentication (HTTP 401)"},
		{"custom-token-denied", 403, false, "permission denied; private-synthetic-body", "access (HTTP 403)"},
		{"manual-list-auth", 401, true, "authentication failed; private-synthetic-body", "authentication (HTTP 401)"},
		{"proxy-quoted-denial", 502, false, "API error 403: permission denied; private-synthetic-body", "cause is unconfirmed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, tc.body, tc.status) }))
			defer server.Close()
			api, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "custom-user@pve!manual", TokenValue: "synthetic-secret"})
			if err != nil {
				t.Fatal(err)
			}
			_, readErr := api.GetStorageContent(context.Background(), "node-a", "backup-store")
			if readErr == nil {
				t.Fatal("HTTP error not observed")
			}
			client := &backupWarningClient{contentErr: readErr}
			endpoint := "/nodes/node-a/storage/backup-store/content"
			tokenName := "custom-user@pve!manual"
			if tc.list {
				client.storageErr = readErr
				endpoint = "/nodes/node-a/storage"
				tokenName = ""
			}
			m := newUnreachableTestMonitor(t, &config.Config{PVEInstances: []config.PVEInstance{{Name: "site-a", TokenName: tokenName, TokenValue: "synthetic-secret"}}})
			m.pollStorageBackupsWithNodes(context.Background(), "site-a", client, []proxmox.Node{{Node: "node-a", Status: "online"}}, map[string]string{"node-a": "online"})
			m.mu.RLock()
			warning := m.backupPermissionWarnings["site-a"]
			m.mu.RUnlock()
			for _, snippet := range []string{tc.expected, endpoint, "saved PVE connection", "both user and token scopes", "without disabling privilege separation", "installed PVE version"} {
				if !strings.Contains(warning, snippet) {
					t.Fatalf("missing %q in producer warning: %q", snippet, warning)
				}
			}
			for _, unsafe := range []string{"pveum", "PVEDatastoreAdmin", "pulse-monitor@pve", "custom-user@pve!manual", "synthetic-secret", "private-synthetic-body"} {
				if strings.Contains(warning, unsafe) {
					t.Fatalf("unsafe prescription/disclosure %q: %q", unsafe, warning)
				}
			}
		})
	}
}
func TestPVEBackupWarningUnknownEvidenceAndBoundedEndpoint(t *testing.T) {
	warning := pveBackupAccessWarning(nil, "/nodes/"+strings.Repeat("界", 300)+"\n/storage", fmt.Errorf("403 permission denied; synthetic-secret"))
	if !strings.Contains(warning, "cause is unconfirmed") || strings.Contains(warning, "HTTP 403") || strings.Contains(warning, "synthetic-secret") {
		t.Fatalf("text-only error became HTTP evidence: %q", warning)
	}
	if strings.ContainsAny(warning, "\r\n") || !strings.Contains(warning, "…") || len([]rune(warning)) > 1100 {
		t.Fatalf("endpoint not safely bounded: %q", warning)
	}
}
