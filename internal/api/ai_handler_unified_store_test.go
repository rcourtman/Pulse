package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
)

func TestAIHandler_UnifiedStoreIsOrgScoped(t *testing.T) {
	handler := &AIHandler{}

	defaultStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	tenantStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())

	handler.SetUnifiedStoreForOrg("default", defaultStore)
	handler.SetUnifiedStoreForOrg("acme", tenantStore)

	if got := handler.GetUnifiedStoreForOrg(""); got != defaultStore {
		t.Fatalf("expected default store for empty org, got %#v", got)
	}
	if got := handler.GetUnifiedStoreForOrg("acme"); got != tenantStore {
		t.Fatalf("expected acme store, got %#v", got)
	}
	if got := handler.GetUnifiedStoreForOrg("other"); got != nil {
		t.Fatalf("expected nil for unrelated org, got %#v", got)
	}
}

func TestAIHandler_RemoveTenantService_ClearsUnifiedStoreWithoutChatService(t *testing.T) {
	handler := &AIHandler{}

	tenantStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	handler.SetUnifiedStoreForOrg("acme", tenantStore)

	if err := handler.RemoveTenantService(context.Background(), "acme"); err != nil {
		t.Fatalf("RemoveTenantService returned error: %v", err)
	}
	if got := handler.GetUnifiedStoreForOrg("acme"); got != nil {
		t.Fatalf("expected acme unified store to be cleared, got %#v", got)
	}
}
