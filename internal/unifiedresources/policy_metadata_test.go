package unifiedresources

import "testing"

func TestRefreshPolicyMetadata_ClassifiesRestrictedResources(t *testing.T) {
	resource := Resource{
		ID:     "vm-100",
		Name:   "customer-payments",
		Type:   ResourceTypeVM,
		Status: StatusOnline,
		Tags:   []string{"customer-data"},
		Identity: ResourceIdentity{
			Hostnames:   []string{"payments.internal"},
			IPAddresses: []string{"10.0.0.44"},
		},
		Canonical: &CanonicalIdentity{
			PlatformID: "payments.internal",
			PrimaryID:  "vm:100",
			Aliases:    []string{"vm-100"},
		},
	}

	RefreshPolicyMetadata(&resource)

	if resource.Policy == nil {
		t.Fatal("expected policy metadata")
	}
	if got := resource.Policy.Sensitivity; got != ResourceSensitivityRestricted {
		t.Fatalf("sensitivity = %q, want %q", got, ResourceSensitivityRestricted)
	}
	if got := resource.Policy.Routing.Scope; got != ResourceRoutingScopeLocalOnly {
		t.Fatalf("routing scope = %q, want %q", got, ResourceRoutingScopeLocalOnly)
	}
	if resource.Policy.Routing.AllowCloudSummary {
		t.Fatal("expected local-only resource to block cloud summary")
	}
	if !containsRedactionHint(resource.Policy.Routing.Redact, ResourceRedactionHostname) {
		t.Fatalf("expected hostname redaction hint, got %#v", resource.Policy.Routing.Redact)
	}
	if !containsRedactionHint(resource.Policy.Routing.Redact, ResourceRedactionIPAddress) {
		t.Fatalf("expected IP redaction hint, got %#v", resource.Policy.Routing.Redact)
	}
	if resource.AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary")
	}
	if resource.AISafeSummary == resource.Name {
		t.Fatalf("aiSafeSummary leaked raw name: %q", resource.AISafeSummary)
	}
}

func TestRefreshPolicyMetadata_ClassifiesInfrastructureAsInternal(t *testing.T) {
	resource := Resource{
		ID:     "agent-1",
		Name:   "pve-node",
		Type:   ResourceTypeAgent,
		Status: StatusOnline,
		Agent: &AgentData{
			Hostname: "pve-node",
		},
	}

	RefreshCanonicalIdentity(&resource)
	RefreshPolicyMetadata(&resource)

	if resource.Policy == nil {
		t.Fatal("expected policy metadata")
	}
	if got := resource.Policy.Sensitivity; got != ResourceSensitivityInternal {
		t.Fatalf("sensitivity = %q, want %q", got, ResourceSensitivityInternal)
	}
	if got := resource.Policy.Routing.Scope; got != ResourceRoutingScopeCloudSummary {
		t.Fatalf("routing scope = %q, want %q", got, ResourceRoutingScopeCloudSummary)
	}
	if !resource.Policy.Routing.AllowCloudSummary {
		t.Fatal("expected internal resource to allow cloud summary")
	}
	if resource.Policy.Routing.AllowCloudRawSignals {
		t.Fatal("expected raw signals to remain local")
	}
}

func TestRefreshCanonicalMetadata(t *testing.T) {
	resource := Resource{
		ID:     "agent-1",
		Name:   "pve-node",
		Type:   ResourceTypeAgent,
		Status: StatusOnline,
		Agent: &AgentData{
			Hostname: "pve-node",
		},
	}

	RefreshCanonicalMetadata(&resource)

	if resource.Canonical == nil {
		t.Fatal("expected canonical identity metadata")
	}
	if resource.Policy == nil {
		t.Fatal("expected policy metadata")
	}
	if resource.AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary")
	}
}

func TestRefreshCanonicalMetadataSlice(t *testing.T) {
	input := []Resource{
		{
			ID:     "agent-1",
			Name:   "pve-node",
			Type:   ResourceTypeAgent,
			Status: StatusOnline,
			Agent: &AgentData{
				Hostname: "pve-node",
			},
		},
	}

	out := RefreshCanonicalMetadataSlice(input)

	if input[0].Canonical != nil {
		t.Fatal("expected input slice to remain unmodified")
	}
	if len(out) != 1 {
		t.Fatalf("expected one resource, got %d", len(out))
	}
	if out[0].Canonical == nil {
		t.Fatal("expected canonical identity metadata in output slice")
	}
	if out[0].Policy == nil {
		t.Fatal("expected policy metadata in output slice")
	}
	if out[0].AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary in output slice")
	}
}

