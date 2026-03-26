package licensing

import (
	"os"
	"sort"
	"strings"
)

func devModeFeatures() []string {
	known := allKnownFeatures()
	filtered := make([]string, 0, len(known))
	for _, feature := range known {
		if devModeFeatureEnabled(feature) {
			filtered = append(filtered, feature)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func devModeFeatureEnabled(feature string) bool {
	switch feature {
	case FeatureMultiTenant:
		return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_MULTI_TENANT_ENABLED")), "true")
	default:
		return true
	}
}
