package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rs/zerolog/log"
)

// LicenseHandlers handles license management API endpoints.
// LicenseHandlers handles license management API endpoints.
type LicenseHandlers struct {
	mtPersistence *config.MultiTenantPersistence
	hostedMode    bool
	services      sync.Map // map[string]*license.Service
}

// NewLicenseHandlers creates a new license handlers instance.

func NewLicenseHandlers(mtp *config.MultiTenantPersistence, hostedMode bool) *LicenseHandlers {
	return &LicenseHandlers{
		mtPersistence: mtp,
		hostedMode:    hostedMode,
	}
}

// getTenantComponents resolves the license service and persistence for the current tenant.
// It initializes them if they haven't been loaded yet.
func (h *LicenseHandlers) getTenantComponents(ctx context.Context) (*license.Service, *license.Persistence, error) {
	orgID := GetOrgID(ctx)

	// Check if service already exists
	if v, ok := h.services.Load(orgID); ok {
		svc := v.(*license.Service)
		// We need persistence too, reconstruct it or cache it?
		// Reconstructing persistence is cheap (just a struct with path).
		// But let's recreate it to be safe and stateless here.
		// Actually, we need the EXACT persistence object if it holds state, but license.Persistence seems stateless (file I/O).
		p, err := h.getPersistenceForOrg(orgID)
		return svc, p, err
	}

	// Initialize for this tenant
	persistence, err := h.getPersistenceForOrg(orgID)
	if err != nil {
		return nil, nil, err
	}

	service := license.NewService()

	// For hosted non-default tenants, derive entitlements from billing state
	if h.hostedMode && orgID != "default" && orgID != "" {
		billingStore := config.NewFileBillingStore(h.mtPersistence.BaseDataDir())
		dbSource := entitlements.NewDatabaseSource(billingStore, orgID, time.Hour)
		evaluator := entitlements.NewEvaluator(dbSource)
		service.SetEvaluator(evaluator)
	}

	// Try to load existing license
	if persistence != nil {
		persisted, err := persistence.LoadWithMetadata()
		if err == nil && persisted.LicenseKey != "" {
			lic, err := service.Activate(persisted.LicenseKey)
			if err != nil {
				log.Warn().Str("org_id", orgID).Err(err).Msg("Failed to load saved license")
			} else {
				if persisted.GracePeriodEnd != nil && lic != nil {
					gracePeriodEnd := time.Unix(*persisted.GracePeriodEnd, 0)
					lic.GracePeriodEnd = &gracePeriodEnd
				}
				log.Info().Str("org_id", orgID).Msg("Loaded saved Pulse Pro license")
			}
		}
	}

	h.services.Store(orgID, service)
	return service, persistence, nil
}

func (h *LicenseHandlers) getPersistenceForOrg(orgID string) (*license.Persistence, error) {
	configPersistence, err := h.mtPersistence.GetPersistence(orgID)
	if err != nil {
		return nil, err
	}
	return license.NewPersistence(configPersistence.GetConfigDir())
}

// Service returns the license service for use by other handlers.
// NOTE: This now requires context to identify the tenant.
// Handlers using this will need to be updated.
func (h *LicenseHandlers) Service(ctx context.Context) *license.Service {
	svc, _, _ := h.getTenantComponents(ctx)
	return svc
}

// HandleLicenseStatus handles GET /api/license/status
// Returns the current license status.
func (h *LicenseHandlers) HandleLicenseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service, _, err := h.getTenantComponents(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get license components")
		http.Error(w, "Tenant error", http.StatusInternalServerError)
		return
	}

	status := service.Status()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// LicenseFeaturesResponse provides a minimal, non-admin license view for feature gating.
type LicenseFeaturesResponse struct {
	LicenseStatus string          `json:"license_status"`
	Features      map[string]bool `json:"features"`
	UpgradeURL    string          `json:"upgrade_url"`
}

// HandleLicenseFeatures handles GET /api/license/features
// Returns license state and feature availability for authenticated users.
func (h *LicenseHandlers) HandleLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service, _, err := h.getTenantComponents(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get license components")
		http.Error(w, "Tenant error", http.StatusInternalServerError)
		return
	}

	state, _ := service.GetLicenseState()
	response := LicenseFeaturesResponse{
		LicenseStatus: string(state),
		Features: map[string]bool{
			// AI features
			license.FeatureAIPatrol:     service.HasFeature(license.FeatureAIPatrol),
			license.FeatureAIAlerts:     service.HasFeature(license.FeatureAIAlerts),
			license.FeatureAIAutoFix:    service.HasFeature(license.FeatureAIAutoFix),
			license.FeatureKubernetesAI: service.HasFeature(license.FeatureKubernetesAI),
			// Monitoring features
			license.FeatureUpdateAlerts: service.HasFeature(license.FeatureUpdateAlerts),
			// Fleet management
			license.FeatureAgentProfiles: service.HasFeature(license.FeatureAgentProfiles),
			// Team & Compliance features
			license.FeatureSSO:               service.HasFeature(license.FeatureSSO),
			license.FeatureAdvancedSSO:       service.HasFeature(license.FeatureAdvancedSSO),
			license.FeatureRBAC:              service.HasFeature(license.FeatureRBAC),
			license.FeatureAuditLogging:      service.HasFeature(license.FeatureAuditLogging),
			license.FeatureAdvancedReporting: service.HasFeature(license.FeatureAdvancedReporting),
			// Multi-tenant
			license.FeatureMultiTenant: service.HasFeature(license.FeatureMultiTenant),
		},
		UpgradeURL: "https://pulserelay.pro/",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ActivateLicenseRequest is the request body for activating a license.
type ActivateLicenseRequest struct {
	LicenseKey string `json:"license_key"`
}

// ActivateLicenseResponse is the response for license activation.
type ActivateLicenseResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Status  *license.LicenseStatus `json:"status,omitempty"`
}

