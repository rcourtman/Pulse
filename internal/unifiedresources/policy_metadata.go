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
	Scope  ResourceRoutingScope    `json:"scope"`
	Redact []ResourceRedactionHint `json:"redact,omitempty"`
}

// CloneResourcePolicy returns a deep copy of the canonical resource policy.
func CloneResourcePolicy(policy *ResourcePolicy) *ResourcePolicy {
	if policy == nil {
		return nil
	}
	copyPolicy := *policy
	copyPolicy.Routing.Redact = append([]ResourceRedactionHint(nil), policy.Routing.Redact...)
	return &copyPolicy
}

// CanonicalGovernanceMetadata returns cloned policy metadata and AI-safe
// summary for a resolved resource, tolerating callers that only have routing
// location data when a full Resource record is not available.
func CanonicalGovernanceMetadata(resource *Resource) (*ResourcePolicy, string) {
	if resource == nil {
		return nil, ""
	}

	resourceCopy := *resource
	RefreshPolicyMetadata(&resourceCopy)
	return CloneResourcePolicy(resourceCopy.Policy), strings.TrimSpace(resourceCopy.AISafeSummary)
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

// RefreshCanonicalMetadata derives the canonical identity and policy metadata
// for a resource in one pass.
func RefreshCanonicalMetadata(resource *Resource) {
	if resource == nil {
		return
	}

	RefreshCanonicalIdentity(resource)
	RefreshPlatformScopes(resource)
	RefreshPolicyMetadata(resource)
}

// RefreshCanonicalMetadataSlice returns a cloned slice with canonical identity
// and policy metadata refreshed for each resource.
func RefreshCanonicalMetadataSlice(resources []Resource) []Resource {
	if len(resources) == 0 {
		return resources
	}

	out := make([]Resource, len(resources))
	for i, resource := range resources {
		out[i] = resource
		RefreshCanonicalMetadata(&out[i])
	}
	return out
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
		ResourceTypeK8sReplicaSet,
		ResourceTypeK8sNamespace,
		ResourceTypeK8sService,
		ResourceTypeK8sStatefulSet,
		ResourceTypeK8sDaemonSet,
		ResourceTypeK8sJob,
		ResourceTypeK8sCronJob,
		ResourceTypeK8sIngress,
		ResourceTypeK8sEndpointSlice,
		ResourceTypeK8sNetworkPolicy,
		ResourceTypeK8sPV,
		ResourceTypeK8sPVC,
		ResourceTypeK8sStorageClass,
		ResourceTypeK8sConfigMap,
		ResourceTypeK8sServiceAccount,
		ResourceTypeK8sEvent,
		ResourceTypeDockerService,
		ResourceTypeStorage,
		ResourceTypeNetwork,
		ResourceTypePBS,
		ResourceTypePhysicalDisk,
		ResourceTypeCeph,
		ResourceTypeNetworkShare:
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
	policy := ResourceRoutingPolicy{}

	switch sensitivity {
	case ResourceSensitivityPublic, ResourceSensitivityInternal:
		policy.Scope = ResourceRoutingScopeCloudSummary
	case ResourceSensitivitySensitive:
		policy.Scope = ResourceRoutingScopeLocalFirst
	case ResourceSensitivityRestricted:
		policy.Scope = ResourceRoutingScopeLocalOnly
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
		parts = append(parts, resourceAISafeSummaryPolicySuffix(sensitivity))
	case ResourceSensitivityRestricted:
		parts = append(parts, resourceAISafeSummaryPolicySuffix(sensitivity))
	}

	return strings.Join(parts, "; ")
}

func resourceAISafeSummaryPolicySuffix(sensitivity ResourceSensitivity) string {
	switch sensitivity {
	case ResourceSensitivitySensitive:
		return "redacted for cloud summary"
	case ResourceSensitivityRestricted:
		return "local-only context"
	default:
		return ""
	}
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
	case ResourceTypeK8sReplicaSet:
		return "kubernetes replicaset"
	case ResourceTypeK8sNamespace:
		return "kubernetes namespace"
	case ResourceTypeK8sService:
		return "kubernetes service"
	case ResourceTypeK8sStatefulSet:
		return "kubernetes statefulset"
	case ResourceTypeK8sDaemonSet:
		return "kubernetes daemonset"
	case ResourceTypeK8sJob:
		return "kubernetes job"
	case ResourceTypeK8sCronJob:
		return "kubernetes cronjob"
	case ResourceTypeK8sIngress:
		return "kubernetes ingress"
	case ResourceTypeK8sEndpointSlice:
		return "kubernetes endpointslice"
	case ResourceTypeK8sNetworkPolicy:
		return "kubernetes network policy"
	case ResourceTypeK8sPV:
		return "kubernetes persistent volume"
	case ResourceTypeK8sPVC:
		return "kubernetes persistent volume claim"
	case ResourceTypeK8sStorageClass:
		return "kubernetes storage class"
	case ResourceTypeK8sConfigMap:
		return "kubernetes configmap"
	case ResourceTypeK8sServiceAccount:
		return "kubernetes serviceaccount"
	case ResourceTypeK8sEvent:
		return "kubernetes event"
	case ResourceTypeStorage:
		return "storage"
	case ResourceTypeNetwork:
		return "network"
	case ResourceTypePBS:
		return "backup server"
	case ResourceTypePMG:
		return "mail gateway"
	case ResourceTypeCeph:
		return "ceph cluster"
	case ResourceTypePhysicalDisk:
		return "physical disk"
	case ResourceTypeNetworkShare:
		return "network share"
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
	if resource.TrueNAS != nil && resource.TrueNAS.Share != nil && strings.TrimSpace(resource.TrueNAS.Share.Path) != "" {
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
