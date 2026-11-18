package licensing

import "testing"

func TestRetiredRelayCapabilitiesHaveNoUpgradeOffer(t *testing.T) {
	for _, feature := range []string{FeatureRelay, FeatureMobileApp, FeaturePushNotifications} {
		t.Run(feature, func(t *testing.T) {
			metadata, ok := GetFeatureMetadata(feature)
			if !ok {
				t.Fatal("retirement must preserve compatibility metadata")
			}
			if metadata.UpgradeReason != "" || metadata.UpgradePriority != 0 {
				t.Errorf("retired capability still markets an upgrade: %+v", metadata)
			}
			for _, entry := range GenericUpgradeFeatureMetadata() {
				if entry.Key == feature {
					t.Error("retired capability remains in marketed metadata")
				}
			}
			for _, tier := range []Tier{TierFree, TierRelay, TierPro} {
				for _, reason := range GenerateUpgradeReasons(TierFeatures[tier]) {
					if reason.Feature == feature {
						t.Errorf("%s still offers retired %s", tier, feature)
					}
				}
			}
		})
	}
}

// Metric history is free on every plan, so no tier is ever offered it.
func TestHistoryIsNeverOfferedAsAnUpgrade(t *testing.T) {
	if !TierHasFeature(TierFree, FeatureLongTermMetrics) {
		t.Fatal("Community must include metric history")
	}
	if got := UpgradeURLForFeature(FeatureLongTermMetrics); got != DefaultUpgradeURL {
		t.Fatalf("history upgrade URL = %q, want generic pricing %q", got, DefaultUpgradeURL)
	}
	for _, tier := range []Tier{TierFree, TierRelay, TierPro, TierProAnnual, TierLifetime} {
		for _, reason := range GenerateUpgradeReasons(TierFeatures[tier]) {
			if reason.Feature == FeatureLongTermMetrics {
				t.Errorf("entitled %s must not offer an already granted capability", tier)
			}
		}
	}
}
