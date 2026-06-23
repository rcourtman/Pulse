package api

import (
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

type AgentCapability = agentcapabilities.Capability
type AgentCapabilitiesManifest = agentcapabilities.Manifest

// HandleAgentCapabilitiesManifest serves
// `GET /api/agent/capabilities` — the discovery document for Pulse's
// agent surface. Cacheable, unauthenticated (the underlying
// capabilities have their own scopes); agents fetch this once and
// learn what's available.
func HandleAgentCapabilitiesManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(agentcapabilities.CanonicalManifest())
}
