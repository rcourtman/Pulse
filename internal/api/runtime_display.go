package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// RuntimeDisplayResponse is the presentation-only slice of system settings that
// every authenticated session needs in order to render the app shell the way the
// admin configured it. It is the sibling of RuntimeBrandingResponse and follows
// the same rule: an explicit whitelist of presentation fields, never a
// projection of config.SystemSettings, which also carries allowed origins, the
// public URL, webhook private CIDRs, telemetry configuration and hideLocalLogin.
//
// Adding a field here publishes it to every authenticated viewer, including
// non-admins and monitoring:read API tokens. Only add values that decide how the
// shell renders.
type RuntimeDisplayResponse struct {
	Theme                      string `json:"theme"`
	FullWidthMode              bool   `json:"fullWidthMode"`
	DisableDockerUpdateActions bool   `json:"disableDockerUpdateActions"`
	ReduceProUpsellNoise       bool   `json:"reduceProUpsellNoise"`
}

// HandleGetRuntimeDisplay returns the server-configured display defaults for the
// active session.
//
// The route is monitoring:read rather than the settings:read + RequireAdmin gate
// that /api/system/settings enforces. Bootstrap read these four values off the
// admin route, so an authenticated non-admin took a 403 on every page load and
// the catch silently fell back to client defaults: the session ignored the theme
// and layout an admin had configured, and rendered Docker update buttons an
// admin had turned off. Serving the presentation slice at the session tier fixes
// that without widening access to the rest of the settings payload.
func (h *SystemSettingsHandler) HandleGetRuntimeDisplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	response := RuntimeDisplayResponse{}
	if h != nil && h.config != nil {
		// Mirror HandleGetSystemSettings: the effective value respects the
		// PULSE_DISABLE_DOCKER_UPDATE_ACTIONS override, which the persisted
		// settings blob does not carry.
		response.DisableDockerUpdateActions = h.config.DisableDockerUpdateActions
	}

	if h == nil || h.persistence == nil {
		_ = utils.WriteJSONResponse(w, response)
		return
	}

	settings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load system settings for runtime display response")
		_ = utils.WriteJSONResponse(w, response)
		return
	}
	if settings != nil {
		response.Theme = settings.Theme
		response.FullWidthMode = settings.FullWidthMode
		response.ReduceProUpsellNoise = settings.ReduceProUpsellNoise
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write runtime display response")
	}
}
