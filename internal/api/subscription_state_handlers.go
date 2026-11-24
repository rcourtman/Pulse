package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
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

	state, err := h.loadEffectiveBillingState(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "billing_state_load_failed", "Failed to load billing state", nil)
		return
	}
	if state == nil {
		state = defaultBillingState()
	}

	writeJSON(w, http.StatusOK, redactBillingStateCredentials(state))
}

// redactBillingStateCredentials returns the billing state as it may leave the
// runtime. The hosted entitlement lease and its refresh token are the
// credentials this runtime presents to the control plane and the relay. No
// caller of the billing-state API needs them, and a hosted org without billing
// state of its own inherits the default org's record, so they are never
// included in a response or a log line.
func redactBillingStateCredentials(state *billingState) *billingState {
	redacted := *normalizeBillingState(state)
	redacted.EntitlementJWT = ""
	redacted.EntitlementRefreshToken = ""
	return &redacted
}

func (h *BillingStateHandlers) loadEffectiveBillingState(orgID string) (*billingState, error) {
	state, err := h.store.GetBillingState(orgID)
	if err != nil {
		return nil, err
	}
	if !h.hostedMode || orgID == "" || orgID == "default" {
		return state, nil
	}
	if state != nil && strings.TrimSpace(string(state.SubscriptionState)) != "" {
		return state, nil
	}
	defaultState, err := h.store.GetBillingState("default")
	if err != nil {
		return nil, err
	}
	if defaultState != nil && strings.TrimSpace(string(defaultState.SubscriptionState)) != "" {
		return defaultState, nil
	}
	return state, nil
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

	var incoming billingState
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

	// Reads do not disclose the entitlement lease or its refresh token, so a
	// caller writing back what it read must not erase the stored ones.
	if before != nil {
		if state.EntitlementJWT == "" {
			state.EntitlementJWT = before.EntitlementJWT
		}
		if state.EntitlementRefreshToken == "" {
			state.EntitlementRefreshToken = before.EntitlementRefreshToken
		}
	}

	if err := h.store.SaveBillingState(orgID, state); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "billing_state_save_failed", "Failed to save billing state", nil)
		return
	}

	log.Info().
		Str("org_id", orgID).
		Interface("before", redactBillingStateCredentials(before)).
		Interface("after", redactBillingStateCredentials(state)).
		Msg("Billing state updated")

	writeJSON(w, http.StatusOK, redactBillingStateCredentials(state))
}

func defaultBillingState() *billingState {
	return defaultBillingStateFromLicensing()
}

func normalizeBillingState(state *billingState) *billingState {
	return normalizeBillingStateFromLicensing(state)
}

func isValidBillingSubscriptionState(state subscriptionState) bool {
	return isValidBillingSubscriptionStateFromLicensing(state)
}
