package unifiedresources

import "testing"

func TestResourcePolicyRedactedTextPreservesWhitespace(t *testing.T) {
	resource := Resource{Name: "private-host", Policy: &ResourcePolicy{Routing: ResourceRoutingPolicy{Redact: []ResourceRedactionHint{ResourceRedactionHostname}}}}
	for _, value := range []string{" ", "\n\n", " Summary ", "\n private-host \n"} {
		want := value
		if value == "\n private-host \n" {
			want = "\n " + ResourcePolicyRedactedLabel + " \n"
		}
		if got := ResourcePolicyRedactedText(value, resource); got != want {
			t.Errorf("redact(%q) = %q, want %q", value, got, want)
		}
		if got := ResourcePolicyRedactedText(value, Resource{}); got != value {
			t.Errorf("without policy: %q != %q", got, value)
		}
	}
}
