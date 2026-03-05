package unifiedresources

import "testing"

func TestCanonicalResourceTypeDoesNotAliasHost(t *testing.T) {
	if got := CanonicalResourceType(ResourceType("host")); got != ResourceType("host") {
		t.Fatalf("CanonicalResourceType(host) = %q, want %q", got, ResourceType("host"))
	}

	if got := CanonicalResourceType(ResourceType("HOST")); got != ResourceType("host") {
		t.Fatalf("CanonicalResourceType(HOST) = %q, want %q", got, ResourceType("host"))
	}

	if got := CanonicalResourceType(ResourceType("agent")); got != ResourceTypeAgent {
		t.Fatalf("CanonicalResourceType(agent) = %q, want %q", got, ResourceTypeAgent)
	}
}

func TestCanonicalResourceIDDoesNotAliasLegacyHostPrefixes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "host colon prefix remains unchanged",
			in:   "host:alpha",
			want: "host:alpha",
		},
		{
			name: "host dash prefix remains unchanged",
			in:   "host-alpha",
			want: "host-alpha",
		},
		{
			name: "agent prefix unchanged",
			in:   "agent:alpha",
			want: "agent:alpha",
		},
		{
			name: "trims surrounding whitespace only",
			in:   "  host:trim-me  ",
			want: "host:trim-me",
		},
		{
			name: "empty becomes empty",
			in:   "   ",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanonicalResourceID(tc.in); got != tc.want {
				t.Fatalf("CanonicalResourceID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsUnsupportedLegacyResourceTypeAlias(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "host alias", in: "host", want: true},
		{name: "host mixed case alias", in: " HoSt ", want: true},
		{name: "legacy system_container alias", in: "system_container", want: true},
		{name: "legacy docker_container alias", in: "docker_container", want: true},
		{name: "legacy app_container alias", in: "app_container", want: true},
		{name: "legacy docker_host alias", in: "docker_host", want: true},
		{name: "legacy kubernetes_cluster alias", in: "kubernetes_cluster", want: true},
		{name: "legacy k8s_cluster alias", in: "k8s_cluster", want: true},
		{name: "agent type", in: "agent", want: false},
		{name: "empty", in: "  ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnsupportedLegacyResourceTypeAlias(tt.in); got != tt.want {
				t.Fatalf("IsUnsupportedLegacyResourceTypeAlias(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalizeLegacyResourceTypeAlias(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "host", in: "host", want: "agent", ok: true},
		{name: "system_container", in: "system_container", want: "system-container", ok: true},
		{name: "docker_container", in: "docker_container", want: "app-container", ok: true},
		{name: "app_container", in: "app_container", want: "app-container", ok: true},
		{name: "docker_host", in: "docker_host", want: "docker-host", ok: true},
		{name: "kubernetes_cluster", in: "kubernetes_cluster", want: "k8s-cluster", ok: true},
		{name: "k8s_cluster", in: "k8s_cluster", want: "k8s-cluster", ok: true},
		{name: "canonical_passthrough_rejected", in: "agent", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CanonicalizeLegacyResourceTypeAlias(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("CanonicalizeLegacyResourceTypeAlias(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestIsUnsupportedLegacyResourceIDAlias(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "host prefixed id", in: "host:alpha", want: true},
		{name: "host mixed case prefixed id", in: " HoSt:alpha ", want: true},
		{name: "agent id", in: "agent:alpha", want: false},
		{name: "host without colon", in: "host-alpha", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnsupportedLegacyResourceIDAlias(tt.in); got != tt.want {
				t.Fatalf("IsUnsupportedLegacyResourceIDAlias(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
