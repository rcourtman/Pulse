package licensing

import (
	"os"
	"strings"
)

func devModeFeatureEnabled(feature string) bool {
	switch feature {
	case FeatureMultiUser, FeatureWhiteLabel, FeatureUnlimited:
		return false
	case FeatureMultiTenant:
		return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_MULTI_TENANT_ENABLED")), "true")
	default:
		return true
	}
}
