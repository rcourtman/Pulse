package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// RuntimeDisplayResponse is the presentation-only slice of system settings that
// every authenticated session needs to render the app shell as configured. It
// follows the same rule as RuntimeBrandingResponse: define an explicit whitelist
// rather than projecting config.SystemSettings, which also contains sensitive
// operator configuration.
//
// Adding a field here publishes it to every authenticated viewer, including
// non-admins and monitoring:read API tokens. Only add values that control how the
// authenticated shell renders.
type RuntimeDisplayResponse struct {
	Theme                      string `json:"theme"`
	FullWidthMode              bool   `json:"fullWidthMode"`
	DisableDockerUpdateActions bool   `json:"disableDockerUpdateActions"`
	TelemetryEnabled           bool   `json:"telemetryEnabled"`
	ReduceProUpsellNoise       bool   `json:"reduceProUpsellNoise"`
	// PVEPollingInterval is the effective PVE polling cadence in seconds. The
	// Settings shell renders it read-only for viewers; without it the
	// Monitoring Cadence card falls back to the Realtime preset and misreports
	// how often the server actually polls.
	PVEPollingInterval int `json:"pvePollingInterval"`
}

// HandleGetRuntimeDisplay returns the effective display defaults needed by an
// authenticated session without widening access to the admin settings payload.
func (h *SystemSettingsHandler) HandleGetRuntimeDisplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	response := RuntimeDisplayResponse{TelemetryEnabled: true}
	if h != nil && h.config != nil {
		// Match HandleGetSystemSettings so the environment override wins over
		// the persisted value for every role. These booleans reveal only the
		// effective operator policy, not the admin-only configuration or
		// telemetry payload.
		response.DisableDockerUpdateActions = h.config.DisableDockerUpdateActions
		response.TelemetryEnabled = h.config.TelemetryEnabled
		// config carries the effective interval for every role: persisted
		// settings are applied to it at startup unless PVE_POLLING_INTERVAL
		// wins, and runtime saves mutate it. HandleGetSystemSettings serves
		// exactly this value to admins, so serving it here keeps both roles
		// reading the same number.
		response.PVEPollingInterval = int(h.config.PVEPollingInterval.Seconds())
	}

	if h == nil || h.persistence == nil {
		_ = utils.WriteJSONResponse(w, response)
		return
	}

	settings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load runtime display settings")
		_ = utils.WriteJSONResponse(w, response)
		return
	}
	if settings != nil {
		response.Theme = settings.Theme
		response.FullWidthMode = settings.FullWidthMode
		response.ReduceProUpsellNoise = settings.ReduceProUpsellNoise
		if h.config == nil || (!h.config.EnvOverrides["disableDockerUpdateActions"] && !h.config.EnvOverrides["PULSE_DISABLE_DOCKER_UPDATE_ACTIONS"]) {
			response.DisableDockerUpdateActions = settings.DisableDockerUpdateActions
		}
		if settings.TelemetryEnabled != nil && (h.config == nil || (!h.config.EnvOverrides["telemetryEnabled"] && !h.config.EnvOverrides["PULSE_TELEMETRY"])) {
			response.TelemetryEnabled = *settings.TelemetryEnabled
		}
		// Unlike the booleans above, persisted state is only a fallback for a
		// missing runtime config, never an override of it — config is already
		// the effective value.
		if response.PVEPollingInterval <= 0 && settings.PVEPollingInterval > 0 {
			response.PVEPollingInterval = settings.PVEPollingInterval
		}
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write runtime display response")
	}
}
