package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

func TestBuildEntitlementPayload_ActiveLicense(t *testing.T) {
	status := &license.LicenseStatus{
		Valid:     true,
		Tier:      license.TierPro,
		Features:  append([]string(nil), license.TierFeatures[license.TierPro]...),
		MaxAgents: 50,
	}

	payload := buildEntitlementPayload(status, "")

	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("expected subscription_state %q, got %q", license.SubStateActive, payload.SubscriptionState)
	}
	if !reflect.DeepEqual(payload.Capabilities, status.Features) {
		t.Fatalf("expected capabilities to match status features")
	}

	var agentLimit *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == "max_agents" {
			agentLimit = &payload.Limits[i]
			break
		}
	}
	if agentLimit == nil {
		t.Fatalf("expected max_agents limit in payload")
	}
	if agentLimit.Limit != 50 {
		t.Fatalf("expected max_agents limit 50, got %d", agentLimit.Limit)
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

	// Upgrade reasons should cover every Pro feature not in Free.
	proMinusFree := countProMinusFreeFeatures()
	if len(payload.UpgradeReasons) != proMinusFree {
		t.Fatalf("expected %d upgrade reasons for free tier, got %d", proMinusFree, len(payload.UpgradeReasons))
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
		MaxAgents: 50,
		MaxGuests: 100,
	}

	payload := buildEntitlementPayloadWithUsage(status, "", entitlementUsageSnapshot{
		Nodes:  12,
		Guests: 44,
	}, nil)

	var agentLimit *LimitStatus
	var guestLimit *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == "max_agents" {
			agentLimit = &payload.Limits[i]
		}
		if payload.Limits[i].Key == "max_guests" {
			guestLimit = &payload.Limits[i]
		}
	}

	if agentLimit == nil {
		t.Fatalf("expected max_agents limit")
	}
	if guestLimit == nil {
		t.Fatalf("expected max_guests limit")
	}
	if agentLimit.Current != 12 {
		t.Fatalf("expected agent current 12, got %d", agentLimit.Current)
	}
	if guestLimit.Current != 44 {
		t.Fatalf("expected guest current 44, got %d", guestLimit.Current)
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

func TestEntitlementHandler_UsesEvaluatorWhenNoLicense(t *testing.T) {
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
			"max_agents": 5,
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

	if payload.SubscriptionState != string(license.SubStateActive) {
		t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateActive)
	}
	if payload.PlanVersion != "pro" {
		t.Fatalf("plan_version=%q, want %q", payload.PlanVersion, "pro")
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

	var maxAgents *LimitStatus
	for i := range payload.Limits {
		if payload.Limits[i].Key == "max_agents" {
			maxAgents = &payload.Limits[i]
			break
		}
	}
	if maxAgents == nil {
		t.Fatalf("expected max_agents limit in payload, got %v", payload.Limits)
	}
	if maxAgents.Limit != 5 {
		t.Fatalf("max_agents.limit=%d, want %d", maxAgents.Limit, 5)
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

func TestEntitlementHandler_TrialEligibility_FreshOrgAllowed(t *testing.T) {
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
	if !payload.TrialEligible {
		t.Fatalf("trial_eligible=%v, want true", payload.TrialEligible)
	}
	if payload.TrialEligibilityReason != "" {
		t.Fatalf("trial_eligibility_reason=%q, want empty", payload.TrialEligibilityReason)
	}
}

func TestEntitlementHandler_TrialEligibility_AlreadyUsedDenied(t *testing.T) {
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
	if payload.TrialEligibilityReason != "already_used" {
		t.Fatalf("trial_eligibility_reason=%q, want %q", payload.TrialEligibilityReason, "already_used")
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
