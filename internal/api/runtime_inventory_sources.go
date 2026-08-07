package api

import (
	"net/http"
	"sort"
	"strings"
)

// RuntimeInventorySource is the narrow, monitoring-tier projection of one
// enabled inventory source. It exists so a non-admin viewer can be told which
// of their monitored sources is failing to deliver inventory — the question a
// monitoring surface has to answer — without handing them the administrative
// connection record.
//
// This is an explicit whitelist, not a filtered view of Connection. Every
// field is here because the workload inventory banner needs it. Nothing that
// says where a source lives or how Pulse authenticates to it has a field to
// travel in: no address, host aliases, agent identity, agent or token IDs,
// agent versions, fleet governance, capabilities, last-seen timestamps, or
// raw error text. Widening this type is a deliberate disclosure decision, and
// it must never be made privilege-dependent — every session that can read
// monitoring data gets exactly these fields.
type RuntimeInventorySource struct {
	ID    string          `json:"id"`
	Type  ConnectionType  `json:"type"`
	Name  string          `json:"name"`
	State ConnectionState `json:"state"`
	// Surfaces is the effective inventory coverage for this source: the
	// enabled scope keys when a scope is configured, otherwise the declared
	// surfaces. Collapsing the two server-side keeps the client from having to
	// understand the scope-overrides-surfaces rule.
	Surfaces []string `json:"surfaces"`
	// CredentialsInvalid collapses the several fleet credential signals into
	// the one fact a viewer can act on: this source needs an administrator to
	// fix its credentials. The fleet governance block itself stays admin-only.
	CredentialsInvalid bool `json:"credentialsInvalid"`
}

// RuntimeInventorySourcesResponse is the envelope for
// GET /api/runtime/inventory-sources.
type RuntimeInventorySourcesResponse struct {
	Sources []RuntimeInventorySource `json:"sources"`
}

// runtimeInventorySourceSurfaces resolves the effective coverage list for a
// connection. A configured scope wins over the declared surfaces so a source
// narrowed to, say, containers only does not advertise VM coverage.
func runtimeInventorySourceSurfaces(conn Connection) []string {
	scoped := make([]string, 0, len(conn.Scope))
	for surface, enabled := range conn.Scope {
		if !enabled {
			continue
		}
		if trimmed := strings.TrimSpace(surface); trimmed != "" {
			scoped = append(scoped, trimmed)
		}
	}
	if len(scoped) > 0 {
		sort.Strings(scoped)
		return scoped
	}

	declared := make([]string, 0, len(conn.Surfaces))
	for _, surface := range conn.Surfaces {
		if trimmed := strings.TrimSpace(surface); trimmed != "" {
			declared = append(declared, trimmed)
		}
	}
	sort.Strings(declared)
	return declared
}

// runtimeInventorySourceCredentialsInvalid reports whether this source's
// credentials are the reason inventory is not flowing. It mirrors the
// administrative credential signals without exposing them.
func runtimeInventorySourceCredentialsInvalid(conn Connection) bool {
	if conn.State == ConnectionStateUnauthorized {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(conn.Fleet.CredentialStatus), "invalid") {
		return true
	}
	if health := conn.Fleet.CredentialHealth; health != nil {
		switch strings.ToLower(strings.TrimSpace(health.Status)) {
		case "invalid", "expired":
			return true
		}
	}
	return false
}

// runtimeInventorySources projects the administrative connection ledger down
// to the monitoring-tier whitelist. Disabled sources are dropped entirely: a
// source an administrator switched off is not part of the viewer's monitoring
// picture, so its name never reaches them.
func runtimeInventorySources(connections []Connection) []RuntimeInventorySource {
	sources := make([]RuntimeInventorySource, 0, len(connections))
	for _, conn := range connections {
		if !conn.Enabled {
			continue
		}
		sources = append(sources, RuntimeInventorySource{
			ID:                 conn.ID,
			Type:               conn.Type,
			Name:               conn.Name,
			State:              conn.State,
			Surfaces:           runtimeInventorySourceSurfaces(conn),
			CredentialsInvalid: runtimeInventorySourceCredentialsInvalid(conn),
		})
	}
	return sources
}

// HandleRuntimeInventorySources serves the monitoring-tier inventory source
// health projection. The route is monitoring:read rather than admin because
// the workload surfaces it feeds are surfaces a viewer is meant to use; a
// viewer who cannot be told that their VM inventory is stale because vCenter
// is unreachable is looking at a silently wrong page.
//
// The agent command-session enrichment HandleList performs is deliberately
// skipped here — it only refines fleet remote-control state, which this
// projection does not carry.
func (h *ConnectionsHandlers) HandleRuntimeInventorySources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}
	if h == nil {
		writeJSON(w, http.StatusOK, RuntimeInventorySourcesResponse{Sources: []RuntimeInventorySource{}})
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
