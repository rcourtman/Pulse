package unifiedresources

import (
	"sort"
	"strings"
)

const proxmoxLXCDockerHostSourcePrefix = "proxmox-lxc-docker:"

var canonicalPlatformScopeOrder = []string{
	"agent",
	"truenas",
	"proxmox-pve",
	"proxmox-pbs",
	"proxmox-pmg",
	"docker",
	"kubernetes",
	"vmware-vsphere",
	"availability",
}

// RefreshPlatformScopes derives the canonical platform-page membership for a
// resource. Platform scopes are intentionally separate from the primary display
// platform: runtime resources such as Docker containers can belong to both the
// runtime lens and the platform that owns the host/guest they run on.
//
// This runs on every resource clone (see cloneResource), which happens on
// every registry read as well as every write, so it deliberately avoids a
// map: resources carry at most a handful of scopes, and a map allocation per
// call is measurable at thousands of resources times many reads per second.
func RefreshPlatformScopes(resource *Resource) {
	if resource == nil {
		return
	}

	scopes := make([]string, 0, 4)
	add := func(scope string) {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope == "" {
			return
		}
		for _, existing := range scopes {
			if existing == scope {
				return
			}
		}
		scopes = append(scopes, scope)
	}

	for _, source := range resource.Sources {
		add(platformScopeForSource(source))
	}

	addPlatformScopesForFacets(add, *resource)
	if shouldAddDockerPlatformScope(*resource) {
		add("docker")
	}

	if resource.Docker != nil {
		hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
		if strings.HasPrefix(hostSourceID, proxmoxLXCDockerHostSourcePrefix) {
			add("proxmox-pve")
			add("docker")
		}
	}

	resource.PlatformScopes = orderPlatformScopes(scopes)
}

func platformScopeForSource(source DataSource) string {
	switch source {
	case SourceAgent:
		return "agent"
	case SourceProxmox:
		return "proxmox-pve"
	case SourceDocker:
		return "docker"
	case SourcePBS:
		return "proxmox-pbs"
	case SourcePMG:
		return "proxmox-pmg"
	case SourceK8s:
		return "kubernetes"
	case SourceTrueNAS:
		return "truenas"
	case SourceVMware:
		return "vmware-vsphere"
	case SourceAvailability:
		return "availability"
	default:
		return ""
	}
}

func addPlatformScopesForFacets(add func(string), resource Resource) {
	if resource.Agent != nil {
		add("agent")
	}
	if resource.TrueNAS != nil {
		add("truenas")
	}
	if resource.Proxmox != nil {
		add("proxmox-pve")
	}
	if resource.PBS != nil {
		add("proxmox-pbs")
	}
	if resource.PMG != nil {
		add("proxmox-pmg")
	}
	if resource.Kubernetes != nil {
		add("kubernetes")
	}
	if resource.VMware != nil {
		add("vmware-vsphere")
	}
	if len(AvailabilityChecksForResource(resource)) > 0 || CanonicalResourceType(resource.Type) == ResourceTypeNetworkEndpoint {
		add("availability")
	}
}

func shouldAddDockerPlatformScope(resource Resource) bool {
	if resource.Docker == nil {
		return false
	}
	if resource.TrueNAS != nil || hasDataSource(resource.Sources, SourceTrueNAS) {
		return false
	}
	return true
}

// orderPlatformScopes sorts the (already deduplicated) scopes slice into
// canonical order, with any scope outside canonicalPlatformScopeOrder sorted
// alphabetically after the known ones. It consumes and reorders the input
// slice in place rather than allocating a second one.
func orderPlatformScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	out := make([]string, 0, len(scopes))
	remaining := scopes
	for _, canonical := range canonicalPlatformScopeOrder {
		for i, scope := range remaining {
			if scope == canonical {
				out = append(out, scope)
				remaining[i] = remaining[len(remaining)-1]
				remaining = remaining[:len(remaining)-1]
				break
			}
		}
	}
	if len(remaining) > 0 {
		sort.Strings(remaining)
		out = append(out, remaining...)
	}
	return out
}
