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

func TestHistoryUpgradeOffersOnlyCurrentProPlan(t *testing.T) {
	const want = "Upgrade to Pro for 90 days of metric history, so you can see what changed before and after an incident."
	found := false
	for _, reason := range GenerateUpgradeReasons(TierFeatures[TierFree]) {
		if reason.Feature == FeatureLongTermMetrics {
			found = true
			if reason.Reason != want {
				t.Errorf("history upgrade = %q, want %q", reason.Reason, want)
			}
			if reason.ActionURL != UpgradeURLForFeature(FeatureLongTermMetrics) {
				t.Error("history upgrade lost its current feature destination")
			}
		}
	}
	if !found {
		t.Fatal("Community must retain its current Pro history upgrade")
	}
	for _, tier := range []Tier{TierRelay, TierPro, TierProAnnual, TierLifetime} {
		for _, reason := range GenerateUpgradeReasons(TierFeatures[tier]) {
			if reason.Feature == FeatureLongTermMetrics {
				t.Errorf("entitled %s must not offer an already granted capability", tier)
			}
		}
	}
}
