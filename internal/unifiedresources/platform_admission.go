package unifiedresources

// Platform admission answers one question for the app shell: which primary
// platform pages exist for this estate. The frontend used to derive it by
// classifying every resource in the legacy full-state payload, which is why the
// shell had to download that payload before it could render its navigation.
// Deriving it here keeps a single definition of admission and lets the shell
// read it from the canonical resource aggregations instead.
//
// Counts cannot answer it. A TrueNAS or Proxmox host reports through the agent
// source and carries the "agent" platform scope, but it is owned by its
// provider and must not admit the standalone page on its own; only a genuine
// Pulse agent does. That distinction is per-resource evidence, so it is
// evaluated per resource here rather than inferred from a source tally.

// PlatformAdmission reports which primary platform pages an estate admits.
type PlatformAdmission struct {
	Proxmox    bool `json:"proxmox"`
	Docker     bool `json:"docker"`
	Kubernetes bool `json:"kubernetes"`
	TrueNAS    bool `json:"truenas"`
	VMware     bool `json:"vmware"`
	Standalone bool `json:"standalone"`
}

// agentProviderOwnerPlatformScopes are the platform scopes that mean an
// agent-typed resource belongs to a provider's platform page rather than the
// standalone one.
var agentProviderOwnerPlatformScopes = map[string]struct{}{
	"proxmox-pve":    {},
	"proxmox-pbs":    {},
	"proxmox-pmg":    {},
	"kubernetes":     {},
	"truenas":        {},
	"vmware-vsphere": {},
}

// platformScopesForResource returns the canonical platform scopes for a
// resource, deriving them when the resource has not been refreshed yet so that
// admission never depends on refresh ordering.
func platformScopesForResource(resource Resource) []string {
	if len(resource.PlatformScopes) > 0 {
		return resource.PlatformScopes
	}
	clone := resource
	RefreshPlatformScopes(&clone)
	return clone.PlatformScopes
}

func hasProviderOwnerPlatformEvidence(resource Resource, scopes []string) bool {
	for _, scope := range scopes {
		if _, ok := agentProviderOwnerPlatformScopes[scope]; ok {
			return true
		}
	}
	for _, source := range resource.Sources {
		if _, ok := agentProviderOwnerPlatformScopes[platformScopeForSource(source)]; ok {
			return true
		}
	}
	for source := range resource.SourceStatus {
		if _, ok := agentProviderOwnerPlatformScopes[platformScopeForSource(source)]; ok {
			return true
		}
	}
	return false
}

func hasPulseAgentSourceEvidence(resource Resource) bool {
	if hasDataSource(resource.Sources, SourceAgent) {
		return true
	}
	_, ok := resource.SourceStatus[SourceAgent]
	return ok
}

// IsPulseAgentPlatformResource reports whether a resource is a Pulse-managed
// host in its own right, rather than a host surfaced through the provider that
// owns it. Only these admit the standalone platform page.
func IsPulseAgentPlatformResource(resource Resource) bool {
	if CanonicalResourceType(resource.Type) != ResourceTypeAgent {
		return false
	}
	if hasProviderOwnerPlatformEvidence(resource, platformScopesForResource(resource)) {
		return false
	}
	return hasPulseAgentSourceEvidence(resource)
}

// BuildPlatformAdmission reports which primary platform pages the given
// resources admit.
func BuildPlatformAdmission(resources []Resource) PlatformAdmission {
	admission := PlatformAdmission{}
	for _, resource := range resources {
		for _, scope := range platformScopesForResource(resource) {
			switch scope {
			case "proxmox-pve", "proxmox-pbs", "proxmox-pmg":
				admission.Proxmox = true
			case "docker":
				admission.Docker = true
			case "kubernetes":
				admission.Kubernetes = true
			case "truenas":
				admission.TrueNAS = true
			case "vmware-vsphere":
				admission.VMware = true
			case "availability":
				// Availability endpoints are operated from the standalone page.
				admission.Standalone = true
			}
		}
		if !admission.Standalone && IsPulseAgentPlatformResource(resource) {
			admission.Standalone = true
		}
	}
	return admission
}
