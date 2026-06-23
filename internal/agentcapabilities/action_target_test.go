package agentcapabilities

import (
	"strings"
	"testing"
)

func TestNormalizeAndValidateOptionalActionTargetTypeUsesSharedVocabulary(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "empty allowed", in: "", want: ""},
		{name: "agent", in: " AGENT ", want: ActionTargetTypeAgent},
		{name: "system container", in: "SYSTEM-CONTAINER", want: ActionTargetTypeSystemContainer},
		{name: "vm", in: "vm", want: ActionTargetTypeVM},
		{name: "legacy host rejected", in: "host", wantErr: "invalid target_type"},
		{name: "legacy container rejected", in: "container", wantErr: "invalid target_type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeAndValidateOptionalActionTargetType(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizeAndValidateOptionalActionTargetType(%q) error = %v, want substring %q", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeAndValidateOptionalActionTargetType(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeAndValidateOptionalActionTargetType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestActionTargetTypeForResourceTypeMapsActionResources(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "vm", in: "vm", want: ActionTargetTypeVM},
		{name: "system container", in: "system-container", want: ActionTargetTypeSystemContainer},
		{name: "oci container", in: "oci-container", want: ActionTargetTypeSystemContainer},
		{name: "app container", in: "app-container", want: ActionTargetTypeAgent},
		{name: "pod", in: "pod", want: ActionTargetTypeAgent},
		{name: "k8s deployment", in: "k8s-deployment", want: ActionTargetTypeAgent},
		{name: "node", in: "node", want: ActionTargetTypeAgent},
		{name: "truenas alias", in: "truenas", want: ActionTargetTypeAgent},
		{name: "legacy host rejected", in: "host", wantErr: "unsupported resource_type"},
		{name: "missing type", in: "", wantErr: "resource_type is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ActionTargetTypeForResourceType(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ActionTargetTypeForResourceType(%q) error = %v, want substring %q", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ActionTargetTypeForResourceType(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ActionTargetTypeForResourceType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestActionTargetTypeAllowedDescriptionMatchesStableAPIText(t *testing.T) {
	if got := ActionTargetTypeAllowedDescription(); got != "agent, system-container, vm" {
		t.Fatalf("ActionTargetTypeAllowedDescription() = %q", got)
	}
}