// HandleActivateLicense handles POST /api/license/activate
// Validates and activates a license key.
func (h *LicenseHandlers) HandleActivateLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ActivateLicenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if req.LicenseKey == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: "License key is required",
		})
		return
	}

	// Activate the license
	service, persistence, err := h.getTenantComponents(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get license components")
		http.Error(w, "Tenant error", http.StatusInternalServerError)
		return
	}

	lic, err := service.Activate(req.LicenseKey)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to activate license")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ActivateLicenseResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	// Persist the license with grace period if applicable
	if persistence != nil {
		var gracePeriodEnd *int64
		if lic.GracePeriodEnd != nil {
			ts := lic.GracePeriodEnd.Unix()
			gracePeriodEnd = &ts
		}
		if err := persistence.SaveWithGracePeriod(req.LicenseKey, gracePeriodEnd); err != nil {
			log.Warn().Err(err).Msg("Failed to persist license, it won't survive restarts")
		}
	}

	log.Info().
		Str("email", lic.Claims.Email).
		Str("tier", string(lic.Claims.Tier)).
		Bool("lifetime", lic.IsLifetime()).
		Msg("Pulse Pro license activated")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ActivateLicenseResponse{
		Success: true,
		Message: "License activated successfully",
		Status:  service.Status(),
	})
}

// HandleClearLicense handles POST /api/license/clear
// Removes the current license.
func (h *LicenseHandlers) HandleClearLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear from service
	service, persistence, err := h.getTenantComponents(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get license components")
		http.Error(w, "Tenant error", http.StatusInternalServerError)
		return
	}

	service.Clear()

	// Clear from persistence
	if persistence != nil {
		if err := persistence.Delete(); err != nil {
			log.Warn().Err(err).Msg("Failed to delete persisted license")
		}
	}

	log.Info().Msg("Pulse Pro license cleared")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "License cleared",
	})
}

// RequireLicenseFeature is a middleware that checks if a license feature is available.
// Returns HTTP 402 Payment Required if the feature is not licensed.
// WriteLicenseRequired writes a 402 Payment Required response for a missing license feature.
// ALL license gate responses in handlers MUST use this function to ensure consistent response format.
func WriteLicenseRequired(w http.ResponseWriter, feature, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":       "license_required",
		"message":     message,
		"feature":     feature,
		"upgrade_url": "https://pulserelay.pro/",
	})
}

// RequireLicenseFeature is a middleware that checks if a license feature is available.
// Returns HTTP 402 Payment Required if the feature is not licensed.
// Note: Changed to take *LicenseHandlers to access service at runtime.
func RequireLicenseFeature(handlers *LicenseHandlers, feature string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service := handlers.Service(r.Context())
		if err := service.RequireFeature(feature); err != nil {
			WriteLicenseRequired(w, feature, err.Error())
			return
		}
		next(w, r)
	}
}

// LicenseGatedEmptyResponse returns an empty array with license metadata header for unlicensed users.
// Use this instead of RequireLicenseFeature when the endpoint should return empty data
// rather than a 402 error (to avoid breaking Promise.all in the frontend).
// The X-License-Required header indicates upgrade is needed.
// LicenseGatedEmptyResponse returns an empty array with license metadata header for unlicensed users.
// Use this instead of RequireLicenseFeature when the endpoint should return empty data
// rather than a 402 error (to avoid breaking Promise.all in the frontend).
// The X-License-Required header indicates upgrade is needed.
func LicenseGatedEmptyResponse(handlers *LicenseHandlers, feature string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service := handlers.Service(r.Context())
		if err := service.RequireFeature(feature); err != nil {
			w.Header().Set("Content-Type", "application/json")
			// Set header to indicate license is required (frontend can check this)
			w.Header().Set("X-License-Required", "true")
			w.Header().Set("X-License-Feature", feature)
			// Return 200 with empty array (compatible with frontend array expectations)
			w.Write([]byte("[]"))
			return
		}
		next(w, r)
	}
}
