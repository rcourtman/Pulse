package licensing

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestBuildEntitlementPayload_ActiveLicense(t *testing.T) {
	status := &LicenseStatus{
		Valid:               true,
		Tier:                TierPro,
		Features:            append([]string(nil), TierFeatures[TierPro]...),
		MaxMonitoredSystems: 50,
	}

	payload := BuildEntitlementPayload(status, "")

	if payload.SubscriptionState != string(SubStateActive) {
		t.Fatalf("expected subscription_state %q, got %q", SubStateActive, payload.SubscriptionState)
	}
	if !reflect.DeepEqual(payload.Capabilities, status.Features) {
		t.Fatalf("expected capabilities to match status features")
	}

	var agentLimit *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == MaxMonitoredSystemsLicenseGateKey {
			agentLimit = &payload.Limits[i]
			break
		}
	}
	if agentLimit == nil {
		t.Fatalf("expected max_monitored_systems limit in payload")
	}
	if agentLimit.Limit != 50 {
		t.Fatalf("expected max_monitored_systems limit 50, got %d", agentLimit.Limit)
	}
	if len(payload.UpgradeReasons) != 0 {
		t.Fatalf("expected no upgrade reasons for pro tier, got %d", len(payload.UpgradeReasons))
	}
}

func TestBuildEntitlementPayload_StripsInternalOnlyCapabilities(t *testing.T) {
	status := &LicenseStatus{
		Valid: true,
		Tier:  TierEnterprise,
		Features: []string{
			FeatureRelay,
			FeatureDemoFixtures,
			FeatureMultiTenant,
		},
	}

	payload := BuildEntitlementPayload(status, "")
	if reflect.DeepEqual(payload.Capabilities, status.Features) {
		t.Fatalf("expected payload capabilities to filter internal-only features, got %v", payload.Capabilities)
	}
	if containsString(payload.Capabilities, FeatureDemoFixtures) {
		t.Fatalf("payload capabilities leaked internal feature %q: %v", FeatureDemoFixtures, payload.Capabilities)
	}
	if !containsString(payload.Capabilities, FeatureRelay) || !containsString(payload.Capabilities, FeatureMultiTenant) {
		t.Fatalf("payload capabilities lost public features: %v", payload.Capabilities)
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
		Valid:               true,
		Tier:                TierPro,
		Features:            append([]string(nil), TierFeatures[TierPro]...),
		MaxMonitoredSystems: 50,
		MaxGuests:           100,
	}

	payload := BuildEntitlementPayloadWithUsage(status, "", EntitlementUsageSnapshot{
		MonitoredSystems:          12,
		MonitoredSystemsAvailable: true,
		Guests:                    44,
		LegacyConnections: LegacyConnectionCounts{
			ProxmoxNodes:       2,
			DockerHosts:        1,
			KubernetesClusters: 3,
		},
	}, nil)

	var agentLimit *LimitStatus
	var guestLimit *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == MaxMonitoredSystemsLicenseGateKey {
			agentLimit = &payload.Limits[i]
		}
		if payload.Limits[i].Key == "max_guests" {
			guestLimit = &payload.Limits[i]
		}
	}

	if agentLimit == nil {
		t.Fatalf("expected max_monitored_systems limit")
	}
	if guestLimit == nil {
		t.Fatalf("expected max_guests limit")
	}
	if agentLimit.Current != 12 {
		t.Fatalf("expected agent current 12, got %d", agentLimit.Current)
	}
	if agentLimit.CurrentAvailable == nil || !*agentLimit.CurrentAvailable {
		t.Fatalf("expected agent current availability to be true, got %+v", agentLimit.CurrentAvailable)
	}
	if guestLimit.Current != 44 {
		t.Fatalf("expected guest current 44, got %d", guestLimit.Current)
	}
	if payload.LegacyConnections.ProxmoxNodes != 2 {
		t.Fatalf("expected proxmox_nodes 2, got %d", payload.LegacyConnections.ProxmoxNodes)
	}
	if payload.LegacyConnections.DockerHosts != 1 {
		t.Fatalf("expected docker_hosts 1, got %d", payload.LegacyConnections.DockerHosts)
	}
	if payload.LegacyConnections.KubernetesClusters != 3 {
		t.Fatalf(
			"expected kubernetes_clusters 3, got %d",
			payload.LegacyConnections.KubernetesClusters,
		)
	}
	if payload.HasMigrationGap {
		t.Fatal("expected has_migration_gap=false under monitored-system counting")
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

func TestBuildEntitlementPayloadWithUsage_RequiresCanonicalMonitoredSystemAvailability(t *testing.T) {
	status := &LicenseStatus{
		Valid:               true,
		Tier:                TierPro,
		Features:            append([]string(nil), TierFeatures[TierPro]...),
		MaxMonitoredSystems: 50,
	}

	payload := BuildEntitlementPayloadWithUsage(status, "", EntitlementUsageSnapshot{
		MonitoredSystems:                  12,
		Nodes:                             99,
		MonitoredSystemsUnavailableReason: "canonical_usage_unavailable",
	}, nil)

	if len(payload.Limits) != 1 {
		t.Fatalf("expected one limit, got %d", len(payload.Limits))
	}
	if payload.Limits[0].Current != 0 {
		t.Fatalf("expected unavailable canonical usage to remain 0, got %d", payload.Limits[0].Current)
	}
	if payload.Limits[0].CurrentAvailable == nil || *payload.Limits[0].CurrentAvailable {
		t.Fatalf("expected canonical usage availability to be false, got %+v", payload.Limits[0].CurrentAvailable)
	}
	if payload.Limits[0].CurrentUnavailableReason != "canonical_usage_unavailable" {
		t.Fatalf("CurrentUnavailableReason=%q, want %q", payload.Limits[0].CurrentUnavailableReason, "canonical_usage_unavailable")
	}
}

func TestBuildEntitlementPayloadWithUsage_MonitoredSystemUsageUnavailable(t *testing.T) {
	status := &LicenseStatus{
		Valid:               true,
		Tier:                TierPro,
		Features:            append([]string(nil), TierFeatures[TierPro]...),
		MaxMonitoredSystems: 50,
	}

	payload := BuildEntitlementPayloadWithUsage(status, "", EntitlementUsageSnapshot{
		MonitoredSystemsUnavailableReason: "supplemental_inventory_unsettled",
	}, nil)
	if len(payload.Limits) != 1 {
		t.Fatalf("expected one limit, got %d", len(payload.Limits))
	}
	if payload.Limits[0].Current != 0 {
		t.Fatalf("expected unresolved current to fall back to 0, got %d", payload.Limits[0].Current)
	}
	if payload.Limits[0].CurrentAvailable == nil || *payload.Limits[0].CurrentAvailable {
		t.Fatalf("expected unresolved current availability to be false, got %+v", payload.Limits[0].CurrentAvailable)
	}
	if payload.Limits[0].CurrentUnavailableReason != "supplemental_inventory_unsettled" {
		t.Fatalf("CurrentUnavailableReason=%q, want %q", payload.Limits[0].CurrentUnavailableReason, "supplemental_inventory_unsettled")
	}
}

func TestBuildEntitlementPayloadWithUsage_CopiesMonitoredSystemContinuity(t *testing.T) {
	status := &LicenseStatus{
		Valid:               true,
		Tier:                TierPro,
		Features:            append([]string(nil), TierFeatures[TierPro]...),
		MaxMonitoredSystems: 23,
		MonitoredSystemContinuity: &MonitoredSystemContinuityStatus{
			PlanLimit:          10,
			GrandfatheredFloor: 23,
			EffectiveLimit:     23,
			CapturePending:     false,
			CapturedAt:         123,
		},
	}

	payload := BuildEntitlementPayloadWithUsage(status, "", EntitlementUsageSnapshot{
		MonitoredSystems:          23,
		MonitoredSystemsAvailable: true,
	}, nil)

	if payload.MonitoredSystemContinuity == nil {
		t.Fatal("expected monitored-system continuity to be copied")
	}
	if payload.MonitoredSystemContinuity.PlanLimit != 10 {
		t.Fatalf("PlanLimit=%d, want %d", payload.MonitoredSystemContinuity.PlanLimit, 10)
	}
	if payload.MonitoredSystemContinuity.EffectiveLimit != 23 {
		t.Fatalf("EffectiveLimit=%d, want %d", payload.MonitoredSystemContinuity.EffectiveLimit, 23)
	}
	if payload.MonitoredSystemContinuity.GrandfatheredFloor != 23 {
		t.Fatalf("GrandfatheredFloor=%d, want %d", payload.MonitoredSystemContinuity.GrandfatheredFloor, 23)
	}
	if payload.MonitoredSystemContinuity.CapturePending {
		t.Fatal("expected continuity capture to be settled")
	}
}

func TestBuildEntitlementPayload_CopiesStatusDisplayFields(t *testing.T) {
	tests := []struct {
		name        string
		planVersion string
	}{
		{name: "lifetime grandfathered", planVersion: "v5_lifetime_grandfathered"},
		{name: "monthly grandfathered", planVersion: "v5_pro_monthly_grandfathered"},
		{name: "annual grandfathered", planVersion: "v5_pro_annual_grandfathered"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expiresAt := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
			graceEnd := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
			status := &LicenseStatus{
				Valid:          true,
				Tier:           TierPro,
				PlanVersion:    tc.planVersion,
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
			if payload.PlanVersion != tc.planVersion {
				t.Fatalf("expected plan_version %q, got %q", tc.planVersion, payload.PlanVersion)
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
		})
	}
}

func TestBuildEntitlementPayload_MaxHistoryDays(t *testing.T) {
	tests := []struct {
		name     string
		tier     Tier
		wantDays int
	}{
		{"free tier gets 7 days", TierFree, 7},
		{"relay tier gets 14 days", TierRelay, 14},
		{"pro tier gets 90 days", TierPro, 90},
		{"pro_plus tier gets 90 days", TierProPlus, 90},
		{"pro_annual tier gets 90 days", TierProAnnual, 90},
		{"lifetime tier gets 90 days", TierLifetime, 90},
		{"cloud tier gets 90 days", TierCloud, 90},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status := &LicenseStatus{
				Valid:    true,
				Tier:     tc.tier,
				Features: append([]string(nil), TierFeatures[tc.tier]...),
			}
			payload := BuildEntitlementPayload(status, "")
			if payload.MaxHistoryDays != tc.wantDays {
				t.Fatalf("expected MaxHistoryDays=%d for tier %q, got %d", tc.wantDays, tc.tier, payload.MaxHistoryDays)
			}
		})
	}
}

