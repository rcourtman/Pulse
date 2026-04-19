package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// ConnectionsHandlers serves the unified connections ledger. It does not own
// any persistence of its own — it composes per-type stores and the
// monitoring scheduler's in-memory health data into a single list.
type ConnectionsHandlers struct {
	getConfig      func(ctx context.Context) *config.Config
	getPersistence func(ctx context.Context) *config.ConfigPersistence
	getMonitor     func(ctx context.Context) *monitoring.Monitor
}

// NewConnectionsHandlers wires the aggregator behind the request-scoped
// tenant resolvers already used by ConfigHandlers, so the endpoint shares
// the same multi-tenant behavior as the per-type routes it aggregates.
func NewConnectionsHandlers(
	getConfig func(ctx context.Context) *config.Config,
	getPersistence func(ctx context.Context) *config.ConfigPersistence,
	getMonitor func(ctx context.Context) *monitoring.Monitor,
) *ConnectionsHandlers {
	return &ConnectionsHandlers{
		getConfig:      getConfig,
		getPersistence: getPersistence,
		getMonitor:     getMonitor,
	}
}

// HandleList returns every configured connection as a unified Connection row.
// No probing or network I/O happens here — state is derived purely from
// cached poller health.
func (h *ConnectionsHandlers) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	cfg := h.getConfig(ctx)
	persistence := h.getPersistence(ctx)
	monitor := h.getMonitor(ctx)

	inputs := aggregatorInputs{}

	if cfg != nil {
		inputs.pveInstances = cfg.PVEInstances
		inputs.pbsInstances = cfg.PBSInstances
		inputs.pmgInstances = cfg.PMGInstances
	}

	if persistence != nil {
		if vmw, err := persistence.LoadVMwareConfig(); err == nil {
			inputs.vmwareInstances = vmw
		}
		if tn, err := persistence.LoadTrueNASConfig(); err == nil {
			inputs.truenasInstances = tn
		}
	}

	if monitor != nil {
		snapshot := monitor.GetState()
		inputs.hosts = snapshot.Hosts
		inputs.instanceHealth = instanceHealthByKey(monitor.SchedulerHealth())
	} else {
		inputs.hosts = []models.Host{}
		inputs.instanceHealth = map[string]monitoring.InstanceHealth{}
	}

	writeJSON(w, http.StatusOK, ConnectionsListResponse{
		Connections: buildConnections(inputs),
	})
}

// HandleProbe fingerprints a user-supplied address and returns the product
// types it detected, if any. No configuration is persisted — the caller
// uses the response to pick a credential slot in ConnectionEditor.
func (h *ConnectionsHandlers) HandleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	defer r.Body.Close()

	var req ProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body", nil)
		return
	}

	host, port, err := parseProbeAddress(req.Address)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_address", err.Error(), nil)
		return
	}

	candidates, elapsed := runProbe(r.Context(), host, port, probeHTTPClient())
	writeJSON(w, http.StatusOK, ProbeResponse{
		Candidates: candidates,
		ProbedMs:   elapsed.Milliseconds(),
	})
}
