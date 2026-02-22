package licensing

// DefaultFeatureKeys defines the standard feature set exposed by /api/license/features.
var DefaultFeatureKeys = []string{
	FeatureAIPatrol,
	FeatureAIAlerts,
	FeatureAIAutoFix,
	FeatureKubernetesAI,
	FeatureUpdateAlerts,
	FeatureAgentProfiles,
	FeatureSSO,
	FeatureAdvancedSSO,
	FeatureRBAC,
	FeatureAuditLogging,
	FeatureAdvancedReporting,
	FeatureMultiTenant,
}

// BuildFeatureMap evaluates feature availability for the given checker.
func BuildFeatureMap(checker FeatureChecker, keys []string) map[string]bool {
	out := make(map[string]bool)
	if checker == nil {
		return out
	}
	if len(keys) == 0 {
		keys = DefaultFeatureKeys
	}
	for _, key := range keys {
		out[key] = checker.HasFeature(key)
	}
	return out
}
