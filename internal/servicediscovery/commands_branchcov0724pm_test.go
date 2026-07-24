package servicediscovery

import (
	"strings"
	"testing"
)

// TestBranchcov0724pmValidateResourceID covers the empty-ID arm (commands.go:16-18)
// and the too-long arm (commands.go:19-21) that the existing suite never reaches,
// plus boundary and rejection-reason assertions.
func TestBranchcov0724pmValidateResourceID(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr string // substring expected in the error message; "" means no error
	}{
		{name: "empty-rejected", id: "", wantErr: "cannot be empty"},
		{name: "too-long-257-rejected", id: strings.Repeat("a", 257), wantErr: "too long"},
		{name: "max-length-256-accepted", id: strings.Repeat("a", 256), wantErr: ""},
		{name: "simple-alphanumeric", id: "abc123", wantErr: ""},
		{name: "all-allowed-chars", id: "a-b_c.d:e", wantErr: ""},
		{name: "single-char", id: "1", wantErr: ""},
		{name: "starts-with-period", id: ".bad", wantErr: "invalid characters"},
		{name: "starts-with-colon", id: ":bad", wantErr: "invalid characters"},
		{name: "starts-with-underscore", id: "_bad", wantErr: "invalid characters"},
		{name: "contains-space", id: "a b", wantErr: "invalid characters"},
		{name: "contains-dollar", id: "a$b", wantErr: "invalid characters"},
		{name: "contains-semicolon", id: "a;b", wantErr: "invalid characters"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateResourceID(tc.id)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateResourceID(%q) returned unexpected error: %v", tc.id, err)
				}
			} else {
				if err == nil {
					t.Fatalf("ValidateResourceID(%q) returned nil, want error containing %q", tc.id, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ValidateResourceID(%q) error = %q, want substring %q", tc.id, err.Error(), tc.wantErr)
				}
			}
		})
	}
}