func TestResourcePolicyPresentationLabels(t *testing.T) {
	if got := ResourceSensitivityLabel(ResourceSensitivityPublic); got != "Public" {
		t.Fatalf("sensitivity label = %q, want %q", got, "Public")
	}
	if got := ResourceRoutingScopeLabel(ResourceRoutingScopeLocalOnly); got != "Local Only" {
		t.Fatalf("routing label = %q, want %q", got, "Local Only")
	}
	if got := ResourceRedactionHintLabel(ResourceRedactionIPAddress); got != "IP Address" {
		t.Fatalf("redaction label = %q, want %q", got, "IP Address")
	}

	policy := &ResourcePolicy{
		Routing: ResourceRoutingPolicy{
			Redact: []ResourceRedactionHint{
				ResourceRedactionPath,
				ResourceRedactionHostname,
			},
		},
	}
	got := ResourcePolicyRedactionLabels(policy)
	if len(got) != 2 || got[0] != "Hostname" || got[1] != "Path" {
		t.Fatalf("redaction labels = %#v, want [Hostname Path]", got)
	}
}

func TestResourceClusterName(t *testing.T) {
	public := Resource{
		Identity: ResourceIdentity{
			ClusterName: "  cluster-a  ",
		},
	}
	if got := ResourceClusterName(public); got != "cluster-a" {
		t.Fatalf("ResourceClusterName() = %q, want cluster-a", got)
	}

	governed := Resource{
		Policy: &ResourcePolicy{
			Routing: ResourceRoutingPolicy{
				Redact: []ResourceRedactionHint{ResourceRedactionHostname},
			},
		},
		Kubernetes: &K8sData{
			ClusterName: "k8s-prod",
		},
	}
	if got := ResourceClusterName(governed); got != ResourcePolicyRedactedLabel {
		t.Fatalf("ResourceClusterName() with governed policy = %q, want redacted label", got)
	}
}

func TestResourceIPSummary(t *testing.T) {
	public := Resource{
		Identity: ResourceIdentity{
			IPAddresses: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		},
	}
	if got := ResourceIPSummary(public, 2); got != " - IPs 10.0.0.1, 10.0.0.2" {
		t.Fatalf("ResourceIPSummary() = %q, want truncated IP list", got)
	}

	governed := Resource{
		Policy: &ResourcePolicy{
			Routing: ResourceRoutingPolicy{
				Redact: []ResourceRedactionHint{ResourceRedactionIPAddress},
			},
		},
		Identity: ResourceIdentity{
			IPAddresses: []string{"10.0.0.10", "10.0.0.11"},
		},
	}
	if got := ResourceIPSummary(governed, 10); got != " - IPs redacted by policy" {
		t.Fatalf("ResourceIPSummary() with governed policy = %q, want redacted summary", got)
	}
}

func TestResourcePolicyRedactionLabelsUseCanonicalOrder(t *testing.T) {
	policy := &ResourcePolicy{
		Routing: ResourceRoutingPolicy{
			Redact: []ResourceRedactionHint{
				ResourceRedactionAlias,
				ResourceRedactionPath,
				ResourceRedactionHostname,
				ResourceRedactionPlatformID,
			},
		},
	}

	got := ResourcePolicyRedactionLabels(policy)
	want := []string{"Hostname", "Platform ID", "Alias", "Path"}
	if len(got) != len(want) {
		t.Fatalf("redaction labels = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("redaction labels = %#v, want %#v", got, want)
		}
	}
}

func TestResourcePolicyCountSummariesUseCanonicalOrder(t *testing.T) {
	sensitivityCounts := map[ResourceSensitivity]int{
		ResourceSensitivityRestricted: 1,
		ResourceSensitivityPublic:     2,
		ResourceSensitivitySensitive:  3,
		ResourceSensitivityInternal:   4,
	}
	if got := ResourcePolicySensitivitySummaryFromCounts(sensitivityCounts); len(got) != 4 || got[0] != "2 Public" || got[1] != "4 Internal" || got[2] != "3 Sensitive" || got[3] != "1 Restricted" {
		t.Fatalf("sensitivity summary = %#v, want [2 Public 4 Internal 3 Sensitive 1 Restricted]", got)
	}

	routingCounts := map[ResourceRoutingScope]int{
		ResourceRoutingScopeLocalOnly:    5,
		ResourceRoutingScopeCloudSummary: 6,
		ResourceRoutingScopeLocalFirst:   7,
	}
	if got := ResourcePolicyRoutingSummaryFromCounts(routingCounts); len(got) != 3 || got[0] != "6 Cloud Summary" || got[1] != "7 Local First" || got[2] != "5 Local Only" {
		t.Fatalf("routing summary = %#v, want [6 Cloud Summary 7 Local First 5 Local Only]", got)
	}
}

