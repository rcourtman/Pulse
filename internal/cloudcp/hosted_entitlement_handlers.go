package cloudcp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
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
	cfg               *CPConfig
	verificationStore *TrialSignupStore
	entitlements      *entitlements.Service
	now               func() time.Time
}

func NewHostedEntitlementHandlers(cfg *CPConfig, verificationStore *TrialSignupStore, svc *entitlements.Service) *HostedEntitlementHandlers {
	return &HostedEntitlementHandlers{
		cfg:               cfg,
		verificationStore: verificationStore,
		entitlements:      svc,
		now:               func() time.Time { return time.Now().UTC() },
	}
}

func (h *HostedEntitlementHandlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.verificationStore == nil && h.entitlements == nil {
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

	if handled, err := h.handleTrialEntitlementRefresh(w, orgID, instanceHost, refreshToken); handled {
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("failed to refresh hosted trial entitlement")
			http.Error(w, "failed to load entitlement refresh record", http.StatusInternalServerError)
		}
		return
	}
	if handled, err := h.handlePaidEntitlementRefresh(w, orgID, instanceHost, refreshToken); handled {
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("failed to refresh paid hosted entitlement")
			http.Error(w, "failed to load entitlement refresh record", http.StatusInternalServerError)
		}
		return
	}
	http.Error(w, "invalid entitlement refresh token", http.StatusUnauthorized)
}

func (h *HostedEntitlementHandlers) handleTrialEntitlementRefresh(w http.ResponseWriter, orgID, instanceHost, refreshToken string) (bool, error) {
	if h == nil || h.verificationStore == nil {
		return false, nil
	}
	record, err := h.verificationStore.GetRecordByEntitlementRefreshToken(refreshToken)
	if err != nil {
		if errors.Is(err, ErrTrialSignupRecordNotFound) {
			return false, nil
		}
		return true, err
	}
	if normalizeTrialOrgID(record.OrgID) != orgID {
		http.Error(w, "invalid entitlement refresh token", http.StatusUnauthorized)
		return true, nil
	}
	recordHost, err := trialSignupReturnURLHost(record.ReturnURL)
	if err != nil || recordHost != instanceHost {
		http.Error(w, "invalid entitlement refresh target", http.StatusUnauthorized)
		return true, nil
	}
	if record.VerifiedAt.IsZero() || record.CheckoutCompletedAt.IsZero() || record.ActivationIssuedAt.IsZero() || record.RedemptionRecordedAt.IsZero() {
		http.Error(w, "trial entitlement is not ready for refresh", http.StatusUnauthorized)
		return true, nil
	}
	privateKey, err := pkglicensing.DecodeEd25519PrivateKey(strings.TrimSpace(h.cfg.TrialActivationPrivateKey))
	if err != nil {
		log.Error().Err(err).Msg("trial activation private key invalid")
		http.Error(w, "trial activation verifier unavailable", http.StatusServiceUnavailable)
		return true, nil
	}

	now := h.now().UTC()
	leaseClaims := buildTrialEntitlementLeaseClaims(record, instanceHost, now)
	if leaseClaims.TrialEndsAt == nil || now.Unix() >= *leaseClaims.TrialEndsAt {
		http.Error(w, "trial entitlement has expired", http.StatusGone)
		return true, nil
	}
	entitlementJWT, err := pkglicensing.SignEntitlementLeaseToken(privateKey, leaseClaims)
	if err != nil {
		return true, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hostedEntitlementRefreshResponse{EntitlementJWT: entitlementJWT}); err != nil {
		return true, err
	}
	return true, nil
}

func (h *HostedEntitlementHandlers) handlePaidEntitlementRefresh(w http.ResponseWriter, orgID, instanceHost, refreshToken string) (bool, error) {
	if h == nil || h.entitlements == nil {
		return false, nil
	}
	if normalizeTrialOrgID(orgID) != trialSignupDefaultOrgID {
		http.Error(w, "invalid entitlement refresh token", http.StatusUnauthorized)
		return true, nil
	}

	result, err := h.entitlements.RefreshPaidEntitlement(refreshToken, instanceHost)
	switch {
	case errors.Is(err, entitlements.ErrHostedEntitlementNotFound):
		return false, nil
	case errors.Is(err, entitlements.ErrHostedEntitlementTargetMismatch):
		http.Error(w, "invalid entitlement refresh target", http.StatusUnauthorized)
		return true, nil
	case errors.Is(err, entitlements.ErrHostedEntitlementInactive):
		http.Error(w, "hosted entitlement is no longer active", http.StatusGone)
		return true, nil
	case err != nil:
		return true, err
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hostedEntitlementRefreshResponse{EntitlementJWT: result.EntitlementJWT}); err != nil {
		return true, err
	}
	return true, nil
}
