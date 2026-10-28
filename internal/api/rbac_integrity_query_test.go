package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

func TestRBACIntegrityQueryRejectsDamagedSelection(t *testing.T) {
	for _, tt := range []struct{ name, query, code string }{
		{"bad escape", "org_id=%zz", "invalid_query"},
		{"bare escape", "org_id=%", "invalid_query"},
		{"short escape", "org_id=%0", "invalid_query"},
		{"damaged selector key", "org_%zz=default", "invalid_query"},
		{"damaged unrelated key", "%zz=ignored", "invalid_query"},
		{"damaged suffix", "org_id=default&future=%zz", "invalid_query"},
		{"damaged prefix", "future=%zz&org_id=default", "invalid_query"},
		{"unescaped semicolon", "org_id=default;future=value", "invalid_query"},
		{"damaged repeated value", "org_id=default&org_id=%zz", "invalid_query"},
		{"damaged encoded duplicate", "org_id=default&%6frg_id=%zz", "invalid_query"},
		{"identical duplicate", "org_id=default&org_id=default", "invalid_query"},
		{"allowed first", "org_id=default&org_id=other", "invalid_query"},
		{"denied first", "org_id=other&org_id=default", "invalid_query"},
		{"empty first", "org_id=&org_id=default", "invalid_query"},
		{"empty second", "org_id=default&org_id=", "invalid_query"},
		{"encoded duplicate key", "%6frg_id=default&org_id=default", "invalid_query"},
		{"two encoded keys", "%6frg_id=default&org_%69d=other", "invalid_query"},
		{"three values", "org_id=default&org_id=default&org_id=default", "invalid_query"},
		{"explicit empty", "org_id=", "invalid_org_id"},
		{"bare selector", "org_id", "invalid_org_id"},
		{"plus whitespace", "org_id=+", "invalid_org_id"},
		{"encoded whitespace", "org_id=%20%09", "invalid_org_id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			provider := NewTenantRBACProvider(dir)
			t.Cleanup(func() { _ = provider.Close() })
			handlers := NewRBACHandlers(nil, provider)
			rec := httptest.NewRecorder()
			handlers.HandleRBACIntegrityCheck(rec, httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity?"+tt.query, nil))
			var body APIError
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest || body.Code != tt.code {
				t.Errorf("status/code = %d/%q, want 400/%q", rec.Code, body.Code, tt.code)
			}
			if provider.ManagerCount() != 0 {
				t.Error("rejected selection initialized an RBAC manager")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 || strings.Contains(rec.Body.String(), `"healthy"`) {
				t.Error("rejected selection created RBAC files or returned an integrity verdict")
			}
		})
	}
}

func TestRBACIntegrityQueryPreservesSupportedSelection(t *testing.T) {
	for _, tt := range []struct{ name, query, contextOrg, wantOrg string }{
		{"omitted legacy context", "", "", "default"},
		{"blank legacy context", "", " \t", "default"},
		{"explicit default", "org_id=default", "", "default"},
		{"encoded key", "%6frg_id=default", "", "default"},
		{"encoded value", "org_id=%64efault", "", "default"},
		{"trimmed value", "org_id=%20default%20", "", "default"},
		{"unknown parameters", "future=a&future=b&org_id=default", "", "default"},
		{"encoded unknown semicolon", "future=a%3Bb", "", "default"},
		{"empty pairs", "&&org_id=default&", "", "default"},
		{"omitted scoped selection", "", "org-a", "org-a"},
		{"encoded scoped selection", "org_%69d=org%2Da", "org-a", "org-a"},
		{"trimmed scoped context", "org_id=+org-a+", " org-a ", "org-a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.wantOrg != "default" {
				if err := os.MkdirAll(filepath.Join(dir, "orgs", tt.wantOrg), 0700); err != nil {
					t.Fatal(err)
				}
			}
			provider := NewTenantRBACProvider(dir)
			t.Cleanup(func() { _ = provider.Close() })
			req := httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity?"+tt.query, nil)
			req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, tt.contextOrg))
			rec := httptest.NewRecorder()
			NewRBACHandlers(nil, provider).HandleRBACIntegrityCheck(rec, req)
			var result RBACIntegrityResult
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || result.OrgID != tt.wantOrg || !result.Healthy || !result.DBAccessible {
				t.Fatalf("status/result = %d/%+v, want healthy %s", rec.Code, result, tt.wantOrg)
			}
			if rec.Header().Get("Content-Type") != "application/json" || provider.ManagerCount() != 1 {
				t.Fatal("supported selection lost its JSON result or scoped manager")
			}
		})
	}
}

