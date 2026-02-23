package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestResourceHandlers_GetStore_DotAndUnderscoreOrgIDsRemainIsolated(t *testing.T) {
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})

	dotStore, err := h.getStore("org.a")
	if err != nil {
		t.Fatalf("getStore(org.a) returned error: %v", err)
	}
	t.Cleanup(func() { _ = dotStore.Close() })

	underscoreStore, err := h.getStore("org_a")
	if err != nil {
		t.Fatalf("getStore(org_a) returned error: %v", err)
	}
	t.Cleanup(func() { _ = underscoreStore.Close() })

	if dotStore == underscoreStore {
		t.Fatal("expected separate store instances for org.a and org_a")
	}

	if err := dotStore.AddLink(unified.ResourceLink{
		ResourceA: "resource-dot-a",
		ResourceB: "resource-dot-b",
		PrimaryID: "resource-dot-a",
		Reason:    "isolation test",
		CreatedBy: "tester",
	}); err != nil {
		t.Fatalf("dotStore.AddLink returned error: %v", err)
	}

	underscoreLinks, err := underscoreStore.GetLinks()
	if err != nil {
		t.Fatalf("underscoreStore.GetLinks returned error: %v", err)
	}
	if len(underscoreLinks) != 0 {
		t.Fatalf("underscore store links length = %d, want 0", len(underscoreLinks))
	}
}
