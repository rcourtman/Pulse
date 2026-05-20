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
func RefreshPlatformScopes(resource *Resource) {
	if resource == nil {
		return
	}

	scopes := make(map[string]struct{}, len(resource.Sources)+2)
	for _, source := range resource.Sources {
		addPlatformScope(scopes, platformScopeForSource(source))
	}

	addPlatformScopesForFacets(scopes, *resource)
	if shouldAddDockerPlatformScope(*resource) {
		addPlatformScope(scopes, "docker")
	}

	if resource.Docker != nil {
		hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
		if strings.HasPrefix(hostSourceID, proxmoxLXCDockerHostSourcePrefix) {
			addPlatformScope(scopes, "proxmox-pve")
			addPlatformScope(scopes, "docker")
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

func addPlatformScopesForFacets(scopes map[string]struct{}, resource Resource) {
	if resource.Agent != nil {
		addPlatformScope(scopes, "agent")
	}
	if resource.TrueNAS != nil {
		addPlatformScope(scopes, "truenas")
	}
	if resource.Proxmox != nil {
		addPlatformScope(scopes, "proxmox-pve")
	}
	if resource.PBS != nil {
		addPlatformScope(scopes, "proxmox-pbs")
	}
	if resource.PMG != nil {
		addPlatformScope(scopes, "proxmox-pmg")
	}
	if resource.Kubernetes != nil {
		addPlatformScope(scopes, "kubernetes")
	}
	if resource.VMware != nil {
		addPlatformScope(scopes, "vmware-vsphere")
	}
	if resource.Availability != nil || CanonicalResourceType(resource.Type) == ResourceTypeNetworkEndpoint {
		addPlatformScope(scopes, "availability")
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

func addPlatformScope(scopes map[string]struct{}, scope string) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		return
	}
	scopes[scope] = struct{}{}
}

func orderPlatformScopes(scopes map[string]struct{}) []string {
	if len(scopes) == 0 {
		return nil
	}
	out := make([]string, 0, len(scopes))
	for _, scope := range canonicalPlatformScopeOrder {
		if _, ok := scopes[scope]; ok {
			out = append(out, scope)
			delete(scopes, scope)
		}
	}
	unknownStart := len(out)
	for scope := range scopes {
		out = append(out, scope)
	}
	sort.Strings(out[unknownStart:])
	return out
}
