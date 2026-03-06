package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// SnapshotProvider is a type alias for models.SnapshotProvider.
// Kept for local convenience; all new code should use models.SnapshotProvider directly.
type SnapshotProvider = models.SnapshotProvider

// TenantStateSnapshotProvider exposes the legacy state snapshot bridge for an org.
// New API seams should prefer tenant-scoped unified read-state and unified-resource
// snapshots over direct StateSnapshot access.
type TenantStateSnapshotProvider interface {
	GetStateForTenant(orgID string) models.StateSnapshot
}

// TenantUnifiedReadStateProvider provides canonical tenant-scoped typed read access.
type TenantUnifiedReadStateProvider interface {
	UnifiedReadStateForTenant(orgID string) unified.ReadState
}

// TenantStateProvider is the API's tenant-scoped resource access contract.
// Unified read-state and unified-resource snapshots are the primary abstractions;
// GetStateForTenant remains only as a compatibility bridge for legacy consumers.
type TenantStateProvider interface {
	TenantUnifiedReadStateProvider
	TenantUnifiedResourceSnapshotProvider
	TenantStateSnapshotProvider
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

// SnapshotReadState bridges a legacy StateSnapshot into the canonical unified
// read-state abstraction for callers that have not yet been moved off snapshots.
func SnapshotReadState(snapshot models.StateSnapshot) unified.ReadState {
	registry := unified.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	return unified.NewMonitorAdapter(registry)
}
