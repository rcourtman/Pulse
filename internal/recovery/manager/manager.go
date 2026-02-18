package manager

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	recoverystore "github.com/rcourtman/pulse-go-rewrite/internal/recovery/store"
)

// Manager owns per-org recovery stores and keeps a small cache of opened DB handles.
// This avoids opening multiple sqlite connections to the same file across the process.
type Manager struct {
	multiTenant *config.MultiTenantPersistence

	mu     sync.Mutex
	stores map[string]*recoverystore.Store
}

func New(mtp *config.MultiTenantPersistence) *Manager {
	return &Manager{
		multiTenant: mtp,
		stores:      make(map[string]*recoverystore.Store),
	}
}

func (m *Manager) StoreForOrg(orgID string) (*recoverystore.Store, error) {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, fmt.Errorf("org ID is required")
	}

	m.mu.Lock()
	if store := m.stores[orgID]; store != nil {
		m.mu.Unlock()
		return store, nil
	}
	m.mu.Unlock()

	if m.multiTenant == nil {
		return nil, fmt.Errorf("multi-tenant persistence is not configured")
	}
	persistence, err := m.multiTenant.GetPersistence(orgID)
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(persistence.DataDir(), "recovery", "recovery.db")
	store, err := recoverystore.Open(dbPath)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if existing := m.stores[orgID]; existing != nil {
		m.mu.Unlock()
		_ = store.Close()
		return existing, nil
	}
	m.stores[orgID] = store
	m.mu.Unlock()

	return store, nil
}
