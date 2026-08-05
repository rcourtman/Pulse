package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// sqliteSidecarsPresent reports whether SQLite's -wal/-shm sidecars exist, which
// they only do while a connection is open. They are the observable signal that a
// store handle was or was not released.
func sqliteSidecarsPresent(t *testing.T, dataDir, orgID string) bool {
	t.Helper()
	base := filepath.Join(dataDir, "orgs", orgID, "resources", "unified_resources.db")
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(base + suffix); err == nil {
			return true
		}
	}
	return false
}

// getStore opens a SQLite handle per org and caches it for the process
// lifetime. Offboarding a tenant must release it, or the handle, its file
// descriptors, and its -wal/-shm files outlive the tenant.
func TestResourceHandlers_CloseTenantStoreReleasesTheHandle(t *testing.T) {
	dataDir := t.TempDir()
	handlers := NewResourceHandlers(&config.Config{DataPath: dataDir})
	t.Cleanup(func() { _ = handlers.CloseStores() })

	if _, err := handlers.getStore("client-x"); err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if !sqliteSidecarsPresent(t, dataDir, "client-x") {
		t.Skip("SQLite did not create -wal/-shm sidecars; nothing observable to assert")
	}

	if err := handlers.CloseTenantStore("client-x"); err != nil {
		t.Fatalf("CloseTenantStore: %v", err)
	}
	if sqliteSidecarsPresent(t, dataDir, "client-x") {
		t.Fatal("-wal/-shm survived CloseTenantStore; the handle was not released")
	}

	handlers.storeMu.Lock()
	_, cached := handlers.stores[cacheKey("client-x")]
	handlers.storeMu.Unlock()
	if cached {
		t.Fatal("closed store is still cached; a later request would use a closed handle")
	}
}

func TestResourceHandlers_CloseStoresReleasesEveryTenant(t *testing.T) {
	dataDir := t.TempDir()
	handlers := NewResourceHandlers(&config.Config{DataPath: dataDir})

	for _, orgID := range []string{"client-a", "client-b"} {
		if _, err := handlers.getStore(orgID); err != nil {
			t.Fatalf("getStore(%s): %v", orgID, err)
		}
	}
	if err := handlers.CloseStores(); err != nil {
		t.Fatalf("CloseStores: %v", err)
	}

	for _, orgID := range []string{"client-a", "client-b"} {
		if sqliteSidecarsPresent(t, dataDir, orgID) {
			t.Errorf("-wal/-shm survived CloseStores for %s", orgID)
		}
	}
	handlers.storeMu.Lock()
	remaining := len(handlers.stores)
	handlers.storeMu.Unlock()
	if remaining != 0 {
		t.Fatalf("%d store(s) still cached after CloseStores", remaining)
	}
}

// Closing must be safe to repeat and safe on a nil handler: the shutdown path
// runs before every dependency is guaranteed to be wired.
func TestResourceHandlers_CloseIsIdempotentAndNilSafe(t *testing.T) {
	var nilHandlers *ResourceHandlers
	if err := nilHandlers.CloseStores(); err != nil {
		t.Fatalf("nil handler CloseStores: %v", err)
	}
	if err := nilHandlers.CloseTenantStore("client-a"); err != nil {
		t.Fatalf("nil handler CloseTenantStore: %v", err)
	}

	handlers := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	if _, err := handlers.getStore("client-a"); err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := handlers.CloseStores(); err != nil {
		t.Fatalf("first CloseStores: %v", err)
	}
	if err := handlers.CloseStores(); err != nil {
		t.Fatalf("second CloseStores must be a no-op: %v", err)
	}
	if err := handlers.CloseTenantStore("client-a"); err != nil {
		t.Fatalf("CloseTenantStore on an evicted org must be a no-op: %v", err)
	}
}
