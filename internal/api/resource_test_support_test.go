package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These fixtures remain in the API integration-test package because action,
// agent-context, and router tests compose the public ResourceHandlers facade.
type resourceUnifiedSeedProvider struct {
	snapshot  models.StateSnapshot
	resources []unified.Resource
}

type resourceStateProvider struct {
	snapshot models.StateSnapshot
}

func (p resourceStateProvider) ReadSnapshot() models.StateSnapshot { return p.snapshot }

type tenantResourceStateProvider struct {
	snapshots map[string]models.StateSnapshot
}

func (p tenantResourceStateProvider) GetStateForTenant(orgID string) models.StateSnapshot {
	return p.snapshots[orgID]
}

func (p tenantResourceStateProvider) UnifiedReadStateForTenant(orgID string) unified.ReadState {
	return SnapshotReadState(p.GetStateForTenant(orgID))
}

func (p tenantResourceStateProvider) UnifiedResourceSnapshotForTenant(orgID string) ([]unified.Resource, time.Time) {
	snapshot := p.GetStateForTenant(orgID)
	if snapshot.LastUpdate.IsZero() {
		return nil, time.Time{}
	}
	return []unified.Resource{{
		ID: "agent-tenant-seeded", Type: unified.ResourceTypeAgent, Name: "tenant-seeded",
		Status: unified.StatusOnline, LastSeen: snapshot.LastUpdate, UpdatedAt: snapshot.LastUpdate,
		Sources:  []unified.DataSource{unified.SourceAgent},
		Identity: unified.ResourceIdentity{Hostnames: []string{"tenant-seeded"}},
	}}, snapshot.LastUpdate
}

func (p resourceUnifiedSeedProvider) ReadSnapshot() models.StateSnapshot { return p.snapshot }

func (p resourceUnifiedSeedProvider) UnifiedResourceSnapshot() ([]unified.Resource, time.Time) {
	return append([]unified.Resource(nil), p.resources...), p.snapshot.LastUpdate
}

type mutableResourceUnifiedSeedProvider struct {
	snapshot  models.StateSnapshot
	resources []unified.Resource
	freshness time.Time
}

func (p *mutableResourceUnifiedSeedProvider) ReadSnapshot() models.StateSnapshot { return p.snapshot }

func (p *mutableResourceUnifiedSeedProvider) UnifiedResourceSnapshot() ([]unified.Resource, time.Time) {
	return append([]unified.Resource(nil), p.resources...), p.freshness
}
