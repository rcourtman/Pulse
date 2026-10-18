package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// unresolvableTenantMonitor returns a tenant manager with no persistence, so
// every organization lookup it is asked for fails.
func unresolvableTenantMonitor() *monitoring.MultiTenantMonitor {
	return &monitoring.MultiTenantMonitor{}
}

func tenantContext(orgID string) context.Context {
	return context.WithValue(context.Background(), OrgIDContextKey, orgID)
}

func TestAgentHandlersGetMonitor_NonDefaultOrgFailsClosed(t *testing.T) {
	defaultMonitor := &monitoring.Monitor{}

	t.Run("tenant resolution error", func(t *testing.T) {
		base := newBaseAgentHandlers(unresolvableTenantMonitor(), defaultMonitor, nil)
		if got := base.getMonitor(tenantContext("tenant-a")); got != nil {
			t.Fatalf("expected nil monitor for unresolvable tenant, got default-org fallback")
		}
	})

	t.Run("no tenant manager", func(t *testing.T) {
		base := newBaseAgentHandlers(nil, defaultMonitor, nil)
		if got := base.getMonitor(tenantContext("tenant-a")); got != nil {
			t.Fatalf("expected nil monitor without a tenant manager, got default-org fallback")
		}
	})

	t.Run("default org keeps primary monitor", func(t *testing.T) {
		base := newBaseAgentHandlers(unresolvableTenantMonitor(), defaultMonitor, nil)
		for _, ctx := range []context.Context{context.Background(), tenantContext("default"), tenantContext("")} {
			if got := base.getMonitor(ctx); got != defaultMonitor {
				t.Fatalf("expected default monitor for default org, got %p", got)
			}
		}
	})
}

func TestAgentHandlers_UnresolvableTenantReturns503(t *testing.T) {
	defaultMonitor := &monitoring.Monitor{}

	cases := []struct {
		name    string
		path    string
		handler http.HandlerFunc
	}{
		{
			name:    "unified agent",
			path:    agentRoutePrefix + "agent-1/allow-reenroll",
			handler: NewUnifiedAgentHandlers(unresolvableTenantMonitor(), defaultMonitor, nil).HandleAllowReenroll,
		},
		{
			name:    "docker runtime",
			path:    "/api/agents/docker/runtimes/host-1/allow-reenroll",
			handler: NewDockerAgentHandlers(unresolvableTenantMonitor(), defaultMonitor, nil, nil).HandleAllowReenroll,
		},
		{
			name:    "kubernetes cluster",
			path:    "/api/agents/kubernetes/clusters/cluster-1/allow-reenroll",
			handler: NewKubernetesAgentHandlers(unresolvableTenantMonitor(), defaultMonitor, nil).HandleAllowReenroll,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil).WithContext(tenantContext("tenant-a"))
			rec := httptest.NewRecorder()
			tc.handler(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if body.Code != "tenant_unavailable" {
				t.Fatalf("expected tenant_unavailable, got %q", body.Code)
			}
		})
	}
}

func TestDockerAgentHandlers_LifecycleActionUnresolvableTenantReturns503(t *testing.T) {
	handler := NewDockerAgentHandlers(unresolvableTenantMonitor(), &monitoring.Monitor{}, nil, nil)
	req := httptest.NewRequest(http.MethodPut, "/api/agents/docker/runtimes/host-1/unhide", nil).
		WithContext(tenantContext("tenant-a"))
	rec := httptest.NewRecorder()

	handler.HandleUnhideHost(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSystemSettingsHandler_GetMonitor_NoTenantManagerFailsClosed(t *testing.T) {
	handler := &SystemSettingsHandler{defaultMonitor: &mockMonitor{}}

	if got := handler.getMonitor(tenantContext("acme")); got != nil {
		t.Fatalf("expected nil monitor without a tenant manager, got %#v", got)
	}
}
