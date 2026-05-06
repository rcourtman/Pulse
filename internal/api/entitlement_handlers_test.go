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
		Valid:    true,
		Tier:     license.TierPro,
		Features: append([]string(nil), license.TierFeatures[license.TierPro]...),
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
		Valid:     true,
		Tier:      license.TierPro,
		Features:  append([]string(nil), license.TierFeatures[license.TierPro]...),
		MaxGuests: 100,
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
	if payload.LegacyConnections.ProxmoxNodes != 2 {
		t.Fatalf("expected proxmox_nodes 2, got %d", payload.LegacyConnections.ProxmoxNodes)
	}
	if payload.HasMigrationGap {
		t.Fatal("expected has_migration_gap=false under monitored-system counting")
	}
}

func TestBuildEntitlementPayloadWithUsage_MonitoredSystemUsageUnavailable(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:    true,
		Tier:     license.TierPro,
		Features: append([]string(nil), license.TierFeatures[license.TierPro]...),
	}

	payload := buildEntitlementPayloadWithUsage(status, "", entitlementUsageSnapshot{}, nil)
	if len(payload.Limits) != 0 {
		t.Fatalf("expected no monitored-system limit, got %d", len(payload.Limits))
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
		Valid:    true,
		Tier:     license.TierFree,
		Features: append([]string(nil), license.TierFeatures[license.TierFree]...),
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

	for i := range payload.Limits {
		if payload.Limits[i].Key == "max_monitored_systems" {
			t.Fatalf("expected retired monitored-system limit to be omitted, got %v", payload.Limits)
		}
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
}

func TestHandleRuntimeCapabilities_CommunityRuntimeBlocksPrivateProCapabilities(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		Capabilities: []string{
			license.FeatureRelay,
			license.FeatureAuditLogging,
			license.FeatureRBAC,
			license.FeatureAIAutoFix,
			license.FeatureAdvancedReporting,
		},
		Limits:            map[string]int64{},
		PlanVersion:       "pro",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("SaveBillingState failed: %v", err)
	}

	h := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	runtimeReq := httptest.NewRequest(http.MethodGet, "/api/license/runtime-capabilities", nil).
		WithContext(ctx)
	runtimeRec := httptest.NewRecorder()
	h.HandleRuntimeCapabilities(runtimeRec, runtimeReq)
	if runtimeRec.Code != http.StatusOK {
		t.Fatalf("runtime status=%d, want %d: %s", runtimeRec.Code, http.StatusOK, runtimeRec.Body.String())
	}

	var runtimePayload RuntimeCapabilitiesPayload
	if err := json.Unmarshal(runtimeRec.Body.Bytes(), &runtimePayload); err != nil {
		t.Fatalf("unmarshal runtime payload failed: %v", err)
	}
	if runtimePayload.Runtime == nil || runtimePayload.Runtime.Build != pkglicensing.RuntimeBuildCommunity {
		t.Fatalf("runtime identity=%+v, want community", runtimePayload.Runtime)
	}
	for _, feature := range []string{license.FeatureAuditLogging, license.FeatureRBAC, license.FeatureAIAutoFix} {
		if containsCapability(runtimePayload.Capabilities, feature) {
			t.Fatalf("runtime capabilities retained private feature %q: %v", feature, runtimePayload.Capabilities)
		}
	}
	for _, feature := range []string{license.FeatureRelay, license.FeatureAdvancedReporting} {
		if !containsCapability(runtimePayload.Capabilities, feature) {
			t.Fatalf("runtime capabilities lost supported feature %q: %v", feature, runtimePayload.Capabilities)
		}
	}
	if len(runtimePayload.BlockedCapabilities) != 3 {
		t.Fatalf("blocked capabilities=%+v, want 3 private runtime blocks", runtimePayload.BlockedCapabilities)
	}
	for _, block := range runtimePayload.BlockedCapabilities {
		if block.Reason != "paid_runtime_required" {
			t.Fatalf("blocked reason=%q, want paid_runtime_required", block.Reason)
		}
		if block.ActionURL != pkglicensing.PulseProDownloadURL {
			t.Fatalf("blocked action_url=%q, want %q", block.ActionURL, pkglicensing.PulseProDownloadURL)
		}
	}

	entitlementReq := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).
		WithContext(ctx)
	entitlementRec := httptest.NewRecorder()
	h.HandleEntitlements(entitlementRec, entitlementReq)
	if entitlementRec.Code != http.StatusOK {
		t.Fatalf("entitlements status=%d, want %d: %s", entitlementRec.Code, http.StatusOK, entitlementRec.Body.String())
	}
	var entitlementPayload EntitlementPayload
	if err := json.Unmarshal(entitlementRec.Body.Bytes(), &entitlementPayload); err != nil {
		t.Fatalf("unmarshal entitlement payload failed: %v", err)
	}
	for _, feature := range []string{license.FeatureAuditLogging, license.FeatureRBAC, license.FeatureAIAutoFix} {
		if !containsCapability(entitlementPayload.Capabilities, feature) {
			t.Fatalf("entitlements lost licensed feature %q: %v", feature, entitlementPayload.Capabilities)
		}
	}
}

