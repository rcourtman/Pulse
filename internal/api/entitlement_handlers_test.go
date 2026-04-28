package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	licensetestsupport "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/testsupport"
)

func containsCapability(values []string, key string) bool {
	for _, value := range values {
		if value == key {
			return true
		}
	}
	return false
}

func TestBuildEntitlementPayload_ActiveLicense(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:               true,
		Tier:                license.TierPro,
		Features:            append([]string(nil), license.TierFeatures[license.TierPro]...),
		MaxMonitoredSystems: 0,
	}

	payload := buildEntitlementPayload(status, "")

	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("expected subscription_state %q, got %q", license.SubStateActive, payload.SubscriptionState)
	}
	if !reflect.DeepEqual(payload.Capabilities, status.Features) {
		t.Fatalf("expected capabilities to match status features")
	}

	if len(payload.Limits) != 0 {
		t.Fatalf("expected no max_monitored_systems limit in payload, got %+v", payload.Limits)
	}
	if payload.MonitoredSystemCapacity == nil {
		t.Fatal("expected monitored_system_capacity in payload")
	}
	if payload.MonitoredSystemCapacity.Mode != "usage_unavailable" {
		t.Fatalf("expected usage_unavailable monitored-system capacity before inventory settles, got %+v", payload.MonitoredSystemCapacity)
	}
	if len(payload.UpgradeReasons) != 0 {
		t.Fatalf("expected no upgrade reasons for pro tier, got %d", len(payload.UpgradeReasons))
	}
}

func TestBuildEntitlementPayload_FreeTier(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:    true,
		Tier:     license.TierFree,
		Features: append([]string(nil), license.TierFeatures[license.TierFree]...),
	}

	payload := buildEntitlementPayload(status, "")

	// Upgrade reasons should cover marketed upgrade surfaces, not every
	// compatibility-only capability that may exist in the Pro feature set.
	wantReasons := pkglicensing.GenerateUpgradeReasons(status.Features)
	if len(payload.UpgradeReasons) != len(wantReasons) {
		t.Fatalf("expected %d upgrade reasons for free tier, got %d", len(wantReasons), len(payload.UpgradeReasons))
	}
	for _, reason := range payload.UpgradeReasons {
		if reason.ActionURL == "" {
			t.Fatalf("expected action_url for reason %q", reason.Key)
		}
	}
}

func TestBuildEntitlementPayloadWithUsage_CurrentValues(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:               true,
		Tier:                license.TierPro,
		Features:            append([]string(nil), license.TierFeatures[license.TierPro]...),
		MaxMonitoredSystems: 0,
		MaxGuests:           100,
	}

	payload := buildEntitlementPayloadWithUsage(status, "", entitlementUsageSnapshot{
		MonitoredSystems:          12,
		MonitoredSystemsAvailable: true,
		Guests:                    44,
		LegacyConnections: pkglicensing.LegacyConnectionCounts{
			ProxmoxNodes:       2,
			DockerHosts:        1,
			KubernetesClusters: 3,
		},
	}, nil)

	var agentLimit *LimitStatus
	var guestLimit *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == "max_monitored_systems" {
			agentLimit = &payload.Limits[i]
		}
		if payload.Limits[i].Key == "max_guests" {
			guestLimit = &payload.Limits[i]
		}
	}

	if guestLimit != nil {
		t.Fatalf("expected self-hosted pro to omit max_guests limit, got %+v", guestLimit)
	}
	if agentLimit != nil {
		t.Fatalf("expected no max_monitored_systems limit for self-hosted pro, got %+v", agentLimit)
	}
	if payload.MonitoredSystemCapacity == nil {
		t.Fatal("expected monitored_system_capacity")
	}
	if payload.MonitoredSystemCapacity.Mode != "unlimited" || payload.MonitoredSystemCapacity.Current != 12 {
		t.Fatalf("expected unlimited monitored-system capacity with current=12, got %+v", payload.MonitoredSystemCapacity)
	}
	if payload.LegacyConnections.ProxmoxNodes != 2 {
		t.Fatalf("expected proxmox_nodes 2, got %d", payload.LegacyConnections.ProxmoxNodes)
	}
	if payload.HasMigrationGap {
		t.Fatal("expected has_migration_gap=false under monitored-system counting")
	}
}

