package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// resourceOperatorStateAPI is the wire shape for the operator-state API
// surface. It mirrors `unified.ResourceOperatorState` but with explicit
// JSON field names and string criticality so the contract is explicit at
// the API boundary regardless of internal type renames. The API layer
// adapts to and from `unified.ResourceOperatorState` so the storage
// type's evolution stays decoupled from the wire format.
type resourceOperatorStateAPI struct {
	CanonicalID              string                              `json:"canonicalId"`
	MonitoringMode           string                              `json:"monitoringMode"`
	LifecycleState           string                              `json:"lifecycleState"`
	IntentionallyOffline     bool                                `json:"intentionallyOffline"`
	NeverAutoRemediate       bool                                `json:"neverAutoRemediate"`
	AutoRemediationPolicy    unified.AutoRemediationPolicy       `json:"autoRemediationPolicy"`
	MaintenanceStartAt       *time.Time                          `json:"maintenanceStartAt,omitempty"`
	MaintenanceEndAt         *time.Time                          `json:"maintenanceEndAt,omitempty"`
	MaintenanceRecurrence    *unified.RecurringMaintenanceWindow `json:"maintenanceRecurrence,omitempty"`
	MaintenanceScope         string                              `json:"maintenanceScope"`
	MaintenanceReason        string                              `json:"maintenanceReason,omitempty"`
	MaintenanceWindowActive  bool                                `json:"maintenanceWindowActive"`
	MaintenanceActiveStartAt *time.Time                          `json:"maintenanceActiveStartAt,omitempty"`
	MaintenanceActiveEndAt   *time.Time                          `json:"maintenanceActiveEndAt,omitempty"`
	Criticality              string                              `json:"criticality,omitempty"`
	Note                     string                              `json:"note,omitempty"`
	SetAt                    time.Time                           `json:"setAt"`
	SetBy                    string                              `json:"setBy,omitempty"`
}

// resourceOperatorStateLookupAPI is the UI-safe read envelope. The canonical
// agent/API contract keeps a missing explicit record as 404 so callers can
// branch on operator_state_not_set. Interactive clients use view=lookup to
// receive the same distinction as data instead of generating a routine failed
// network request for every newly discovered resource.
type resourceOperatorStateLookupAPI struct {
	Configured bool                      `json:"configured"`
	State      *resourceOperatorStateAPI `json:"state,omitempty"`
}

func toResourceOperatorStateAPI(state unified.ResourceOperatorState) resourceOperatorStateAPI {
	state = unified.NormalizeResourceOperatorState(state)
	result := resourceOperatorStateAPI{
		CanonicalID:           state.CanonicalID,
		MonitoringMode:        string(state.MonitoringMode),
		LifecycleState:        string(state.LifecycleState),
		IntentionallyOffline:  state.IntentionallyOffline,
		NeverAutoRemediate:    state.NeverAutoRemediate,
		AutoRemediationPolicy: state.AutoRemediationPolicy,
		MaintenanceStartAt:    state.MaintenanceStartAt,
		MaintenanceEndAt:      state.MaintenanceEndAt,
		MaintenanceRecurrence: state.MaintenanceRecurrence,
		MaintenanceScope:      string(state.MaintenanceScope),
		MaintenanceReason:     state.MaintenanceReason,
		Criticality:           string(state.Criticality),
		Note:                  state.Note,
		SetAt:                 state.SetAt,
		SetBy:                 state.SetBy,
	}
	if occurrence, active := state.ActiveMaintenanceOccurrenceAt(time.Now().UTC()); active {
		startAt, endAt := occurrence.StartAt, occurrence.EndAt
		result.MaintenanceWindowActive = true
		result.MaintenanceActiveStartAt = &startAt
		result.MaintenanceActiveEndAt = &endAt
	}
	return result
}

