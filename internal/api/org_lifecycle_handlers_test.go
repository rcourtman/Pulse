package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestOrgLifecycleSuspendSuccess(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	seedOrg := &models.Organization{ID: "acme", DisplayName: "Acme"}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	req := withLifecycleUser(
		httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/suspend", strings.NewReader(`{"reason":"non-payment"}`)),
		"admin",
	)
	req.SetPathValue("id", "acme")
	rec := httptest.NewRecorder()

	h.HandleSuspendOrg(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var org models.Organization
	if err := json.NewDecoder(rec.Body).Decode(&org); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if org.Status != models.OrgStatusSuspended {
		t.Fatalf("expected status suspended, got %q", org.Status)
	}
	if org.SuspendedAt == nil {
		t.Fatalf("expected suspended_at to be set")
	}
	if org.SuspendReason != "non-payment" {
		t.Fatalf("expected suspend reason non-payment, got %q", org.SuspendReason)
	}

	persisted, err := persistence.LoadOrganization("acme")
	if err != nil {
		t.Fatalf("load persisted org: %v", err)
	}
	if persisted.Status != models.OrgStatusSuspended {
		t.Fatalf("expected persisted status suspended, got %q", persisted.Status)
	}
}

func TestOrgLifecycleUnsuspendSuccess(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	now := time.Now().UTC()
	seedOrg := &models.Organization{
		ID:            "acme",
		DisplayName:   "Acme",
		Status:        models.OrgStatusSuspended,
		SuspendedAt:   &now,
		SuspendReason: "manual",
	}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	req := withLifecycleUser(httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/unsuspend", nil), "admin")
	req.SetPathValue("id", "acme")
	rec := httptest.NewRecorder()

	h.HandleUnsuspendOrg(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var org models.Organization
	if err := json.NewDecoder(rec.Body).Decode(&org); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if org.Status != models.OrgStatusActive {
		t.Fatalf("expected status active, got %q", org.Status)
	}
	if org.SuspendedAt != nil {
		t.Fatalf("expected suspended_at to be cleared")
	}
	if org.SuspendReason != "" {
		t.Fatalf("expected suspend reason to be cleared, got %q", org.SuspendReason)
	}
}

func TestOrgLifecycleSoftDeleteSuccess(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	seedOrg := &models.Organization{ID: "acme", DisplayName: "Acme"}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	req := withLifecycleUser(httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/soft-delete", nil), "admin")
	req.SetPathValue("id", "acme")
	rec := httptest.NewRecorder()

	h.HandleSoftDeleteOrg(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var org models.Organization
	if err := json.NewDecoder(rec.Body).Decode(&org); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if org.Status != models.OrgStatusPendingDeletion {
		t.Fatalf("expected status pending_deletion, got %q", org.Status)
	}
	if org.DeletionRequestedAt == nil {
		t.Fatalf("expected deletion_requested_at to be set")
	}
	if org.RetentionDays != defaultSoftDeleteRetentionDays {
		t.Fatalf("expected retention days %d, got %d", defaultSoftDeleteRetentionDays, org.RetentionDays)
	}
}

func TestOrgLifecycleDefaultOrgGuard(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	testCases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		path    string
		body    string
	}{
		{
			name:    "suspend",
			handler: h.HandleSuspendOrg,
			path:    "/api/admin/orgs/default/suspend",
			body:    `{"reason":"x"}`,
		},
		{
			name:    "soft-delete",
			handler: h.HandleSoftDeleteOrg,
			path:    "/api/admin/orgs/default/soft-delete",
			body:    `{"retention_days":30}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := withLifecycleUser(httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)), "admin")
			req.SetPathValue("id", "default")
			rec := httptest.NewRecorder()

			tc.handler(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOrgLifecycleHostedModeGate(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, false)

	now := time.Now().UTC()
	seedOrg := &models.Organization{ID: "acme", DisplayName: "Acme", Status: models.OrgStatusSuspended, SuspendedAt: &now}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	testCases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		path    string
		body    string
	}{
		{
			name:    "suspend",
			handler: h.HandleSuspendOrg,
			path:    "/api/admin/orgs/acme/suspend",
			body:    `{"reason":"x"}`,
		},
		{
			name:    "unsuspend",
			handler: h.HandleUnsuspendOrg,
			path:    "/api/admin/orgs/acme/unsuspend",
			body:    "",
		},
		{
			name:    "soft-delete",
			handler: h.HandleSoftDeleteOrg,
			path:    "/api/admin/orgs/acme/soft-delete",
			body:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := withLifecycleUser(httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)), "admin")
			req.SetPathValue("id", "acme")
			rec := httptest.NewRecorder()

			tc.handler(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOrgLifecycleSuspendAlreadySuspendedConflict(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	now := time.Now().UTC()
	seedOrg := &models.Organization{
		ID:            "acme",
		DisplayName:   "Acme",
		Status:        models.OrgStatusSuspended,
		SuspendedAt:   &now,
		SuspendReason: "prior",
	}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	req := withLifecycleUser(
		httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/suspend", strings.NewReader(`{"reason":"again"}`)),
		"admin",
	)
	req.SetPathValue("id", "acme")
	rec := httptest.NewRecorder()

	h.HandleSuspendOrg(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}

	var apiErr APIError
	if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if apiErr.Code != "already_suspended" {
		t.Fatalf("expected error code already_suspended, got %q", apiErr.Code)
	}
}

func withLifecycleUser(req *http.Request, username string) *http.Request {
	return req.WithContext(internalauth.WithUser(req.Context(), username))
}
