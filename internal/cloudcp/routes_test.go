package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestRegisterRoutes_AccountAndTenantMethodDispatch(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry: reg,
		Version:  "test",
	})

	tests := []struct {
		name string
		path string
		want int
	}{
		// /members collection handler: GET/POST supported, others rejected.
		{name: "members-get-dispatches", path: "/api/accounts/acct_1/members", want: http.StatusNotFound},
		{name: "members-post-dispatches", path: "/api/accounts/acct_1/members", want: http.StatusNotFound},
		{name: "members-put-rejected", path: "/api/accounts/acct_1/members", want: http.StatusMethodNotAllowed},

		// /members/{user_id} handler: PATCH/DELETE supported, others rejected.
		{name: "member-patch-dispatches", path: "/api/accounts/acct_1/members/user_1", want: http.StatusNotFound},
		{name: "member-delete-dispatches", path: "/api/accounts/acct_1/members/user_1", want: http.StatusNotFound},
		{name: "member-get-rejected", path: "/api/accounts/acct_1/members/user_1", want: http.StatusMethodNotAllowed},

		// /tenants collection handler: GET/POST supported, others rejected.
		{name: "tenants-get-dispatches", path: "/api/accounts/acct_1/tenants", want: http.StatusNotFound},
		{name: "tenants-post-dispatches", path: "/api/accounts/acct_1/tenants", want: http.StatusNotFound},
		{name: "tenants-put-rejected", path: "/api/accounts/acct_1/tenants", want: http.StatusMethodNotAllowed},

		// /tenants/{tenant_id} handler: PATCH/DELETE supported, others rejected.
		{name: "tenant-patch-dispatches", path: "/api/accounts/acct_1/tenants/tenant_1", want: http.StatusNotFound},
		{name: "tenant-delete-dispatches", path: "/api/accounts/acct_1/tenants/tenant_1", want: http.StatusNotFound},
		{name: "tenant-get-rejected", path: "/api/accounts/acct_1/tenants/tenant_1", want: http.StatusMethodNotAllowed},
	}

	methodFor := map[string]string{
		"members-get-dispatches":   http.MethodGet,
		"members-post-dispatches":  http.MethodPost,
		"members-put-rejected":     http.MethodPut,
		"member-patch-dispatches":  http.MethodPatch,
		"member-delete-dispatches": http.MethodDelete,
		"member-get-rejected":      http.MethodGet,
		"tenants-get-dispatches":   http.MethodGet,
		"tenants-post-dispatches":  http.MethodPost,
		"tenants-put-rejected":     http.MethodPut,
		"tenant-patch-dispatches":  http.MethodPatch,
		"tenant-delete-dispatches": http.MethodDelete,
		"tenant-get-rejected":      http.MethodGet,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(methodFor[tt.name], tt.path, nil)
			req.Header.Set("X-Admin-Key", "test-admin-key")

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("%s %s status = %d, want %d (body=%q)",
					methodFor[tt.name], tt.path, rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}
