package dockeragent

import (
	"strings"
	"testing"
)

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		wantReg  string
		wantRepo string
		wantTag  string
	}{
		{
			name:     "official image without tag",
			image:    "nginx",
			wantReg:  "registry-1.docker.io",
			wantRepo: "library/nginx",
			wantTag:  "latest",
		},
		{
			name:     "official image with tag",
			image:    "nginx:1.25",
			wantReg:  "registry-1.docker.io",
			wantRepo: "library/nginx",
			wantTag:  "1.25",
		},
		{
			name:     "docker hub with namespace",
			image:    "myrepo/myapp:v1",
			wantReg:  "registry-1.docker.io",
			wantRepo: "myrepo/myapp",
			wantTag:  "v1",
		},
		{
			name:     "docker hub with namespace no tag",
			image:    "linuxserver/plex",
			wantReg:  "registry-1.docker.io",
			wantRepo: "linuxserver/plex",
			wantTag:  "latest",
		},
		{
			name:     "ghcr.io image",
			image:    "ghcr.io/owner/repo:tag",
			wantReg:  "ghcr.io",
			wantRepo: "owner/repo",
			wantTag:  "tag",
		},
		{
			name:     "private registry with port",
			image:    "registry.example.com:5000/app:v2",
			wantReg:  "registry.example.com:5000",
			wantRepo: "app",
			wantTag:  "v2",
		},
		{
			name:     "localhost registry",
			image:    "localhost:5000/myimage:dev",
			wantReg:  "localhost:5000",
			wantRepo: "myimage",
			wantTag:  "dev",
		},
		{
			name:     "digest pinned image",
			image:    "nginx@sha256:abc123def456",
			wantReg:  "",
			wantRepo: "",
			wantTag:  "",
		},
		{
			name:     "lscr.io image",
			image:    "lscr.io/linuxserver/plex:latest",
			wantReg:  "lscr.io",
			wantRepo: "linuxserver/plex",
			wantTag:  "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReg, gotRepo, gotTag := parseImageReference(tt.image)
			if gotReg != tt.wantReg {
				t.Errorf("registry = %q, want %q", gotReg, tt.wantReg)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("repository = %q, want %q", gotRepo, tt.wantRepo)
			}
			if gotTag != tt.wantTag {
				t.Errorf("tag = %q, want %q", gotTag, tt.wantTag)
			}
		})
	}
}

func TestParseImageReference_ImageIDForms(t *testing.T) {
	hexImageID := strings.Repeat("a", 64)
	nonHexImageID := strings.Repeat("a", 63) + "g"

	tests := []struct {
		name     string
		image    string
		wantReg  string
		wantRepo string
		wantTag  string
	}{
		{
			name:     "sha256 digest string is skipped",
			image:    "sha256:" + strings.Repeat("a", 64),
			wantReg:  "",
			wantRepo: "",
			wantTag:  "",
		},
		{
			name:     "64-char hex image id is skipped",
			image:    hexImageID,
			wantReg:  "",
			wantRepo: "",
			wantTag:  "",
		},
		{
			name:     "64-char non-hex value is parsed as official image",
			image:    nonHexImageID,
			wantReg:  "registry-1.docker.io",
			wantRepo: "library/" + nonHexImageID,
			wantTag:  "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReg, gotRepo, gotTag := parseImageReference(tt.image)
			if gotReg != tt.wantReg {
				t.Errorf("registry = %q, want %q", gotReg, tt.wantReg)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("repository = %q, want %q", gotRepo, tt.wantRepo)
			}
			if gotTag != tt.wantTag {
				t.Errorf("tag = %q, want %q", gotTag, tt.wantTag)
			}
		})
	}
}

func TestRegistryChecker_DigestsDiffer(t *testing.T) {
	checker := &RegistryChecker{}

	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{
			name:    "same digest",
			current: "sha256:abc123",
			latest:  "sha256:abc123",
			want:    false,
		},
		{
			name:    "different digest",
			current: "sha256:abc123",
			latest:  "sha256:def456",
			want:    true,
		},
		{
			name:    "empty current",
			current: "",
			latest:  "sha256:abc123",
			want:    false,
		},
		{
			name:    "empty latest",
			current: "sha256:abc123",
			latest:  "",
			want:    false,
		},
		{
			name:    "both empty",
			current: "",
			latest:  "",
			want:    false,
		},
		{
			name:    "different with prefix normalization",
			current: "sha256:abc123",
			latest:  "abc123",
			want:    false, // Should match after normalization
		},
		{
			name:    "match second digest in list",
			current: "sha256:abc123",
			latest:  "def456,abc123",
			want:    false, // Should match one of them
		},
		{
			name:    "match first digest in list",
			current: "sha256:def456",
			latest:  "def456,abc123",
			want:    false, // Should match one of them
		},
		{
			name:    "no match in list",
			current: "sha256:xyz789",
			latest:  "def456,abc123",
			want:    true, // No match found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.digestsDiffer(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("digestsDiffer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestRegistryChecker_EnableDisable(t *testing.T) {
	checker := &RegistryChecker{enabled: true}

	if !checker.Enabled() {
		t.Error("Expected checker to be enabled initially")
	}

	checker.SetEnabled(false)
	if checker.Enabled() {
		t.Error("Expected checker to be disabled")
	}

	checker.SetEnabled(true)
	if !checker.Enabled() {
		t.Error("Expected checker to be enabled")
	}
}

func TestIsValidDigest(t *testing.T) {
	tests := []struct {
		digest string
		valid  bool
	}{
		{"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", true},
		{"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", true},
		{"sha256:short", false},
		{"md5:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4", false},
		{"", false},
		{"notadigest", false},
	}

	for _, tt := range tests {
		t.Run(tt.digest, func(t *testing.T) {
			got := isValidDigest(tt.digest)
			if got != tt.valid {
				t.Errorf("isValidDigest(%q) = %v, want %v", tt.digest, got, tt.valid)
			}
		})
	}
}
