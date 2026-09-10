package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestConfiguredAdminAuthorizerSyncReplacesAndClearsIdentity(t *testing.T) {
	manager, err := auth.NewFileManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	authorizer := auth.NewRBACAuthorizer(manager)
	cfg := &config.Config{}
	router := &Router{config: cfg, authorizer: authorizer}
	for _, current := range []string{"original-admin", "replacement-admin", ""} {
		cfg.AuthUser = current
		router.syncConfiguredAdminAuthorizer()
		for _, user := range []string{"original-admin", "replacement-admin", "outsider"} {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			allowed, err := authorizer.Authorize(auth.WithUser(req.Context(), user), auth.ActionAdmin, auth.ResourceUsers)
			if err != nil || allowed != (user == current) {
				t.Fatalf("configured %q, user %q: allowed=%v err=%v", current, user, allowed, err)
			}
		}
	}
}