func TestBuildEntitlementPayload_NilStatus_MaxHistoryDays(t *testing.T) {
	payload := BuildEntitlementPayloadWithUsage(nil, "", EntitlementUsageSnapshot{}, nil)
	if payload.MaxHistoryDays != TierHistoryDays[TierFree] {
		t.Fatalf("expected MaxHistoryDays=%d for nil status, got %d", TierHistoryDays[TierFree], payload.MaxHistoryDays)
	}
	if payload.HasMigrationGap {
		t.Fatal("expected has_migration_gap=false for nil status payload")
	}
	if payload.LegacyConnections.Total() != 0 {
		t.Fatalf("expected zero legacy connections for nil status payload, got %+v", payload.LegacyConnections)
	}
}

func TestBuildCommercialPosturePayloadWithUsage_CurrentValues(t *testing.T) {
	status := &LicenseStatus{
		Valid:               true,
		Tier:                TierFree,
		Features:            append([]string(nil), TierFeatures[TierFree]...),
		MaxMonitoredSystems: 5,
	}

	payload := BuildCommercialPosturePayloadWithUsage(status, "", EntitlementUsageSnapshot{
		MonitoredSystems:          7,
		MonitoredSystemsAvailable: true,
		LegacyConnections: LegacyConnectionCounts{
			ProxmoxNodes: 2,
			DockerHosts:  1,
		},
	}, nil)

	if payload.Tier != string(TierFree) {
		t.Fatalf("expected tier=%q, got %q", TierFree, payload.Tier)
	}
	if payload.SubscriptionState != string(SubStateActive) {
		t.Fatalf("expected subscription_state=%q, got %q", SubStateActive, payload.SubscriptionState)
	}
	if len(payload.UpgradeReasons) == 0 {
		t.Fatal("expected commercial posture to preserve upgrade reasons")
	}
	if payload.LegacyConnections.ProxmoxNodes != 2 || payload.LegacyConnections.DockerHosts != 1 {
		t.Fatalf("expected legacy connection counts to be preserved, got %+v", payload.LegacyConnections)
	}
	if payload.HasMigrationGap {
		t.Fatal("expected has_migration_gap=false under canonical monitored-system counting")
	}
}

