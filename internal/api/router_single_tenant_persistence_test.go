package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestNewRouter_V5SingleTenantConfigHandlersUseLegacyPersistence(t *testing.T) {
	prevSingleTenant := isV5SingleTenantMode()
	setV5SingleTenantModeForTests(true)
	t.Cleanup(func() { setV5SingleTenantModeForTests(prevSingleTenant) })

	cfg := &config.Config{DataPath: t.TempDir()}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	if router.configHandlers == nil {
		t.Fatal("expected config handlers to be initialized")
	}
	if router.configHandlers.legacyPersistence == nil {
		t.Fatal("expected legacy persistence to be wired in single-tenant mode")
	}
	if router.configHandlers.getPersistence(context.Background()) == nil {
		t.Fatal("expected getPersistence to return legacy persistence in single-tenant mode")
	}
}
