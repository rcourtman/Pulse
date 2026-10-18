package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestOrgDeleteRetainsDataWhenGuestMetadataPersistenceFails(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	previous := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(previous) })
	root := t.TempDir()
	persistence := config.NewMultiTenantPersistence(root)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{DataPath: root}, persistence, nil)
	t.Cleanup(mtm.Stop)
	h := NewOrgHandlers(persistence, mtm)
	// Exercise the same session-owner setup as ordinary organisation creation.
	create := withUser(httptest.NewRequest(http.MethodPost, "/api/orgs", strings.NewReader(`{"id":"retained","displayName":"Retained"}`)), "alice")
	rec := httptest.NewRecorder()
	h.HandleCreateOrg(rec, create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	org, err := persistence.LoadOrganization("retained")
	if err != nil || org == nil {
		t.Fatal(err)
	}
	monitor, err := mtm.GetMonitor("retained")
	if err != nil {
		t.Fatal(err)
	}
	orgPath := filepath.Join(root, "orgs", "retained")
	// A real rename refusal, confined to this disposable test directory.
	if err := os.Mkdir(filepath.Join(orgPath, "guest_metadata.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := monitor.GuestMetadataStore().RememberIdentity("pve:node:100", "unsaved", "qemu"); err != nil {
		t.Fatal(err)
	}
	if !monitor.GuestMetadataStore().WaitForPendingWrites(time.Second) {
		t.Fatal("failed attempt did not quiesce")
	}
	called := false
	h.SetOnDelete(func(context.Context, string) error { called = true; return nil })
	req := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/retained", nil), "alice")
	req.SetPathValue("id", "retained")
	rec = httptest.NewRecorder()
	h.HandleDeleteOrg(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "tenant_shutdown_incomplete") {
		t.Fatalf("unsafe deletion result: %d %s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("offboarding callback ran after incomplete shutdown")
	}
	if _, err := os.Stat(filepath.Join(orgPath, "org.json")); err != nil {
		t.Fatalf("organisation data removed: %v", err)
	}
	if got, ok := mtm.PeekMonitor("retained"); !ok || got != monitor {
		t.Fatal("failed writer owner lost")
	}
	if _, err := mtm.GetMonitor("retained"); err == nil {
		t.Fatal("new monitor can write into retained shutdown path")
	}
	// Another tenant remains usable: the failed writer is not a global hold.
	if err := persistence.SaveOrganization(&models.Organization{ID: "other"}); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.GetPersistence("other"); err != nil {
		t.Fatal(err)
	}
}
