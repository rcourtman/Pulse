package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

type BillingStateHandlers struct {
	store      *config.FileBillingStore
	hostedMode bool
}

func NewBillingStateHandlers(store *config.FileBillingStore, hostedMode bool) *BillingStateHandlers {
	return &BillingStateHandlers{
		store:      store,
		hostedMode: hostedMode,
	}
}

func (h *BillingStateHandlers) HandleGetBillingState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "billing_store_unavailable", "Billing persistence is not configured", nil)
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	if !isValidOrganizationID(orgID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_org_id", "Invalid organization ID", nil)
		return
	}

	state, err := h.store.GetBillingState(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "billing_state_load_failed", "Failed to load billing state", nil)
		return
	}
	if state == nil {
		state = defaultBillingState()
	}

	writeJSON(w, http.StatusOK, normalizeBillingState(state))
}

func (h *BillingStateHandlers) HandlePutBillingState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "billing_store_unavailable", "Billing persistence is not configured", nil)
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	if !isValidOrganizationID(orgID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_org_id", "Invalid organization ID", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, orgRequestBodyLimit)

	var incoming pkglicensing.BillingState
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	state := normalizeBillingState(&incoming)
	if !isValidBillingSubscriptionState(state.SubscriptionState) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_subscription_state", "Invalid subscription_state", nil)
		return
	}

	before, err := h.store.GetBillingState(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "billing_state_load_failed", "Failed to load current billing state", nil)
		return
	}

	if err := h.store.SaveBillingState(orgID, state); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "billing_state_save_failed", "Failed to save billing state", nil)
		return
	}

	log.Info().
		Str("org_id", orgID).
		Interface("before", normalizeBillingState(before)).
		Interface("after", state).
		Msg("Billing state updated")

	writeJSON(w, http.StatusOK, state)
}

func defaultBillingState() *pkglicensing.BillingState {
	return pkglicensing.DefaultBillingState()
}

func normalizeBillingState(state *pkglicensing.BillingState) *pkglicensing.BillingState {
	return pkglicensing.NormalizeBillingState(state)
}

func isValidBillingSubscriptionState(state pkglicensing.SubscriptionState) bool {
	return pkglicensing.IsValidBillingSubscriptionState(state)
}
