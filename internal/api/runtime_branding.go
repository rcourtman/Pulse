package api

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// RuntimeBrandingResponse is the narrow presentation-only branding contract
// available to authenticated monitoring clients. It intentionally omits the
// rest of system settings and returns no configured brand when the active
// runtime lacks the white_label entitlement.
type RuntimeBrandingResponse struct {
	Enabled     bool   `json:"enabled"`
	DisplayName string `json:"displayName"`
	LogoDataURL string `json:"logoDataUrl"`
}

func emptyRuntimeBrandingResponse() RuntimeBrandingResponse {
	return RuntimeBrandingResponse{}
}

func runtimeBrandingResponse(settings *config.ReportBrandSettings, entitled bool) RuntimeBrandingResponse {
	if !entitled || settings == nil {
		return emptyRuntimeBrandingResponse()
	}

	displayName := strings.TrimSpace(settings.DisplayName)
	logoDataURL := runtimeBrandLogoDataURL(*settings)
	if displayName == "" && logoDataURL == "" {
		return emptyRuntimeBrandingResponse()
	}

	return RuntimeBrandingResponse{
		Enabled:     true,
		DisplayName: displayName,
		LogoDataURL: logoDataURL,
	}
}

func runtimeBrandLogoDataURL(settings config.ReportBrandSettings) string {
	decoded, err := config.DecodeReportBrandLogoBase64(settings.LogoBase64)
	if err != nil || len(decoded) == 0 {
		return ""
	}

	detectedFormat := ""
	switch http.DetectContentType(decoded) {
	case "image/png":
		detectedFormat = "png"
	case "image/jpeg":
		detectedFormat = "jpg"
	case "image/gif":
		detectedFormat = "gif"
	default:
		return ""
	}

	format, ok := config.CanonicalReportBrandLogoFormat(settings.LogoFormat)
	if !ok {
		return ""
	}
	if format == "" {
		format = detectedFormat
	}
	if format != detectedFormat {
		return ""
	}

	mediaType := "image/" + format
	if format == "jpg" {
		mediaType = "image/jpeg"
	}
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(decoded)
}

// HandleGetRuntimeBranding returns only the effective application brand for
// the active licensed runtime. The route itself is monitoring:read so the
// header can render consistently for non-admin viewers.
func (h *SystemSettingsHandler) HandleGetRuntimeBranding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	service := getLicenseServiceForContext(r.Context())
	entitled := service != nil && service.HasFeature(featureWhiteLabelValue)
	if !entitled {
		_ = utils.WriteJSONResponse(w, emptyRuntimeBrandingResponse())
		return
	}
	if h == nil || h.persistence == nil {
		_ = utils.WriteJSONResponse(w, emptyRuntimeBrandingResponse())
		return
	}

	settings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load runtime branding settings")
		_ = utils.WriteJSONResponse(w, emptyRuntimeBrandingResponse())
		return
	}

	var configured *config.ReportBrandSettings
	if settings != nil {
		configured = settings.ReportBranding
	}
	if err := utils.WriteJSONResponse(w, runtimeBrandingResponse(configured, true)); err != nil {
		log.Error().Err(err).Msg("Failed to write runtime branding response")
	}
}
