package api

//go:generate python3 ../../scripts/release_control/generate_mobile_compatibility.py --write

import (
	"fmt"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type relayMobileRuntimeRouteID string

type relayMobileRuntimeRouteSpec struct {
	id            relayMobileRuntimeRouteID
	method        string
	path          string
	requiredScope string
	legacyScope   string
}

func relayMobileRuntimeRouteInventory() []relayMobileRuntimeRouteSpec {
	inventory := make([]relayMobileRuntimeRouteSpec, 0, len(relayMobileRuntimeRouteOrder))
	for _, routeID := range relayMobileRuntimeRouteOrder {
		inventory = append(inventory, relayMobileRuntimeRouteSpecFor(routeID))
	}
	return inventory
}

func relayMobileRuntimeRouteSpecFor(routeID relayMobileRuntimeRouteID) relayMobileRuntimeRouteSpec {
	spec, ok := relayMobileRuntimeRouteSpecs[routeID]
	if !ok {
		panic(fmt.Sprintf("unknown relay mobile runtime route %q", routeID))
	}
	return spec
}

func (spec relayMobileRuntimeRouteSpec) compatibleScopes() []string {
	scopes := []string{config.ScopeRelayMobileAccess, spec.requiredScope}
	if spec.legacyScope != "" && spec.legacyScope != spec.requiredScope {
		scopes = append(scopes, spec.legacyScope)
	}
	return scopes
}

func requireRelayMobileRuntimeRoute(routeID relayMobileRuntimeRouteID, handler http.HandlerFunc) http.HandlerFunc {
	return RequireAnyScope(relayMobileRuntimeRouteSpecFor(routeID).compatibleScopes(), handler)
}

func ensureRelayMobileRuntimeRoute(w http.ResponseWriter, r *http.Request, routeID relayMobileRuntimeRouteID) bool {
	return ensureAnyScope(w, r, relayMobileRuntimeRouteSpecFor(routeID).compatibleScopes()...)
}
