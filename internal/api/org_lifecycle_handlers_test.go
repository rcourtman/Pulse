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

func TestOrgLifecycleSuspendUnsuspendRoundTrip(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	seedOrg := &models.Organization{
		ID:          "acme",
		DisplayName: "Acme",
		Status:      models.OrgStatusActive,
	}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	suspendReq := withLifecycleUser(
		httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/suspend", strings.NewReader(`{"reason":"billing_hold"}`)),
		"admin",
	)
	suspendReq.SetPathValue("id", "acme")
	suspendRec := httptest.NewRecorder()

	h.HandleSuspendOrg(suspendRec, suspendReq)
	if suspendRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", suspendRec.Code, suspendRec.Body.String())
	}

	var suspendedOrg models.Organization
	if err := json.NewDecoder(suspendRec.Body).Decode(&suspendedOrg); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if suspendedOrg.Status != models.OrgStatusSuspended {
		t.Fatalf("expected status suspended, got %q", suspendedOrg.Status)
	}
	if suspendedOrg.SuspendedAt == nil {
		t.Fatalf("expected suspended_at to be set")
	}
	if suspendedOrg.SuspendReason != "billing_hold" {
		t.Fatalf("expected suspend reason billing_hold, got %q", suspendedOrg.SuspendReason)
	}

	unsuspendReq := withLifecycleUser(httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/unsuspend", nil), "admin")
	unsuspendReq.SetPathValue("id", "acme")
	unsuspendRec := httptest.NewRecorder()

	h.HandleUnsuspendOrg(unsuspendRec, unsuspendReq)
	if unsuspendRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", unsuspendRec.Code, unsuspendRec.Body.String())
	}

	var unsuspendedOrg models.Organization
	if err := json.NewDecoder(unsuspendRec.Body).Decode(&unsuspendedOrg); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if unsuspendedOrg.Status != models.OrgStatusActive {
		t.Fatalf("expected status active, got %q", unsuspendedOrg.Status)
	}
	if unsuspendedOrg.SuspendedAt != nil {
		t.Fatalf("expected suspended_at to be cleared")
	}
	if unsuspendedOrg.SuspendReason != "" {
		t.Fatalf("expected suspend reason to be cleared, got %q", unsuspendedOrg.SuspendReason)
	}

	persisted, err := persistence.LoadOrganization("acme")
	if err != nil {
		t.Fatalf("load persisted org: %v", err)
	}
	if persisted.Status != models.OrgStatusActive {
		t.Fatalf("expected persisted status active, got %q", persisted.Status)
	}
	if persisted.SuspendedAt != nil {
		t.Fatalf("expected persisted suspended_at to be cleared")
	}
	if persisted.SuspendReason != "" {
		t.Fatalf("expected persisted suspend reason to be cleared, got %q", persisted.SuspendReason)
	}
}

func TestOrgLifecycleUnsuspendNotSuspendedConflict(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	testCases := []struct {
		name   string
		orgID  string
		status models.OrgStatus
	}{
		{
			name:   "active_status",
			orgID:  "acme-active",
			status: models.OrgStatusActive,
		},
		{
			name:   "empty_status_normalizes_to_active",
			orgID:  "acme-empty",
			status: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seedOrg := &models.Organization{
				ID:          tc.orgID,
				DisplayName: "Acme",
				Status:      tc.status,
			}
			if err := persistence.SaveOrganization(seedOrg); err != nil {
				t.Fatalf("seed org: %v", err)
			}

			req := withLifecycleUser(httptest.NewRequest(http.MethodPost, "/api/admin/orgs/"+tc.orgID+"/unsuspend", nil), "admin")
			req.SetPathValue("id", tc.orgID)
			rec := httptest.NewRecorder()

			h.HandleUnsuspendOrg(rec, req)
			if rec.Code != http.StatusConflict {
				t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
			}

			var apiErr APIError
			if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if apiErr.Code != "not_suspended" {
				t.Fatalf("expected error code not_suspended, got %q", apiErr.Code)
			}
		})
	}
}