func TestCommercialPosturePayloadFromEntitlementPayload_StripsBillingIdentityFields(t *testing.T) {
	expiresAt := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	payload := BuildEntitlementPayloadWithUsage(&LicenseStatus{
		Valid:               true,
		Tier:                TierPro,
		PlanVersion:         "v5_lifetime_grandfathered",
		Email:               "owner@example.com",
		ExpiresAt:           &expiresAt,
		IsLifetime:          false,
		DaysRemaining:       2,
		Features:            append([]string(nil), TierFeatures[TierPro]...),
		MaxMonitoredSystems: 50,
	}, string(SubStateActive), EntitlementUsageSnapshot{
		MonitoredSystems:          12,
		MonitoredSystemsAvailable: true,
		LegacyConnections: LegacyConnectionCounts{
			ProxmoxNodes: 1,
		},
	}, nil)

	posture := CommercialPosturePayloadFromEntitlementPayload(payload)

	if posture.Tier != string(TierPro) {
		t.Fatalf("expected tier=%q, got %q", TierPro, posture.Tier)
	}
	if posture.SubscriptionState != string(SubStateActive) {
		t.Fatalf("expected subscription_state=%q, got %q", SubStateActive, posture.SubscriptionState)
	}
	if posture.LegacyConnections.ProxmoxNodes != 1 {
		t.Fatalf("expected proxmox_nodes=1, got %+v", posture.LegacyConnections)
	}

	body, err := json.Marshal(posture)
	if err != nil {
		t.Fatalf("marshal commercial posture: %v", err)
	}
	jsonBody := string(body)
	for _, forbidden := range []string{
		`"capabilities"`,
		`"limits"`,
		`"licensed_email"`,
		`"plan_version"`,
		`"hosted_mode"`,
		`"valid"`,
		`"max_history_days"`,
	} {
		if strings.Contains(jsonBody, forbidden) {
			t.Fatalf("commercial posture leaked billing/runtime field %s in %s", forbidden, jsonBody)
		}
	}
}

