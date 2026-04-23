package licensing

import "sort"

// ReasonEntry defines an actionable upgrade prompt tied to a missing feature.
type ReasonEntry struct {
	Feature   string // Feature key constant (e.g., "ai_autofix")
	Reason    string // User-facing description
	ActionURL string // Parameterized upgrade URL with UTM
	Priority  int    // Sort order (lower = more important)
}

// UpgradeReasonMatrix is the canonical feature-to-upgrade-reason mapping.
// Relay features use "Upgrade to Relay" messaging; Pro features use "Upgrade to Pro".
// Compatibility-only capabilities must not appear here as generic marketed
// upgrade reasons.
var UpgradeReasonMatrix = buildUpgradeReasonMatrix()

func buildUpgradeReasonMatrix() []ReasonEntry {
	metadata := GenericUpgradeFeatureMetadata()
	reasons := make([]ReasonEntry, 0, len(metadata))
	for _, entry := range metadata {
		reasons = append(reasons, ReasonEntry{
			Feature:   entry.Key,
			Reason:    entry.UpgradeReason,
			ActionURL: UpgradeURLForFeature(entry.Key),
			Priority:  entry.UpgradePriority,
		})
	}
	return reasons
}

// GenerateUpgradeReasons returns upgrade reasons for features missing in capabilities.
func GenerateUpgradeReasons(capabilities []string) []ReasonEntry {
	capSet := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		capSet[capability] = struct{}{}
	}

	reasons := make([]ReasonEntry, 0, len(UpgradeReasonMatrix))
	for _, entry := range UpgradeReasonMatrix {
		if _, hasFeature := capSet[entry.Feature]; hasFeature {
			continue
		}
		reasons = append(reasons, entry)
	}

	sort.SliceStable(reasons, func(i, j int) bool {
		if reasons[i].Priority == reasons[j].Priority {
			return reasons[i].Feature < reasons[j].Feature
		}
		return reasons[i].Priority < reasons[j].Priority
	})

	return reasons
}
