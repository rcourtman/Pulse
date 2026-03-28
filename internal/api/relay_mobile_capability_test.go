package api

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRelayMobileRuntimeRouteInventory(t *testing.T) {
	got := make([]string, 0, len(relayMobileRuntimeRouteOrder))
	seenIDs := make(map[relayMobileRuntimeRouteID]struct{}, len(relayMobileRuntimeRouteOrder))
	seenRoutes := make(map[string]struct{}, len(relayMobileRuntimeRouteOrder))

	for _, spec := range relayMobileRuntimeRouteInventory() {
		if _, exists := seenIDs[spec.id]; exists {
			t.Fatalf("duplicate relay mobile runtime route id %q", spec.id)
		}
		seenIDs[spec.id] = struct{}{}

		scopes := spec.compatibleScopes()
		if len(scopes) != 2 {
			t.Fatalf("compatible scopes for %q = %d, want 2", spec.id, len(scopes))
		}
		if scopes[0] != config.ScopeRelayMobileAccess {
			t.Fatalf("compatible scopes for %q start with %q, want %q", spec.id, scopes[0], config.ScopeRelayMobileAccess)
		}
		if scopes[1] != spec.requiredScope {
			t.Fatalf("compatible scopes for %q end with %q, want %q", spec.id, scopes[1], spec.requiredScope)
		}

		route := fmt.Sprintf("%s %s => %s", spec.method, spec.path, spec.requiredScope)
		if _, exists := seenRoutes[route]; exists {
			t.Fatalf("duplicate relay mobile runtime route %q", route)
		}
		seenRoutes[route] = struct{}{}
		got = append(got, route)
	}

	want := []string{
		"GET /api/onboarding/qr => settings:read",
		"POST /api/onboarding/validate => settings:read",
		"GET /api/onboarding/deep-link => settings:read",
		"GET /api/ai/patrol/findings => ai:execute",
		"GET /api/ai/findings/{finding_id}/investigation => ai:execute",
		"GET /api/ai/findings/{finding_id}/investigation/messages => ai:execute",
		"POST /api/ai/patrol/acknowledge => ai:execute",
		"POST /api/ai/patrol/dismiss => ai:execute",
		"POST /api/ai/patrol/snooze => ai:execute",
		"GET /api/ai/approvals => ai:execute",
		"POST /api/ai/approvals/{approval_id}/approve => ai:execute",
		"POST /api/ai/approvals/{approval_id}/deny => ai:execute",
		"POST /api/ai/chat => ai:chat",
		"GET /api/ai/sessions => ai:chat",
		"POST /api/ai/sessions => ai:chat",
		"GET /api/ai/sessions/{session_id}/messages => ai:chat",
		"POST /api/ai/sessions/{session_id}/abort => ai:chat",
		"DELETE /api/ai/sessions/{session_id} => ai:chat",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("relay mobile runtime route inventory = %#v, want %#v", got, want)
	}
}
