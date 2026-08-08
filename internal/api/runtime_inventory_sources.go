package api

import (
	"net/http"
	"sort"
	"strings"
)

var runtimeInventoryWorkloadTypes = map[ConnectionType]struct{}{
	ConnectionTypePVE:        {},
	ConnectionTypeVMware:     {},
	ConnectionTypeDocker:     {},
	ConnectionTypeKubernetes: {},
}

var runtimeInventoryWorkloadSurfaces = map[string]struct{}{
	"containers": {},
	"docker":     {},
	"kubernetes": {},
	"pods":       {},
	"vms":        {},
}

var runtimeInventoryBlockingStates = map[ConnectionState]struct{}{
	ConnectionStatePaused:       {},
	ConnectionStatePending:      {},
	ConnectionStateStale:        {},
	ConnectionStateUnauthorized: {},
	ConnectionStateUnreachable:  {},
}

// RuntimeInventorySource is the complete monitoring-tier wire shape for one
// enabled source currently blocking workload inventory. It is deliberately a
// standalone whitelist rather than a serialized or embedded Connection.
//
// Name is the operator-facing source label needed to identify the problem.
// State is normalized to unauthorized when any cached credential signal says
// the credentials are invalid. Surfaces contains only workload coverage labels.
// No source locator, stable connection ID, raw error, timestamp, agent identity,
// fleet policy, capability, credential, or mutation field can cross this type.
type RuntimeInventorySource struct {
	Type     ConnectionType  `json:"type"`
	Name     string          `json:"name"`
	State    ConnectionState `json:"state"`
	Surfaces []string        `json:"surfaces"`
}

type RuntimeInventorySourcesResponse struct {
	Sources []RuntimeInventorySource `json:"sources"`
}

func runtimeInventorySourceCredentialsInvalid(connection Connection) bool {
	if connection.State == ConnectionStateUnauthorized {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(connection.Fleet.CredentialStatus), "invalid") {
		return true
	}
	if health := connection.Fleet.CredentialHealth; health != nil {
		switch strings.ToLower(strings.TrimSpace(health.Status)) {
		case "expired", "invalid":
			return true
		}
	}
	return false
}

// runtimeInventorySourceSurfaces resolves scope-over-declared-surfaces on the
// server, then drops every non-workload label before the projection crosses the
// monitoring boundary.
func runtimeInventorySourceSurfaces(connection Connection) []string {
	selected := make([]string, 0, len(connection.Scope))
	for surface, enabled := range connection.Scope {
		if enabled {
			selected = append(selected, surface)
		}
	}
	if len(selected) == 0 {
		selected = append(selected, connection.Surfaces...)
	}

	seen := make(map[string]struct{}, len(selected))
	workloadSurfaces := make([]string, 0, len(selected))
	for _, surface := range selected {
		surface = strings.ToLower(strings.TrimSpace(surface))
		if _, allowed := runtimeInventoryWorkloadSurfaces[surface]; !allowed {
			continue
		}
		if _, duplicate := seen[surface]; duplicate {
			continue
		}
		seen[surface] = struct{}{}
		workloadSurfaces = append(workloadSurfaces, surface)
	}
	sort.Strings(workloadSurfaces)
	return workloadSurfaces
}

// runtimeInventorySources projects only actionable workload-inventory health
// issues. Healthy, disabled, non-workload, and coverage-free administrative
// records never reach a viewer.
func runtimeInventorySources(connections []Connection) []RuntimeInventorySource {
	sources := make([]RuntimeInventorySource, 0, len(connections))
	for _, connection := range connections {
		if !connection.Enabled {
			continue
		}
		if _, workloadType := runtimeInventoryWorkloadTypes[connection.Type]; !workloadType {
			continue
		}

		surfaces := runtimeInventorySourceSurfaces(connection)
		if len(surfaces) == 0 {
			continue
		}

		state := connection.State
		if runtimeInventorySourceCredentialsInvalid(connection) {
			state = ConnectionStateUnauthorized
		}
		if _, blocking := runtimeInventoryBlockingStates[state]; !blocking {
			continue
		}

		sources = append(sources, RuntimeInventorySource{
			Type:     connection.Type,
			Name:     connection.Name,
			State:    state,
			Surfaces: surfaces,
		})
	}

	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Type != sources[j].Type {
			return sources[i].Type < sources[j].Type
		}
		return strings.ToLower(sources[i].Name) < strings.ToLower(sources[j].Name)
	})
	return sources
}

// HandleRuntimeInventorySources serves cached workload-inventory health only.
// It performs no probing or persistence. Missing handler dependencies fail
// closed instead of returning an empty response that would look healthy.
func (h *ConnectionsHandlers) HandleRuntimeInventorySources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}
	if h == nil || h.getConfig == nil || h.getPersistence == nil || h.getMonitor == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "inventory_sources_unavailable", "Inventory source health unavailable", nil)
		return
	}

	ctx := r.Context()
	inputs := buildAggregatorInputsWithRuntimeSources(
		ctx,
		h.getConfig(ctx),
		h.getPersistence(ctx),
		h.getMonitor(ctx),
		h.runtimeSources(ctx, resolveTenantOrgID(r)),
	)
	writeJSON(w, http.StatusOK, RuntimeInventorySourcesResponse{
		Sources: runtimeInventorySources(buildConnections(inputs)),
	})
}
