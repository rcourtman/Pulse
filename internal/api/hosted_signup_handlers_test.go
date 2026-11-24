package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestHostedSignupSuccess(t *testing.T) {
	router, persistence, rbacProvider, emailer, _ := newHostedSignupTestRouter(t, true)

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var response hostedSignupResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.OrgID != "" {
		t.Fatalf("expected org_id to be omitted, got %q", response.OrgID)
	}
	if response.UserID != "" {
		t.Fatalf("expected user_id to be omitted, got %q", response.UserID)
	}
	if response.Message != hostedSignupAcceptedMessage {
		t.Fatalf("unexpected message: %q", response.Message)
	}

	orgID := requireHostedSignupProvisionedOrgID(t, persistence, "owner@example.com")
	org, err := persistence.LoadOrganization(orgID)
	if err != nil {
		t.Fatalf("load org: %v", err)
	}
	if org.DisplayName != "My Organization" {
		t.Fatalf("expected org display name My Organization, got %q", org.DisplayName)
	}
	if org.OwnerUserID == "" || strings.Contains(org.OwnerUserID, "@") {
		t.Fatalf("expected stable owner user id, got %q", org.OwnerUserID)
	}
	if org.OwnerEmail != "owner@example.com" {
		t.Fatalf("expected owner email owner@example.com, got %q", org.OwnerEmail)
	}

	manager, err := rbacProvider.GetManager(orgID)
	if err != nil {
		t.Fatalf("get rbac manager: %v", err)
	}
	assignment, ok := manager.GetUserAssignment(org.OwnerUserID)
	if !ok {
		t.Fatal("expected user role assignment to exist")
	}
	if !containsRoleID(assignment.RoleIDs, auth.RoleAdmin) {
		t.Fatalf("expected admin role assignment, got roles: %v", assignment.RoleIDs)
	}

	// Verify a magic link was "sent".
	emailer.mu.Lock()
	defer emailer.mu.Unlock()
	if emailer.calls != 1 {
		t.Fatalf("expected 1 magic link email, got %d", emailer.calls)
	}
	if emailer.to != "owner@example.com" {
		t.Fatalf("magic link to = %q, want owner@example.com", emailer.to)
	}
	if !strings.Contains(emailer.magicLinkURL, "/api/public/magic-link/verify?token=") {
		t.Fatalf("magic link url = %q, expected verify endpoint with token", emailer.magicLinkURL)
	}
}

