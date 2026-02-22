package server

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
)

type captureAuditLogger struct {
	events []audit.Event
}

func (c *captureAuditLogger) Log(event audit.Event) error {
	c.events = append(c.events, event)
	return nil
}

func (c *captureAuditLogger) Query(filter audit.QueryFilter) ([]audit.Event, error) {
	return nil, nil
}

func (c *captureAuditLogger) Count(filter audit.QueryFilter) (int, error) {
	return len(c.events), nil
}

func (c *captureAuditLogger) GetWebhookURLs() []string {
	return nil
}

func (c *captureAuditLogger) UpdateWebhookURLs(urls []string) error {
	return nil
}

func (c *captureAuditLogger) Close() error {
	return nil
}

func setCaptureAuditLogger(t *testing.T) *captureAuditLogger {
	t.Helper()
	previous := audit.GetLogger()
	capture := &captureAuditLogger{}
	audit.SetLogger(capture)
	t.Cleanup(func() {
		audit.SetLogger(previous)
	})
	return capture
}

func TestShouldAutoImport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")

	if ShouldAutoImport() {
		t.Fatal("expected auto-import false without config")
	}

	t.Setenv("PULSE_INIT_CONFIG_DATA", "payload")
	if !ShouldAutoImport() {
		t.Fatal("expected auto-import true with data")
	}

	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "/tmp/file")
	if !ShouldAutoImport() {
		t.Fatal("expected auto-import true with file")
	}

	file := filepath.Join(dir, "nodes.enc")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("write nodes: %v", err)
	}
	if ShouldAutoImport() {
		t.Fatal("expected auto-import false when nodes.enc exists")
	}
}

func TestNormalizeImportPayload(t *testing.T) {
	if _, err := NormalizeImportPayload([]byte("")); err == nil {
		t.Fatal("expected error for empty payload")
	}

	raw := []byte("hello")
	encoded := base64.StdEncoding.EncodeToString(raw)
	out, err := NormalizeImportPayload([]byte(encoded))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if out != encoded {
		t.Fatalf("unexpected output: %s", out)
	}

	double := base64.StdEncoding.EncodeToString([]byte(encoded))
	out, err = NormalizeImportPayload([]byte(double))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if out != encoded {
		t.Fatalf("unexpected output: %s", out)
	}

	out, err = NormalizeImportPayload([]byte("not-base64"))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if out == "not-base64" {
		t.Fatal("expected payload to be encoded")
	}
}

func TestLooksLikeBase64(t *testing.T) {
	if LooksLikeBase64("") {
		t.Fatal("expected false for empty")
	}
	if !LooksLikeBase64("aGVsbG8=") {
		t.Fatal("expected true for base64")
	}
	if LooksLikeBase64("nope!!") {
		t.Fatal("expected false for invalid")
	}
}

func TestPerformAutoImportErrors(t *testing.T) {
	t.Setenv("PULSE_INIT_CONFIG_DATA", "data")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "")
	if err := PerformAutoImport(); err == nil {
		t.Fatal("expected error without passphrase")
	}

	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "pass")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "/tmp/missing-file")
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	if err := PerformAutoImport(); err == nil {
		t.Fatal("expected error for missing file")
	}

	t.Setenv("PULSE_INIT_CONFIG_FILE", "")
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	if err := PerformAutoImport(); err == nil {
		t.Fatal("expected error for missing data")
	}
}

func TestPerformAutoImport_AuditFailureMissingPassphrase(t *testing.T) {
	capture := setCaptureAuditLogger(t)
	t.Setenv("PULSE_INIT_CONFIG_DATA", "data")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "")

	if err := PerformAutoImport(); err == nil {
		t.Fatal("expected error without passphrase")
	}

	if len(capture.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(capture.events))
	}

	event := capture.events[0]
	if event.EventType != "config_auto_import" {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
	if event.Success {
		t.Fatal("expected failure audit event")
	}
	if event.User != "system" {
		t.Fatalf("unexpected audit user: %q", event.User)
	}
	if event.Path != "/startup/auto-import" {
		t.Fatalf("unexpected audit path: %q", event.Path)
	}
	if !strings.Contains(event.Details, "source=env_data") {
		t.Fatalf("expected source in details, got %q", event.Details)
	}
	if !strings.Contains(event.Details, "reason=missing_passphrase") {
		t.Fatalf("expected failure reason in details, got %q", event.Details)
	}
}

func TestEnsureDefaultOrgOwnerMembership_SeedsMissingDefaultOrg(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())

	if err := ensureDefaultOrgOwnerMembership(mtp, "admin"); err != nil {
		t.Fatalf("ensureDefaultOrgOwnerMembership: %v", err)
	}

	org, err := mtp.LoadOrganization("default")
	if err != nil {
		t.Fatalf("LoadOrganization(default): %v", err)
	}
	if org.ID != "default" {
		t.Fatalf("org.ID = %q, want default", org.ID)
	}
	if org.DisplayName != "default" {
		t.Fatalf("org.DisplayName = %q, want default", org.DisplayName)
	}
	if org.OwnerUserID != "admin" {
		t.Fatalf("org.OwnerUserID = %q, want admin", org.OwnerUserID)
	}
	if org.CreatedAt.IsZero() {
		t.Fatal("expected org.CreatedAt to be set")
	}
	if len(org.Members) != 1 {
		t.Fatalf("org.Members length = %d, want 1", len(org.Members))
	}
	if org.Members[0].UserID != "admin" || org.Members[0].Role != models.OrgRoleOwner {
		t.Fatalf("expected admin owner member, got user=%q role=%q", org.Members[0].UserID, org.Members[0].Role)
	}
}

func TestEnsureDefaultOrgOwnerMembership_PreservesOwnerAndAddsAdminOwnerMembership(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	seed := &models.Organization{
		ID:          "default",
		DisplayName: "Default",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner},
		},
	}
	if err := mtp.SaveOrganization(seed); err != nil {
		t.Fatalf("SaveOrganization(seed): %v", err)
	}

	if err := ensureDefaultOrgOwnerMembership(mtp, "admin"); err != nil {
		t.Fatalf("ensureDefaultOrgOwnerMembership: %v", err)
	}

	org, err := mtp.LoadOrganization("default")
	if err != nil {
		t.Fatalf("LoadOrganization(default): %v", err)
	}
	if org.OwnerUserID != "alice" {
		t.Fatalf("org.OwnerUserID = %q, want alice", org.OwnerUserID)
	}

	roles := map[string]models.OrganizationRole{}
	for _, member := range org.Members {
		roles[member.UserID] = member.Role
	}
	if roles["alice"] != models.OrgRoleOwner {
		t.Fatalf("alice role = %q, want owner", roles["alice"])
	}
	if roles["admin"] != models.OrgRoleOwner {
		t.Fatalf("admin role = %q, want owner", roles["admin"])
	}
}

func TestEnsureDefaultOrgOwnerMembership_NoOpWithoutAdminUser(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())

	if err := ensureDefaultOrgOwnerMembership(mtp, ""); err != nil {
		t.Fatalf("ensureDefaultOrgOwnerMembership: %v", err)
	}

	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}
	if _, err := persistence.LoadOrganization(); err == nil {
		t.Fatal("expected no persisted org metadata when admin user is empty")
	}
}
