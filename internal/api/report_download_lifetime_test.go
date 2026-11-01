package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// Decorate the real public endpoint adapter at its supported extension seam.
// This holds admitted generation, not authentication, before its first output.
// No private implementation or installed workload is involved.
type heldReportDownloads struct {
	extensions.ReportingAdminEndpoints
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *heldReportDownloads) run(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	h.once.Do(func() { close(h.entered) })
	select {
	case <-h.release:
	case <-r.Context().Done():
		return
	}
	next(w, r)
}
func (h *heldReportDownloads) HandleGenerateReport(w http.ResponseWriter, r *http.Request) {
	h.run(w, r, h.ReportingAdminEndpoints.HandleGenerateReport)
}
func (h *heldReportDownloads) HandleGenerateMultiReport(w http.ResponseWriter, r *http.Request) {
	h.run(w, r, h.ReportingAdminEndpoints.HandleGenerateMultiReport)
}
func (h *heldReportDownloads) HandleExportVMInventory(w http.ResponseWriter, r *http.Request) {
	h.run(w, r, h.ReportingAdminEndpoints.HandleExportVMInventory)
}

type reportDownloadCase struct{ name, method, path, body string }

func reportDownloadCases() []reportDownloadCase {
	return []reportDownloadCase{
		{"single_get", http.MethodGet, "/api/admin/reports/generate?format=csv&resourceType=vm&resourceId=fixture-node-100", ""},
		{"single_post", http.MethodPost, "/api/admin/reports/generate", `{"resourceType":"vm","resourceId":"fixture-node-100","format":"csv"}`},
		{"fleet", http.MethodPost, "/api/admin/reports/generate-multi", `{"resources":[{"resourceType":"vm","resourceId":"fixture-node-100"}],"format":"csv"}`},
		{"inventory", http.MethodGet, "/api/admin/reports/inventory/vms/export?format=csv", ""},
	}
}

func newHeldReportDownloadRouter(t *testing.T) (*Router, *config.Config, *httptest.Server, *heldReportDownloads) {
	t.Helper()
	held := &heldReportDownloads{entered: make(chan struct{}), release: make(chan struct{})}
	SetReportingAdminEndpointsBinder(func(defaults extensions.ReportingAdminEndpoints, _ extensions.ReportingAdminRuntime) extensions.ReportingAdminEndpoints {
		held.ReportingAdminEndpoints = defaults
		return held
	})
	t.Cleanup(func() { SetReportingAdminEndpointsBinder(nil) })
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, logStreamToken, []string{config.ScopeSettingsRead}, nil),
		newTokenRecord(t, logStreamControl, []string{config.ScopeWildcard}, nil))
	cfg.AuthUser = "operator"
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	setLicenseTierForHandlersForTests(t, router.licenseHandlers, "default", pkglicensing.TierEnterprise)
	inventory, _, _, _, _ := inventoryScheduleFixture(t)
	router.reportingHandlers.mtMonitor.Store(inventory.reportMonitorManager())
	// The existing real handlers perform all parsing, scoping, generation and
	// activity accounting. Only the performance engine's data source is synthetic.
	original := reporting.GetEngine()
	reporting.SetEngine(&stubReportingEngine{data: []byte("synthetic,private-report\n"), contentType: "text/csv"})
	t.Cleanup(func() { reporting.SetEngine(original) })
	server := httptest.NewServer(router.Handler())
	// Unblock on any failed assertion before waiting for server cleanup.
	t.Cleanup(func() {
		select {
		case <-held.release:
		default:
			close(held.release)
		}
		server.Close()
	})
	return router, cfg, server, held
}

func reportDownloadRequest(t *testing.T, ctx context.Context, server *httptest.Server, tc reportDownloadCase, raw, transport string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, tc.method, server.URL+tc.path, strings.NewReader(tc.body))
	if err != nil {
		t.Fatal(err)
	}
	if tc.method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	switch transport {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
		req.Header.Set("X-CSRF-Token", generateCSRFToken(raw))
	default:
		req.Header.Set("X-API-Token", raw)
	}
	return req
}

func reportDownloadNativeWithdrawal(t *testing.T) {
	for _, tc := range reportDownloadCases() {
		for _, transport := range []string{"header", "bearer", "session"} {
			t.Run(tc.name+"/"+transport, func(t *testing.T) {
				router, cfg, server, held := newHeldReportDownloadRouter(t)
				raw := logStreamToken
				if transport == "session" {
					raw = "synthetic-held-report-session"
					GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", cfg.AuthUser)
					t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				req := reportDownloadRequest(t, ctx, server, tc, raw, transport)
				type result struct {
					response *http.Response
					err      error
				}
				done := make(chan result, 1)
				go func() { response, err := server.Client().Do(req); done <- result{response, err} }()
				select {
				case <-held.entered:
				case response := <-done:
					t.Fatalf("generation not admitted: response=%v err=%v", response.response, response.err)
				case <-ctx.Done():
					t.Fatal("generation did not reach hold")
				}
				if transport == "session" {
					logoutReadSession(t, server.URL, raw)
				} else {
					config.Mu.RLock()
					id := cfg.APITokens[0].ID
					config.Mu.RUnlock()
					deletion, _ := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/security/tokens/"+id, nil)
					deletion.Header.Set("X-API-Token", logStreamControl)
					response, err := server.Client().Do(deletion)
					if err != nil {
						t.Fatal(err)
					}
					response.Body.Close()
					persisted, err := router.persistence.LoadAPITokens()
					if response.StatusCode != http.StatusNoContent || err != nil || len(persisted) != 1 || persisted[0].ID == id {
						t.Fatal("DELETE did not persist withdrawal")
					}
				}
				fresh, err := server.Client().Do(reportDownloadRequest(t, ctx, server, tc, raw, transport))
				if err != nil {
					t.Fatal(err)
				}
				fresh.Body.Close()
				if fresh.StatusCode != http.StatusUnauthorized {
					t.Fatalf("fresh admission=%d", fresh.StatusCode)
				}
				close(held.release)
				var completed result
				select {
				case completed = <-done:
				case <-ctx.Done():
					t.Fatal("held report did not finish")
				}
				if completed.err != nil {
					t.Fatal(completed.err)
				}
				body, err := io.ReadAll(completed.response.Body)
				completed.response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				marker := "private-report"
				if tc.name == "inventory" {
					marker = "inventory-vm-000"
				}
				if completed.response.StatusCode != http.StatusUnauthorized || string(body) != "Authentication required\n" || completed.response.Header.Get("Content-Disposition") != "" {
					t.Errorf("held report=%d after persisted withdrawal (private data disclosed=%v), want generic 401 without attachment", completed.response.StatusCode, strings.Contains(string(body), marker))
				}
				// An independent settings reader still receives the original real-handler
				// download. Withdrawal does not cancel a job or alter an inventory/schedule.
				control, err := server.Client().Do(reportDownloadRequest(t, ctx, server, tc, logStreamControl, "header"))
				if err != nil {
					t.Fatal(err)
				}
				controlBody, err := io.ReadAll(control.Body)
				control.Body.Close()
				if err != nil || control.StatusCode != http.StatusOK || !strings.Contains(string(controlBody), marker) || control.Header.Get("Content-Disposition") == "" {
					t.Fatalf("independent report lost: %d %v", control.StatusCode, err)
				}
			})
		}
	}
}
