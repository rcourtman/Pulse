package api

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// StateProvider provides a current state snapshot.
type StateProvider interface {
	GetState() models.StateSnapshot
}

// TenantStateProvider provides a current state snapshot scoped to an org.
type TenantStateProvider interface {
	GetStateForTenant(orgID string) models.StateSnapshot
}
