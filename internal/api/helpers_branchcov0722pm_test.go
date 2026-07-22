package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// This file raises branch/function coverage for a set of previously
// uncovered pure helper functions across the internal/api package:
//   - normalizePBSUser / normalizePMGUser           (config_node_handlers.go)
//   - canonicalAutoRegisterCheckMissingFieldsMessage (config_setup_handlers.go)
//   - redactMagicLinkURL                             (magic_link.go)
//   - metadataSaveErrorMessage                        (metadata_helpers.go)
//   - shellQuote                                      (docker_container_action_executor.go)
//   - dockerPodmanAgentCommandCount                   (diagnostics.go)
//   - hostedEntitlementRefreshBackoff                 (hosted_entitlement_refresh.go)
//
// Every top-level function is prefixed with TestBranchcov0722PM so the run
// can be scoped with -run "^TestBranchcov0722PM".

// panics reports whether f panics when invoked. Used only to document the
// nil-deref behaviour of metadataSaveErrorMessage (a latent robustness bug
// that is reported, not fixed).
func panics(f func()) (did bool) {
	defer func() { did = recover() != nil }()
	f()
	return
}

func TestBranchcov0722PMNormalizePBSUser(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"empty_returns_empty", "", ""},
		{"whitespace_returns_empty", "   ", ""},
		{"plain_appends_suffix", "root", "root@pbs"},
		{"trims_then_appends", "  root  ", "root@pbs"},
		{"email_passthrough", "alice@example.com", "alice@example.com"},
		{"email_passthrough_after_trim", "  alice@example.com ", "alice@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizePBSUser(tc.in); got != tc.want {
				t.Fatalf("normalizePBSUser(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMNormalizePMGUser(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"empty_returns_empty", "", ""},
		{"whitespace_returns_empty", "\t\n", ""},
		{"plain_appends_suffix", "admin", "admin@pmg"},
		{"trims_then_appends", "  admin ", "admin@pmg"},
		{"email_passthrough", "bob@mail.com", "bob@mail.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizePMGUser(tc.in); got != tc.want {
				t.Fatalf("normalizePMGUser(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMCanonicalAutoRegisterCheckMissingFieldsMessage(t *testing.T) {
	const base = "Missing required canonical auto-register check fields"
	cases := []struct {
		name                        string
		typeValue, host, serverName string
		want                        string
	}{
		{"all_present", "pve", "10.0.0.1", "node1", base},
		{"whitespace_counts_as_missing", "   ", "10.0.0.1", "node1", base + ": type"},
		{"type_missing", "", "10.0.0.1", "node1", base + ": type"},
		{"host_missing", "pve", "", "node1", base + ": host"},
		{"serverName_missing", "pve", "10.0.0.1", "", base + ": serverName"},
		{"type_and_host_missing", "", "", "node1", base + ": type, host"},
		{"all_missing", "", "", "", base + ": type, host, serverName"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalAutoRegisterCheckMissingFieldsMessage(tc.typeValue, tc.host, tc.serverName)
			if got != tc.want {
				t.Fatalf("canonicalAutoRegisterCheckMissingFieldsMessage(%q,%q,%q) = %q, want %q",
					tc.typeValue, tc.host, tc.serverName, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMRedactMagicLinkURL(t *testing.T) {
	cases := []struct {
		name, in, want, leak string
	}{
		{
			name: "token_in_query_stripped",
			in:   "https://mail.example.com/accept?token=ml1_SUPERSECRET",
			want: "https://mail.example.com/accept",
			leak: "ml1_SUPERSECRET",
		},
		{
			name: "token_in_fragment_stripped",
			in:   "https://mail.example.com/accept#token=ml1_FRAGSECRET",
			want: "https://mail.example.com/accept",
			leak: "ml1_FRAGSECRET",
		},
		{
			name: "token_in_query_and_fragment_stripped",
			in:   "https://mail.example.com/accept?token=ml1_Q#token=ml1_F",
			want: "https://mail.example.com/accept",
			leak: "ml1_Q",
		},
		{
			name: "path_preserved_with_creds_dropped",
			in:   "https://app.test/a/b/c?token=ml1_X&other=keep#ml1_Y",
			want: "https://app.test/a/b/c",
			leak: "ml1_X",
		},
		{
			// Unclosed IPv6 host bracket -> url.Parse reports an error -> "".
			name: "malformed_unparseable_returns_empty",
			in:   "http://[",
			want: "",
		},
		{
			name: "empty_returns_empty",
			in:   "",
			want: "",
		},
		{
			name: "whitespace_only_returns_empty",
			in:   "    ",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactMagicLinkURL(tc.in)
			if got != tc.want {
				t.Fatalf("redactMagicLinkURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
			// Security invariant: the secret token must never survive redaction.
			if tc.leak != "" && strings.Contains(got, tc.leak) {
				t.Fatalf("secret %q leaked into redacted output %q", tc.leak, got)
			}
			// And defensively, neither the fragment value nor the literal key.
			if strings.Contains(got, "token=") {
				t.Fatalf("token query key leaked into redacted output %q", got)
			}
		})
	}
}

func TestBranchcov0722PMMetadataSaveErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "permission_substring",
			err:  errors.New("disk permission denied by OS"),
			want: "Permission denied - check file permissions",
		},
		{
			name: "wrapped_permission",
			err:  fmt.Errorf("save failed: %w", errors.New("permission issue")),
			want: "Permission denied - check file permissions",
		},
		{
			name: "no_space_substring",
			err:  errors.New("write failed: no space left on device"),
			want: "Disk full - cannot save metadata",
		},
		{
			name: "plain_error_default",
			err:  errors.New("network timeout"),
			want: "Failed to save metadata",
		},
		{
			name: "wrapped_plain_error_default",
			err:  fmt.Errorf("boom: %w", errors.New("database locked")),
			want: "Failed to save metadata",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := metadataSaveErrorMessage(tc.err); got != tc.want {
				t.Fatalf("metadataSaveErrorMessage(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}

	// The signature accepts (err error) but the body calls err.Error()
	// unconditionally, so a nil error is NOT safe: it nil-derefs. Assert that
	// real behaviour and surface it as a latent robustness bug (reported, not fixed).
	t.Run("nil_panics_nil_deref_bug", func(t *testing.T) {
		if !panics(func() { _ = metadataSaveErrorMessage(nil) }) {
			t.Fatalf("expected metadataSaveErrorMessage(nil) to panic (nil deref in err.Error()), but it did not")
		}
	})
}

func TestBranchcov0722PMShellQuote(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", "''"},
		{"plain", "hello", "'hello'"},
		{"spaces_preserved", "hello world", "'hello world'"},
		{"newline_preserved", "line1\nline2", "'line1\nline2'"},
		{"tab_preserved", "a\tb", "'a\tb'"},
		{"single_quote_escaped", "a'b", `'a'"'"'b'`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellQuote(tc.in); got != tc.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMDockerPodmanAgentCommandCount(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want string
	}{
		{"zero_uses_plural", 0, "0 Docker / Podman module commands"},
		{"singular", 1, "1 Docker / Podman module command"},
		{"plural", 3, "3 Docker / Podman module commands"},
		{"plural_large", 42, "42 Docker / Podman module commands"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dockerPodmanAgentCommandCount(tc.in); got != tc.want {
				t.Fatalf("dockerPodmanAgentCommandCount(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMHostedEntitlementRefreshBackoff(t *testing.T) {
	const (
		def        = 2 * time.Hour
		backoffMin = 30 * time.Second
		backoffMax = 30 * time.Minute
	)
	cases := []struct {
		name string
		in   int
		want time.Duration
	}{
		{"zero_uses_default_interval", 0, def},
		{"negative_uses_default_interval", -5, def},
		{"failure_1", 1, backoffMin},                 // 30s * (1<<0) = 30s
		{"failure_2", 2, 2 * backoffMin},             // 30s * (1<<1) = 60s
		{"failure_3", 3, 4 * backoffMin},             // 30s * (1<<2) = 120s
		{"failure_6_below_cap", 6, 32 * backoffMin},  // 30s * (1<<5) = 960s = 16m < 30m
		{"failure_7_hits_cap", 7, backoffMax},        // 30s * (1<<6) = 1920s = 32m > 30m
		{"failure_11_shift_clamped", 11, backoffMax}, // (1<<min(10,10)) -> capped
		{"huge_count_capped", 100000, backoffMax},    // (1<<10) -> capped
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hostedEntitlementRefreshBackoff(tc.in)
			if got != tc.want {
				t.Fatalf("hostedEntitlementRefreshBackoff(%d) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}

	// Contract: a strictly positive backoff for every possible input.
	for _, failures := range []int{-100, -1, 0, 1, 5, 50, 100000} {
		if d := hostedEntitlementRefreshBackoff(failures); d <= 0 {
			t.Fatalf("hostedEntitlementRefreshBackoff(%d) returned non-positive duration %v", failures, d)
		}
	}
}
