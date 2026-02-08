package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

func TestBuildEntitlementPayload_ActiveLicense(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:    true,
		Tier:     license.TierPro,
		Features: append([]string(nil), license.TierFeatures[license.TierPro]...),
		MaxNodes: 50,
	}

	payload := buildEntitlementPayload(status, "")

	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("expected subscription_state %q, got %q", license.SubStateActive, payload.SubscriptionState)
	}
	if !reflect.DeepEqual(payload.Capabilities, status.Features) {
		t.Fatalf("expected capabilities to match status features")
	}

	var nodeLimit *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == "max_nodes" {
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
}

func TestBuildEntitlementPayload_FreeTier(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:    true,
		Tier:     license.TierFree,
		Features: append([]string(nil), license.TierFeatures[license.TierFree]...),
	}

	payload := buildEntitlementPayload(status, "")

	if len(payload.UpgradeReasons) == 0 {
		t.Fatalf("expected upgrade reasons for free tier")
	}

	reasonsByKey := make(map[string]UpgradeReason, len(payload.UpgradeReasons))
	for _, reason := range payload.UpgradeReasons {
		reasonsByKey[reason.Key] = reason
	}

	if _, ok := reasonsByKey["ai_autofix"]; !ok {
		t.Fatalf("expected ai_autofix upgrade reason")
	}
	if _, ok := reasonsByKey["rbac"]; !ok {
		t.Fatalf("expected rbac upgrade reason")
	}
}

func TestBuildEntitlementPayload_Expired(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:         false,
		InGracePeriod: false,
	}

	payload := buildEntitlementPayload(status, "")
	if payload.SubscriptionState != string(license.SubStateExpired) {
		t.Fatalf("expected subscription_state %q, got %q", license.SubStateExpired, payload.SubscriptionState)
	}
}

func TestBuildEntitlementPayload_GracePeriod(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:         true,
		InGracePeriod: true,
	}

	payload := buildEntitlementPayload(status, "")
	if payload.SubscriptionState != string(license.SubStateGrace) {
		t.Fatalf("expected subscription_state %q, got %q", license.SubStateGrace, payload.SubscriptionState)
	}
}

func TestBuildEntitlementPayload_NilCapabilities(t *testing.T) {
	status := &license.LicenseStatus{
		Features: nil,
	}

	payload := buildEntitlementPayload(status, "")
	if payload.Capabilities == nil {
		t.Fatalf("expected capabilities to be an empty slice, got nil")
	}
	if len(payload.Capabilities) != 0 {
		t.Fatalf("expected capabilities length 0, got %d", len(payload.Capabilities))
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
			got := limitState(tc.current, tc.limit)
			if got != tc.want {
				t.Fatalf("limitState(%d, %d) = %q, want %q", tc.current, tc.limit, got, tc.want)
			}
		})
	}
}

func TestBuildEntitlementPayload_TrialState(t *testing.T) {
	expiresAt := time.Now().Add(36 * time.Hour).UTC().Format(time.RFC3339)
	status := &license.LicenseStatus{
		Valid:     true,
		Tier:      license.TierPro,
		Features:  append([]string(nil), license.TierFeatures[license.TierPro]...),
		ExpiresAt: &expiresAt,
	}

	payload := buildEntitlementPayload(status, string(license.SubStateTrial))

	if payload.SubscriptionState != string(license.SubStateTrial) {
		t.Fatalf("expected subscription_state %q, got %q", license.SubStateTrial, payload.SubscriptionState)
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
