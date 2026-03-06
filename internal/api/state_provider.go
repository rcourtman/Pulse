package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// SnapshotProvider is a type alias for models.SnapshotProvider.
// Kept for local convenience; all new code should use models.SnapshotProvider directly.
type SnapshotProvider = models.SnapshotProvider

// TenantStateProvider provides a current state snapshot scoped to an org.
type TenantStateProvider interface {
	GetStateForTenant(orgID string) models.StateSnapshot
}

// UnifiedResourceSnapshotProvider provides a canonical unified-resource seed
// plus a freshness marker suitable for registry caching.
type UnifiedResourceSnapshotProvider interface {
	UnifiedResourceSnapshot() ([]unified.Resource, time.Time)
}

// TenantUnifiedResourceSnapshotProvider provides tenant-scoped unified-resource
// seeds plus a freshness marker suitable for registry caching.
type TenantUnifiedResourceSnapshotProvider interface {
	UnifiedResourceSnapshotForTenant(orgID string) ([]unified.Resource, time.Time)
}