func TestResourcePolicySummaryLines(t *testing.T) {
	policy := &ResourcePolicy{
		Sensitivity: ResourceSensitivityRestricted,
		Routing: ResourceRoutingPolicy{
			Scope:                ResourceRoutingScopeLocalOnly,
			AllowCloudSummary:    false,
			AllowCloudRawSignals: false,
			Redact: []ResourceRedactionHint{
				ResourceRedactionAlias,
				ResourceRedactionHostname,
				ResourceRedactionPath,
			},
		},
	}

	got := ResourcePolicySummaryLines(policy)
	want := []string{
		"Policy: sensitivity=Restricted, routing=Local Only, cloud_summary=false, cloud_raw_signals=false",
		"Redactions: Hostname, Alias, Path",
	}
	if len(got) != len(want) {
		t.Fatalf("summary lines = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("summary line[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResourcePolicyGovernedSummaryGuidance(t *testing.T) {
	if got := ResourcePolicyGovernedSummaryPreamble(); got != "Raw hostnames, paths, and local identifiers are withheld when governed resource policy requires redaction." {
		t.Fatalf("ResourcePolicyGovernedSummaryPreamble() = %q", got)
	}
	if got := ResourcePolicyGovernedSummaryFooter(); got != "Raw routing coordinates, bind mounts, hostnames, and discovery file paths withheld by canonical resource policy." {
		t.Fatalf("ResourcePolicyGovernedSummaryFooter() = %q", got)
	}
}

func TestFormatResourcePolicyGovernedSummary(t *testing.T) {
	policy := &ResourcePolicy{
		Sensitivity: ResourceSensitivityRestricted,
		Routing: ResourceRoutingPolicy{
			Scope: ResourceRoutingScopeLocalOnly,
			Redact: []ResourceRedactionHint{
				ResourceRedactionHostname,
				ResourceRedactionAlias,
			},
		},
	}

	got := FormatResourcePolicyGovernedSummary("system container resource; status online; local-only context", policy)
	want := "## Governed resource\nsystem container resource; status online; local-only context\nPolicy: sensitivity=Restricted, routing=Local Only, cloud_summary=false, cloud_raw_signals=false\nRedactions: Hostname, Alias\nRaw routing coordinates, bind mounts, hostnames, and discovery file paths withheld by canonical resource policy.\n\n"
	if got != want {
		t.Fatalf("FormatResourcePolicyGovernedSummary() = %q, want %q", got, want)
	}
}

func TestResourcePolicyRedactsAndUsesAISafeSummary(t *testing.T) {
	policy := &ResourcePolicy{
		Sensitivity: ResourceSensitivitySensitive,
		Routing: ResourceRoutingPolicy{
			Scope:                ResourceRoutingScopeLocalFirst,
			AllowCloudSummary:    true,
			AllowCloudRawSignals: false,
			Redact: []ResourceRedactionHint{
				ResourceRedactionHostname,
				ResourceRedactionIPAddress,
			},
		},
	}

	if !ResourcePolicyRedacts(policy, ResourceRedactionHostname) {
		t.Fatal("expected hostname to be redacted")
	}
	if ResourcePolicyRedacts(policy, ResourceRedactionAlias) {
		t.Fatal("did not expect alias to be redacted")
	}
	if !ResourcePolicyUsesAISafeSummary("resource summary safe for remote AI use", policy) {
		t.Fatal("expected AI-safe summary to be used")
	}
	if ResourcePolicyUsesAISafeSummary("", policy) {
		t.Fatal("did not expect empty summary to be used")
	}
	if ResourcePolicyUsesAISafeSummary("resource summary safe for remote AI use", nil) {
		t.Fatal("did not expect nil policy to use AI-safe summary")
	}
}

func TestResourcePolicyRequiresGovernedSummary(t *testing.T) {
	if ResourcePolicyRequiresGovernedSummary(nil) {
		t.Fatal("expected nil policy to not require governed summary")
	}

	if ResourcePolicyRequiresGovernedSummary(&ResourcePolicy{}) {
		t.Fatal("expected empty policy to not require governed summary")
	}

	if !ResourcePolicyRequiresGovernedSummary(&ResourcePolicy{
		Routing: ResourceRoutingPolicy{
			Scope: ResourceRoutingScopeLocalOnly,
		},
	}) {
		t.Fatal("expected local-only policy to require governed summary")
	}

	if !ResourcePolicyRequiresGovernedSummary(&ResourcePolicy{
		Routing: ResourceRoutingPolicy{
			Scope:  ResourceRoutingScopeLocalFirst,
			Redact: []ResourceRedactionHint{ResourceRedactionHostname},
		},
	}) {
		t.Fatal("expected redacted policy to require governed summary")
	}
}

func TestResourcePolicyLabelHelpers(t *testing.T) {
	policy := &ResourcePolicy{
		Sensitivity: ResourceSensitivityRestricted,
		Routing: ResourceRoutingPolicy{
			Scope: ResourceRoutingScopeLocalOnly,
			Redact: []ResourceRedactionHint{
				ResourceRedactionHostname,
				ResourceRedactionAlias,
			},
		},
	}

	if got := ResourcePolicyLabel(" fallback-name ", " governed summary ", policy); got != "governed summary" {
		t.Fatalf("ResourcePolicyLabel() = %q, want governed summary", got)
	}
	if got := ResourcePolicyLabel(" fallback-name ", " governed summary ", nil); got != "fallback-name" {
		t.Fatalf("ResourcePolicyLabel() with nil policy = %q, want fallback-name", got)
	}
	if got := ResourcePolicyRedactedValue(" host-01 ", policy, ResourceRedactionHostname, ResourceRedactionAlias); got != ResourcePolicyRedactedLabel {
		t.Fatalf("ResourcePolicyRedactedValue() = %q, want %q", got, ResourcePolicyRedactedLabel)
	}
	if got := ResourcePolicyRedactedValue(" host-01 ", nil, ResourceRedactionHostname); got != "host-01" {
		t.Fatalf("ResourcePolicyRedactedValue() with nil policy = %q, want host-01", got)
	}
}

func TestResourceRedactionLabelsFromHints(t *testing.T) {
	got := ResourceRedactionLabelsFromHints([]ResourceRedactionHint{
		ResourceRedactionPath,
		ResourceRedactionHostname,
		ResourceRedactionHostname,
		ResourceRedactionAlias,
	})
	want := []string{"Hostname", "Alias", "Path"}
	if len(got) != len(want) {
		t.Fatalf("labels = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("labels[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResourceDisplayName(t *testing.T) {
	if got := ResourceDisplayName(Resource{
		Name: "  node-a  ",
		ID:   "id-a",
		Canonical: &CanonicalIdentity{
			DisplayName: "canonical-node-a",
		},
	}); got != "canonical-node-a" {
		t.Fatalf("ResourceDisplayName() with canonical name = %q, want canonical-node-a", got)
	}
	if got := ResourceDisplayName(Resource{Name: "  node-a  ", ID: "id-a"}); got != "node-a" {
		t.Fatalf("ResourceDisplayName() with name = %q, want node-a", got)
	}
	if got := ResourceDisplayName(Resource{Name: "   ", ID: "  id-b  "}); got != "id-b" {
		t.Fatalf("ResourceDisplayName() with fallback ID = %q, want id-b", got)
	}
	if got := ResourceDisplayName(Resource{}); got != "" {
		t.Fatalf("ResourceDisplayName() empty resource = %q, want empty string", got)
	}
}

func TestCloneResourcePolicy(t *testing.T) {
	t.Parallel()

	original := &ResourcePolicy{
		Sensitivity: ResourceSensitivityRestricted,
		Routing: ResourceRoutingPolicy{
			Scope:                ResourceRoutingScopeLocalOnly,
			AllowCloudSummary:    false,
			AllowCloudRawSignals: false,
			Redact: []ResourceRedactionHint{
				ResourceRedactionHostname,
				ResourceRedactionIPAddress,
			},
		},
	}

	clone := CloneResourcePolicy(original)
	if clone == nil {
		t.Fatal("expected clone")
	}
	if clone == original {
		t.Fatal("expected clone to allocate a new policy")
	}
	if clone.Sensitivity != original.Sensitivity || clone.Routing.Scope != original.Routing.Scope {
		t.Fatalf("clone does not match original: %#v vs %#v", clone, original)
	}
	if len(clone.Routing.Redact) != len(original.Routing.Redact) {
		t.Fatalf("clone redactions = %#v, want %#v", clone.Routing.Redact, original.Routing.Redact)
	}
	clone.Routing.Redact[0] = ResourceRedactionPath
	if original.Routing.Redact[0] != ResourceRedactionHostname {
		t.Fatalf("expected clone to deep copy redactions, original mutated to %#v", original.Routing.Redact)
	}
	if CloneResourcePolicy(nil) != nil {
		t.Fatal("expected nil clone for nil policy")
	}
}

func containsRedactionHint(hints []ResourceRedactionHint, want ResourceRedactionHint) bool {
	for _, hint := range hints {
		if hint == want {
			return true
		}
	}
	return false
}
