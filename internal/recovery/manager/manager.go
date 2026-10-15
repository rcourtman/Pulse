package manager

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverystore "github.com/rcourtman/pulse-go-rewrite/internal/recovery/store"
)

// Manager owns per-org recovery stores and keeps a small cache of opened DB handles.
// This avoids opening multiple sqlite connections to the same file across the process.
type Manager struct {
	multiTenant *config.MultiTenantPersistence
	pointSource PointsSource

	mu     sync.Mutex
	stores map[string]*recoverystore.Store
}

// PointsSource supplies read-only recovery evidence from an alternative runtime
// source, such as the canonical demo graph. It never replaces persisted writes.
type PointsSource interface {
	Active() bool
	ListPoints(context.Context, string, recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error)
}

func New(mtp *config.MultiTenantPersistence) *Manager {
	return NewWithPointSource(mtp, nil)
}

// NewWithPointSource binds the read source once, before runtime consumers share
// the manager. Inactive sources defer to the manager's same org-scoped store.
func NewWithPointSource(mtp *config.MultiTenantPersistence, source PointsSource) *Manager {
	return &Manager{
		multiTenant: mtp,
		pointSource: source,
		stores:      make(map[string]*recoverystore.Store),
	}
}

// ListPointsForOrg is the shared API/tool boundary for recovery point reads.
func (m *Manager) ListPointsForOrg(ctx context.Context, orgID string, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	if m == nil {
		return nil, 0, fmt.Errorf("recovery manager is not configured")
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, 0, fmt.Errorf("org ID is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if m.pointSource != nil && m.pointSource.Active() {
		return m.pointSource.ListPoints(ctx, orgID, opts)
	}
	store, err := m.StoreForOrg(orgID)
	if err != nil {
		return nil, 0, err
	}
	return store.ListPoints(ctx, opts)
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
