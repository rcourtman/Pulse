package cloudcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// TestNormalizeHostedEntitlementOrgID covers every branch of
// normalizeHostedEntitlementOrgID: empty -> default sentinel, whitespace-only ->
// default sentinel, and a populated value returned verbatim (trim only; the
// function deliberately does not lowercase).
func TestNormalizeHostedEntitlementOrgID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: hostedEntitlementDefaultOrgID},
		{name: "whitespace only", raw: "   \t\n", want: hostedEntitlementDefaultOrgID},
		{name: "default passthrough", raw: "default", want: "default"},
		{name: "trim preserves case", raw: "  Acme-Org-ID  ", want: "Acme-Org-ID"},
		{name: "surrogate default with surrounding spaces", raw: "  default  ", want: "default"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeHostedEntitlementOrgID(tc.raw); got != tc.want {
				t.Fatalf("normalizeHostedEntitlementOrgID(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestRedactCloudMagicLinkURL exercises every return path of
// redactCloudMagicLinkURL: parse failure, empty scheme, empty host, and the
// success path that strips query/fragment from a well-formed URL.
func TestRedactCloudMagicLinkURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "parse error invalid control char", raw: "http://x/\x7f", want: ""},
		{name: "parse error invalid escape", raw: "http://x/%zz", want: ""},
		{name: "missing scheme bare path", raw: "/just/path", want: ""},
		{name: "missing scheme opaque host", raw: "example.com", want: ""},
		{name: "missing host", raw: "http://", want: ""},
		{
			name: "strips query and fragment",
			raw:  "http://example.com/auth/magic-link/verify?token=SECRET&keep=1#frag",
			want: "http://example.com/auth/magic-link/verify",
		},
		{
			name: "trims surrounding whitespace before parse",
			raw:  "   http://example.com/x?token=abc   ",
			want: "http://example.com/x",
		},
		{
			name: "does not redact path segments",
			raw:  "https://cloud.example.com/verify/abc123?token=xyz",
			want: "https://cloud.example.com/verify/abc123",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := redactCloudMagicLinkURL(tc.raw); got != tc.want {
				t.Fatalf("redactCloudMagicLinkURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestRedactCloudMagicLinkURL_OnlyStripsQueryAndFragment ensures that even when
// the URL has no token in the query, the function still returns the canonical
// "no-token" form (the success arm with no query/fragment).
func TestRedactCloudMagicLinkURL_NoTokenArm(t *testing.T) {
	t.Parallel()
	got := redactCloudMagicLinkURL("https://cloud.example.com/verify")
	if got != "https://cloud.example.com/verify" {
		t.Fatalf("got %q, want canonical URL unchanged", got)
	}
}

// TestDefaultProviderMSPBackupPath covers every branch of
// DefaultProviderMSPBackupPath: nil cfg, cfg with blank DataDir, cfg with
// whitespace-only DataDir, and a populated DataDir that produces the
// "<datadir>/backups/provider-msp/<name>" layout.
func TestDefaultProviderMSPBackupPath(t *testing.T) {
	t.Parallel()
	fixedNow := time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC)
	const wantName = "provider-msp-backup-20240315T143045Z.tar.gz"

	t.Run("nil cfg returns bare name", func(t *testing.T) {
		t.Parallel()
		if got := DefaultProviderMSPBackupPath(nil, fixedNow); got != wantName {
			t.Fatalf("got %q, want %q", got, wantName)
		}
	})

	t.Run("empty DataDir returns bare name", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{}
		if got := DefaultProviderMSPBackupPath(cfg, fixedNow); got != wantName {
			t.Fatalf("got %q, want %q", got, wantName)
		}
	})

	t.Run("whitespace DataDir returns bare name", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{DataDir: "   "}
		if got := DefaultProviderMSPBackupPath(cfg, fixedNow); got != wantName {
			t.Fatalf("got %q, want %q", got, wantName)
		}
	})

	t.Run("populated DataDir returns nested path", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{DataDir: "/tmp/pulse-data"}
		want := filepath.Join("/tmp/pulse-data", "backups", "provider-msp", wantName)
		if got := DefaultProviderMSPBackupPath(cfg, fixedNow); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("zero now falls back to wall clock but preserves shape", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{DataDir: "/tmp/pulse-data"}
		got := DefaultProviderMSPBackupPath(cfg, time.Time{})
		if !strings.HasPrefix(got, "/tmp/pulse-data/backups/provider-msp/provider-msp-backup-") {
			t.Fatalf("got %q, want shape with current timestamp", got)
		}
		if !strings.HasSuffix(got, ".tar.gz") {
			t.Fatalf("got %q, want .tar.gz suffix", got)
		}
	})
}

// TestProviderMSPPortalAccessForEmail_NotAMember covers the final return path
// of providerMSPPortalAccessForEmail: an empty registry (no user, no
// invitation) yields the operator-facing "not a member" error with empty
// accessState/role.
func TestProviderMSPPortalAccessForEmail_NotAMember(t *testing.T) {
	t.Parallel()
	reg := newEmptyRegistryForTest(t)
	defer reg.Close()

	accessState, role, err := providerMSPPortalAccessForEmail(reg, "nobody@example.com")
	if err == nil {
		t.Fatal("expected error for unknown email, got nil")
	}
	if !strings.Contains(err.Error(), "is not an account member") {
		t.Fatalf("err = %q, want substring 'is not an account member'", err.Error())
	}
	if !strings.Contains(err.Error(), "nobody@example.com") {
		t.Fatalf("err = %q, want substring 'nobody@example.com'", err.Error())
	}
	if accessState != "" || role != "" {
		t.Fatalf("accessState=%q role=%q, want both empty", accessState, role)
	}
}

// TestProviderMSPPortalLink_ErrorPaths covers the cfg/email validation branches
// of ProviderMSPPortalLink that are reachable without a live magic-link
// service. It drives: nil cfg, non-MSP cfg, invalid email, and the
// "no member / no invitation" arm via an empty registry.
func TestProviderMSPPortalLink_ErrorPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil cfg", func(t *testing.T) {
		t.Parallel()
		_, err := ProviderMSPPortalLink(ctx, nil, ProviderMSPPortalLinkOptions{Email: "owner@example.com"})
		if err == nil || !strings.Contains(err.Error(), "control plane config is required") {
			t.Fatalf("err = %v, want 'control plane config is required'", err)
		}
	})

	t.Run("non-MSP control plane mode", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{ControlPlaneMode: ControlPlaneModePulseHosted}
		_, err := ProviderMSPPortalLink(ctx, cfg, ProviderMSPPortalLinkOptions{Email: "owner@example.com"})
		if err == nil || !strings.Contains(err.Error(), "provider MSP portal-link requires") {
			t.Fatalf("err = %v, want 'provider MSP portal-link requires'", err)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{ControlPlaneMode: ControlPlaneModeProviderHostedMSP}
		_, err := ProviderMSPPortalLink(ctx, cfg, ProviderMSPPortalLinkOptions{Email: "not-an-email"})
		if err == nil || !strings.Contains(err.Error(), "email is invalid") {
			t.Fatalf("err = %v, want 'email is invalid'", err)
		}
	})

	t.Run("empty email", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{
			ControlPlaneMode: ControlPlaneModeProviderHostedMSP,
			DataDir:          filepath.Join(t.TempDir(), "data"),
		}
		_, err := ProviderMSPPortalLink(ctx, cfg, ProviderMSPPortalLinkOptions{Email: ""})
		if err == nil || !strings.Contains(err.Error(), "email is invalid") {
			t.Fatalf("err = %v, want 'email is invalid'", err)
		}
	})

	t.Run("valid email but no access in empty registry", func(t *testing.T) {
		t.Parallel()
		cfg := &CPConfig{
			ControlPlaneMode: ControlPlaneModeProviderHostedMSP,
			DataDir:          filepath.Join(t.TempDir(), "data"),
		}
		_, err := ProviderMSPPortalLink(ctx, cfg, ProviderMSPPortalLinkOptions{Email: "stranger@example.com"})
		if err == nil || !strings.Contains(err.Error(), "is not an account member") {
			t.Fatalf("err = %v, want 'is not an account member'", err)
		}
	})
}

// newEmptyRegistryForTest constructs a real (SQLite-backed) registry rooted at
// a per-test tempdir. The dir is created by NewTenantRegistry, so no prior
// setup is required; the registry has no users/accounts/memberships/invitations
// which is exactly what the "no access" tests below require.
func newEmptyRegistryForTest(t *testing.T) *registry.TenantRegistry {
	t.Helper()
	reg, err := registry.NewTenantRegistry(filepath.Join(t.TempDir(), "control-plane"))
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	return reg
}
