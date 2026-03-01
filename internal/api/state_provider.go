package api

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// SnapshotProvider is a type alias for models.SnapshotProvider.
// Kept for local convenience; all new code should use models.SnapshotProvider directly.
type SnapshotProvider = models.SnapshotProvider

// TenantStateProvider provides a current state snapshot scoped to an org.
type TenantStateProvider interface {
	GetStateForTenant(orgID string) models.StateSnapshot
}
