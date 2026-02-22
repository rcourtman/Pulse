package conversion

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

// ReasonEntry defines an actionable upgrade prompt tied to a missing feature.
type ReasonEntry = pkglicensing.ReasonEntry

// UpgradeReasonMatrix is the canonical feature-to-upgrade-reason mapping.
var UpgradeReasonMatrix = pkglicensing.UpgradeReasonMatrix

// GenerateUpgradeReasons returns upgrade reasons for features missing in capabilities.
func GenerateUpgradeReasons(capabilities []string) []ReasonEntry {
	return pkglicensing.GenerateUpgradeReasons(capabilities)
}

// UpgradeURLForFeature returns the action URL for a given feature from the upgrade matrix.
// Falls back to a generic pricing URL if the feature is not in the matrix.
func UpgradeURLForFeature(feature string) string {
	return pkglicensing.UpgradeURLForFeature(feature)
}
