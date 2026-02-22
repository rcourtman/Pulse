package licensing

import (
	"encoding/json"
	"net/http"
)

// UpgradeURLResolver resolves a feature-specific upgrade URL.
type UpgradeURLResolver func(feature string) string

// WritePaymentRequired writes a JSON 402 response payload.
func WritePaymentRequired(w http.ResponseWriter, payload map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteLicenseRequired writes the canonical 402 response for missing features.
func WriteLicenseRequired(w http.ResponseWriter, feature, message string, resolveURL UpgradeURLResolver) {
	upgradeURL := ""
	if resolveURL != nil {
		upgradeURL = resolveURL(feature)
	}

	WritePaymentRequired(w, map[string]interface{}{
		"error":       "license_required",
		"message":     message,
		"feature":     feature,
		"upgrade_url": upgradeURL,
	})
}
