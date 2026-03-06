package servicediscovery

import (
	"strings"
	"testing"
)

func TestResourceIDHelpers(t *testing.T) {
	id := MakeResourceID(ResourceTypeDocker, "host1", "app")
	if id != "docker:host1:app" {
		t.Fatalf("unexpected id: %s", id)
	}

	rt, host, res, err := ParseResourceID(id)
	if err != nil {
		t.Fatalf("ParseResourceID error: %v", err)
	}
	if rt != ResourceTypeDocker || host != "host1" || res != "app" {
		t.Fatalf("unexpected parse result: %s %s %s", rt, host, res)
	}

	if _, _, _, err := ParseResourceID("invalid"); err == nil {
		t.Fatalf("expected parse error for invalid id")
	}
}

func TestResourceTypeClassificationAndValidation(t *testing.T) {
	t.Run("canonical discovery types validate", func(t *testing.T) {
		cases := []ResourceType{
			ResourceTypeVM,
			ResourceTypeSystemContainer,
			ResourceTypeDocker,
			ResourceTypeK8s,
			ResourceTypeAgent,
		}

		for _, tc := range cases {
			if !tc.IsCanonicalDiscoveryType() {
				t.Fatalf("expected %q to be canonical", tc)
			}
			if err := ValidateCanonicalDiscoveryResourceType(tc); err != nil {
				t.Fatalf("ValidateCanonicalDiscoveryResourceType(%q) error = %v", tc, err)
			}
		}
	})

	t.Run("execution-only types are classified but rejected", func(t *testing.T) {
		cases := []ResourceType{
			ResourceTypeDockerVM,
			ResourceTypeDockerSystemContainer,
		}

		for _, tc := range cases {
			if !tc.IsExecutionOnlyDiscoveryType() {
				t.Fatalf("expected %q to be execution-only", tc)
			}
			err := ValidateCanonicalDiscoveryResourceType(tc)
			if err == nil || !strings.Contains(err.Error(), "execution-only") {
				t.Fatalf("expected execution-only validation error for %q, got %v", tc, err)
			}
		}
	})

	t.Run("legacy aliases remain non-canonical", func(t *testing.T) {
		cases := []ResourceType{
			legacyResourceTypeHost,
			legacyResourceTypeLXC,
			legacyResourceTypeDockerLXC,
		}

		for _, tc := range cases {
			if tc.IsCanonicalDiscoveryType() {
				t.Fatalf("expected %q to remain non-canonical", tc)
			}
			err := ValidateCanonicalDiscoveryResourceType(tc)
			if err == nil || !strings.Contains(err.Error(), "legacy alias") {
				t.Fatalf("expected legacy alias validation error for %q, got %v", tc, err)
			}
		}
	})
}
