package mock

import (
	"strings"
	"sync/atomic"

	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

const (
	metricRoleGeneral    = "general"
	metricRoleWeb        = "web"
	metricRoleAPI        = "api"
	metricRoleDatabase   = "database"
	metricRoleCache      = "cache"
	metricRoleQueue      = "queue"
	metricRoleMonitoring = "monitoring"
	metricRoleBackup     = "backup"
	metricRoleStorage    = "storage"
	metricRoleIngress    = "ingress"
	metricRoleCI         = "ci"
	metricRoleMedia      = "media"
	metricRoleSecurity   = "security"
	metricRoleBatch      = "batch"
)

type weightedMetricRole struct {
	role   string
	weight int
}

var metricRoleRegistry atomic.Value

func init() {
	metricRoleRegistry.Store(map[string]string{})
}

func metricRoleRegistryKey(resourceClass, resourceID string) string {
	return normalizeMetricClass(resourceClass) + "::" + strings.TrimSpace(resourceID)
}

func setMetricRoleRegistry(registry map[string]string) {
	cloned := make(map[string]string, len(registry))
	for key, value := range registry {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		cloned[key] = value
	}
	metricRoleRegistry.Store(cloned)
}

func clearMetricRoleRegistry() {
	metricRoleRegistry.Store(map[string]string{})
}

func currentMetricRoleRegistry() map[string]string {
	raw := metricRoleRegistry.Load()
	if raw == nil {
		return nil
	}
	registry, _ := raw.(map[string]string)
	return registry
}

// MetricRole resolves the canonical mock persona for the given resource. When
// the fixture graph has richer naming metadata, it is used; otherwise the role
// falls back to stable token inference from the resource identity itself.
func MetricRole(resourceClass, resourceID string) string {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID != "" {
		if registry := currentMetricRoleRegistry(); registry != nil {
			if role := strings.TrimSpace(registry[metricRoleRegistryKey(resourceClass, resourceID)]); role != "" {
				return role
			}
		}
	}
	return inferMetricRole(resourceClass, resourceID)
}

func syncMetricRoleRegistryFromGraph(graph FixtureGraph) {
	setMetricRoleRegistry(buildMetricRoleRegistry(graph))
}

func buildMetricRoleRegistry(graph FixtureGraph) map[string]string {
	registry := make(map[string]string)
	register := func(resourceClass, resourceID string, hints ...string) {
		resourceID = strings.TrimSpace(resourceID)
		if resourceID == "" {
			return
		}
		if role := inferMetricRole(resourceClass, resourceID, hints...); role != "" {
			registry[metricRoleRegistryKey(resourceClass, resourceID)] = role
		}
	}

	for _, node := range graph.State.Nodes {
		register("node", node.ID, node.Name, node.DisplayName, node.ClusterName, node.Host)
	}
	for _, vm := range graph.State.VMs {
		register("vm", vm.ID, vm.Name, vm.OSName, vm.Node, vm.Pool, strings.Join(vm.Tags, " "))
	}
	for _, ct := range graph.State.Containers {
		register("container", ct.ID, ct.Name, ct.OSName, ct.OSTemplate, ct.Node, ct.Pool, strings.Join(ct.Tags, " "))
	}
	for _, host := range graph.State.Hosts {
		register("agent", host.ID, host.Hostname, host.DisplayName, host.Platform, host.OSName, strings.Join(host.Tags, " "))
	}
	for _, host := range graph.State.DockerHosts {
		register("dockerHost", host.ID, host.Hostname, host.DisplayName, host.OS, host.Runtime)
		for _, container := range host.Containers {
			hints := []string{container.Name, container.Image}
			for key, value := range container.Labels {
				hints = append(hints, key, value)
			}
			register("dockerContainer", container.ID, hints...)
		}
	}
	for _, storage := range graph.State.Storage {
		register("storage", storage.ID, storage.Name, storage.Type, storage.Content, storage.Path, storage.Node, storage.Instance)
	}
	for _, ceph := range graph.State.CephClusters {
		register("ceph", ceph.ID, ceph.Name, "ceph", "storage")
	}
	for _, pbs := range graph.State.PBSInstances {
		register("pbs", pbs.ID, pbs.Name, pbs.Host, "backup")
		for _, datastore := range pbs.Datastores {
			register("pbsDatastore", pbs.ID+":"+datastore.Name, pbs.Name, datastore.Name, "backup", "datastore")
		}
	}

	truenasFixtures := graph.PlatformFixtures.TrueNAS
	register("agent", truenasFixtures.System.Hostname, truenasFixtures.System.Hostname, "truenas", "storage")
	for _, pool := range truenasFixtures.Pools {
		register("storage", "pool:"+pool.Name, pool.Name, "truenas", "pool", "storage")
	}
	for _, dataset := range truenasFixtures.Datasets {
		register("storage", "dataset:"+dataset.Name, dataset.Name, "truenas", "dataset", "storage")
	}
	for _, disk := range truenasFixtures.Disks {
		register("disk", trueNASDiskMetricID(disk), disk.Name, disk.Model, disk.Serial, "storage", "disk")
	}
	for _, app := range truenasFixtures.Apps {
		appID := strings.TrimSpace(app.ID)
		if appID == "" {
			appID = strings.TrimSpace(app.Name)
		}
		if appID == "" {
			continue
		}
		hints := []string{app.Name, app.ID}
		hints = append(hints, app.Images...)
		register("dockerContainer", appID, hints...)
	}

	vmwareFixtures := graph.PlatformFixtures.VMware
	for _, host := range vmwareFixtures.Hosts {
		sourceID := vmware.SourceID(vmwareFixtures.ConnectionID, "host", host.Host)
		register("agent", sourceID, host.Name, host.Host, host.ClusterName, host.DatacenterName, "vmware", "esxi")
	}
	for _, guest := range vmwareFixtures.VMs {
		sourceID := vmware.SourceID(vmwareFixtures.ConnectionID, "vm", guest.VM)
		register("vm", sourceID, guest.Name, guest.GuestOSFamily, guest.GuestHostname, guest.RuntimeHostName, guest.ClusterName, guest.FolderName)
	}
	for _, datastore := range vmwareFixtures.Datastores {
		sourceID := vmware.SourceID(vmwareFixtures.ConnectionID, "datastore", datastore.Datastore)
		register("storage", sourceID, datastore.Name, datastore.Type, datastore.URL, datastore.DatacenterName, datastore.FolderName, strings.Join(datastore.VMNames, " "))
	}

	return registry
}

func inferMetricRole(resourceClass, resourceID string, hints ...string) string {
	classKey := normalizeMetricClass(resourceClass)
	tokens := normalizeMetricRoleTokens(append([]string{resourceID, classKey}, hints...)...)

	for _, classifier := range []struct {
		role     string
		keywords []string
	}{
		{metricRoleDatabase, []string{"postgres", "mysql", "mariadb", "mongodb", "database", "db", "sql", "pg", "influxdb"}},
		{metricRoleCache, []string{"redis", "memcached", "cache", "valkey"}},
		{metricRoleMonitoring, []string{"prometheus", "grafana", "loki", "monitoring", "metrics", "telemetry", "alertmanager", "jaeger"}},
		{metricRoleBackup, []string{"backup", "pbs", "archive", "snapshot", "replica", "replication", "offsite", "borg", "restic"}},
		{metricRoleStorage, []string{"storage", "datastore", "dataset", "pool", "zfs", "nfs", "smb", "ceph", "nas", "minio"}},
		{metricRoleIngress, []string{"nginx", "traefik", "haproxy", "envoy", "proxy", "ingress", "loadbalancer", "load-balancer"}},
		{metricRoleSecurity, []string{"bitwarden", "vaultwarden", "wireguard", "openvpn", "firewall", "pihole", "cloudflare", "auth", "vpn"}},
		{metricRoleMedia, []string{"jellyfin", "plex", "sonarr", "radarr", "transmission", "deluge", "sabnzbd", "media", "stream"}},
		{metricRoleCI, []string{"jenkins", "gitlab", "runner", "build", "ci"}},
		{metricRoleQueue, []string{"queue", "worker", "rabbitmq", "kafka", "broker"}},
		{metricRoleAPI, []string{"api", "backend", "service", "worker-api"}},
		{metricRoleWeb, []string{"web", "frontend", "portal", "site", "ui", "dashboard", "nextcloud", "seafile", "owncloud", "gitea"}},
	} {
		for _, keyword := range classifier.keywords {
			if keyword != "" && strings.Contains(tokens, keyword) {
				return classifier.role
			}
		}
	}

	switch classKey {
	case "pbs", "pbsdatastore":
		return metricRoleBackup
	case "storage", "pool", "dataset", "disk", "ceph":
		return metricRoleStorage
	case "node", "agent", "dockerhost":
		return stableMetricRoleChoice(resourceID, []weightedMetricRole{
			{role: metricRoleGeneral, weight: 4},
			{role: metricRoleStorage, weight: 2},
			{role: metricRoleMonitoring, weight: 2},
			{role: metricRoleBackup, weight: 1},
			{role: metricRoleIngress, weight: 1},
		})
	case "vm", "container", "dockercontainer":
		return stableMetricRoleChoice(resourceID, []weightedMetricRole{
			{role: metricRoleWeb, weight: 4},
			{role: metricRoleAPI, weight: 3},
			{role: metricRoleDatabase, weight: 2},
			{role: metricRoleCache, weight: 1},
			{role: metricRoleMonitoring, weight: 1},
			{role: metricRoleMedia, weight: 1},
			{role: metricRoleCI, weight: 1},
			{role: metricRoleBatch, weight: 1},
		})
	default:
		return metricRoleGeneral
	}
}

func stableMetricRoleChoice(seed string, roles []weightedMetricRole) string {
	if len(roles) == 0 {
		return metricRoleGeneral
	}

	totalWeight := 0
	for _, role := range roles {
		if role.weight > 0 {
			totalWeight += role.weight
		}
	}
	if totalWeight <= 0 {
		return roles[0].role
	}

	index := int(mockStableHash64(strings.TrimSpace(seed), "metric-role") % uint64(totalWeight))
	for _, role := range roles {
		if role.weight <= 0 {
			continue
		}
		if index < role.weight {
			return role.role
		}
		index -= role.weight
	}
	return roles[0].role
}

func normalizeMetricRoleTokens(values ...string) string {
	var builder strings.Builder
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		normalized = strings.NewReplacer("-", " ", "_", " ", "/", " ", ":", " ").Replace(normalized)
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(normalized)
	}
	return builder.String()
}
