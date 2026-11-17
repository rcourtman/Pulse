package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
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

type lifecycleJSONFixture struct {
	handlers    *OrgLifecycleHandlers
	persistence *config.MultiTenantPersistence
	before      map[string][]byte
}

func newLifecycleJSONFixture(t *testing.T) lifecycleJSONFixture {
	t.Helper()
	dir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(dir)
	before := make(map[string][]byte)
	for _, id := range []string{"selected", "peer"} {
		require.NoError(t, persistence.SaveOrganization(&models.Organization{
			ID: id, DisplayName: "Saved " + id, Status: models.OrgStatusActive,
			OwnerUserID: id + "-owner", RetentionDays: 90,
		}))
		path := filepath.Join(dir, "orgs", id, "org.json")
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		before[path] = raw
	}
	return lifecycleJSONFixture{NewOrgLifecycleHandlers(persistence, true), persistence, before}
}

func (f lifecycleJSONFixture) assertSaved(t *testing.T, includeSelected bool) {
	t.Helper()
	for path, want := range f.before {
		if !includeSelected && filepath.Base(filepath.Dir(path)) == "selected" {
			continue
		}
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, string(want), string(got), "saved organization changed: %s", path)
	}
}

func (f lifecycleJSONFixture) serve(t *testing.T, operation string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	r := withLifecycleUser(httptest.NewRequest(http.MethodPost, "/api/admin/orgs/selected/"+operation, body), "fixture-admin")
	r.SetPathValue("id", "selected")
	w := httptest.NewRecorder()
	if operation == "suspend" {
		f.handlers.HandleSuspendOrg(w, r)
	} else {
		f.handlers.HandleSoftDeleteOrg(w, r)
	}
	return w
}

func TestOrgLifecycleJSONRejectsBeforeSaving(t *testing.T) {
	for _, operation := range []string{"suspend", "soft-delete"} {
		t.Run(operation, func(t *testing.T) {
			field, first, second := "reason", `"private-first"`, `"private-second"`
			if operation == "soft-delete" {
				field, first, second = "retention_days", "90", "1"
			}
			valid := `{"` + field + `":` + first + `}`
			for _, tc := range []struct{ name, body string }{
				{"second-object", valid + `{}`},
				{"second-scalar", valid + ` true`},
				{"trailing-junk", valid + `x`},
				{"truncated-suffix", valid + ` {`},
				{"oversized-suffix", valid + strings.Repeat(" ", orgRequestBodyLimit)},
				{"oversized-whitespace", strings.Repeat(" ", orgRequestBodyLimit+1)},
				{"oversized-field", `{"future":"` + strings.Repeat("x", orgRequestBodyLimit) + `"}`},
				{"null", `null`},
				{"array", `[]`},
				{"scalar", `true`},
				{"duplicate", `{"` + field + `":` + first + `,"` + field + `":` + second + `}`},
				{"identical-duplicate", `{"` + field + `":` + first + `,"` + field + `":` + first + `}`},
				{"folded-duplicate", `{"` + field + `":` + first + `,"` + strings.ToUpper(field) + `":` + second + `}`},
				{"escaped-duplicate", `{"` + field + `":` + first + `,"\u0072` + field[1:] + `":` + second + `}`},
				{"wrong-type", `{"` + field + `":[]}`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					f := newLifecycleJSONFixture(t)
					w := f.serve(t, operation, strings.NewReader(tc.body))
					f.assertSaved(t, true)
					require.Equal(t, http.StatusBadRequest, w.Code, "body = %s", w.Body.String())
					var result APIError
					require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
					require.Equal(t, "invalid_request", result.Code)
					require.NotContains(t, w.Body.String(), "private-")
				})
			}
		})
	}
}

// A reader can return a complete object and fail on a later read. No prefix,
// including empty whitespace or an error wrapping EOF, authorizes defaults.
type lifecycleJSONReadFailure struct {
	*strings.Reader
	err error
}

func (r lifecycleJSONReadFailure) Read(p []byte) (int, error) {
	if r.Reader.Len() > 0 {
		return r.Reader.Read(p)
	}
	return 0, r.err
}

func TestOrgLifecycleJSONReadFailureKeepsSavedState(t *testing.T) {
	for _, operation := range []string{"suspend", "soft-delete"} {
		for _, prefix := range []string{"", " \n", "{}"} {
			for _, failure := range []error{io.ErrUnexpectedEOF, errors.New("private-read-error"), errors.Join(errors.New("failed read"), io.EOF)} {
				t.Run(operation+"/"+prefix+"/"+failure.Error(), func(t *testing.T) {
					f := newLifecycleJSONFixture(t)
					w := f.serve(t, operation, lifecycleJSONReadFailure{strings.NewReader(prefix), failure})
					f.assertSaved(t, true)
					require.Equal(t, http.StatusBadRequest, w.Code)
					require.Contains(t, w.Body.String(), `"invalid_request"`)
					require.NotContains(t, w.Body.String(), "private-read-error")
				})
			}
		}
	}
}

