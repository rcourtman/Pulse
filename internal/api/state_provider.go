package api

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// SnapshotProvider provides a current state snapshot.
// This is the migration target for the legacy StateProvider interface.
// Implementors return a snapshot via ReadSnapshot() (not GetState()).
type SnapshotProvider interface {
	ReadSnapshot() models.StateSnapshot
}

// TenantStateProvider provides a current state snapshot scoped to an org.
type TenantStateProvider interface {
	GetStateForTenant(orgID string) models.StateSnapshot
}
