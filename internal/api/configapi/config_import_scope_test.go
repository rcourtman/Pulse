package configapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestHandleImportConfigTenantWithoutRuntimeDoesNotFallBack(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), PublicURL: "https://default.invalid"}
	h := newTestConfigHandlers(t, cfg)
	settings := config.DefaultSystemSettings()
	settings.PublicURL = cfg.PublicURL
	if err := h.defaultPersistence.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	source := config.NewConfigPersistence(t.TempDir())
	settings.PublicURL = "https://must-not-apply.invalid"
	if err := source.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	archive, err := source.ExportConfig("synthetic-import-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(ImportConfigRequest{Data: archive, Passphrase: "synthetic-import-passphrase"})
	if err != nil {
		t.Fatal(err)
	}
	reloads := 0
	h.reloadFunc = func() error { reloads++; return nil }
	ctx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, "tenant-a")
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleImportConfig(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing tenant runtime status = %d, want 503", rec.Code)
	}
	after, err := h.defaultPersistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after.PublicURL != "https://default.invalid" || cfg.PublicURL != "https://default.invalid" || reloads != 0 {
		t.Error("missing tenant runtime fell back to default persistence or reload")
	}
}

func TestConfigHandlersTenantManagerReferenceTracksReplacement(t *testing.T) {
	first := monitoring.NewMultiTenantMonitor(&config.Config{}, nil, nil)
	second := monitoring.NewMultiTenantMonitor(&config.Config{}, nil, nil)
	t.Cleanup(first.Stop)
	t.Cleanup(second.Stop)
	h := &ConfigHandlers{mtMonitor: first}
	if h.MultiTenantMonitor() != first {
		t.Fatal("initial scoped owner missing")
	}
	h.SetMultiTenantMonitor(second)
	if h.MultiTenantMonitor() != second {
		t.Fatal("replacement scoped owner not visible")
	}
	h.SetMultiTenantMonitor(nil)
	if h.MultiTenantMonitor() != nil {
		t.Fatal("removed scoped owner remained usable")
	}
}
