package portal

import (
	"reflect"
	"testing"
)

// TestBranchcov0723Am_AnonymousBootstrap_ZeroEnvironment_AssertsEveryField
// drives BuildAnonymousBootstrapDataWithEnvironment with a zero-value
// PortalEnvironment and asserts every field of the returned BootstrapData,
// including that Accounts is a non-nil empty slice (the source uses make over
// the nil accounts input rather than leaving the field nil).
func TestBranchcov0723Am_AnonymousBootstrap_ZeroEnvironment_AssertsEveryField(t *testing.T) {
	got := BuildAnonymousBootstrapDataWithEnvironment(PortalEnvironment{})

	want := BootstrapData{
		// Identity/security-relevant fields: anonymous must leave all of these unset.
		Authenticated:           false,
		Email:                   "",
		HasSelfHostedCommercial: false,
		// Environment-derived fields: a zero PortalEnvironment propagates zero.
		EmailSignInAvailable: false,
		ProviderHostedMode:   false,
		SignupPath:           "",
		// Static defaults copied from the package-level default*/Portal* constants.
		PublicSiteURL:         "https://pulserelay.pro",
		SupportEmail:          "support@pulserelay.pro",
		CommercialAPIBaseURL:  "/api/portal/commercial",
		CommercialAPIBasePath: "/api/portal/commercial",
		PortalPath:            "/portal",
		BootstrapPath:         "/api/portal/bootstrap",
		MagicLinkRequestPath:  "/api/public/magic-link/request",
		LogoutPath:            "/auth/logout",
		AccountAPIBasePath:    "/api/accounts",
		PortalAPIBasePath:     "/api/portal",
		// Source runs make([]BootstrapAccount, 0, len(nil)) -> non-nil empty slice.
		Accounts: []BootstrapAccount{},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildAnonymousBootstrapDataWithEnvironment(zero env) mismatch:\n got = %#v\nwant = %#v", got, want)
	}
	if got.Accounts == nil {
		t.Errorf("Accounts = nil, want non-nil empty slice (source uses make())")
	}
}

// TestBranchcov0723Am_AnonymousBootstrap_PopulatedEnvironment_AssertsEveryField
// drives every PortalEnvironment field from the populated side and asserts
// each propagates onto the result while every static default and the anonymity
// invariant remain unchanged.
func TestBranchcov0723Am_AnonymousBootstrap_PopulatedEnvironment_AssertsEveryField(t *testing.T) {
	env := PortalEnvironment{
		SignupPath:           "/custom-signup",
		EmailSignInAvailable: true,
		ProviderHostedMode:   true,
	}
	got := BuildAnonymousBootstrapDataWithEnvironment(env)

	want := BootstrapData{
		// Still anonymous: identity fields stay empty regardless of env.
		Authenticated:           false,
		Email:                   "",
		HasSelfHostedCommercial: false,
		// Environment-derived: every env field propagates.
		EmailSignInAvailable: true,
		ProviderHostedMode:   true,
		SignupPath:           "/custom-signup",
		// Static defaults are env-independent.
		PublicSiteURL:         "https://pulserelay.pro",
		SupportEmail:          "support@pulserelay.pro",
		CommercialAPIBaseURL:  "/api/portal/commercial",
		CommercialAPIBasePath: "/api/portal/commercial",
		PortalPath:            "/portal",
		BootstrapPath:         "/api/portal/bootstrap",
		MagicLinkRequestPath:  "/api/public/magic-link/request",
		LogoutPath:            "/auth/logout",
		AccountAPIBasePath:    "/api/accounts",
		PortalAPIBasePath:     "/api/portal",
		Accounts:              []BootstrapAccount{},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildAnonymousBootstrapDataWithEnvironment(populated env) mismatch:\n got = %#v\nwant = %#v", got, want)
	}
	if got.Accounts == nil {
		t.Errorf("Accounts = nil, want non-nil empty slice")
	}
}

