package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

func TestNewReportingAdminRuntime_DefaultHooks(t *testing.T) {
	runtime := newReportingAdminRuntime(nil)
	if runtime.GetEngine == nil {
		t.Fatal("expected GetEngine callback to be set")
	}
	if runtime.GetRequestOrgID == nil {
		t.Fatal("expected GetRequestOrgID callback to be set")
	}
	if runtime.SanitizeFilename == nil {
		t.Fatal("expected SanitizeFilename callback to be set")
	}
	if runtime.WriteError == nil {
		t.Fatal("expected WriteError callback to be set")
	}
	if runtime.GetStateSnapshot != nil {
		t.Fatal("expected GetStateSnapshot callback to be nil when handlers are missing")
	}
	if runtime.ListBackupsForResource != nil {
		t.Fatal("expected ListBackupsForResource callback to be nil when handlers are missing")
	}
	if runtime.EnrichReportRequest != nil {
		t.Fatal("expected EnrichReportRequest callback to be nil when handlers are missing")
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "org-runtime")
	if orgID := runtime.GetRequestOrgID(ctx); orgID != "org-runtime" {
		t.Fatalf("expected org-runtime from context, got %q", orgID)
	}

	name := runtime.SanitizeFilename("\"bad/../resource\\\r\n")
	if strings.ContainsAny(name, "\"\\/\r\n") {
		t.Fatalf("sanitize hook returned unsafe value %q", name)
	}
}

func TestNewReportingAdminRuntime_EnrichHookWithNilMonitorIsSafe(t *testing.T) {
	handlers := &ReportingHandlers{}
	runtime := newReportingAdminRuntime(handlers)
	if runtime.EnrichReportRequest == nil {
		t.Fatal("expected EnrichReportRequest callback when handlers are provided")
	}
	if runtime.GetStateSnapshot == nil {
		t.Fatal("expected GetStateSnapshot callback when handlers are provided")
	}
	if runtime.ListBackupsForResource == nil {
		t.Fatal("expected ListBackupsForResource callback when handlers are provided")
	}

	if state, ok := runtime.GetStateSnapshot(context.Background(), "default"); ok {
		t.Fatalf("expected no runtime snapshot with nil monitor, got %+v", state)
	}

	req := &reporting.MetricReportRequest{
		ResourceType: "node",
		ResourceID:   "node-1",
	}
	runtime.EnrichReportRequest(context.Background(), "default", req, time.Now().Add(-time.Hour), time.Now())
	if req.Resource != nil || len(req.Alerts) != 0 || len(req.Backups) != 0 {
		t.Fatal("expected no enrichment when monitor is unavailable")
	}

	if backups := runtime.ListBackupsForResource(context.Background(), "default", "node-1", time.Now().Add(-time.Hour), time.Now()); len(backups) != 0 {
		t.Fatalf("expected no backups when recovery manager is unavailable, got %d", len(backups))
	}
}
