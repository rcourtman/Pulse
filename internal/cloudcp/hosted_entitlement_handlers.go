package cloudcp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	"github.com/rs/zerolog/log"
)

type hostedEntitlementRefreshRequest struct {
	OrgID                   string `json:"org_id"`
	InstanceHost            string `json:"instance_host"`
	EntitlementRefreshToken string `json:"entitlement_refresh_token"`
}

type hostedEntitlementRefreshResponse struct {
	EntitlementJWT string `json:"entitlement_jwt"`
}

type HostedEntitlementHandlers struct {
	entitlements *entitlements.Service
}

func NewHostedEntitlementHandlers(svc *entitlements.Service) *HostedEntitlementHandlers {
	return &HostedEntitlementHandlers{entitlements: svc}
}

func (h *HostedEntitlementHandlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.entitlements == nil {
		http.Error(w, "hosted entitlement refresh is unavailable", http.StatusServiceUnavailable)
		return
	}

	var reqBody hostedEntitlementRefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	orgID := normalizeTrialOrgID(reqBody.OrgID)
	instanceHost := strings.ToLower(strings.TrimSpace(reqBody.InstanceHost))
	refreshToken := strings.TrimSpace(reqBody.EntitlementRefreshToken)
	if instanceHost == "" || refreshToken == "" {
		http.Error(w, "instance_host and entitlement_refresh_token are required", http.StatusBadRequest)
		return
	}
	if orgID != trialSignupDefaultOrgID {
		http.Error(w, "invalid entitlement refresh token", http.StatusUnauthorized)
		return
	}

	result, err := h.entitlements.RefreshEntitlement(refreshToken, instanceHost)
	switch {
	case errors.Is(err, entitlements.ErrHostedEntitlementNotFound):
		http.Error(w, "invalid entitlement refresh token", http.StatusUnauthorized)
		return
	case errors.Is(err, entitlements.ErrHostedEntitlementTargetMismatch):
		http.Error(w, "invalid entitlement refresh target", http.StatusUnauthorized)
		return
	case errors.Is(err, entitlements.ErrHostedEntitlementInactive):
		http.Error(w, "hosted entitlement is no longer active", http.StatusGone)
		return
	case err != nil:
		log.Error().Err(err).Str("org_id", orgID).Msg("failed to refresh hosted entitlement")
		http.Error(w, "failed to load entitlement refresh record", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hostedEntitlementRefreshResponse{EntitlementJWT: result.EntitlementJWT}); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("failed to encode hosted entitlement refresh response")
	}
}
