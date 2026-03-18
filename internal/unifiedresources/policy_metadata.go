package unifiedresources

import (
	"fmt"
	"sort"
	"strings"
)

// ResourceSensitivity classifies how carefully Pulse should handle resource data.
type ResourceSensitivity string

const (
	ResourceSensitivityPublic     ResourceSensitivity = "public"
	ResourceSensitivityInternal   ResourceSensitivity = "internal"
	ResourceSensitivitySensitive  ResourceSensitivity = "sensitive"
	ResourceSensitivityRestricted ResourceSensitivity = "restricted"
)

// ResourceRoutingScope expresses the default local-vs-cloud decision for a resource.
type ResourceRoutingScope string

const (
	ResourceRoutingScopeCloudSummary ResourceRoutingScope = "cloud-summary"
	ResourceRoutingScopeLocalFirst   ResourceRoutingScope = "local-first"
	ResourceRoutingScopeLocalOnly    ResourceRoutingScope = "local-only"
)

// ResourceRedactionHint describes identifiers that should be removed before cloud use.
type ResourceRedactionHint string

const (
	ResourceRedactionHostname   ResourceRedactionHint = "hostname"
	ResourceRedactionIPAddress  ResourceRedactionHint = "ip-address"
	ResourceRedactionPlatformID ResourceRedactionHint = "platform-id"
	ResourceRedactionAlias      ResourceRedactionHint = "alias"
	ResourceRedactionPath       ResourceRedactionHint = "path"
)

// ResourcePolicy captures the canonical policy posture for a resource.
type ResourcePolicy struct {
	Sensitivity ResourceSensitivity   `json:"sensitivity"`
	Routing     ResourceRoutingPolicy `json:"routing"`
}

// ResourceRoutingPolicy captures the current routing posture for AI-safe handling.
type ResourceRoutingPolicy struct {
	Scope                ResourceRoutingScope    `json:"scope"`
	AllowCloudSummary    bool                    `json:"allowCloudSummary"`
	AllowCloudRawSignals bool                    `json:"allowCloudRawSignals"`
	Redact               []ResourceRedactionHint `json:"redact,omitempty"`
}

// RefreshPolicyMetadata derives canonical sensitivity, routing, and AI-safe summary data.
func RefreshPolicyMetadata(resource *Resource) {
	if resource == nil {
		return
	}

	sensitivity := classifyResourceSensitivity(*resource)
	resource.Policy = &ResourcePolicy{
		Sensitivity: sensitivity,
		Routing:     buildResourceRoutingPolicy(*resource, sensitivity),
	}
	resource.AISafeSummary = buildAISafeSummary(*resource, sensitivity)
}

func classifyResourceSensitivity(resource Resource) ResourceSensitivity {
	tagSet := normalizedTagSet(resource.Tags)

	switch {
	case tagSet["public"]:
		return ResourceSensitivityPublic
	case tagSet["restricted"] || tagSet["customer-data"] || tagSet["customer_data"] ||
		tagSet["pii"] || tagSet["phi"] || tagSet["pci"] || tagSet["regulated"] ||
		tagSet["secret"] || tagSet["secrets"]:
		return ResourceSensitivityRestricted
	case tagSet["sensitive"] || tagSet["backup"] || tagSet["mail"] ||
		tagSet["storage"] || tagSet["database"] || tagSet["dataset"]:
		return ResourceSensitivitySensitive
	}

	if resource.PMG != nil {
		return ResourceSensitivityRestricted
	}

	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeVM,
		ResourceTypeSystemContainer,
		ResourceTypeAppContainer,
		ResourceTypePod,
		ResourceTypeK8sDeployment,
		ResourceTypeDockerService,
		ResourceTypeStorage,
		ResourceTypePBS,
		ResourceTypePhysicalDisk,
		ResourceTypeCeph:
		return ResourceSensitivitySensitive
	}

	if resource.TrueNAS != nil {
		return ResourceSensitivitySensitive
	}
	if resource.Agent != nil && (resource.Agent.Unraid != nil || resource.Agent.StorageRisk != nil) {
		return ResourceSensitivitySensitive
	}

	return ResourceSensitivityInternal
}

func buildResourceRoutingPolicy(resource Resource, sensitivity ResourceSensitivity) ResourceRoutingPolicy {
	policy := ResourceRoutingPolicy{
		AllowCloudRawSignals: false,
	}

	switch sensitivity {
	case ResourceSensitivityPublic, ResourceSensitivityInternal:
		policy.Scope = ResourceRoutingScopeCloudSummary
		policy.AllowCloudSummary = true
	case ResourceSensitivitySensitive:
		policy.Scope = ResourceRoutingScopeLocalFirst
		policy.AllowCloudSummary = true
	case ResourceSensitivityRestricted:
		policy.Scope = ResourceRoutingScopeLocalOnly
		policy.AllowCloudSummary = false
	default:
		policy.Scope = ResourceRoutingScopeLocalFirst
	}

	if sensitivity == ResourceSensitivitySensitive || sensitivity == ResourceSensitivityRestricted {
		if resourceHasHostname(resource) {
			policy.Redact = append(policy.Redact, ResourceRedactionHostname)
		}
		if len(resource.Identity.IPAddresses) > 0 {
			policy.Redact = append(policy.Redact, ResourceRedactionIPAddress)
		}
		if resourceHasPlatformIdentity(resource) {
			policy.Redact = append(policy.Redact, ResourceRedactionPlatformID, ResourceRedactionAlias)
		}
	}
	if resourceHasFilesystemPath(resource) {
		policy.Redact = append(policy.Redact, ResourceRedactionPath)
	}

	policy.Redact = uniqueRedactionHints(policy.Redact)
	return policy
}