func TestBuildRuntimeCapabilitiesPayloadWithUsage_CurrentValues(t *testing.T) {
	status := &LicenseStatus{
		Valid:               true,
		Tier:                TierRelay,
		Features:            append([]string(nil), TierFeatures[TierRelay]...),
		MaxMonitoredSystems: 12,
	}

	payload := BuildRuntimeCapabilitiesPayloadWithUsage(status, "", EntitlementUsageSnapshot{
		MonitoredSystems:          5,
		MonitoredSystemsAvailable: true,
	})

	if !reflect.DeepEqual(payload.Capabilities, status.Features) {
		t.Fatalf("expected capabilities to match status features")
	}
	if payload.MaxHistoryDays != TierHistoryDays[TierRelay] {
		t.Fatalf("expected relay max history days %d, got %d", TierHistoryDays[TierRelay], payload.MaxHistoryDays)
	}
	if len(payload.Limits) != 1 {
		t.Fatalf("expected one runtime limit, got %d", len(payload.Limits))
	}
	if payload.Limits[0].Key != MaxMonitoredSystemsLicenseGateKey {
		t.Fatalf("expected runtime limit key %q, got %q", MaxMonitoredSystemsLicenseGateKey, payload.Limits[0].Key)
	}
	if payload.Limits[0].Current != 5 {
		t.Fatalf("expected runtime current 5, got %d", payload.Limits[0].Current)
	}
	if payload.Limits[0].CurrentAvailable == nil || !*payload.Limits[0].CurrentAvailable {
		t.Fatalf("expected runtime current availability true, got %+v", payload.Limits[0].CurrentAvailable)
	}
}

func TestBuildRuntimeCapabilitiesPayloadWithUsage_StripsInternalOnlyCapabilities(t *testing.T) {
	status := &LicenseStatus{
		Valid: true,
		Tier:  TierEnterprise,
		Features: []string{
			FeatureRelay,
			FeatureDemoFixtures,
		},
		MaxMonitoredSystems: 12,
	}

	payload := BuildRuntimeCapabilitiesPayloadWithUsage(status, "", EntitlementUsageSnapshot{})
	if containsString(payload.Capabilities, FeatureDemoFixtures) {
		t.Fatalf("runtime capabilities leaked internal feature %q: %v", FeatureDemoFixtures, payload.Capabilities)
	}
	if !containsString(payload.Capabilities, FeatureRelay) {
		t.Fatalf("runtime capabilities lost public feature %q: %v", FeatureRelay, payload.Capabilities)
	}
}

func TestBuildRuntimeCapabilitiesPayload_NilStatusDefaultsToFreeTier(t *testing.T) {
	payload := BuildRuntimeCapabilitiesPayload(nil, "")
	if payload.MaxHistoryDays != TierHistoryDays[TierFree] {
		t.Fatalf("expected MaxHistoryDays=%d for nil status, got %d", TierHistoryDays[TierFree], payload.MaxHistoryDays)
	}
	if len(payload.Capabilities) != 0 {
		t.Fatalf("expected no runtime capabilities for nil status, got %+v", payload.Capabilities)
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
		// Small-limit behavior: warn at N-1
		{name: "small_limit_ok", current: 3, limit: 5, want: "ok"},
		{name: "small_limit_warning_at_n_minus_1", current: 4, limit: 5, want: "warning"},
		{name: "small_limit_enforced_at_limit", current: 5, limit: 5, want: "enforced"},
		{name: "small_limit_8_warning", current: 7, limit: 8, want: "warning"},
		{name: "small_limit_8_ok", current: 6, limit: 8, want: "ok"},
		{name: "small_limit_10_warning", current: 9, limit: 10, want: "warning"},
		{name: "small_limit_10_ok", current: 8, limit: 10, want: "ok"},
		{name: "limit_1_zero_usage_ok", current: 0, limit: 1, want: "ok"},
		{name: "limit_1_at_limit_enforced", current: 1, limit: 1, want: "enforced"},
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