// TestBranchcov0723Am_AnonymousBootstrap_EnvironmentFieldPropagation drives
// each PortalEnvironment-driven conditional from both arms independently so
// the propagation of SignupPath, EmailSignInAvailable and ProviderHostedMode
// is exercised in isolation rather than only via the all-zero/all-set cases.
func TestBranchcov0723Am_AnonymousBootstrap_EnvironmentFieldPropagation(t *testing.T) {
	cases := []struct {
		name             string
		env              PortalEnvironment
		wantSignupPath   string
		wantEmailSignIn  bool
		wantProviderHost bool
	}{
		{
			name:             "signup_path_empty",
			env:              PortalEnvironment{SignupPath: ""},
			wantSignupPath:   "",
			wantEmailSignIn:  false,
			wantProviderHost: false,
		},
		{
			name:             "signup_path_populated",
			env:              PortalEnvironment{SignupPath: "/sign-up-here"},
			wantSignupPath:   "/sign-up-here",
			wantEmailSignIn:  false,
			wantProviderHost: false,
		},
		{
			name:             "email_sign_in_off",
			env:              PortalEnvironment{EmailSignInAvailable: false},
			wantSignupPath:   "",
			wantEmailSignIn:  false,
			wantProviderHost: false,
		},
		{
			name:             "email_sign_in_on",
			env:              PortalEnvironment{EmailSignInAvailable: true},
			wantSignupPath:   "",
			wantEmailSignIn:  true,
			wantProviderHost: false,
		},
		{
			name:             "provider_hosted_off",
			env:              PortalEnvironment{ProviderHostedMode: false},
			wantSignupPath:   "",
			wantEmailSignIn:  false,
			wantProviderHost: false,
		},
		{
			name:             "provider_hosted_on",
			env:              PortalEnvironment{ProviderHostedMode: true},
			wantSignupPath:   "",
			wantEmailSignIn:  false,
			wantProviderHost: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := BuildAnonymousBootstrapDataWithEnvironment(tc.env)
			if got.SignupPath != tc.wantSignupPath {
				t.Errorf("SignupPath = %q, want %q", got.SignupPath, tc.wantSignupPath)
			}
			if got.EmailSignInAvailable != tc.wantEmailSignIn {
				t.Errorf("EmailSignInAvailable = %v, want %v", got.EmailSignInAvailable, tc.wantEmailSignIn)
			}
			if got.ProviderHostedMode != tc.wantProviderHost {
				t.Errorf("ProviderHostedMode = %v, want %v", got.ProviderHostedMode, tc.wantProviderHost)
			}
		})
	}
}

// TestBranchcov0723Am_AnonymousBootstrap_NeverPopulatesIdentityFields locks
// the security-relevant invariant of the anonymous builder: no tenant/user
// identity may leak onto the bootstrap payload even when every environment
// feature flag is enabled.
func TestBranchcov0723Am_AnonymousBootstrap_NeverPopulatesIdentityFields(t *testing.T) {
	env := PortalEnvironment{
		SignupPath:           "/signup",
		EmailSignInAvailable: true,
		ProviderHostedMode:   true,
	}
	got := BuildAnonymousBootstrapDataWithEnvironment(env)

	if got.Authenticated {
		t.Errorf("Authenticated = true on anonymous bootstrap; must always be false")
	}
	if got.Email != "" {
		t.Errorf("Email = %q on anonymous bootstrap; must be empty (no user identity)", got.Email)
	}
	if got.HasSelfHostedCommercial {
		t.Errorf("HasSelfHostedCommercial = true on anonymous bootstrap; must be false")
	}
	if len(got.Accounts) != 0 {
		t.Errorf("len(Accounts) = %d on anonymous bootstrap; must be 0 (no tenant data)", len(got.Accounts))
	}
	for i, acc := range got.Accounts {
		t.Errorf("Accounts[%d] = %+v on anonymous bootstrap; no tenant data permitted", i, acc)
	}
}

// TestBranchcov0723Am_AnonymousBootstrap_ResultIndependentOfEnvAfterReturn
// confirms the returned BootstrapData is decoupled from the caller's
// PortalEnvironment: mutating the input after the call must not change the
// result, and the returned Accounts slice must be a fresh allocation per call
// (not aliased to any package-level shared slice).
func TestBranchcov0723Am_AnonymousBootstrap_ResultIndependentOfEnvAfterReturn(t *testing.T) {
	env := PortalEnvironment{
		SignupPath:           "/before",
		EmailSignInAvailable: true,
		ProviderHostedMode:   true,
	}
	got := BuildAnonymousBootstrapDataWithEnvironment(env)

	wantSignup := got.SignupPath
	wantEmailSignIn := got.EmailSignInAvailable
	wantProviderHost := got.ProviderHostedMode
	wantAccountsLen := len(got.Accounts)

	// Mutate the input environment after the call.
	env.SignupPath = "/mutated"
	env.EmailSignInAvailable = false
	env.ProviderHostedMode = false

	if got.SignupPath != wantSignup {
		t.Errorf("SignupPath changed after env mutation: got %q, want %q", got.SignupPath, wantSignup)
	}
	if got.EmailSignInAvailable != wantEmailSignIn {
		t.Errorf("EmailSignInAvailable changed after env mutation: got %v, want %v", got.EmailSignInAvailable, wantEmailSignIn)
	}
	if got.ProviderHostedMode != wantProviderHost {
		t.Errorf("ProviderHostedMode changed after env mutation: got %v, want %v", got.ProviderHostedMode, wantProviderHost)
	}
	if len(got.Accounts) != wantAccountsLen {
		t.Errorf("len(Accounts) changed after env mutation: got %d, want %d", len(got.Accounts), wantAccountsLen)
	}

	// Mutate the returned slice header and confirm a second call still returns
	// a fresh empty slice, proving no shared state between calls.
	got.Accounts = append(got.Accounts, BootstrapAccount{ID: "injected"})
	got2 := BuildAnonymousBootstrapDataWithEnvironment(env)
	if len(got2.Accounts) != 0 {
		t.Errorf("second call returned %d accounts; builder must not share slice state, want 0", len(got2.Accounts))
	}
}