func TestBuildEntitlementPayloadWithUsage_MonitoredSystemUsageUnavailable(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:               true,
		Tier:                license.TierPro,
		Features:            append([]string(nil), license.TierFeatures[license.TierPro]...),
		MaxMonitoredSystems: 0,
	}

	payload := buildEntitlementPayloadWithUsage(status, "", entitlementUsageSnapshot{}, nil)
	if len(payload.Limits) != 0 {
		t.Fatalf("expected no monitored-system limit, got %d", len(payload.Limits))
	}
	if payload.MonitoredSystemCapacity == nil {
		t.Fatal("expected monitored_system_capacity")
	}
	if payload.MonitoredSystemCapacity.Mode != "usage_unavailable" {
		t.Fatalf("expected usage_unavailable monitored-system capacity, got %+v", payload.MonitoredSystemCapacity)
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

func TestBuildCommercialPosturePayloadWithUsage_CurrentValues(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:               true,
		Tier:                license.TierFree,
		Features:            append([]string(nil), license.TierFeatures[license.TierFree]...),
		MaxMonitoredSystems: 0,
	}

	payload := buildCommercialPosturePayloadWithUsage(status, "", entitlementUsageSnapshot{
		MonitoredSystems:          7,
		MonitoredSystemsAvailable: true,
		LegacyConnections: pkglicensing.LegacyConnectionCounts{
			ProxmoxNodes: 2,
			DockerHosts:  1,
		},
	}, nil)

	if payload.Tier != string(license.TierFree) {
		t.Fatalf("expected tier=%q, got %q", license.TierFree, payload.Tier)
	}
	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("expected subscription_state=%q, got %q", license.SubStateActive, payload.SubscriptionState)
	}
	if len(payload.UpgradeReasons) == 0 {
		t.Fatal("expected upgrade reasons for free-tier commercial posture")
	}
	if payload.MonitoredSystemCapacity == nil || payload.MonitoredSystemCapacity.Mode != "unlimited" {
		t.Fatalf("expected unlimited monitored-system capacity in commercial posture, got %+v", payload.MonitoredSystemCapacity)
	}
	if payload.LegacyConnections.ProxmoxNodes != 2 || payload.LegacyConnections.DockerHosts != 1 {
		t.Fatalf("expected legacy counts to be preserved, got %+v", payload.LegacyConnections)
	}
	if payload.HasMigrationGap {
		t.Fatal("expected has_migration_gap=false under canonical monitored-system counting")
	}
}