func TestRBACIntegrityQueryPreservesDenials(t *testing.T) {
	for _, tt := range []struct {
		name, query, contextOrg, code string
		status                        int
	}{
		{"foreign org", "org_id=org-b", "org-a", "access_denied", http.StatusForbidden},
		{"encoded foreign org", "org_id=org%2Db", "org-a", "access_denied", http.StatusForbidden},
		{"foreign org without context", "org_id=org-b", "", "access_denied", http.StatusForbidden},
		{"path", "org_id=..%2Fother", "org-a", "invalid_org_id", http.StatusBadRequest},
		{"dot", "org_id=.", "org-a", "invalid_org_id", http.StatusBadRequest},
		{"invalid character", "org_id=org%2Ba", "org-a", "invalid_org_id", http.StatusBadRequest},
		{"too long", "org_id=" + strings.Repeat("a", 65), "org-a", "invalid_org_id", http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewTenantRBACProvider(t.TempDir())
			t.Cleanup(func() { _ = provider.Close() })
			req := httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity?"+tt.query, nil)
			req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, tt.contextOrg))
			rec := httptest.NewRecorder()
			NewRBACHandlers(nil, provider).HandleRBACIntegrityCheck(rec, req)
			var body APIError
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rec.Code != tt.status || body.Code != tt.code || provider.ManagerCount() != 0 {
				t.Fatalf("status/code/managers = %d/%q/%d, want %d/%q/0", rec.Code, body.Code, provider.ManagerCount(), tt.status, tt.code)
			}
		})
	}
}

func TestRBACIntegrityFallbackUsesQueryAdmission(t *testing.T) {
	for _, mode := range []string{"no binder", "binder returns nil"} {
		t.Run(mode, func(t *testing.T) {
			SetRBACAdminEndpointsBinder(nil)
			t.Cleanup(func() { SetRBACAdminEndpointsBinder(nil) })
			if mode == "binder returns nil" {
				SetRBACAdminEndpointsBinder(func(extensions.RBACAdminEndpoints, extensions.RBACAdminRuntime) extensions.RBACAdminEndpoints {
					return nil
				})
			}
			provider := NewTenantRBACProvider(t.TempDir())
			t.Cleanup(func() { _ = provider.Close() })
			handlers := NewRBACHandlers(nil, provider)
			endpoints := resolveRBACAdminEndpoints(rbacAdminEndpointAdapter{handlers: handlers}, newRBACAdminRuntime(handlers))
			for _, query := range []string{"org_id=%zz", "org_id=default&org_id=other", "org_id="} {
				rec := httptest.NewRecorder()
				endpoints.HandleIntegrityCheck(rec, httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity?"+query, nil))
				if rec.Code != http.StatusBadRequest || provider.ManagerCount() != 0 {
					t.Fatalf("fallback admitted %q: status/managers = %d/%d", query, rec.Code, provider.ManagerCount())
				}
			}
			rec := httptest.NewRecorder()
			endpoints.HandleIntegrityCheck(rec, httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity", nil))
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"healthy":true`) {
				t.Fatalf("fallback lost its ordinary check: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRBACIntegrityQueryPreservesUnavailableAndMethodChecks(t *testing.T) {
	for _, tt := range []struct {
		method string
		status int
	}{{http.MethodGet, http.StatusNotImplemented}, {http.MethodPost, http.StatusMethodNotAllowed}} {
		rec := httptest.NewRecorder()
		NewRBACHandlers(nil, nil).HandleRBACIntegrityCheck(rec, httptest.NewRequest(tt.method, "/api/admin/rbac/integrity?org_id=%zz", nil))
		if rec.Code != tt.status {
			t.Errorf("%s without provider = %d, want %d", tt.method, rec.Code, tt.status)
		}
	}
}
