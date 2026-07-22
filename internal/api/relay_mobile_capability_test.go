package api

import (
	"crypto/sha256"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRelayMobileRuntimeProjectionMatchesManifestDigest(t *testing.T) {
	manifest, err := os.ReadFile("../../docs/release-control/v6/internal/MOBILE_COMPATIBILITY_MANIFEST.json")
	if err != nil {
		t.Fatalf("read mobile compatibility manifest: %v", err)
	}
	generated, err := os.ReadFile("relay_mobile_capability_generated.go")
	if err != nil {
		t.Fatalf("read generated mobile route projection: %v", err)
	}

	digest := fmt.Sprintf("%x", sha256.Sum256(manifest))
	if !strings.Contains(string(generated), "Source SHA256: "+digest) {
		t.Fatalf("generated mobile route projection does not match manifest digest %s", digest)
	}
}

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
		wantLen := 2
		if spec.legacyScope != "" && spec.legacyScope != spec.requiredScope {
			wantLen++
		}
		if len(scopes) != wantLen {
			t.Fatalf("compatible scopes for %q = %d, want %d", spec.id, len(scopes), wantLen)
		}
		if scopes[0] != config.ScopeRelayMobileAccess {
			t.Fatalf("compatible scopes for %q start with %q, want %q", spec.id, scopes[0], config.ScopeRelayMobileAccess)
		}
		if scopes[1] != spec.requiredScope {
			t.Fatalf("compatible scopes for %q primary scope = %q, want %q", spec.id, scopes[1], spec.requiredScope)
		}
		if wantLen == 3 && scopes[2] != spec.legacyScope {
			t.Fatalf("compatible scopes for %q legacy scope = %q, want %q", spec.id, scopes[2], spec.legacyScope)
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
		"GET /api/actions/pending => actions:approve",
		"GET /api/actions => actions:approve",
		"GET /api/actions/{action_id} => actions:approve",
		"POST /api/actions/{action_id}/decision => actions:approve",
		"POST /api/actions/{action_id}/execute => actions:execute",
		"POST /api/ai/chat => ai:chat",
		"GET /api/ai/sessions => ai:chat",
		"POST /api/ai/sessions => ai:chat",
		"GET /api/ai/sessions/{session_id}/messages => ai:chat",
		"POST /api/ai/sessions/{session_id}/abort => ai:chat",
		"PATCH /api/ai/sessions/{session_id} => ai:chat",
		"DELETE /api/ai/sessions/{session_id} => ai:chat",
		"GET /api/ai/patrol/attention => monitoring:read",
		"GET /api/ai/patrol/attention/{item_id} => monitoring:read",
		"POST /api/ai/patrol/attention/{item_id}/{mutation} => monitoring:write",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("relay mobile runtime route inventory = %#v, want %#v", got, want)
	}
}
