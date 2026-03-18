package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

func TestTenantMiddleware_UsesOrgBoundTokenWhenNoOrgSelected(t *testing.T) {
	// Attach an org-bound token to request context (simulates AuthContextMiddleware).
	record, err := config.NewAPITokenRecord("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "bound", []string{config.ScopeMonitoringRead})
	require.NoError(t, err)
	record.OrgID = "acme"

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req = req.WithContext(auth.WithAPIToken(req.Context(), record))

	got := resolveTenantOrgID(req)
	require.Equal(t, "acme", got)
}