func buildAISafeSummary(resource Resource, sensitivity ResourceSensitivity) string {
	parts := []string{
		fmt.Sprintf("%s resource", resourceSummaryType(resource)),
	}

	if status := strings.TrimSpace(string(resource.Status)); status != "" {
		parts = append(parts, "status "+status)
	}

	if len(resource.Sources) > 0 {
		sources := make([]string, 0, len(resource.Sources))
		for _, source := range resource.Sources {
			if trimmed := strings.TrimSpace(string(source)); trimmed != "" {
				sources = append(sources, trimmed)
			}
		}
		sort.Strings(sources)
		sources = uniqueTrimmed(sources...)
		if len(sources) > 0 {
			parts = append(parts, "sources "+strings.Join(sources, "+"))
		}
	}

	if resource.ParentID != nil && strings.TrimSpace(*resource.ParentID) != "" {
		parts = append(parts, "linked to parent resource")
	}
	if resource.ChildCount > 0 {
		parts = append(parts, fmt.Sprintf("%d child resources", resource.ChildCount))
	}
	if resource.IncidentCount > 0 {
		incidentText := fmt.Sprintf("%d incident", resource.IncidentCount)
		if resource.IncidentCount != 1 {
			incidentText += "s"
		}
		if severity := strings.TrimSpace(string(resource.IncidentSeverity)); severity != "" {
			incidentText += ", " + severity + " severity"
		}
		parts = append(parts, incidentText)
	}

	switch sensitivity {
	case ResourceSensitivitySensitive:
		parts = append(parts, "redacted for cloud summary")
	case ResourceSensitivityRestricted:
		parts = append(parts, "local-only context")
	}

	return strings.Join(parts, "; ")
}

func resourceSummaryType(resource Resource) string {
	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeAgent:
		return "agent"
	case ResourceTypeVM:
		return "virtual machine"
	case ResourceTypeSystemContainer:
		return "system container"
	case ResourceTypeAppContainer:
		return "application container"
	case ResourceTypeDockerService:
		return "docker service"
	case ResourceTypeK8sCluster:
		return "kubernetes cluster"
	case ResourceTypeK8sNode:
		return "kubernetes node"
	case ResourceTypePod:
		return "kubernetes pod"
	case ResourceTypeK8sDeployment:
		return "kubernetes deployment"
	case ResourceTypeStorage:
		return "storage"
	case ResourceTypePBS:
		return "backup server"
	case ResourceTypePMG:
		return "mail gateway"
	case ResourceTypeCeph:
		return "ceph cluster"
	case ResourceTypePhysicalDisk:
		return "physical disk"
	default:
		if trimmed := strings.TrimSpace(string(CanonicalResourceType(resource.Type))); trimmed != "" {
			return trimmed
		}
		return "resource"
	}
}

func normalizedTagSet(tags []string) map[string]bool {
	out := make(map[string]bool, len(tags))
	for _, tag := range tags {
		trimmed := strings.ToLower(strings.TrimSpace(tag))
		if trimmed == "" {
			continue
		}
		out[trimmed] = true
	}
	return out
}

func resourceHasHostname(resource Resource) bool {
	if canonicalHostname(resource) != "" {
		return true
	}
	return strings.TrimSpace(resource.Name) != ""
}

func resourceHasPlatformIdentity(resource Resource) bool {
	if resource.Canonical == nil {
		return false
	}
	if strings.TrimSpace(resource.Canonical.PlatformID) != "" || strings.TrimSpace(resource.Canonical.PrimaryID) != "" {
		return true
	}
	return len(resource.Canonical.Aliases) > 0
}

func resourceHasFilesystemPath(resource Resource) bool {
	if resource.Storage != nil && strings.TrimSpace(resource.Storage.Path) != "" {
		return true
	}
	return false
}

func uniqueRedactionHints(in []ResourceRedactionHint) []ResourceRedactionHint {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[ResourceRedactionHint]struct{}, len(in))
	out := make([]ResourceRedactionHint, 0, len(in))
	for _, hint := range in {
		if _, ok := seen[hint]; ok {
			continue
		}
		seen[hint] = struct{}{}
		out = append(out, hint)
	}
	return out
}