func TestHandleCommercialPosture_ActiveLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	handler := createTestHandler(t)
	licenseKey, err := licensetestsupport.GenerateLicenseForTesting(
		"owner@example.com",
		pkglicensing.TierPro,
		24*time.Hour,
	)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}
	if _, err := handler.Service(context.Background()).Activate(licenseKey); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/license/commercial-posture", nil)
	rec := httptest.NewRecorder()

	handler.HandleCommercialPosture(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, forbidden := range []string{
		`"capabilities"`,
		`"limits"`,
		`"licensed_email"`,
		`"plan_version"`,
		`"max_history_days"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("commercial posture leaked field %s in %s", forbidden, body)
		}
	}

	var payload CommercialPosturePayload
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode commercial posture payload: %v", err)
	}
	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("expected subscription_state %q, got %q", license.SubStateActive, payload.SubscriptionState)
	}
	if payload.Tier != string(license.TierPro) {
		t.Fatalf("expected tier %q, got %q", license.TierPro, payload.Tier)
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

func TestBuildEntitlementPayload_PreservesPlanVersionForSelfHostedJWT(t *testing.T) {
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
			status := &license.LicenseStatus{
				Valid:       true,
				Tier:        license.TierPro,
				PlanVersion: tc.planVersion,
				Features:    append([]string(nil), license.TierFeatures[license.TierPro]...),
			}

			payload := buildEntitlementPayload(status, string(license.SubStateActive))

			if payload.PlanVersion != tc.planVersion {
				t.Fatalf("plan_version=%q, want %q", payload.PlanVersion, tc.planVersion)
			}
		})
	}
}

func TestEntitlementHandler_HostedEvaluatorKeepsCloudLimitsWhenNoLicense(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	orgID := "test-hosted-entitlements"
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s) failed: %v", orgID, err)
	}

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities: []string{
			license.FeatureAIPatrol,
			license.FeatureAIAutoFix,
		},
		Limits: map[string]int64{
			"max_monitored_systems": 5,
		},
		PlanVersion:       "cloud_starter",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("SaveBillingState(%s) failed: %v", orgID, err)
	}

	h := NewLicenseHandlers(mtp, true)

	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateActive)
	}
	if payload.PlanVersion != "cloud_starter" {
		t.Fatalf("plan_version=%q, want %q", payload.PlanVersion, "cloud_starter")
	}

	contains := func(values []string, key string) bool {
		for _, v := range values {
			if v == key {
				return true
			}
		}
		return false
	}

	if !contains(payload.Capabilities, license.FeatureAIAutoFix) {
		t.Fatalf("expected capabilities to include %q, got %v", license.FeatureAIAutoFix, payload.Capabilities)
	}
	if !contains(payload.Capabilities, license.FeatureAIPatrol) {
		t.Fatalf("expected capabilities to include %q, got %v", license.FeatureAIPatrol, payload.Capabilities)
	}

	var maxMonitoredSystems *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == "max_monitored_systems" {
			maxMonitoredSystems = &payload.Limits[i]
			break
		}
	}
	if maxMonitoredSystems == nil {
		t.Fatalf("expected max_monitored_systems limit in payload, got %v", payload.Limits)
	}
	if maxMonitoredSystems.Limit != 10 {
		t.Fatalf("max_monitored_systems.limit=%d, want %d", maxMonitoredSystems.Limit, 10)
	}

	// Parity: every advertised capability must be enforced by HasFeature.
	ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
	svc, _, err := h.getTenantComponents(ctx)
	if err != nil {
		t.Fatalf("getTenantComponents failed: %v", err)
	}
	for _, cap := range payload.Capabilities {
		if !svc.HasFeature(cap) {
			t.Fatalf("parity mismatch: HasFeature(%q)=false but capability present in payload", cap)
		}
	}
}

func TestEntitlementHandler_SelfHostedPaidEvaluatorStateIsUncapped(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	orgID := "test-self-hosted-paid-entitlements"
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s) failed: %v", orgID, err)
	}

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities: []string{
			license.FeatureAIPatrol,
			license.FeatureAIAutoFix,
		},
		Limits: map[string]int64{
			"max_monitored_systems": 5,
			"max_guests":            50,
		},
		PlanVersion:       "pro",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("SaveBillingState(%s) failed: %v", orgID, err)
	}

	h := NewLicenseHandlers(mtp, true)

	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	if payload.PlanVersion != "pro" {
		t.Fatalf("plan_version=%q, want %q", payload.PlanVersion, "pro")
	}
	for _, limit := range payload.Limits {
		if limit.Key == "max_monitored_systems" || limit.Key == "max_guests" {
			t.Fatalf("expected self-hosted paid evaluator payload to omit volume caps, got %+v", payload.Limits)
		}
	}
}

func TestEntitlementHandler_GrandfatheredRecurringEvaluatorStateIsUncapped(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	orgID := "test-grandfathered-recurring-entitlements"
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s) failed: %v", orgID, err)
	}

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities: []string{
			license.FeatureAIPatrol,
			license.FeatureAIAutoFix,
		},
		Limits: map[string]int64{
			"max_monitored_systems": 10,
			"max_guests":            50,
		},
		PlanVersion:       "v5_pro_monthly_grandfathered",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("SaveBillingState(%s) failed: %v", orgID, err)
	}

	h := NewLicenseHandlers(mtp, true)

	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	if payload.PlanVersion != "v5_pro_monthly_grandfathered" {
		t.Fatalf("plan_version=%q, want %q", payload.PlanVersion, "v5_pro_monthly_grandfathered")
	}
	for _, limit := range payload.Limits {
		if limit.Key == "max_monitored_systems" || limit.Key == "max_guests" {
			t.Fatalf("expected grandfathered recurring evaluator payload to omit capped limits, got %+v", payload.Limits)
		}
	}
}

func TestHandleRuntimeCapabilities_HostedCommunityEvaluatorStateStripsLegacyCommercialCaps(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	orgID := "test-hosted-community-runtime-capabilities"
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s) failed: %v", orgID, err)
	}

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities: []string{
			license.FeatureAIPatrol,
		},
		Limits: map[string]int64{
			"max_monitored_systems": 1,
			"max_guests":            5,
		},
		PlanVersion:       "community",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("SaveBillingState(%s) failed: %v", orgID, err)
	}

	h := NewLicenseHandlers(mtp, true)

	req := httptest.NewRequest(http.MethodGet, "/api/license/runtime-capabilities", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
	rec := httptest.NewRecorder()
	h.HandleRuntimeCapabilities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}

	var payload RuntimeCapabilitiesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	for _, limit := range payload.Limits {
		if limit.Key == "max_monitored_systems" || limit.Key == "max_guests" {
			t.Fatalf("expected runtime capabilities to omit stale commercial caps, got %+v", payload.Limits)
		}
	}
	if payload.MonitoredSystemCapacity == nil {
		t.Fatal("expected monitored_system_capacity in runtime capabilities payload")
	}
	if payload.MonitoredSystemCapacity.Limit != 0 {
		t.Fatalf("monitored_system_capacity.limit=%d, want %d", payload.MonitoredSystemCapacity.Limit, 0)
	}
}

func TestEntitlementHandler_SelfHostedTrialEligibilityRetiredForFreshOrg(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateActive)
	}
	if payload.TrialEligible {
		t.Fatalf("trial_eligible=%v, want false", payload.TrialEligible)
	}
	if payload.TrialEligibilityReason != "" {
		t.Fatalf("trial_eligibility_reason=%q, want empty", payload.TrialEligibilityReason)
	}
	for _, limit := range payload.Limits {
		if limit.Key == "max_monitored_systems" {
			t.Fatalf("expected no max_monitored_systems limit in payload, got %+v", payload.Limits)
		}
	}
	if payload.MonitoredSystemCapacity == nil {
		t.Fatal("expected monitored_system_capacity in payload")
	}
	if payload.MonitoredSystemCapacity.Limit != 0 || payload.MonitoredSystemCapacity.BlocksNewSystems {
		t.Fatalf(
			"expected uncapped monitored_system_capacity for fresh community org, got %+v",
			payload.MonitoredSystemCapacity,
		)
	}
}

func TestEntitlementHandler_OverflowOnlyBillingStateReportsActiveCommunity(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	now := time.Now().Unix()
	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		Capabilities:      []string{},
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		OverflowGrantedAt: &now,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	h := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateActive)
	}
	if payload.TrialEligible {
		t.Fatalf("trial_eligible=%v, want false", payload.TrialEligible)
	}
	if payload.MonitoredSystemCapacity == nil || payload.MonitoredSystemCapacity.Limit != 0 || payload.MonitoredSystemCapacity.BlocksNewSystems {
		t.Fatalf("expected uncapped monitored_system_capacity, got %+v", payload.MonitoredSystemCapacity)
	}
}

func TestEntitlementHandler_DevModeMirrorsFeatureGateCapabilities(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_MULTI_TENANT_ENABLED", "")

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	if !containsCapability(payload.Capabilities, license.FeatureAdvancedReporting) {
		t.Fatalf("expected dev entitlements to include %q, got %v", license.FeatureAdvancedReporting, payload.Capabilities)
	}
	if containsCapability(payload.Capabilities, license.FeatureMultiTenant) {
		t.Fatalf("expected dev entitlements to omit %q while runtime flag is disabled, got %v", license.FeatureMultiTenant, payload.Capabilities)
	}
	for _, feature := range []string{
		license.FeatureMultiUser,
		license.FeatureWhiteLabel,
		license.FeatureUnlimited,
	} {
		if containsCapability(payload.Capabilities, feature) {
			t.Fatalf("expected dev entitlements to omit non-runtime capability %q, got %v", feature, payload.Capabilities)
		}
	}
	if len(payload.UpgradeReasons) != 0 {
		t.Fatalf("expected no upgrade reasons in dev mode, got %v", payload.UpgradeReasons)
	}

	svc, _, err := h.getTenantComponents(ctx)
	if err != nil {
		t.Fatalf("getTenantComponents failed: %v", err)
	}
	for _, cap := range payload.Capabilities {
		if !svc.HasFeature(cap) {
			t.Fatalf("parity mismatch: HasFeature(%q)=false but capability present in payload", cap)
		}
	}
}

func TestEntitlementHandler_DevModeIncludesMultiTenantWhenRuntimeEnabled(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_MULTI_TENANT_ENABLED", "true")

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	if !containsCapability(payload.Capabilities, license.FeatureMultiTenant) {
		t.Fatalf("expected dev entitlements to include %q when runtime flag is enabled, got %v", license.FeatureMultiTenant, payload.Capabilities)
	}
}

func TestEntitlementHandler_ExpiredTrialStateDoesNotExposeStartReason(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	orgID := "default"
	now := time.Now()
	startedAt := now.Add(-15 * 24 * time.Hour).Unix()
	endsAt := now.Add(-24 * time.Hour).Unix()
	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities:      []string{},
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       "trial",
		SubscriptionState: entitlements.SubStateExpired,
		TrialStartedAt:    &startedAt,
		TrialEndsAt:       &endsAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	h := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if payload.TrialEligible {
		t.Fatalf("trial_eligible=%v, want false", payload.TrialEligible)
	}
	if payload.TrialEligibilityReason != "" {
		t.Fatalf("trial_eligibility_reason=%q, want empty", payload.TrialEligibilityReason)
	}
}

func TestEntitlementHandler_CommercialMigrationDoesNotExposeTrialStartReason(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	orgID := "default"
	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities:      []string{},
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       string(entitlements.SubStateExpired),
		SubscriptionState: entitlements.SubStateExpired,
		CommercialMigration: &pkglicensing.CommercialMigrationStatus{
			Source:            pkglicensing.CommercialMigrationSourceV5License,
			State:             pkglicensing.CommercialMigrationStatePending,
			Reason:            pkglicensing.CommercialMigrationReasonExchangeUnavailable,
			RecommendedAction: pkglicensing.CommercialMigrationActionRetryActivation,
		},
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	h := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if payload.CommercialMigration == nil {
		t.Fatal("expected commercial_migration payload")
	}
	if payload.CommercialMigration.State != pkglicensing.CommercialMigrationStatePending {
		t.Fatalf("commercial_migration.state=%q, want %q", payload.CommercialMigration.State, pkglicensing.CommercialMigrationStatePending)
	}
	if payload.TrialEligible {
		t.Fatalf("trial_eligible=%v, want false", payload.TrialEligible)
	}
	if payload.TrialEligibilityReason != "" {
		t.Fatalf("trial_eligibility_reason=%q, want empty", payload.TrialEligibilityReason)
	}
}

// countProMinusFreeFeatures returns the number of Pro features not included in Free.
func countProMinusFreeFeatures() int {
	freeSet := make(map[string]struct{}, len(license.TierFeatures[license.TierFree]))
	for _, f := range license.TierFeatures[license.TierFree] {
		freeSet[f] = struct{}{}
	}
	count := 0
	for _, f := range license.TierFeatures[license.TierPro] {
		if _, ok := freeSet[f]; !ok {
			count++
		}
	}
	return count
}
