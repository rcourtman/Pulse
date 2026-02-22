package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func setMaxUsersLimitForTests(t *testing.T, limits map[string]int64) {
	t.Helper()

	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	service := pkglicensing.NewService()
	licenseKey, err := pkglicensing.GenerateLicenseForTesting("users-limit@example.com", pkglicensing.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate test license: %v", err)
	}

	lic, err := service.Activate(licenseKey)
	if err != nil {
		t.Fatalf("failed to activate test license: %v", err)
	}
	lic.Claims.Limits = limits

	SetLicenseServiceProvider(&staticLicenseProvider{service: service})
	t.Cleanup(func() {
		SetLicenseServiceProvider(nil)
	})
}

func organizationWithMemberCount(count int) *models.Organization {
	members := make([]models.OrganizationMember, count)
	for i := 0; i < count; i++ {
		members[i] = models.OrganizationMember{
			UserID: fmt.Sprintf("user-%d", i+1),
			Role:   models.OrgRoleViewer,
		}
	}

	return &models.Organization{
		ID:      "test-org",
		Members: members,
	}
}

func TestMaxUsersLimitForContext_NoService(t *testing.T) {
	SetLicenseServiceProvider(nil)
	t.Cleanup(func() {
		SetLicenseServiceProvider(nil)
	})

	limit := maxUsersLimitForContext(context.Background())
	if limit != 0 {
		t.Fatalf("expected limit 0, got %d", limit)
	}
}

func TestMaxUsersLimitForContext_NoLicense(t *testing.T) {
	service := pkglicensing.NewService()
	SetLicenseServiceProvider(&staticLicenseProvider{service: service})
	t.Cleanup(func() {
		SetLicenseServiceProvider(nil)
	})

	limit := maxUsersLimitForContext(context.Background())
	if limit != 0 {
		t.Fatalf("expected limit 0, got %d", limit)
	}
}

func TestMaxUsersLimitForContext_NoLimit(t *testing.T) {
	setMaxUsersLimitForTests(t, map[string]int64{
		maxNodesLicenseGateKey: 10,
	})

	limit := maxUsersLimitForContext(context.Background())
	if limit != 0 {
		t.Fatalf("expected limit 0, got %d", limit)
	}
}

func TestMaxUsersLimitForContext_WithLimit(t *testing.T) {
	setMaxUsersLimitForTests(t, map[string]int64{
		maxUsersLicenseGateKey: 5,
	})

	limit := maxUsersLimitForContext(context.Background())
	if limit != 5 {
		t.Fatalf("expected limit 5, got %d", limit)
	}
}

func TestCurrentUserCount_Nil(t *testing.T) {
	count := currentUserCount(nil)
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}
}

func TestCurrentUserCount_WithMembers(t *testing.T) {
	org := organizationWithMemberCount(3)

	count := currentUserCount(org)
	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}
}

func TestEnforceUserLimitForMemberAdd_Unlimited(t *testing.T) {
	SetLicenseServiceProvider(nil)
	t.Cleanup(func() {
		SetLicenseServiceProvider(nil)
	})

	rec := httptest.NewRecorder()
	blocked := enforceUserLimitForMemberAdd(rec, context.Background(), organizationWithMemberCount(10))

	if blocked {
		t.Fatalf("expected request to be allowed")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestEnforceUserLimitForMemberAdd_UnderLimit(t *testing.T) {
	setMaxUsersLimitForTests(t, map[string]int64{
		maxUsersLicenseGateKey: 5,
	})

	rec := httptest.NewRecorder()
	blocked := enforceUserLimitForMemberAdd(rec, context.Background(), organizationWithMemberCount(2))

	if blocked {
		t.Fatalf("expected request to be allowed")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestEnforceUserLimitForMemberAdd_AtLimit(t *testing.T) {
	setMaxUsersLimitForTests(t, map[string]int64{
		maxUsersLicenseGateKey: 5,
	})

	rec := httptest.NewRecorder()
	blocked := enforceUserLimitForMemberAdd(rec, context.Background(), organizationWithMemberCount(5))

	if !blocked {
		t.Fatalf("expected request to be blocked")
	}
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if payload["error"] != "license_required" {
		t.Fatalf("expected error license_required, got %v", payload["error"])
	}
	if payload["feature"] != maxUsersLicenseGateKey {
		t.Fatalf("expected feature %q, got %v", maxUsersLicenseGateKey, payload["feature"])
	}

	message, _ := payload["message"].(string)
	if !strings.Contains(message, "User limit reached (5/5)") {
		t.Fatalf("expected message to include limit details, got %q", message)
	}
}

func TestEnforceUserLimitForMemberAdd_OverLimit(t *testing.T) {
	setMaxUsersLimitForTests(t, map[string]int64{
		maxUsersLicenseGateKey: 5,
	})

	rec := httptest.NewRecorder()
	blocked := enforceUserLimitForMemberAdd(rec, context.Background(), organizationWithMemberCount(6))

	if !blocked {
		t.Fatalf("expected request to be blocked")
	}
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if payload["error"] != "license_required" {
		t.Fatalf("expected error license_required, got %v", payload["error"])
	}
	if payload["feature"] != maxUsersLicenseGateKey {
		t.Fatalf("expected feature %q, got %v", maxUsersLicenseGateKey, payload["feature"])
	}

	message, _ := payload["message"].(string)
	if !strings.Contains(message, "User limit reached (6/5)") {
		t.Fatalf("expected message to include limit details, got %q", message)
	}
}