func TestHandleRuntimeCapabilities_ProRuntimePreservesPrivateProCapabilities(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		Capabilities: []string{
			license.FeatureRelay,
			license.FeatureAuditLogging,
			license.FeatureRBAC,
			license.FeatureAIAutoFix,
		},
		Limits:            map[string]int64{},
		PlanVersion:       "pro",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("SaveBillingState failed: %v", err)
	}

	h := NewLicenseHandlers(mtp, false)
	h.SetRuntimeIdentity(proRuntimeIdentityFromLicensing())

	req := httptest.NewRequest(http.MethodGet, "/api/license/runtime-capabilities", nil).
		WithContext(context.WithValue(context.Background(), OrgIDContextKey, "default"))
	rec := httptest.NewRecorder()
	h.HandleRuntimeCapabilities(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("runtime status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload RuntimeCapabilitiesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal runtime payload failed: %v", err)
	}
	if payload.Runtime == nil || payload.Runtime.Build != pkglicensing.RuntimeBuildPro {
		t.Fatalf("runtime identity=%+v, want pro", payload.Runtime)
	}
	for _, feature := range []string{license.FeatureAuditLogging, license.FeatureRBAC, license.FeatureAIAutoFix} {
		if !containsCapability(payload.Capabilities, feature) {
			t.Fatalf("pro runtime capabilities lost private feature %q: %v", feature, payload.Capabilities)
		}
	}
	if len(payload.BlockedCapabilities) != 0 {
		t.Fatalf("pro runtime blocked capabilities=%+v, want none", payload.BlockedCapabilities)
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
}

func TestEntitlementHandler_DevModeDoesNotExposeSyntheticFeatureGateCapabilities(t *testing.T) {
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

	for _, feature := range []string{license.FeatureUpdateAlerts, license.FeatureSSO, license.FeatureAIPatrol} {
		if !containsCapability(payload.Capabilities, feature) {
			t.Fatalf("expected community entitlement payload to include %q, got %v", feature, payload.Capabilities)
		}
	}
	if containsCapability(payload.Capabilities, license.FeatureAdvancedReporting) {
		t.Fatalf("dev entitlement payload exposed synthetic feature %q: %v", license.FeatureAdvancedReporting, payload.Capabilities)
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

	svc, _, err := h.getTenantComponents(ctx)
	if err != nil {
		t.Fatalf("getTenantComponents failed: %v", err)
	}
	if !svc.HasFeature(license.FeatureAdvancedReporting) {
		t.Fatalf("HasFeature(%q)=false, want true for dev-mode backend gate bypass", license.FeatureAdvancedReporting)
	}
}

func TestEntitlementHandler_DevModeRuntimeFlagDoesNotExposeMultiTenantInPayload(t *testing.T) {
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

	if containsCapability(payload.Capabilities, license.FeatureMultiTenant) {
		t.Fatalf("dev entitlement payload exposed synthetic feature %q: %v", license.FeatureMultiTenant, payload.Capabilities)
	}

	svc, _, err := h.getTenantComponents(ctx)
	if err != nil {
		t.Fatalf("getTenantComponents failed: %v", err)
	}
	if !svc.HasFeature(license.FeatureMultiTenant) {
		t.Fatalf("HasFeature(%q)=false, want true when runtime flag is enabled", license.FeatureMultiTenant)
	}
}

func TestRuntimeCapabilitiesHandler_MockModeDoesNotExposeSyntheticCapabilities(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodGet, "/api/license/runtime-capabilities", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleRuntimeCapabilities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload RuntimeCapabilitiesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	for _, feature := range []string{license.FeatureUpdateAlerts, license.FeatureSSO, license.FeatureAIPatrol} {
		if !containsCapability(payload.Capabilities, feature) {
			t.Fatalf("expected community runtime capabilities to include %q, got %v", feature, payload.Capabilities)
		}
	}
	for _, feature := range []string{license.FeatureAdvancedReporting, license.FeatureRelay, license.FeatureDemoFixtures} {
		if containsCapability(payload.Capabilities, feature) {
			t.Fatalf("mock runtime capabilities exposed synthetic feature %q: %v", feature, payload.Capabilities)
		}
	}

	svc, _, err := h.getTenantComponents(ctx)
	if err != nil {
		t.Fatalf("getTenantComponents failed: %v", err)
	}
	if !svc.HasFeature(license.FeatureDemoFixtures) {
		t.Fatalf("HasFeature(%q)=false, want true for mock fixture gate bypass", license.FeatureDemoFixtures)
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
