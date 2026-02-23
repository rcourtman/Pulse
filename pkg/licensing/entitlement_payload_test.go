package licensing

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildEntitlementPayload_ActiveLicense(t *testing.T) {
	status := &LicenseStatus{
		Valid:    true,
		Tier:     TierPro,
		Features: append([]string(nil), TierFeatures[TierPro]...),
		MaxNodes: 50,
	}

	payload := BuildEntitlementPayload(status, "")

	if payload.SubscriptionState != string(SubStateActive) {
		t.Fatalf("expected subscription_state %q, got %q", SubStateActive, payload.SubscriptionState)
	}
	if !reflect.DeepEqual(payload.Capabilities, status.Features) {
		t.Fatalf("expected capabilities to match status features")
	}

	var nodeLimit *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == MaxNodesLicenseGateKey {
			nodeLimit = &payload.Limits[i]
			break
		}
	}
	if nodeLimit == nil {
		t.Fatalf("expected max_nodes limit in payload")
	}
	if nodeLimit.Limit != 50 {
		t.Fatalf("expected max_nodes limit 50, got %d", nodeLimit.Limit)
	}
	if len(payload.UpgradeReasons) != 0 {
		t.Fatalf("expected no upgrade reasons for pro tier, got %d", len(payload.UpgradeReasons))
	}
}

func TestBuildEntitlementPayload_FreeTier(t *testing.T) {
	status := &LicenseStatus{
		Valid:    true,
		Tier:     TierFree,
		Features: append([]string(nil), TierFeatures[TierFree]...),
	}

	payload := BuildEntitlementPayload(status, "")
	if len(payload.UpgradeReasons) == 0 {
		t.Fatalf("expected upgrade reasons for free tier")
	}
	for _, reason := range payload.UpgradeReasons {
		if reason.ActionURL == "" {
			t.Fatalf("expected action_url for reason %q", reason.Key)
		}
	}
}

func TestBuildEntitlementPayloadWithUsage_CurrentValues(t *testing.T) {
	status := &LicenseStatus{
		Valid:     true,
		Tier:      TierPro,
		Features:  append([]string(nil), TierFeatures[TierPro]...),
		MaxNodes:  50,
		MaxGuests: 100,
	}

	payload := BuildEntitlementPayloadWithUsage(status, "", EntitlementUsageSnapshot{
		Nodes:  12,
		Guests: 44,
	}, nil)

	var nodeLimit *LimitStatus
	var guestLimit *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == MaxNodesLicenseGateKey {
			nodeLimit = &payload.Limits[i]
		}
		if payload.Limits[i].Key == "max_guests" {
			guestLimit = &payload.Limits[i]
		}
	}

	if nodeLimit == nil {
		t.Fatalf("expected max_nodes limit")
	}
	if guestLimit == nil {
		t.Fatalf("expected max_guests limit")
	}
	if nodeLimit.Current != 12 {
		t.Fatalf("expected node current 12, got %d", nodeLimit.Current)
	}
	if guestLimit.Current != 44 {
		t.Fatalf("expected guest current 44, got %d", guestLimit.Current)
	}
}

func TestBuildEntitlementPayload_TrialState(t *testing.T) {
	expiresAt := time.Now().Add(36 * time.Hour).UTC().Format(time.RFC3339)
	status := &LicenseStatus{
		Valid:     true,
		Tier:      TierPro,
		Features:  append([]string(nil), TierFeatures[TierPro]...),
		ExpiresAt: &expiresAt,
	}

	payload := BuildEntitlementPayload(status, string(SubStateTrial))

	if payload.SubscriptionState != string(SubStateTrial) {
		t.Fatalf("expected subscription_state %q, got %q", SubStateTrial, payload.SubscriptionState)
	}
	if payload.TrialExpiresAt == nil {
		t.Fatalf("expected trial_expires_at to be populated for trial state")
	}
	if payload.TrialDaysRemaining == nil {
		t.Fatalf("expected trial_days_remaining to be populated for trial state")
	}
	if *payload.TrialDaysRemaining != 2 {
		t.Fatalf("expected trial_days_remaining 2, got %d", *payload.TrialDaysRemaining)
	}
}

func TestBuildEntitlementPayload_CopiesStatusDisplayFields(t *testing.T) {
	expiresAt := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	graceEnd := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	status := &LicenseStatus{
		Valid:          true,
		Tier:           TierPro,
		Email:          "owner@example.com",
		ExpiresAt:      &expiresAt,
		IsLifetime:     false,
		DaysRemaining:  3,
		InGracePeriod:  true,
		GracePeriodEnd: &graceEnd,
		Features:       append([]string(nil), TierFeatures[TierPro]...),
	}

	payload := BuildEntitlementPayload(status, string(SubStateGrace))

	if !payload.Valid {
		t.Fatalf("expected valid=true, got %v", payload.Valid)
	}
	if payload.LicensedEmail != "owner@example.com" {
		t.Fatalf("expected licensed_email %q, got %q", "owner@example.com", payload.LicensedEmail)
	}
	if payload.ExpiresAt == nil || *payload.ExpiresAt != expiresAt {
		t.Fatalf("expected expires_at %q, got %v", expiresAt, payload.ExpiresAt)
	}
	if payload.DaysRemaining != 3 {
		t.Fatalf("expected days_remaining 3, got %d", payload.DaysRemaining)
	}
	if !payload.InGracePeriod {
		t.Fatalf("expected in_grace_period=true, got %v", payload.InGracePeriod)
	}
	if payload.GracePeriodEnd == nil || *payload.GracePeriodEnd != graceEnd {
		t.Fatalf("expected grace_period_end %q, got %v", graceEnd, payload.GracePeriodEnd)
	}
}

func TestLimitState(t *testing.T) {
	tests := []struct {
		name    string
		current int64
		limit   int64
		want    string
	}{
		{name: "ok_below_threshold", current: 50, limit: 100, want: "ok"},
		{name: "warning_at_90_percent", current: 90, limit: 100, want: "warning"},
		{name: "enforced_at_limit", current: 100, limit: 100, want: "enforced"},
		{name: "enforced_above_limit", current: 110, limit: 100, want: "enforced"},
		{name: "ok_unlimited", current: 50, limit: 0, want: "ok"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := LimitState(tc.current, tc.limit)
			if got != tc.want {
				t.Fatalf("LimitState(%d, %d) = %q, want %q", tc.current, tc.limit, got, tc.want)
			}
		})
	}
}
