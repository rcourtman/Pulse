package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHostedLifecycleSignupToEntitlement(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	orgID := "test-hosted-org"

	// Ensure the tenant directory exists (RBAC provider and other components rely on it existing).
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s) failed: %v", orgID, err)
	}

	if err := mtp.SaveOrganization(&models.Organization{
		ID:          orgID,
		Status:      models.OrgStatusActive,
		OwnerUserID: "test@example.com",
		DisplayName: "Test Org",
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveOrganization(%s) failed: %v", orgID, err)
	}

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities:  []string{license.FeatureAIPatrol, license.FeatureAIAlerts},
		Limits:        map[string]int64{},
		MetersEnabled: []string{},
		PlanVersion:   "trial",
		// Use active to represent "trial started and currently allowed" in hosted billing terms.
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("SaveBillingState(%s) failed: %v", orgID, err)
	}

	handlers := NewLicenseHandlers(mtp, true)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)

	svc, _, err := handlers.getTenantComponents(ctx)
	if err != nil {
		t.Fatalf("getTenantComponents failed: %v", err)
	}

	// Free tier includes ai_patrol; keep a basic sanity check on the resolved service.
	if !svc.HasFeature(license.FeatureAIPatrol) {
		t.Fatalf("expected service.HasFeature(%q)=true", license.FeatureAIPatrol)
	}

	// Hosted billing-derived entitlements are provided via the evaluator.
	eval := svc.Evaluator()
	if eval == nil {
		t.Fatal("expected service evaluator to be initialized in hosted mode")
	}
	if !eval.HasCapability(license.FeatureAIAlerts) {
		t.Fatalf("expected evaluator.HasCapability(%q)=true", license.FeatureAIAlerts)
	}
	if got := eval.SubscriptionState(); got != entitlements.SubStateActive {
		t.Fatalf("evaluator.SubscriptionState()=%q, want %q", got, entitlements.SubStateActive)
	}
}

func TestHostedLifecycleReaperWithCleanupHook(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	orgID := "test-org"
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s) failed: %v", orgID, err)
	}

	rbacProvider := NewTenantRBACProvider(baseDir)

	if got := rbacProvider.ManagerCount(); got != 0 {
		t.Fatalf("ManagerCount()=%d, want 0", got)
	}
	if _, err := rbacProvider.GetManager(orgID); err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgID, err)
	}
	if got := rbacProvider.ManagerCount(); got != 1 {
		t.Fatalf("ManagerCount()=%d, want 1", got)
	}

	router := &Router{
		rbacProvider: rbacProvider,
	}

	if err := router.CleanupTenant(context.Background(), orgID); err != nil {
		t.Fatalf("CleanupTenant(%s) failed: %v", orgID, err)
	}
	if got := rbacProvider.ManagerCount(); got != 0 {
		t.Fatalf("ManagerCount() after cleanup=%d, want 0", got)
	}
}

func TestHostedLifecycleTenantRateLimiting(t *testing.T) {
	trl := NewTenantRateLimiter(3, time.Minute)
	defer trl.Stop()

	h := TenantRateLimitMiddleware(trl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= 3; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, requestWithOrg("test-org"))
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d status=%d, want %d", i, rr.Code, http.StatusOK)
		}
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, requestWithOrg("test-org"))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked request status=%d, want %d", rr.Code, http.StatusTooManyRequests)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header should be set")
	}
	if rr.Header().Get("X-RateLimit-Limit") != "3" {
		t.Fatalf("X-RateLimit-Limit=%q, want %q", rr.Header().Get("X-RateLimit-Limit"), "3")
	}
	if rr.Header().Get("X-Pulse-Org-ID") != "test-org" {
		t.Fatalf("X-Pulse-Org-ID=%q, want %q", rr.Header().Get("X-Pulse-Org-ID"), "test-org")
	}
}

func TestHostedLifecycleLicenseServiceEviction(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	orgID := "test-org"
	h.services.Store(orgID, license.NewService())

	if _, ok := h.services.Load(orgID); !ok {
		t.Fatalf("expected %q to exist in license service cache", orgID)
	}

	h.RemoveTenantService(orgID)

	if _, ok := h.services.Load(orgID); ok {
		t.Fatalf("expected %q to be evicted from license service cache", orgID)
	}
}

func TestHostedLifecycleCleanupCascade(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	orgID := "test-org"
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatalf("GetPersistence(%s) failed: %v", orgID, err)
	}

	rbacProvider := NewTenantRBACProvider(baseDir)
	if _, err := rbacProvider.GetManager(orgID); err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgID, err)
	}
	if got := rbacProvider.ManagerCount(); got != 1 {
		t.Fatalf("ManagerCount()=%d, want 1", got)
	}

	licenseHandlers := NewLicenseHandlers(mtp, false)
	licenseHandlers.services.Store(orgID, license.NewService())
	if _, ok := licenseHandlers.services.Load(orgID); !ok {
		t.Fatalf("expected %q to exist in license service cache", orgID)
	}

	router := &Router{
		rbacProvider:    rbacProvider,
		licenseHandlers: licenseHandlers,
	}

	if err := router.CleanupTenant(context.Background(), orgID); err != nil {
		t.Fatalf("CleanupTenant(%s) failed: %v", orgID, err)
	}

	if got := rbacProvider.ManagerCount(); got != 0 {
		t.Fatalf("ManagerCount() after cleanup=%d, want 0", got)
	}
	if _, ok := licenseHandlers.services.Load(orgID); ok {
		t.Fatalf("expected %q to be evicted from license service cache", orgID)
	}
}
