package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// denyAllAuthorizer always denies, so tests can prove the gate short-circuits to
// allow-all (returns true) WITHOUT consulting the wrapped authorizer.
type denyAllAuthorizer struct{}

func (denyAllAuthorizer) Authorize(_ context.Context, _ string, _ string) (bool, error) {
	return false, nil
}

type fakeLicenseProvider struct {
	svc *license.Service
}

func (f fakeLicenseProvider) Service(_ context.Context) *license.Service {
	return f.svc
}

func withLicenseProvider(t *testing.T, provider LicenseServiceProvider) {
	t.Helper()
	licenseServiceMu.Lock()
	prev := licenseServiceProvider
	licenseServiceMu.Unlock()
	SetLicenseServiceProvider(provider)
	t.Cleanup(func() {
		licenseServiceMu.Lock()
		licenseServiceProvider = prev
		licenseServiceMu.Unlock()
	})
}

func TestGatedRBACAuthorizer_AllowsAllWhenDisabled(t *testing.T) {
	g := &gatedRBACAuthorizer{
		cfg:   &config.Config{RBACEnforcementEnabled: false},
		inner: denyAllAuthorizer{},
	}
	ok, err := g.Authorize(context.Background(), auth.ActionRead, auth.ResourceNodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected allow-all when enforcement is disabled, got deny")
	}
}

func TestGatedRBACAuthorizer_EnvOverrideForcesDisabled(t *testing.T) {
	disabled := false
	g := &gatedRBACAuthorizer{
		cfg:         &config.Config{RBACEnforcementEnabled: true}, // setting on...
		inner:       denyAllAuthorizer{},
		envOverride: &disabled, // ...but env break-glass wins
	}
	ok, err := g.Authorize(context.Background(), auth.ActionRead, auth.ResourceNodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected env override to force allow-all, got deny")
	}
}

func TestGatedRBACAuthorizer_NoLicenseFailsOpen(t *testing.T) {
	// A service with no activated license grants only free-tier features, so the
	// RBAC feature gate must keep enforcement off (allow-all) even when the
	// operator turned the setting on.
	withLicenseProvider(t, fakeLicenseProvider{svc: license.NewService()})

	g := &gatedRBACAuthorizer{
		cfg:   &config.Config{RBACEnforcementEnabled: true},
		inner: denyAllAuthorizer{},
	}
	ok, err := g.Authorize(context.Background(), auth.ActionRead, auth.ResourceNodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected allow-all without an RBAC-licensed service, got deny")
	}
}

// TestGatedRBACAuthorizer_EndToEndEnforcement drives the real enforcement path
// (gate -> RBACAuthorizer -> PolicyEvaluator -> FileManager) with a genuine
// read-only role, proving that the configured admin bypasses, an assigned
// read-only user is allowed reads but denied writes, and an unassigned user is
// denied. This is the behaviour a v5 Pro customer like Luis expects.
func TestGatedRBACAuthorizer_EndToEndEnforcement(t *testing.T) {
	t.Setenv("PULSE_DEV", "true") // grant the RBAC feature without a signed license
	withLicenseProvider(t, fakeLicenseProvider{svc: license.NewService()})

	mgr, err := auth.NewFileManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileManager: %v", err)
	}
	if err := mgr.SaveRole(auth.Role{
		ID:          "nodes-reader",
		Name:        "Nodes Reader",
		Permissions: []auth.Permission{{Action: auth.ActionRead, Resource: auth.ResourceNodes, Effect: auth.EffectAllow}},
	}); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}
	if err := mgr.UpdateUserRoles("viewer-user", []string{"nodes-reader"}); err != nil {
		t.Fatalf("UpdateUserRoles: %v", err)
	}

	g := &gatedRBACAuthorizer{
		cfg:   &config.Config{RBACEnforcementEnabled: true},
		inner: auth.NewRBACAuthorizer(mgr),
	}
	g.SetAdminUser("admin") // mirrors router.go calling auth.SetAdminUser(cfg.AuthUser)

	cases := []struct {
		name     string
		user     string
		action   string
		resource string
		want     bool
	}{
		{"admin bypasses on write", "admin", auth.ActionAdmin, auth.ResourceSettings, true},
		{"viewer allowed read nodes", "viewer-user", auth.ActionRead, auth.ResourceNodes, true},
		{"viewer denied write nodes", "viewer-user", "write", auth.ResourceNodes, false},
		{"viewer denied other resource", "viewer-user", auth.ActionRead, auth.ResourceSettings, false},
		{"unassigned user denied", "ghost", auth.ActionRead, auth.ResourceNodes, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := auth.WithUser(context.Background(), tc.user)
			got, err := g.Authorize(ctx, tc.action, tc.resource)
			if err != nil {
				t.Fatalf("Authorize: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Authorize(%s, %s, %s) = %v, want %v", tc.user, tc.action, tc.resource, got, tc.want)
			}
		})
	}
}

