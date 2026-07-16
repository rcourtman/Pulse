package securityutil

import (
	"strings"
	"testing"
)

// TestBranchCovHashedStorageName pins HashedStorageName to the published
// SHA-256 test vectors (an independent oracle, not a re-implementation of the
// function under test) and exercises the edge cases the function's single
// expression quietly handles: empty input, multibyte/Unicode input, and the
// filename-safety contract of the returned stem.
func TestBranchCovHashedStorageName(t *testing.T) {
	t.Run("known_sha256_vectors", func(t *testing.T) {
		tests := []struct {
			name string
			id   string
			want string
		}{
			{name: "empty string maps to sha256 of empty input", id: "", want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			{name: "nist abc vector", id: "abc", want: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
			{name: "hello vector", id: "hello", want: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
			{name: "multibyte unicode input is hashed over utf-8 bytes", id: "unicode-→-é-你", want: "2efbbf4012701ef1bdceac2226531cc5ad0f940083dbf7bc75d1d883c0012586"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := HashedStorageName(tt.id); got != tt.want {
					t.Fatalf("HashedStorageName(%q) = %q, want %q", tt.id, got, tt.want)
				}
			})
		}
	})

	t.Run("deterministic_and_distinct", func(t *testing.T) {
		a := HashedStorageName("tenant-a/user-123")
		b := HashedStorageName("tenant-a/user-123")
		if a != b {
			t.Fatalf("HashedStorageName is not deterministic: %q vs %q", a, b)
		}
		// Distinct inputs (including the empty/non-empty boundary) must not collide.
		empty := HashedStorageName("")
		nonEmpty := HashedStorageName("x")
		if empty == nonEmpty {
			t.Fatalf("HashedStorageName collapsed empty and %q to the same digest %q", "x", empty)
		}
	})

	t.Run("filename_safe_fixed_width_lowercase_hex", func(t *testing.T) {
		for _, id := range []string{"", "a", strings.Repeat("long-", 64), "with/slash", "with..dot"} {
			got := HashedStorageName(id)
			if len(got) != 64 {
				t.Fatalf("HashedStorageName(%q) length = %d, want fixed 64 hex chars", id, len(got))
			}
			for i, r := range got {
				isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
				if !isHex {
					t.Fatalf("HashedStorageName(%q) produced non-lowercase-hex rune %q at %d in %q", id, r, i, got)
				}
			}
			// A storage stem must never smuggle path separators or extensions.
			if strings.ContainsAny(got, `/\.`) {
				t.Fatalf("HashedStorageName(%q) produced unsafe filename %q", id, got)
			}
		}
	})
}

// TestBranchCovNormalizeAbsoluteHTTPURL drives every branch of the underlying
// NormalizeAbsoluteHTTPURL validator through the in-package wrapper: the
// empty-input guard, the url.Parse error path, the scheme/host/userinfo/hostname
// rejection arms, and the success path (which preserves query and fragment and
// relies on url.Parse to lowercase the scheme while leaving the host cased as
// given).
func TestBranchCovNormalizeAbsoluteHTTPURL(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      string
		wantError string
	}{
		{name: "empty input rejected", raw: "", wantError: "URL is required"},
		{name: "whitespace-only input rejected after trim", raw: " \t\n ", wantError: "URL is required"},

		{name: "malformed ipv6 host triggers parse error", raw: "http://[::1", wantError: "invalid URL"},

		{name: "unsupported ftp scheme rejected", raw: "ftp://example.com", wantError: "URL scheme must be http or https"},
		{name: "scheme-relative url rejected as missing scheme", raw: "//example.com/path", wantError: "URL scheme must be http or https"},
		{name: "bare path rejected as missing scheme", raw: "example.com", wantError: "URL scheme must be http or https"},

		{name: "empty host on bare scheme rejected", raw: "http://", wantError: "URL host is required"},
		{name: "empty host with path rejected", raw: "http:///path", wantError: "URL host is required"},

		{name: "userinfo user password rejected", raw: "http://user:pass@example.com", wantError: "URL userinfo is not allowed"},
		{name: "userinfo user only rejected", raw: "http://user@example.com", wantError: "URL userinfo is not allowed"},

		{name: "port-only host has empty hostname", raw: "http://:8080/path", wantError: "URL hostname is required"},

		{name: "uppercase scheme accepted via url parse lowercasing, host case preserved", raw: "HTTPS://Example.com", want: "https://Example.com"},
		{name: "leading and trailing whitespace is trimmed before parsing", raw: "  https://example.com/path?q=1#frag  ", want: "https://example.com/path?q=1#frag"},
		{name: "success preserves query and fragment unlike base url normalizers", raw: "https://example.com/path?q=1#frag", want: "https://example.com/path?q=1#frag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeAbsoluteHTTPURL(tt.raw)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("NormalizeAbsoluteHTTPURL(%q) = %q, want error containing %q", tt.raw, got, tt.wantError)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("NormalizeAbsoluteHTTPURL(%q) error = %q, want substring %q", tt.raw, err.Error(), tt.wantError)
				}
				if got != nil {
					t.Fatalf("NormalizeAbsoluteHTTPURL(%q) returned non-nil URL %q alongside error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeAbsoluteHTTPURL(%q) error = %v", tt.raw, err)
			}
			if got == nil {
				t.Fatalf("NormalizeAbsoluteHTTPURL(%q) returned nil URL", tt.raw)
			}
			if got.String() != tt.want {
				t.Fatalf("NormalizeAbsoluteHTTPURL(%q) = %q, want %q", tt.raw, got.String(), tt.want)
			}
		})
	}
}