func TestHostedSignupValidationFailures(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{
			name: "missing email",
			body: `{"org_name":"My Organization"}`,
		},
		{
			name: "invalid email",
			body: `{"email":"userexample.com","org_name":"My Organization"}`,
		},
		{
			name: "invalid org_name",
			body: `{"email":"user@example.com","org_name":"../evil"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, _, _, _, _ := newHostedSignupTestRouter(t, true)
			rec := doHostedSignupRequest(router, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestIsValidHostedOrgName(t *testing.T) {
	testCases := []struct {
		name    string
		orgName string
		want    bool
	}{
		{name: "simple", orgName: "Acme", want: true},
		{name: "spaces allowed", orgName: "My Organization", want: true},
		{name: "punctuation allowed", orgName: "Acme, Inc.", want: true},
		{name: "empty", orgName: "", want: false},
		{name: "too short", orgName: "ab", want: false},
		{name: "too long", orgName: strings.Repeat("a", 65), want: false},
		{name: "path traversal", orgName: "../evil", want: false},
		{name: "slash", orgName: "acme/ops", want: false},
		{name: "backslash", orgName: "acme\\ops", want: false},
		{name: "control char", orgName: "acme\nops", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidHostedOrgName(tc.orgName)
			if got != tc.want {
				t.Fatalf("isValidHostedOrgName(%q) = %v, want %v", tc.orgName, got, tc.want)
			}
		})
	}
}

func TestHostedSignupHostedModeGate(t *testing.T) {
	router, _, _, _, _ := newHostedSignupTestRouter(t, false)

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when hosted mode is disabled, got %d: %s", rec.Code, rec.Body.String())
	}
}

// A runtime the control plane provisioned for one workspace is reachable at
// that workspace's public address, and its organization, owner and entitlement
// come from the control plane. Its signup route must stay closed to everyone.
func TestHostedSignupClosedOnProvisionedWorkspaceRuntime(t *testing.T) {
	t.Setenv("PULSE_TENANT_ID", "t-workspace")
	router, persistence, _, emailer, baseDir := newHostedSignupTestRouter(t, true)

	addedAt := time.Unix(1700000000, 0).UTC()
	if err := persistence.SaveOrganization(&models.Organization{
		ID:          "t-workspace",
		DisplayName: "Workspace",
		CreatedAt:   addedAt,
		OwnerUserID: "u_workspace_owner",
		OwnerEmail:  "alice.smith@example.com",
		Members: []models.OrganizationMember{
			{UserID: "u_workspace_owner", Email: "alice.smith@example.com", Role: models.OrgRoleOwner, AddedAt: addedAt, AddedBy: "u_workspace_owner"},
		},
	}); err != nil {
		t.Fatalf("save workspace org: %v", err)
	}

	for _, email := range []string{"alice.smith@example.com", "alice.\u017fmith@example.com", "stranger@example.com"} {
		rec := doHostedSignupRequest(router, fmt.Sprintf(`{"email":%q,"org_name":"anything"}`, email))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("signup for %q on a provisioned runtime: expected 404, got %d: %s", email, rec.Code, rec.Body.String())
		}
	}

	org, err := persistence.LoadOrganization("t-workspace")
	if err != nil {
		t.Fatalf("load workspace org: %v", err)
	}
	if org.OwnerUserID != "u_workspace_owner" || org.OwnerEmail != "alice.smith@example.com" {
		t.Fatalf("workspace owner = (%q, %q), want it unchanged", org.OwnerUserID, org.OwnerEmail)
	}
	if len(org.Members) != 1 || org.Members[0].UserID != "u_workspace_owner" {
		t.Fatalf("workspace members = %+v, want the owner unchanged", org.Members)
	}
	state, err := config.NewFileBillingStore(baseDir).GetBillingState("t-workspace")
	if err != nil {
		t.Fatalf("get workspace billing state: %v", err)
	}
	if state != nil {
		t.Fatalf("expected no billing state beside the control plane's, got %+v", state)
	}
	orgs, err := persistence.ListOrganizations()
	if err != nil {
		t.Fatalf("list orgs: %v", err)
	}
	for _, listed := range orgs {
		if listed != nil && listed.ID != "default" && listed.ID != "t-workspace" {
			t.Fatalf("signup created organization %q inside a provisioned runtime", listed.ID)
		}
	}

	emailer.mu.Lock()
	defer emailer.mu.Unlock()
	if emailer.calls != 0 {
		t.Fatalf("expected no magic link email, got %d", emailer.calls)
	}
}

// Signing up again with an owner's address resolves to the organization that
// owner already has. The request must not put it back on a trial.
func TestHostedSignupExistingOwnerKeepsBillingState(t *testing.T) {
	router, persistence, _, emailer, baseDir := newHostedSignupTestRouter(t, true)

	first := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first signup expected 202, got %d: %s", first.Code, first.Body.String())
	}
	orgID := requireHostedSignupProvisionedOrgID(t, persistence, "owner@example.com")

	store := config.NewFileBillingStore(baseDir)
	seeded, err := store.GetBillingState(orgID)
	if err != nil {
		t.Fatalf("get seeded billing state: %v", err)
	}
	if seeded == nil || seeded.SubscriptionState != entitlements.SubStateTrial || seeded.PlanVersion != "cloud_trial" {
		t.Fatalf("expected a seeded cloud trial for the new organization, got %+v", seeded)
	}
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities:      seeded.Capabilities,
		Limits:            map[string]int64{"max_monitored_systems": 10},
		PlanVersion:       "cloud_starter",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("save active billing state: %v", err)
	}

	second := doHostedSignupRequest(router, `{"email":"Owner@Example.com","org_name":"Another Name"}`)
	if second.Code != http.StatusAccepted {
		t.Fatalf("second signup expected 202, got %d: %s", second.Code, second.Body.String())
	}

	state, err := store.GetBillingState(orgID)
	if err != nil {
		t.Fatalf("get billing state after second signup: %v", err)
	}
	if state == nil || state.SubscriptionState != entitlements.SubStateActive || state.PlanVersion != "cloud_starter" {
		t.Fatalf("expected the active subscription to survive a repeated signup, got %+v", state)
	}
	if state.TrialStartedAt != nil || state.TrialEndsAt != nil {
		t.Fatalf("expected no trial window on the active subscription, got started=%v ends=%v", state.TrialStartedAt, state.TrialEndsAt)
	}

	emailer.mu.Lock()
	defer emailer.mu.Unlock()
	if emailer.calls != 2 {
		t.Fatalf("expected a sign-in link for each accepted request, got %d", emailer.calls)
	}
}

// Rollback undoes what a failed signup created. An organization that was
// already there is not that request's to delete.
func TestHostedSignupExistingOrganizationSurvivesMagicLinkFailure(t *testing.T) {
	router, persistence, rbacProvider, emailer, _ := newHostedSignupTestRouter(t, true)

	first := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first signup expected 202, got %d: %s", first.Code, first.Body.String())
	}
	orgID := requireHostedSignupProvisionedOrgID(t, persistence, "owner@example.com")
	before, err := persistence.LoadOrganization(orgID)
	if err != nil {
		t.Fatalf("load org: %v", err)
	}

	emailer.mu.Lock()
	emailer.err = fmt.Errorf("forced SendMagicLink failure")
	emailer.mu.Unlock()

	second := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if second.Code != http.StatusInternalServerError {
		t.Fatalf("second signup expected 500, got %d: %s", second.Code, second.Body.String())
	}

	after, err := persistence.LoadOrganization(orgID)
	if err != nil {
		t.Fatalf("expected the existing organization to survive a failed repeat signup: %v", err)
	}
	if after.OwnerUserID != before.OwnerUserID || after.OwnerEmail != before.OwnerEmail || len(after.Members) != len(before.Members) {
		t.Fatalf("existing organization changed: before=%+v after=%+v", before, after)
	}
	manager, err := rbacProvider.GetManager(orgID)
	if err != nil {
		t.Fatalf("get rbac manager: %v", err)
	}
	assignment, ok := manager.GetUserAssignment(before.OwnerUserID)
	if !ok || !containsRoleID(assignment.RoleIDs, auth.RoleAdmin) {
		t.Fatalf("expected the owner to keep the admin role, got %+v (found=%v)", assignment, ok)
	}
}

func TestHostedSignupRateLimit(t *testing.T) {
	router, _, _, _, _ := newHostedSignupTestRouter(t, true)

	for i := 1; i <= 6; i++ {
		body := fmt.Sprintf(
			`{"email":"user%d@example.com","org_name":"Org %d"}`,
			i,
			i,
		)
		rec := doHostedSignupRequest(router, body)
		if i <= 5 && rec.Code != http.StatusAccepted {
			t.Fatalf("request %d expected 202, got %d: %s", i, rec.Code, rec.Body.String())
		}
		if i == 6 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d expected 429, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}
}

func TestHostedSignupRateLimit_NoProvisioningSideEffects(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	rbacProvider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		_ = rbacProvider.Close()
	})

	router, emailer := newHostedSignupTestRouterWithDepsAndPublicURLAndMagicLinkLimit(
		t,
		true,
		persistence,
		rbacProvider,
		"https://pulse.example.test",
		1,
	)

	first := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"Org One"}`)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first request expected 202, got %d: %s", first.Code, first.Body.String())
	}

	orgsBefore, err := persistence.ListOrganizations()
	if err != nil {
		t.Fatalf("list orgs before rate-limit check: %v", err)
	}
	beforeCount := len(orgsBefore)

	second := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"Org Two"}`)
	if second.Code != http.StatusAccepted {
		t.Fatalf("second request expected 202, got %d: %s", second.Code, second.Body.String())
	}

	var firstResp hostedSignupResponse
	if err := json.NewDecoder(first.Body).Decode(&firstResp); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	var secondResp hostedSignupResponse
	if err := json.NewDecoder(second.Body).Decode(&secondResp); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if firstResp != secondResp {
		t.Fatalf("expected uniform accepted responses, got first=%+v second=%+v", firstResp, secondResp)
	}

	orgsAfter, err := persistence.ListOrganizations()
	if err != nil {
		t.Fatalf("list orgs after rate-limit check: %v", err)
	}
	afterCount := len(orgsAfter)

	if afterCount != beforeCount {
		t.Fatalf("expected org count unchanged on rate limit, before=%d after=%d", beforeCount, afterCount)
	}

	emailer.mu.Lock()
	defer emailer.mu.Unlock()
	if emailer.calls != 1 {
		t.Fatalf("expected exactly one magic-link email, got %d", emailer.calls)
	}
}

func TestHostedSignupCleanupOnRBACFailure(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	realRBAC := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		_ = realRBAC.Close()
	})

	wrapped := &failingRBACProvider{
		inner:      realRBAC,
		failUpdate: true,
	}

	router, _ := newHostedSignupTestRouterWithDeps(t, true, persistence, wrapped)

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if wrapped.removeCalls == 0 {
		t.Fatalf("expected RBAC provider RemoveTenant to be called during cleanup")
	}

	// Cleanup should remove any partially provisioned org directories.
	orgsDir := baseDir + "/orgs"
	entries, err := os.ReadDir(orgsDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read orgs dir: %v", err)
	}
	if err == nil && len(entries) != 0 {
		t.Fatalf("expected no org directories after cleanup; found %d", len(entries))
	}
}

func TestHostedSignupFailsClosedWithoutPublicURL(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	rbacProvider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		_ = rbacProvider.Close()
	})

	router, emailer := newHostedSignupTestRouterWithDepsAndPublicURL(t, true, persistence, rbacProvider, "")

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}

	orgs, err := persistence.ListOrganizations()
	if err != nil {
		t.Fatalf("list orgs after public-url failure: %v", err)
	}
	if len(orgs) != 1 || orgs[0].ID != "default" {
		t.Fatalf("expected no provisioned orgs when public URL is missing, got %+v", orgs)
	}

	emailer.mu.Lock()
	defer emailer.mu.Unlock()
	if emailer.calls != 0 {
		t.Fatalf("expected 0 magic link emails, got %d (public url missing must fail closed)", emailer.calls)
	}
}

func newHostedSignupTestRouter(t *testing.T, hostedMode bool) (*Router, *config.MultiTenantPersistence, *TenantRBACProvider, *captureMagicLinkEmailer, string) {
	t.Helper()

	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	rbacProvider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		_ = rbacProvider.Close()
	})

	router, emailer := newHostedSignupTestRouterWithDepsAndPublicURL(t, hostedMode, persistence, rbacProvider, "https://pulse.example.test")
	return router, persistence, rbacProvider, emailer, baseDir
}

func newHostedSignupTestRouterWithDeps(t *testing.T, hostedMode bool, persistence *config.MultiTenantPersistence, rbacProvider HostedRBACProvider) (*Router, *captureMagicLinkEmailer) {
	t.Helper()

	return newHostedSignupTestRouterWithDepsAndPublicURL(t, hostedMode, persistence, rbacProvider, "https://pulse.example.test")
}

func newHostedSignupTestRouterWithDepsAndPublicURL(t *testing.T, hostedMode bool, persistence *config.MultiTenantPersistence, rbacProvider HostedRBACProvider, publicURL string) (*Router, *captureMagicLinkEmailer) {
	t.Helper()

	return newHostedSignupTestRouterWithDepsAndPublicURLAndMagicLinkLimit(t, hostedMode, persistence, rbacProvider, publicURL, 1000)
}

func newHostedSignupTestRouterWithDepsAndPublicURLAndMagicLinkLimit(t *testing.T, hostedMode bool, persistence *config.MultiTenantPersistence, rbacProvider HostedRBACProvider, publicURL string, magicLinkLimit int) (*Router, *captureMagicLinkEmailer) {
	t.Helper()

	emailer := &captureMagicLinkEmailer{}
	// Use a stable key to avoid flakiness.
	key := []byte("0123456789abcdef0123456789abcdef")
	magicLinks := NewMagicLinkServiceWithKey(key, nil, emailer, NewRateLimiter(magicLinkLimit, 1*time.Hour))
	t.Cleanup(func() { magicLinks.Stop() })

	router := &Router{
		mux:               http.NewServeMux(),
		hostedMode:        hostedMode,
		signupRateLimiter: NewRateLimiter(5, 1*time.Hour),
	}
	t.Cleanup(func() {
		router.signupRateLimiter.Stop()
	})

	resolvePublicURL := func(_ *http.Request) string {
		return strings.TrimSpace(publicURL)
	}
	hostedSignupHandlers := NewHostedSignupHandlers(persistence, rbacProvider, magicLinks, resolvePublicURL, hostedMode)
	router.registerHostedRoutes(hostedSignupHandlers, nil, nil)

	return router, emailer
}

func doHostedSignupRequest(router *Router, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/public/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)
	return rec
}

func requireHostedSignupProvisionedOrgID(t *testing.T, persistence *config.MultiTenantPersistence, ownerEmail string) string {
	t.Helper()

	orgs, err := persistence.ListOrganizations()
	if err != nil {
		t.Fatalf("list orgs: %v", err)
	}
	for _, org := range orgs {
		if org == nil || org.ID == "default" {
			continue
		}
		if strings.EqualFold(org.OwnerEmail, ownerEmail) {
			return org.ID
		}
	}
	t.Fatalf("expected provisioned org for owner %q", ownerEmail)
	return ""
}

func containsRoleID(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type failingRBACProvider struct {
	inner      *TenantRBACProvider
	failUpdate bool

	removeCalls int
}

func (p *failingRBACProvider) GetManager(orgID string) (auth.ExtendedManager, error) {
	manager, err := p.inner.GetManager(orgID)
	if err != nil {
		return nil, err
	}
	if !p.failUpdate {
		return manager, nil
	}
	return failingManager{ExtendedManager: manager}, nil
}

func (p *failingRBACProvider) RemoveTenant(orgID string) error {
	p.removeCalls++
	return p.inner.RemoveTenant(orgID)
}

type failingManager struct {
	auth.ExtendedManager
}

func (m failingManager) UpdateUserRoles(username string, roleIDs []string) error {
	return fmt.Errorf("forced UpdateUserRoles failure")
}

type captureMagicLinkEmailer struct {
	mu           sync.Mutex
	calls        int
	to           string
	magicLinkURL string
	err          error
}

func (c *captureMagicLinkEmailer) SendMagicLink(to, magicLinkURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.to = to
	c.magicLinkURL = magicLinkURL
	return c.err
}