func TestGatedRBACAuthorizer_EnforcesWhenLicensedAndEnabled(t *testing.T) {
	// Dev mode makes HasFeature return true for all Pro features, standing in for
	// an RBAC-licensed deployment without needing a signed license key.
	t.Setenv("PULSE_DEV", "true")
	withLicenseProvider(t, fakeLicenseProvider{svc: license.NewService()})

	g := &gatedRBACAuthorizer{
		cfg:   &config.Config{RBACEnforcementEnabled: true},
		inner: denyAllAuthorizer{},
	}
	ok, err := g.Authorize(context.Background(), auth.ActionRead, auth.ResourceNodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected enforcement to delegate to the (deny-all) inner authorizer, got allow")
	}
}

func TestEnsureScope_EnforcesRBACForSessionUsers(t *testing.T) {
	t.Setenv("PULSE_DEV", "true") // grant the RBAC feature without a signed license
	withLicenseProvider(t, fakeLicenseProvider{svc: license.NewService()})

	mgr, err := auth.NewFileManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileManager: %v", err)
	}
	if err := mgr.UpdateUserRoles("viewer-user", []string{auth.RoleViewer}); err != nil {
		t.Fatalf("UpdateUserRoles: %v", err)
	}

	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&gatedRBACAuthorizer{
		cfg:   &config.Config{RBACEnforcementEnabled: true},
		inner: auth.NewRBACAuthorizer(mgr),
	})
	t.Cleanup(func() {
		auth.SetAuthorizer(prevAuthorizer)
	})

	readReq := httptest.NewRequest(http.MethodGet, "/api/charts", nil)
	readReq = readReq.WithContext(auth.WithUser(readReq.Context(), "viewer-user"))
	readRec := httptest.NewRecorder()
	if !ensureScope(readRec, readReq, config.ScopeMonitoringRead) {
		t.Fatalf("expected viewer to pass monitoring read scope, got %d", readRec.Code)
	}

	writeReq := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", nil)
	writeReq = writeReq.WithContext(auth.WithUser(writeReq.Context(), "viewer-user"))
	writeRec := httptest.NewRecorder()
	if ensureScope(writeRec, writeReq, config.ScopeSettingsWrite) {
		t.Fatal("expected viewer to fail settings write scope")
	}
	if writeRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for denied RBAC scope, got %d", writeRec.Code)
	}
}

func TestEnsureScope_AllowsSessionUsersWhenRBACDisabled(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&gatedRBACAuthorizer{
		cfg:   &config.Config{RBACEnforcementEnabled: false},
		inner: denyAllAuthorizer{},
	})
	t.Cleanup(func() {
		auth.SetAuthorizer(prevAuthorizer)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "viewer-user"))
	rec := httptest.NewRecorder()
	if !ensureScope(rec, req, config.ScopeSettingsWrite) {
		t.Fatalf("expected disabled RBAC gate to preserve legacy session access, got %d", rec.Code)
	}
}
