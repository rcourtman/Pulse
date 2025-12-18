package api

import "net/http"

func (r *Router) isSensorProxyEnabled() bool {
	return r != nil && r.config != nil && r.config.EnableSensorProxy
}

func (r *Router) requireSensorProxyEnabled(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if r.isSensorProxyEnabled() {
			next(w, req)
			return
		}

		w.Header().Set("Warning", `299 - "pulse-sensor-proxy is deprecated and disabled by default in v5"`)
		writeErrorResponse(
			w,
			http.StatusGone,
			"sensor_proxy_disabled",
			"pulse-sensor-proxy is deprecated and disabled by default in v5",
			map[string]string{
				"migration":  "Use pulse-agent --enable-proxmox for temperature monitoring.",
				"enable_env": "Set PULSE_ENABLE_SENSOR_PROXY=true (unsupported legacy) and restart Pulse to re-enable these endpoints.",
			},
		)
	}
}