// HandleResourceOperatorState dispatches GET / PUT / DELETE on
// /api/resources/{id}/operator-state. Method-keyed scope enforcement is
// done by the route registration; this handler only re-checks the method
// and routes to the underlying store calls.
//
// Wire-format contract:
//   - GET returns 404 with a stable JSON error body when no entry exists,
//     and 200 with the persisted state otherwise. Operators who have not
//     set any state see 404, distinct from a "default state was explicitly
//     written" 200 with all zero fields.
//   - PUT replaces the entire record (no per-field merge). The request
//     body is the full state shape; the canonical_id is taken from the
//     URL path and overrides any value in the body. SetAt / SetBy are
//     populated by the server from the authenticated identity and
//     request time, ignoring any client-supplied values to keep the
//     audit attribution honest.
//   - DELETE clears the entry, idempotent. Returns 204 whether or not
//     an entry was present.
func (h *ResourceHandlers) HandleResourceOperatorState(w http.ResponseWriter, r *http.Request) {
	resourceID := extractOperatorStateResourceID(r.URL.Path)
	if resourceID == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	orgID := GetOrgID(r.Context())
	resourceID = h.resolveOperatorStateCanonicalID(orgID, resourceID)
	store, err := h.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		state, found, err := store.GetResourceOperatorState(resourceID)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		if r.URL.Query().Get("view") == "lookup" {
			result := resourceOperatorStateLookupAPI{Configured: found}
			if found {
				wireState := toResourceOperatorStateAPI(state)
				result.State = &wireState
			}
			writeJSON(w, http.StatusOK, result)
			return
		}
		if !found {
			writeJSONError(w, http.StatusNotFound, agentcapabilities.AgentErrCodeOperatorStateNotSet,
				"No operator-set state recorded for this resource.")
			return
		}
		writeJSON(w, http.StatusOK, toResourceOperatorStateAPI(state))

	case http.MethodPut:
		var payload resourceOperatorStateAPI
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		// canonical_id from URL wins over body to prevent scope confusion
		// (operator wrote vm:101 in the URL but vm:102 in the body).
		state := unified.ResourceOperatorState{
			CanonicalID:           resourceID,
			MonitoringMode:        unified.ResourceMonitoringMode(payload.MonitoringMode),
			LifecycleState:        unified.ResourceLifecycleState(payload.LifecycleState),
			IntentionallyOffline:  payload.IntentionallyOffline,
			NeverAutoRemediate:    payload.NeverAutoRemediate,
			AutoRemediationPolicy: payload.AutoRemediationPolicy,
			MaintenanceStartAt:    payload.MaintenanceStartAt,
			MaintenanceEndAt:      payload.MaintenanceEndAt,
			MaintenanceRecurrence: payload.MaintenanceRecurrence,
			MaintenanceScope:      unified.MaintenanceScope(payload.MaintenanceScope),
			MaintenanceReason:     payload.MaintenanceReason,
			Criticality:           unified.ResourceCriticality(payload.Criticality),
			Note:                  payload.Note,
			// Server-populated attribution: ignore any client values so
			// the audit trail can't be spoofed.
			SetAt: time.Now().UTC(),
			SetBy: getUserID(r),
		}
		var persisted unified.ResourceOperatorState
		err := h.ActionLifecycle().WithPolicyMutation(func() error {
			var writeErr error
			persisted, writeErr = unified.SetResourceOperatorStateWithMaintenanceLifecycle(store, state)
			return writeErr
		})
		if err != nil {
			if errors.Is(err, unified.ErrResourceOperatorStateInvalid) {
				writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeOperatorStateInvalid, err.Error())
				return
			}
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		if h.operatorStateChanged != nil {
			h.operatorStateChanged(orgID, resourceID)
		}
		writeJSON(w, http.StatusOK, toResourceOperatorStateAPI(persisted))

	case http.MethodDelete:
		observedAt := time.Now().UTC()
		actor := getUserID(r)
		if err := h.ActionLifecycle().WithPolicyMutation(func() error {
			return unified.ClearResourceOperatorStateWithMaintenanceLifecycle(store, resourceID, observedAt, actor)
		}); err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		if h.operatorStateChanged != nil {
			h.operatorStateChanged(orgID, resourceID)
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// resolveOperatorStateCanonicalID ensures every UI and agent entry point writes
// one policy record even when it starts from a source-native alert ID, a
// superseded ID, or another canonical identity alias. If the live registry
// cannot resolve the reference, preserving the normalized input retains the
// established API behavior for resources that are not currently inventoried.
func (h *ResourceHandlers) resolveOperatorStateCanonicalID(orgID, resourceID string) string {
	resourceID = unified.CanonicalResourceID(resourceID)
	if resourceID == "" {
		return ""
	}
	registry, err := h.buildRegistry(orgID)
	if err != nil || registry == nil {
		return resourceID
	}
	if _, canonicalID, found := registry.GetByReference(resourceID); found {
		return canonicalID
	}
	return resourceID
}

// extractOperatorStateResourceID pulls the canonical resource ID out of a
// `/api/resources/<id>/operator-state` URL path. Tolerates a trailing
// slash on the URL (defense-in-depth — Go 1.22 ServeMux normally rejects
// it on pattern-match, but proxies might rewrite the path) and returns
// "" if the ID resolves to empty after canonical normalization.
func extractOperatorStateResourceID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/resources/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, "/operator-state")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return unified.CanonicalResourceID(trimmed)
}
