package api

import (
	"net/http"
	"strings"
)

// applyConfiguredCORSHeaders applies CORS headers based on configured allowlist
// semantics used by API handlers:
//   - "*" allows any origin but never with credentials.
//   - explicit origins allow credentials only for exact origin matches.
func applyConfiguredCORSHeaders(w http.ResponseWriter, requestOrigin, allowedOrigins, methods, headers string) {
	origin := strings.TrimSpace(requestOrigin)
	if origin == "" {
		return
	}

	allowed := strings.TrimSpace(allowedOrigins)
	if allowed == "" {
		return
	}

	allowOrigin, allowCredentials := resolveAllowedOrigin(origin, allowed)
	if allowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	}
	if allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	w.Header().Set("Access-Control-Allow-Methods", methods)
	w.Header().Set("Access-Control-Allow-Headers", headers)

	// Non-wildcard policy is origin-dependent and must vary for caches.
	if allowed != "*" {
		w.Header().Set("Vary", "Origin")
	}
}

func resolveAllowedOrigin(requestOrigin, allowedOrigins string) (allowOrigin string, allowCredentials bool) {
	if allowedOrigins == "*" {
		return "*", false
	}

	for _, candidate := range strings.Split(allowedOrigins, ",") {
		if strings.TrimSpace(candidate) == requestOrigin {
			return requestOrigin, true
		}
	}
	return "", false
}