func TestOrgLifecycleSoftDeleteAlreadyPendingDeletionConflict(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	now := time.Now().UTC()
	seedOrg := &models.Organization{
		ID:                  "acme",
		DisplayName:         "Acme",
		Status:              models.OrgStatusPendingDeletion,
		DeletionRequestedAt: &now,
		RetentionDays:       defaultSoftDeleteRetentionDays,
	}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	req := withLifecycleUser(httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/soft-delete", nil), "admin")
	req.SetPathValue("id", "acme")
	rec := httptest.NewRecorder()

	h.HandleSoftDeleteOrg(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}

	var apiErr APIError
	if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if apiErr.Code != "already_pending_deletion" {
		t.Fatalf("expected error code already_pending_deletion, got %q", apiErr.Code)
	}
}

func TestOrgLifecycleSuspendThenSoftDelete(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	seedOrg := &models.Organization{
		ID:          "acme",
		DisplayName: "Acme",
		Status:      models.OrgStatusActive,
	}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	suspendReq := withLifecycleUser(
		httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/suspend", strings.NewReader(`{"reason":"fraud_review"}`)),
		"admin",
	)
	suspendReq.SetPathValue("id", "acme")
	suspendRec := httptest.NewRecorder()

	h.HandleSuspendOrg(suspendRec, suspendReq)
	if suspendRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", suspendRec.Code, suspendRec.Body.String())
	}

	softDeleteReq := withLifecycleUser(httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/soft-delete", nil), "admin")
	softDeleteReq.SetPathValue("id", "acme")
	softDeleteRec := httptest.NewRecorder()

	h.HandleSoftDeleteOrg(softDeleteRec, softDeleteReq)
	if softDeleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", softDeleteRec.Code, softDeleteRec.Body.String())
	}

	var org models.Organization
	if err := json.NewDecoder(softDeleteRec.Body).Decode(&org); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if org.Status != models.OrgStatusPendingDeletion {
		t.Fatalf("expected status pending_deletion, got %q", org.Status)
	}

	persisted, err := persistence.LoadOrganization("acme")
	if err != nil {
		t.Fatalf("load persisted org: %v", err)
	}
	if persisted.Status != models.OrgStatusPendingDeletion {
		t.Fatalf("expected persisted status pending_deletion, got %q", persisted.Status)
	}
}

func TestOrgLifecycleNonExistentOrg(t *testing.T) {
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
			path:    "/api/admin/orgs/nonexistent/suspend",
			body:    `{"reason":"x"}`,
		},
		{
			name:    "unsuspend",
			handler: h.HandleUnsuspendOrg,
			path:    "/api/admin/orgs/nonexistent/unsuspend",
			body:    "",
		},
		{
			name:    "soft-delete",
			handler: h.HandleSoftDeleteOrg,
			path:    "/api/admin/orgs/nonexistent/soft-delete",
			body:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := withLifecycleUser(httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)), "admin")
			req.SetPathValue("id", "nonexistent")
			rec := httptest.NewRecorder()

			tc.handler(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOrgLifecycleSoftDeleteCustomRetentionDays(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	seedOrg := &models.Organization{
		ID:          "acme",
		DisplayName: "Acme",
		Status:      models.OrgStatusActive,
	}
	if err := persistence.SaveOrganization(seedOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	req := withLifecycleUser(
		httptest.NewRequest(http.MethodPost, "/api/admin/orgs/acme/soft-delete", strings.NewReader(`{"retention_days":90}`)),
		"admin",
	)
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
	if org.RetentionDays != 90 {
		t.Fatalf("expected retention days 90, got %d", org.RetentionDays)
	}
}

func TestOrgLifecycleSoftDeleteInvalidRetentionDays(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgLifecycleHandlers(persistence, true)

	testCases := []struct {
		name  string
		orgID string
		body  string
	}{
		{
			name:  "zero_retention_days",
			orgID: "acme-zero",
			body:  `{"retention_days":0}`,
		},
		{
			name:  "negative_retention_days",
			orgID: "acme-negative",
			body:  `{"retention_days":-5}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seedOrg := &models.Organization{
				ID:          tc.orgID,
				DisplayName: "Acme",
				Status:      models.OrgStatusActive,
			}
			if err := persistence.SaveOrganization(seedOrg); err != nil {
				t.Fatalf("seed org: %v", err)
			}

			req := withLifecycleUser(
				httptest.NewRequest(http.MethodPost, "/api/admin/orgs/"+tc.orgID+"/soft-delete", strings.NewReader(tc.body)),
				"admin",
			)
			req.SetPathValue("id", tc.orgID)
			rec := httptest.NewRecorder()

			h.HandleSoftDeleteOrg(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}

			var apiErr APIError
			if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if apiErr.Code != "invalid_retention_days" {
				t.Fatalf("expected error code invalid_retention_days, got %q", apiErr.Code)
			}
		})
	}
}

func withLifecycleUser(req *http.Request, username string) *http.Request {
	return req.WithContext(internalauth.WithUser(req.Context(), username))
}
