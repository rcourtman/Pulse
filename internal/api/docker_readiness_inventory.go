package api

import (
	"context"
	"strings"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type dockerReadinessInventoryKey struct {
	resources *ResourceHandlers
	orgID     string
}

type dockerReadinessInventory struct {
	registry    *unified.ResourceRegistry
	reportHosts dockerReportHostIndex
}

// Only the read-only availability projection supplies a memo. Planning,
// preflight, and dispatch use the current registry on every check as before.
func (e dockerContainerActionExecutor) dockerReadinessRegistry(ctx context.Context) (*unified.ResourceRegistry, *dockerReadinessInventory) {
	if e.resources == nil {
		return nil, nil
	}
	key := dockerReadinessInventoryKey{resources: e.resources, orgID: GetOrgID(ctx)}
	memo := agentLookupMemoForContext(ctx)
	if memo != nil {
		if inventory := memo.dockerInventories[key]; inventory != nil {
			return inventory.registry, inventory
		}
	}
	registry, err := e.resources.buildRegistry(key.orgID)
	if err != nil {
		return nil, nil // Do not freeze a failed lookup for the rest of the pass.
	}
	if memo == nil {
		return registry, nil
	}
	if memo.dockerInventories == nil {
		memo.dockerInventories = make(map[dockerReadinessInventoryKey]*dockerReadinessInventory)
	}
	inventory := &dockerReadinessInventory{registry: registry}
	memo.dockerInventories[key] = inventory
	return registry, inventory
}

type dockerReportHostKey struct {
	tokenID string
	agentID string // Empty for the token-wide entry.
}

// Readiness needs to know only whether a second distinct host was observed,
// not how many rows (including containers) reported for each host.
type dockerReportHostCount struct {
	first    string
	multiple bool
}

type dockerReportHostIndex map[dockerReportHostKey]dockerReportHostCount

func (index dockerReportHostIndex) add(key dockerReportHostKey, host string) {
	entry := index[key]
	if entry.first == "" {
		entry.first = host
	} else if entry.first != host {
		entry.multiple = true
	}
	index[key] = entry
}

// Build from the canonical tenant registry, not from the filtered or
// coalesced rows being presented. Rows outside the projection still count.
func indexDockerReportHosts(resources []unified.Resource, onlyToken string) dockerReportHostIndex {
	index := make(dockerReportHostIndex)
	for i := range resources {
		docker := resources[i].Docker
		if docker == nil {
			continue
		}
		tokenID := strings.TrimSpace(docker.TokenID)
		hostID := strings.TrimSpace(docker.HostSourceID)
		if tokenID == "" || hostID == "" || (onlyToken != "" && tokenID != onlyToken) {
			continue
		}
		index.add(dockerReportHostKey{tokenID: tokenID}, hostID)
		if agentID := strings.TrimSpace(docker.AgentID); agentID != "" {
			index.add(dockerReportHostKey{tokenID: tokenID, agentID: agentID}, hostID)
		}
	}
	return index
}

func (index dockerReportHostIndex) ambiguous(tokenID, agentID string, singleTokenSession bool) bool {
	return (agentID != "" && index[dockerReportHostKey{tokenID: tokenID, agentID: agentID}].multiple) ||
		(singleTokenSession && index[dockerReportHostKey{tokenID: tokenID}].multiple)
}
