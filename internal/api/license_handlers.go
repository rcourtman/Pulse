package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rs/zerolog/log"
)

// LicenseHandlers handles license management API endpoints.
type LicenseHandlers struct {
	service     *license.Service
	persistence *license.Persistence
}

// NewLicenseHandlers creates a new license handlers instance.
func NewLicenseHandlers(configDir string) *LicenseHandlers {
	persistence, err := license.NewPersistence(configDir)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize license persistence, licenses won't persist across restarts")
	}

	service := license.NewService()

	// Try to load existing license with metadata
	if persistence != nil {
		persisted, err := persistence.LoadWithMetadata()
		if err == nil && persisted.LicenseKey != "" {
			lic, err := service.Activate(persisted.LicenseKey)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to load saved license, may be expired or invalid")
			} else {
				// Restore grace period if it was persisted
				if persisted.GracePeriodEnd != nil && lic != nil {
					gracePeriodEnd := time.Unix(*persisted.GracePeriodEnd, 0)
					lic.GracePeriodEnd = &gracePeriodEnd
				}
				log.Info().Msg("Loaded saved Pulse Pro license")
			}
		}
	}

	return &LicenseHandlers{
		service:     service,
		persistence: persistence,
	}
}

// Service returns the license service for use by other handlers.
func (h *LicenseHandlers) Service() *license.Service {
	return h.service
}

// HandleLicenseStatus handles GET /api/license/status
// Returns the current license status.
func (h *LicenseHandlers) HandleLicenseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.service.Status()

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

	state, _ := h.service.GetLicenseState()
	response := LicenseFeaturesResponse{
		LicenseStatus: string(state),
		Features: map[string]bool{
			// AI features
			license.FeatureAIPatrol:     h.service.HasFeature(license.FeatureAIPatrol),
			license.FeatureAIAlerts:     h.service.HasFeature(license.FeatureAIAlerts),
			license.FeatureAIAutoFix:    h.service.HasFeature(license.FeatureAIAutoFix),
			license.FeatureKubernetesAI: h.service.HasFeature(license.FeatureKubernetesAI),
			// Monitoring features
			license.FeatureUpdateAlerts: h.service.HasFeature(license.FeatureUpdateAlerts),
			// Fleet management
			license.FeatureAgentProfiles: h.service.HasFeature(license.FeatureAgentProfiles),
			// Team & Compliance features
			license.FeatureSSO:               h.service.HasFeature(license.FeatureSSO),
			license.FeatureAdvancedSSO:       h.service.HasFeature(license.FeatureAdvancedSSO),
			license.FeatureRBAC:              h.service.HasFeature(license.FeatureRBAC),
			license.FeatureAuditLogging:      h.service.HasFeature(license.FeatureAuditLogging),
			license.FeatureAdvancedReporting: h.service.HasFeature(license.FeatureAdvancedReporting),
		},
		UpgradeURL: "https://pulse.sh/pro",
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
	lic, err := h.service.Activate(req.LicenseKey)
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
	if h.persistence != nil {
		var gracePeriodEnd *int64
		if lic.GracePeriodEnd != nil {
			ts := lic.GracePeriodEnd.Unix()
			gracePeriodEnd = &ts
		}
		if err := h.persistence.SaveWithGracePeriod(req.LicenseKey, gracePeriodEnd); err != nil {
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
		Status:  h.service.Status(),
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
	h.service.Clear()

	// Clear from persistence
	if h.persistence != nil {
		if err := h.persistence.Delete(); err != nil {
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
func RequireLicenseFeature(service *license.Service, feature string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := service.RequireFeature(feature); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":       "license_required",
				"message":     err.Error(),
				"feature":     feature,
				"upgrade_url": "https://pulse.sh/pro",
			})
			return
		}
		next(w, r)
	}
}

// LicenseGatedEmptyResponse returns an empty array with license metadata header for unlicensed users.
// Use this instead of RequireLicenseFeature when the endpoint should return empty data
// rather than a 402 error (to avoid breaking Promise.all in the frontend).
// The X-License-Required header indicates upgrade is needed.
func LicenseGatedEmptyResponse(service *license.Service, feature string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