func TestOrgLifecycleJSONPreservesSupportedOptions(t *testing.T) {
	for _, operation := range []string{"suspend", "soft-delete"} {
		t.Run(operation, func(t *testing.T) {
			t.Run("nil-body", func(t *testing.T) {
				f := newLifecycleJSONFixture(t)
				require.Equal(t, http.StatusOK, f.serve(t, operation, nil).Code)
				f.assertSaved(t, false)
			})
			for _, tc := range []struct {
				name, body, reason string
				days               int
			}{
				{"empty", "", "", 30},
				{"whitespace", " \r\n\t", "", 30},
				{"object", "{}", "", 30},
				{"unknown-fields", `{"future":{"reason":"a","reason":"b"},"other":true}`, "", 30},
				{"null-options", `{"reason":null,"retention_days":null}`, "", 30},
				{"singleton-aliases", `{"REASON":"  planned hold  ","RETENTION_DAYS":90}`, "planned hold", 90},
				{"escaped-singletons", `{"\u0072eason":"hold","\u0072etention_days":1}` + " \n", "hold", 1},
				{"exact-byte-limit", `{}` + strings.Repeat(" ", orgRequestBodyLimit-2), "", 30},
			} {
				t.Run(tc.name, func(t *testing.T) {
					f := newLifecycleJSONFixture(t)
					w := f.serve(t, operation, strings.NewReader(tc.body))
					require.Equal(t, http.StatusOK, w.Code, "%s", w.Body.String())
					var response models.Organization
					require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
					persisted, err := f.persistence.LoadOrganization("selected")
					require.NoError(t, err)
					persisted.PersistenceVersion = "" // Internal save guard is not a response field.
					require.Equal(t, response, *persisted)
					require.Equal(t, "selected-owner", persisted.OwnerUserID)
					if operation == "suspend" {
						require.Equal(t, models.OrgStatusSuspended, persisted.Status)
						require.Equal(t, tc.reason, persisted.SuspendReason)
						require.NotNil(t, persisted.SuspendedAt)
						require.Equal(t, 90, persisted.RetentionDays)
					} else {
						require.Equal(t, models.OrgStatusPendingDeletion, persisted.Status)
						require.Equal(t, tc.days, persisted.RetentionDays)
						require.NotNil(t, persisted.DeletionRequestedAt)
					}
					f.assertSaved(t, false)
				})
			}
		})
	}
}

func TestOrgLifecycleJSONNativeHTTP(t *testing.T) {
	f := newLifecycleJSONFixture(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/orgs/{id}/suspend", f.handlers.HandleSuspendOrg)
	mux.HandleFunc("POST /api/admin/orgs/{id}/soft-delete", f.handlers.HandleSoftDeleteOrg)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 5 * time.Second
	for _, operation := range []string{"suspend", "soft-delete"} {
		response, err := client.Post(server.URL+"/api/admin/orgs/selected/"+operation, "application/json", strings.NewReader(`{} {}`))
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, err)
		f.assertSaved(t, true)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	}
	// Rejection leaves the same saved org usable for a deliberate lifecycle
	// change, not a new org or a replay of the malformed request.
	for _, tc := range []struct{ operation, body string }{
		{"suspend", `{"reason":"deliberate"}`},
		{"soft-delete", `{"retention_days":90}`},
	} {
		response, err := client.Post(server.URL+"/api/admin/orgs/selected/"+tc.operation, "application/json", strings.NewReader(tc.body))
		require.NoError(t, err)
		var saved models.Organization
		err = json.NewDecoder(response.Body).Decode(&saved)
		require.NoError(t, response.Body.Close())
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		if tc.operation == "suspend" {
			require.Equal(t, models.OrgStatusSuspended, saved.Status)
			require.Equal(t, "deliberate", saved.SuspendReason)
		} else {
			require.Equal(t, models.OrgStatusPendingDeletion, saved.Status)
			require.Equal(t, 90, saved.RetentionDays)
		}
		persisted, err := f.persistence.LoadOrganization("selected")
		require.NoError(t, err)
		persisted.PersistenceVersion = ""
		require.Equal(t, saved, *persisted)
		f.assertSaved(t, false)
	}
}

type lifecycleJSONUnreadBody struct{ reads int }

func (b *lifecycleJSONUnreadBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("unexpected body read")
}

func TestOrgLifecycleJSONKeepsAdmissionGates(t *testing.T) {
	for _, operation := range []string{"suspend", "soft-delete"} {
		for _, tc := range []struct {
			name, method, org string
			hosted, store     bool
			status            int
		}{
			{"method", http.MethodGet, "selected", true, true, http.StatusMethodNotAllowed},
			{"not-hosted", http.MethodPost, "selected", false, true, http.StatusNotFound},
			{"no-store", http.MethodPost, "selected", true, false, http.StatusServiceUnavailable},
			{"default", http.MethodPost, "default", true, true, http.StatusBadRequest},
			{"missing", http.MethodPost, "absent", true, true, http.StatusNotFound},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				f := newLifecycleJSONFixture(t)
				f.handlers.hostedMode = tc.hosted
				if !tc.store {
					f.handlers.persistence = nil
				}
				body := &lifecycleJSONUnreadBody{}
				r := httptest.NewRequest(tc.method, "/api/admin/orgs/"+tc.org+"/"+operation, body)
				r.SetPathValue("id", tc.org)
				w := httptest.NewRecorder()
				if operation == "suspend" {
					f.handlers.HandleSuspendOrg(w, r)
				} else {
					f.handlers.HandleSoftDeleteOrg(w, r)
				}
				require.Equal(t, tc.status, w.Code)
				require.Zero(t, body.reads)
				f.assertSaved(t, true)
			})
		}
	}
}
