package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// resourceOperatorStateAPI is the wire shape for the operator-state API
// surface. It mirrors `unified.ResourceOperatorState` but with explicit
// JSON field names and string criticality so the contract is explicit at
// the API boundary regardless of internal type renames. The API layer
// adapts to and from `unified.ResourceOperatorState` so the storage
// type's evolution stays decoupled from the wire format.
type resourceOperatorStateAPI struct {
	CanonicalID          string     `json:"canonicalId"`
	IntentionallyOffline bool       `json:"intentionallyOffline"`
	NeverAutoRemediate   bool       `json:"neverAutoRemediate"`
	MaintenanceStartAt   *time.Time `json:"maintenanceStartAt,omitempty"`
	MaintenanceEndAt     *time.Time `json:"maintenanceEndAt,omitempty"`
	MaintenanceReason    string     `json:"maintenanceReason,omitempty"`
	Criticality          string     `json:"criticality,omitempty"`
	Note                 string     `json:"note,omitempty"`
	SetAt                time.Time  `json:"setAt"`
	SetBy                string     `json:"setBy,omitempty"`
}

func toResourceOperatorStateAPI(state unified.ResourceOperatorState) resourceOperatorStateAPI {
	return resourceOperatorStateAPI{
		CanonicalID:          state.CanonicalID,
		IntentionallyOffline: state.IntentionallyOffline,
		NeverAutoRemediate:   state.NeverAutoRemediate,
		MaintenanceStartAt:   state.MaintenanceStartAt,
		MaintenanceEndAt:     state.MaintenanceEndAt,
		MaintenanceReason:    state.MaintenanceReason,
		Criticality:          string(state.Criticality),
		Note:                 state.Note,
		SetAt:                state.SetAt,
		SetBy:                state.SetBy,
	}
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
		if !found {
			writeJSONError(w, http.StatusNotFound, "operator_state_not_set",
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
			CanonicalID:          resourceID,
			IntentionallyOffline: payload.IntentionallyOffline,
			NeverAutoRemediate:   payload.NeverAutoRemediate,
			MaintenanceStartAt:   payload.MaintenanceStartAt,
			MaintenanceEndAt:     payload.MaintenanceEndAt,
			MaintenanceReason:    payload.MaintenanceReason,
			Criticality:          unified.ResourceCriticality(payload.Criticality),
			Note:                 payload.Note,
			// Server-populated attribution: ignore any client values so
			// the audit trail can't be spoofed.
			SetAt: time.Now().UTC(),
			SetBy: getUserID(r),
		}
		if err := store.SetResourceOperatorState(state); err != nil {
			if errors.Is(err, unified.ErrResourceOperatorStateInvalid) {
				writeJSONError(w, http.StatusBadRequest, "operator_state_invalid", err.Error())
				return
			}
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		// Read-after-write: return the persisted state so the caller can
		// see exactly what the server stored, including the
		// server-populated attribution fields.
		persisted, _, err := store.GetResourceOperatorState(resourceID)
		if err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, toResourceOperatorStateAPI(persisted))

	case http.MethodDelete:
		if err := store.ClearResourceOperatorState(resourceID); err != nil {
			http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