// TestBranchcov0724pmBuildLXCCommand covers the error arm (commands.go:344-347)
// that the existing suite never reaches, and asserts the exact happy-path string.
func TestBranchcov0724pmBuildLXCCommand(t *testing.T) {
	t.Run("valid-exact-string", func(t *testing.T) {
		got := BuildLXCCommand("101", "echo hi")
		want := "pct exec 101 -- sh -c 'echo hi'"
		if got != want {
			t.Fatalf("BuildLXCCommand =\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("invalid-vmid-returns-error-command", func(t *testing.T) {
		got := BuildLXCCommand("", "echo hi")
		want := "sh -c 'echo \"Discovery error: invalid container ID\" >&2; exit 1'"
		if got != want {
			t.Fatalf("BuildLXCCommand(invalid) =\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("option-like-vmid-returns-error-command", func(t *testing.T) {
		got := BuildLXCCommand("--malicious", "echo hi")
		want := "sh -c 'echo \"Discovery error: invalid container ID\" >&2; exit 1'"
		if got != want {
			t.Fatalf("BuildLXCCommand(option-like) =\n got: %q\nwant: %q", got, want)
		}
	})
}

// TestBranchcov0724pmBuildNestedDockerCommand covers the invalid-vmid arm
// (commands.go:377-379) and the invalid-container-name arm (commands.go:382-384)
// that the existing suite never reaches, plus the leading-slash trim.
func TestBranchcov0724pmBuildNestedDockerCommand(t *testing.T) {
	t.Run("lxc-nested-exact", func(t *testing.T) {
		got := BuildNestedDockerCommand("201", true, "web", "echo hi")
		// dockerCmd = docker exec 'web' sh -c 'echo hi'
		// That string is shell-quoted inside the pct exec wrapper.
		want := "pct exec 201 -- sh -c 'docker exec '\"'\"'web'\"'\"' sh -c '\"'\"'echo hi'\"'\"''"
		if got != want {
			t.Fatalf("BuildNestedDockerCommand(lxc) =\n got: %s\nwant: %s", got, want)
		}
	})

	t.Run("vm-nested-exact", func(t *testing.T) {
		got := BuildNestedDockerCommand("301", false, "web", "echo hi")
		want := "qm guest exec 301 -- sh -c 'docker exec '\"'\"'web'\"'\"' sh -c '\"'\"'echo hi'\"'\"''"
		if got != want {
			t.Fatalf("BuildNestedDockerCommand(vm) =\n got: %s\nwant: %s", got, want)
		}
	})

	t.Run("leading-slash-trimmed-from-container-name", func(t *testing.T) {
		// Docker API returns names with leading slash; it must be trimmed
		// before validation so "/web" and "web" produce the same command.
		withSlash := BuildNestedDockerCommand("201", true, "/web", "echo hi")
		noSlash := BuildNestedDockerCommand("201", true, "web", "echo hi")
		if withSlash != noSlash {
			t.Fatalf("leading slash not trimmed:\n withSlash: %s\n  noSlash: %s", withSlash, noSlash)
		}
	})

	t.Run("invalid-vmid-error", func(t *testing.T) {
		got := BuildNestedDockerCommand("", true, "web", "echo hi")
		want := "sh -c 'echo \"Discovery error: invalid VM/LXC ID\" >&2; exit 1'"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("invalid-container-name-error", func(t *testing.T) {
		got := BuildNestedDockerCommand("201", true, "-bad", "echo hi")
		want := "sh -c 'echo \"Discovery error: invalid container name\" >&2; exit 1'"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

// TestBranchcov0724pmBuildK8sCommand covers the invalid-pod-name arm
// (commands.go:398-400) and the invalid-container-name arm (commands.go:402-404)
// that the existing suite never reaches, and asserts exact command strings.
func TestBranchcov0724pmBuildK8sCommand(t *testing.T) {
	t.Run("with-container-exact", func(t *testing.T) {
		got := BuildK8sCommand("default", "pod1", "app", "echo hi")
		want := "kubectl exec -n 'default' 'pod1' -c 'app' -- sh -c 'echo hi'"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("without-container-exact", func(t *testing.T) {
		got := BuildK8sCommand("default", "pod1", "", "echo hi")
		want := "kubectl exec -n 'default' 'pod1' -- sh -c 'echo hi'"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("invalid-pod-name-error", func(t *testing.T) {
		got := BuildK8sCommand("default", "", "app", "echo hi")
		want := "sh -c 'echo \"Discovery error: invalid pod name\" >&2; exit 1'"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("invalid-container-name-error", func(t *testing.T) {
		got := BuildK8sCommand("default", "pod1", "-bad", "echo hi")
		want := "sh -c 'echo \"Discovery error: invalid container name\" >&2; exit 1'"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

// TestBranchcov0724pmFormatScopeHint covers the empty-summary arm
// (formatters.go:275-277) and the multi-discovery "+N more" arm
// (formatters.go:278-280) that the existing suite never reaches.
func TestBranchcov0724pmFormatScopeHint(t *testing.T) {
	t.Run("multiple-discoveries-adds-more-count", func(t *testing.T) {
		d1 := &ResourceDiscovery{ServiceName: "App1", ServiceType: "app"}
		d2 := &ResourceDiscovery{ServiceName: "App2", ServiceType: "app"}
		hint := FormatScopeHint([]*ResourceDiscovery{d1, d2})
		if !strings.Contains(hint, "Discovery:") {
			t.Fatalf("expected 'Discovery:' prefix, got %q", hint)
		}
		if !strings.Contains(hint, "(+1 more)") {
			t.Fatalf("expected '(+1 more)' for 2 discoveries, got %q", hint)
		}
	})

	t.Run("three-discoveries-shows-2-more", func(t *testing.T) {
		d1 := &ResourceDiscovery{ServiceName: "A"}
		d2 := &ResourceDiscovery{ServiceName: "B"}
		d3 := &ResourceDiscovery{ServiceName: "C"}
		hint := FormatScopeHint([]*ResourceDiscovery{d1, d2, d3})
		if !strings.Contains(hint, "(+2 more)") {
			t.Fatalf("expected '(+2 more)' for 3 discoveries, got %q", hint)
		}
	})

	t.Run("empty-identity-returns-empty", func(t *testing.T) {
		// A discovery with no identity fields produces an empty summary,
		// which FormatScopeHint must propagate as "".
		hint := FormatScopeHint([]*ResourceDiscovery{{}})
		if hint != "" {
			t.Fatalf("expected empty hint for identity-less discovery, got %q", hint)
		}
	})

	t.Run("nil-primary-in-slice-returns-empty", func(t *testing.T) {
		// A nil primary entry also yields an empty summary from
		// formatScopeDiscoverySummary.
		hint := FormatScopeHint([]*ResourceDiscovery{nil})
		if hint != "" {
			t.Fatalf("expected empty hint for nil primary discovery, got %q", hint)
		}
	})
}
